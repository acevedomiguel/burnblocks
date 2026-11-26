package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/acevedomiguel/burnblocks/kimi"
)

// kimiTimeout bounds the one-shot read.
const kimiTimeout = 40 * time.Second

// runKimi performs one read-only Kimi Code status check. It keeps the JSON
// schema, the cache, and the runway history of the KimiBurn application, but
// starts no interactive terminal UI.
func runKimi(args []string, stdout, stderr io.Writer, d deps) int {
	opts, err := parseOptions("kimi", args)
	if err != nil {
		return reportUsageError(stderr, err)
	}

	paths, err := d.kimiPaths()
	if err != nil {
		fmt.Fprintf(stderr, "burnblocks: %s\n", kimi.Redact(err.Error()))
		return exitFailure
	}
	cfg, err := kimi.LoadConfig(configFilesFor(paths.ConfigFile, opts.configPath), d.environment, kimi.Overrides{})
	if err != nil {
		fmt.Fprintf(stderr, "burnblocks: %s\n", kimi.Redact(err.Error()))
		return exitUsage
	}

	tokenStore := kimi.NewTokenStore(kimi.TokenStoreOptions{
		Keyring:        d.kimiKeyring,
		TokenFile:      paths.TokenFile(),
		KeyringEnabled: cfg.Keyring,
		Env:            d.environment,
	})
	token, err := tokenStore.Resolve(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "burnblocks: credentials: %s\n", kimi.Redact(err.Error()))
		return exitFailure
	}
	if token == "" {
		fmt.Fprintln(stderr, "burnblocks: no Kimi Code token is configured. Set KIMIBURN_TOKEN or run `kimiburn setup`.")
		return exitCredentials
	}
	redactor := kimi.NewRedactor(token, tokenStore.EnvironmentToken())

	stateStore := d.kimiState
	if stateStore == nil {
		if opts.noCache {
			stateStore = kimi.NewDisabledStateStore()
		} else {
			stateStore = kimi.NewStateStore(paths.CacheFile)
		}
	}

	// The cache carries the runway history across runs. A read failure is a
	// warning, never a fatal error: a cold start is still a valid answer.
	state := kimi.EmptyStoredState()
	var cacheWarning *kimi.Warning
	if !opts.noCache {
		loaded, loadErr := stateStore.Load()
		if loadErr != nil {
			warning := kimi.Warning{Section: "Cache", Message: "ignored cache: " + redactor.Redact(loadErr.Error())}
			cacheWarning = &warning
		} else {
			state = loaded
		}
	}

	if d.kimiSource == nil {
		fmt.Fprintln(stderr, "burnblocks: usage reader is unavailable")
		return exitFailure
	}
	source, err := d.kimiSource(cfg.BaseURL, token, version)
	if err != nil {
		fmt.Fprintf(stderr, "burnblocks: %s\n", redactor.Redact(err.Error()))
		return exitFailure
	}

	ctx, cancel := context.WithTimeout(context.Background(), kimiTimeout)
	defer cancel()

	now := d.now().UTC()
	var snapshot kimi.Snapshot
	cached := false

	reading, fetchErr := source.ReadUsage(ctx)
	switch {
	case fetchErr == nil:
		snapshot = kimi.BuildSnapshot(reading, &state)
		if cacheWarning != nil {
			snapshot.Warnings = append(snapshot.Warnings, *cacheWarning)
			// Keep the persisted snapshot identical to the printed one.
			if state.Snapshot != nil {
				state.Snapshot.Warnings = append(state.Snapshot.Warnings, *cacheWarning)
			}
		}
		if !opts.noCache {
			if saveErr := stateStore.Save(state); saveErr != nil {
				snapshot.Warnings = append(snapshot.Warnings, kimi.Warning{
					Section: "Cache",
					Message: "could not save state: " + redactor.Redact(saveErr.Error()),
				})
			}
		}
	case !opts.noCache && state.Snapshot != nil:
		snapshot = *state.Snapshot
		snapshot.Warnings = append([]kimi.Warning{}, state.Snapshot.Warnings...)
		snapshot.Warnings = append(snapshot.Warnings, kimi.Warning{
			Section: "Refresh",
			Message: redactor.Redact(fetchErr.Error()),
		})
		cached = true
	default:
		if cacheWarning != nil {
			fmt.Fprintf(stderr, "burnblocks: cache: %s\n", cacheWarning.Message)
		}
		fmt.Fprintf(stderr, "burnblocks: %s\n", redactor.Redact(fetchErr.Error()))
		return exitFailure
	}

	if opts.jsonOutput {
		if err := kimi.WriteJSON(stdout, snapshot, kimi.JSONOptions{APIBase: cfg.BaseURL, Cached: cached, Now: now}); err != nil {
			fmt.Fprintf(stderr, "burnblocks: %s\n", redactor.Redact(err.Error()))
			return exitFailure
		}
		return exitOK
	}
	writeKimiText(stdout, snapshot, cfg.BaseURL, cached, now)
	return exitOK
}
