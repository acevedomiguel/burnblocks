package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/acevedomiguel/burnblocks/kimi"
)

// fakeKimiSource is an in-process UsageSource: no network, no credentials.
type fakeKimiSource struct {
	reading kimi.UsageReading
	err     error
}

func (f fakeKimiSource) ReadUsage(context.Context) (kimi.UsageReading, error) {
	return f.reading, f.err
}

func (fakeKimiSource) NextRefreshAllowedAt() time.Time { return time.Time{} }

// memoryState is an in-memory runway cache, so no test touches the real one.
type memoryState struct {
	state   kimi.StoredState
	loadErr error
	saveErr error
}

func (m *memoryState) Load() (kimi.StoredState, error) {
	if m.loadErr != nil {
		return kimi.StoredState{}, m.loadErr
	}
	return m.state, nil
}

func (m *memoryState) Save(state kimi.StoredState) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.state = state
	return nil
}

func kimiNow() time.Time { return time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC) }

func syntheticKimiReading(now time.Time) kimi.UsageReading {
	used := 0.25
	reset := now.Add(4 * time.Hour)
	return kimi.UsageReading{
		ObservedAt: now,
		Quotas: map[kimi.QuotaKey]kimi.QuotaReading{
			kimi.QuotaLimit5h: {Source: kimi.SourceRatio, UsedRatio: &used, ResetAt: &reset},
		},
	}
}

// kimiTestDeps builds hermetic dependencies: a synthetic token from the
// environment map, an injected source, an in-memory cache, and a temporary
// configuration layout.
func kimiTestDeps(t *testing.T, source kimi.UsageSource, state kimi.StateStore) deps {
	t.Helper()
	return kimiDepsFor(t.TempDir(), source, state, map[string]string{kimi.TokenEnvVar: "synthetic-token"})
}

// kimiDepsFor builds hermetic Kimi dependencies rooted at dir. It takes no
// *testing.T so the runnable examples, which have none, can reuse it.
func kimiDepsFor(dir string, source kimi.UsageSource, state kimi.StateStore, env map[string]string) deps {
	return deps{
		environment: env,
		kimiSource:  func(_, _, _ string) (kimi.UsageSource, error) { return source, nil },
		kimiState:   state,
		kimiPaths: func() (kimi.Paths, error) {
			return kimi.Paths{
				ConfigDir:  dir,
				ConfigFile: filepath.Join(dir, "config.yaml"),
				CacheDir:   dir,
				CacheFile:  filepath.Join(dir, "state.json"),
			}, nil
		},
		now: kimiNow,
	}
}

func TestKimiMissingTokenExitsCredentials(t *testing.T) {
	d := kimiTestDeps(t, fakeKimiSource{}, &memoryState{})
	d.environment = map[string]string{}

	var stdout, stderr bytes.Buffer
	code := runWithDeps([]string{"kimi"}, &stdout, &stderr, d)

	if code != exitCredentials {
		t.Fatalf("exit = %d, want %d", code, exitCredentials)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "no Kimi Code token") {
		t.Errorf("stderr = %q, want a missing-token diagnostic", stderr.String())
	}
}

func TestKimiJSONEmitsSchemaOne(t *testing.T) {
	d := kimiTestDeps(t, fakeKimiSource{reading: syntheticKimiReading(kimiNow())}, &memoryState{})

	var stdout, stderr bytes.Buffer
	code := runWithDeps([]string{"kimi", "--json"}, &stdout, &stderr, d)

	if code != exitOK {
		t.Fatalf("exit = %d (stderr %q)", code, stderr.String())
	}
	var doc struct {
		SchemaVersion int             `json:"schema_version"`
		Quotas        map[string]*any `json:"quotas"`
		Warnings      []kimi.Warning  `json:"warnings"`
		Source        struct {
			API string `json:"api_base"`
		} `json:"source"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, stdout.String())
	}
	if doc.SchemaVersion != kimi.PublicSchemaVersion {
		t.Errorf("schema_version = %d, want %d", doc.SchemaVersion, kimi.PublicSchemaVersion)
	}
	if _, ok := doc.Quotas[string(kimi.QuotaLimit5h)]; !ok {
		t.Errorf("quotas missing %s: %v", kimi.QuotaLimit5h, doc.Quotas)
	}
	if doc.Warnings == nil {
		t.Error("warnings must be [] rather than null")
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty on success", stderr.String())
	}
}

func TestKimiTextReportsEachQuota(t *testing.T) {
	d := kimiTestDeps(t, fakeKimiSource{reading: syntheticKimiReading(kimiNow())}, &memoryState{})

	var stdout, stderr bytes.Buffer
	code := runWithDeps([]string{"kimi"}, &stdout, &stderr, d)

	if code != exitOK {
		t.Fatalf("exit = %d (stderr %q)", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"Kimi Code", "limit_5h", "25%", "updated"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	// Every canonical quota is listed, including the unreported ones.
	for _, key := range kimi.CanonicalQuotaKeys {
		if !strings.Contains(out, string(key)) {
			t.Errorf("output missing quota %s:\n%s", key, out)
		}
	}
}

func TestKimiFetchFailureWithoutCacheExitsFailure(t *testing.T) {
	d := kimiTestDeps(t, fakeKimiSource{err: errors.New("synthetic provider failure")}, &memoryState{})

	var stdout, stderr bytes.Buffer
	code := runWithDeps([]string{"kimi"}, &stdout, &stderr, d)

	if code != exitFailure {
		t.Fatalf("exit = %d, want %d", code, exitFailure)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "synthetic provider failure") {
		t.Errorf("stderr = %q, want the provider failure", stderr.String())
	}
}

func TestKimiCachedFallbackSucceedsWithWarning(t *testing.T) {
	snapshot := kimi.BuildSnapshot(syntheticKimiReading(kimiNow()), nil)
	state := &memoryState{state: kimi.StoredState{
		CacheVersion: kimi.CacheSchemaVersion,
		Snapshot:     &snapshot,
		Samples:      map[kimi.QuotaKey][]kimi.Sample{},
	}}
	d := kimiTestDeps(t, fakeKimiSource{err: errors.New("synthetic provider failure")}, state)

	var stdout, stderr bytes.Buffer
	code := runWithDeps([]string{"kimi"}, &stdout, &stderr, d)

	if code != exitOK {
		t.Fatalf("exit = %d, want %d with a cached fallback (stderr %q)", code, exitOK, stderr.String())
	}
	if !strings.Contains(stdout.String(), "cached") {
		t.Errorf("output must label a cached snapshot:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "synthetic provider failure") {
		t.Errorf("output must warn about the failed refresh:\n%s", stdout.String())
	}
}

func TestKimiUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"unknown flag", []string{"kimi", "--bogus"}},
		{"positional argument", []string{"kimi", "extra"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := kimiTestDeps(t, fakeKimiSource{}, &memoryState{})
			var stdout, stderr bytes.Buffer
			if code := runWithDeps(tc.args, &stdout, &stderr, d); code != exitUsage {
				t.Fatalf("exit = %d, want %d", code, exitUsage)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), "Usage:") {
				t.Errorf("stderr = %q, want the usage block", stderr.String())
			}
		})
	}
}

func TestKimiNoCacheUsesDisabledStore(t *testing.T) {
	state := &memoryState{}
	d := kimiTestDeps(t, fakeKimiSource{reading: syntheticKimiReading(kimiNow())}, state)
	d.kimiState = nil // select the --no-cache disabled store instead
	d.kimiPaths = func() (kimi.Paths, error) {
		return kimi.Paths{ConfigFile: "", CacheFile: filepath.Join(t.TempDir(), "state.json")}, nil
	}

	var stdout, stderr bytes.Buffer
	if code := runWithDeps([]string{"kimi", "--no-cache"}, &stdout, &stderr, d); code != exitOK {
		t.Fatalf("exit = %d (stderr %q)", code, stderr.String())
	}
	if state.state.Snapshot != nil {
		t.Error("--no-cache must not write the caller-provided store")
	}
}

// A help request is not a usage error: it prints the usage block on stdout and
// exits 0.
func TestKimiHelpExitsZero(t *testing.T) {
	d := kimiTestDeps(t, fakeKimiSource{}, &memoryState{})

	var stdout, stderr bytes.Buffer
	if code := runWithDeps([]string{"kimi", "--help"}, &stdout, &stderr, d); code != exitOK {
		t.Fatalf("exit = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("stdout = %q, want the usage block", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty for a help request", stderr.String())
	}
}
