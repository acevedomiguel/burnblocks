package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/acevedomiguel/burnblocks/openrouter"
)

// openRouterTimeout bounds the one-shot read, matching the documented client
// budget plus its bounded retries.
const openRouterTimeout = 60 * time.Second

// runOpenRouter performs one read-only OpenRouter status check. It keeps the
// schema-2 JSON, the aggregate-only cache, and the management/regular key
// separation of the RouteBurn application, but starts no interactive UI.
func runOpenRouter(args []string, stdout, stderr io.Writer, d deps) int {
	opts, err := parseOptions("openrouter", args)
	if err != nil {
		return reportUsageError(stderr, err)
	}
	if opts.help {
		printUsage(stdout)
		return exitOK
	}

	paths, err := d.openRouterPaths()
	if err != nil {
		fmt.Fprintf(stderr, "burnblocks: %s\n", openrouter.Redact(err.Error()))
		return exitFailure
	}
	cfg, err := openrouter.LoadConfig(configFilesFor(paths.ConfigFile, opts.configPath), d.environment, openrouter.Overrides{})
	if err != nil {
		fmt.Fprintf(stderr, "burnblocks: %s\n", openrouter.Redact(err.Error()))
		return exitUsage
	}

	store := openrouter.NewCredentialStore(openrouter.CredentialOptions{
		Keyring:             d.openRouterKeyring,
		ManagementTokenFile: paths.ManagementTokenFile(),
		APIKeyFile:          paths.APIKeyFile(),
		KeyringEnabled:      cfg.Keyring,
		Env:                 d.environment,
	})
	creds, err := store.Resolve(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "burnblocks: credentials: %s\n", openrouter.Redact(err.Error()))
		return exitFailure
	}
	if !creds.HasManagement() {
		fmt.Fprintln(stderr, "burnblocks: no OpenRouter management key is configured. Set ROUTEBURN_MANAGEMENT_KEY or run `routeburn setup`.")
		return exitCredentials
	}
	redactor := openrouter.NewRedactor(creds.Management, creds.APIKey)

	cache := d.openRouterCache
	if cache == nil && !opts.noCache {
		cache = openrouter.NewCacheStore(paths.CacheFile)
	}

	now := d.now().UTC()
	fetchOptions := openrouter.FetchOptions{Now: now}
	if cache != nil {
		// The previous snapshot lets optional analytics degrade to a warning
		// instead of blanking usable credit data.
		if previous, loadErr := cache.Load(); loadErr == nil {
			fetchOptions.Previous = &previous
		}
	}

	if d.openRouterAPI == nil {
		fmt.Fprintln(stderr, "burnblocks: provider reader is unavailable")
		return exitFailure
	}
	api, err := d.openRouterAPI(openrouter.DefaultBaseURL, creds.Management, creds.APIKey, version)
	if err != nil {
		fmt.Fprintf(stderr, "burnblocks: %s\n", redactor.Redact(err.Error()))
		return exitFailure
	}

	ctx, cancel := context.WithTimeout(context.Background(), openRouterTimeout)
	defer cancel()

	snapshot, fetchErr := openrouter.Fetch(ctx, api, fetchOptions)
	cached := false
	if fetchErr != nil {
		if cache != nil {
			previous, loadErr := cache.Load()
			if loadErr != nil {
				fmt.Fprintf(stderr, "burnblocks: %s\n", redactor.Redact(fetchErr.Error()))
				return exitFailure
			}
			snapshot = previous
			cached = true
			snapshot.Warnings = append(snapshot.Warnings, openrouter.Warning{
				Section: "Data",
				Message: "live refresh failed; showing cached snapshot: " + redactor.Redact(fetchErr.Error()),
			})
		} else {
			fmt.Fprintf(stderr, "burnblocks: %s\n", redactor.Redact(fetchErr.Error()))
			return exitFailure
		}
	} else if cache != nil {
		if saveErr := cache.Save(snapshot); saveErr != nil {
			snapshot.Warnings = append(snapshot.Warnings, openrouter.Warning{
				Section: "Cache",
				Message: "could not write the snapshot cache: " + redactor.Redact(saveErr.Error()),
			})
		}
	}

	if opts.jsonOutput {
		if err := openrouter.WriteJSON(stdout, snapshot, cached); err != nil {
			fmt.Fprintf(stderr, "burnblocks: %s\n", redactor.Redact(err.Error()))
			return exitFailure
		}
		return exitOK
	}
	writeOpenRouterText(stdout, snapshot, cached, now)
	return exitOK
}
