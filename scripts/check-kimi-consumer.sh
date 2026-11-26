#!/usr/bin/env bash
# Exercises the public kimi package from a separate module with synthetic data
# and no external network. It proves the published surface a consumer such as
# KimiBurn or DashBurn depends on: source, fetch, runway state, schema-1 JSON,
# the cache store, configuration, credentials, and redaction.
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel)
consumer_dir=$(mktemp -d "${TMPDIR:-/tmp}/burnblocks-consumer.XXXXXX")
trap 'rm -rf "$consumer_dir"' EXIT

cat > "$consumer_dir/main.go" <<'EOF'
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/acevedomiguel/burnblocks/kimi"
)

// fakeSource is an in-process UsageSource: no network, no credentials.
type fakeSource struct{ now time.Time }

func (f fakeSource) ReadUsage(context.Context) (kimi.UsageReading, error) {
	used := 0.25
	reset := f.now.Add(4 * time.Hour)
	return kimi.UsageReading{
		ObservedAt: f.now,
		Quotas: map[kimi.QuotaKey]kimi.QuotaReading{
			kimi.QuotaLimit5h: {Source: kimi.SourceRatio, UsedRatio: &used, ResetAt: &reset},
		},
	}, nil
}

func (fakeSource) NextRefreshAllowedAt() time.Time { return time.Time{} }

func main() {
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)

	var state kimi.StoredState = kimi.EmptyStoredState()
	snap, err := kimi.Fetch(context.Background(), fakeSource{now: now}, &state)
	if err != nil {
		panic(err)
	}
	if snap.SchemaVersion != kimi.SnapshotSchemaVersion {
		panic(fmt.Sprintf("snapshot schema = %d", snap.SchemaVersion))
	}
	if _, ok := snap.Quotas[kimi.QuotaLimit5h]; !ok {
		panic("five-hour quota missing")
	}
	if state.CacheVersion != kimi.CacheSchemaVersion || state.Snapshot == nil {
		panic("state was not updated after a successful read")
	}

	// Schema-1 public projection with a trailing newline.
	var buf bytes.Buffer
	if err := kimi.WriteJSON(&buf, snap, kimi.JSONOptions{APIBase: kimi.DefaultBaseURL, Now: now}); err != nil {
		panic(err)
	}
	out := buf.Bytes()
	if len(out) == 0 || out[len(out)-1] != '\n' {
		panic("projection must end with a newline")
	}
	var doc struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		panic(err)
	}
	if doc.SchemaVersion != kimi.PublicSchemaVersion {
		panic(fmt.Sprintf("public schema = %d", doc.SchemaVersion))
	}

	// Caller-owned cache round trip.
	dir, err := os.MkdirTemp("", "burnblocks-consumer")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	store := kimi.NewStateStore(filepath.Join(dir, "state.json"))
	if err := store.Save(state); err != nil {
		panic(err)
	}
	loaded, err := store.Load()
	if err != nil {
		panic(err)
	}
	if len(loaded.Samples) != len(state.Samples) {
		panic("cache round trip lost samples")
	}

	// Configuration defaults and the documented base URLs.
	cfg, err := kimi.LoadConfig(nil, map[string]string{})
	if err != nil {
		panic(err)
	}
	if err := cfg.Validate(); err != nil {
		panic(err)
	}
	if !kimi.IsAllowedBaseURL(kimi.DefaultBaseURL) || !kimi.IsAllowedBaseURL(kimi.OverseasBaseURL) {
		panic("documented base URLs must be allowed")
	}
	if kimi.IsAllowedBaseURL("https://api.moonshot.ai/") {
		panic("Moonshot must never be accepted")
	}

	// Credential resolution from an explicit environment map, no keyring, no
	// real process environment.
	tokens := kimi.NewTokenStore(kimi.TokenStoreOptions{
		KeyringEnabled: false,
		Env:            map[string]string{kimi.TokenEnvVar: "synthetic-token"},
	})
	token, err := tokens.Resolve(context.Background())
	if err != nil {
		panic(err)
	}
	if token != "synthetic-token" {
		panic("explicit environment token was not resolved")
	}
	if kimi.Redact("Authorization: Bearer sk-synthetic-secret-value-000000000000") == "Authorization: Bearer sk-synthetic-secret-value-000000000000" {
		panic("redaction did not alter a secret-shaped value")
	}

	// BuildSnapshot is the post-read half of Fetch, for callers that perform
	// their own read and substitute a clock only when the provider omitted the
	// observation time.
	reading := kimi.UsageReading{
		ObservedAt: now,
		Quotas: map[kimi.QuotaKey]kimi.QuotaReading{
			kimi.QuotaLimit5h: {Source: kimi.SourceRatio, UsedRatio: ptr(0.5), ResetAt: ptr(now.Add(3 * time.Hour))},
		},
	}
	var built kimi.StoredState = kimi.EmptyStoredState()
	builtSnap := kimi.BuildSnapshot(reading, &built)
	if builtSnap.GeneratedAt != now {
		panic("BuildSnapshot did not stamp the reading time")
	}
	if _, ok := builtSnap.Runways[kimi.QuotaLimit5h]; !ok {
		panic("BuildSnapshot did not derive a runway")
	}
	if built.Snapshot == nil || built.CacheVersion != kimi.CacheSchemaVersion {
		panic("BuildSnapshot did not update state")
	}

	// Building a source accepts a version option and performs no I/O until
	// ReadUsage is called.
	if _, err := kimi.NewSource(kimi.DefaultBaseURL, "synthetic-token", kimi.WithVersion("test-version")); err != nil {
		panic(err)
	}

	fmt.Println("external kimi consumer passed")
}

func ptr[T any](v T) *T { return &v }
EOF

cd "$consumer_dir"
go mod init example.com/burnblocks-consumer >/dev/null
go mod edit -replace=github.com/acevedomiguel/burnblocks="$repo_root"
# Resolve the module graph through the normal proxy. Only the build and run are
# offline, which is what shows the package itself reaches no network.
go mod tidy
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local go run .
