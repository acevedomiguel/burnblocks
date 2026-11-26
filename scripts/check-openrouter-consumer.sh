#!/usr/bin/env bash
# Exercises the public openrouter package from a separate module with synthetic
# data and no external network. It proves the published surface a consumer such
# as RouteBurn or DashBurn depends on: the read-only API boundary, snapshot
# fetch, schema-2 JSON, the aggregate-only cache, configuration, credentials,
# and redaction.
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

	"github.com/acevedomiguel/burnblocks/openrouter"
)

// fakeAPI is an in-process read-only API: no network, no credentials.
type fakeAPI struct{}

func (fakeAPI) Credits(context.Context) (openrouter.CreditSummary, error) {
	return openrouter.CreditSummary{TotalPurchased: 100, TotalUsed: 57.9, Remaining: 42.1}, nil
}

func (fakeAPI) Key(context.Context) (openrouter.KeyInfo, error) {
	return openrouter.KeyInfo{}, openrouter.ErrMissingAPIKey
}

func (fakeAPI) QueryAnalytics(context.Context, openrouter.AnalyticsQuery) ([]openrouter.AnalyticsRow, error) {
	return nil, nil
}

func (fakeAPI) NextRefreshAllowedAt() time.Time { return time.Time{} }

func main() {
	now := time.Date(2026, 11, 26, 15, 42, 0, 0, time.UTC)
	snap, err := openrouter.Fetch(context.Background(), fakeAPI{}, openrouter.FetchOptions{Now: now})
	if err != nil {
		panic(err)
	}
	if snap.Account.Remaining != 42.1 {
		panic(fmt.Sprintf("remaining = %v, want 42.1", snap.Account.Remaining))
	}
	if len(snap.Daily) != 30 {
		panic(fmt.Sprintf("daily buckets = %d, want 30", len(snap.Daily)))
	}
	// A missing optional regular key must not fabricate key details.
	if snap.Key != nil {
		panic("missing regular key produced key details")
	}

	var buf bytes.Buffer
	if err := openrouter.WriteJSON(&buf, snap, false); err != nil {
		panic(err)
	}
	out := buf.Bytes()
	if len(out) == 0 || out[len(out)-1] != '\n' {
		panic("schema-2 JSON must end with a newline")
	}
	var doc struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		panic(err)
	}
	if doc.SchemaVersion != openrouter.SchemaVersion {
		panic(fmt.Sprintf("schema version = %d, want %d", doc.SchemaVersion, openrouter.SchemaVersion))
	}

	// The aggregate-only cache round-trips a snapshot with no credentials.
	dir, err := os.MkdirTemp("", "burnblocks-openrouter-consumer")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	if err := openrouter.NewCacheStore(filepath.Join(dir, "snapshot.json")).Save(snap); err != nil {
		panic(err)
	}
	loaded, err := openrouter.NewCacheStore(filepath.Join(dir, "snapshot.json")).Load()
	if err != nil {
		panic(err)
	}
	if loaded.Account.Remaining != 42.1 {
		panic("cache round trip lost the snapshot")
	}

	// Configuration defaults and the documented base URL.
	if err := openrouter.DefaultConfig().Validate(); err != nil {
		panic(err)
	}
	if openrouter.DefaultBaseURL != "https://openrouter.ai/api/v1" {
		panic("unexpected default base URL")
	}

	// Credential resolution from an explicit environment map, no keyring and no
	// real process environment. OPENROUTER_API_KEY fills the regular slot only.
	creds, err := openrouter.ResolveCreds(context.Background(), openrouter.NewCredentialStore(
		openrouter.CredentialOptions{
			KeyringEnabled: false,
			Env: map[string]string{
				openrouter.EnvManagementKey: "synthetic-management-key",
				openrouter.EnvOpenRouterKey: "synthetic-regular-key",
			},
		},
	))
	if err != nil {
		panic(err)
	}
	if !creds.HasManagement() || !creds.HasAPIKey() {
		panic("credentials were not resolved from the explicit environment")
	}

	secret := "Authorization: Bearer sk-synthetic-secret-value-000000000000"
	if openrouter.Redact(secret) == secret {
		panic("redaction did not alter a secret-shaped value")
	}

	fmt.Println("external openrouter consumer passed")
}
EOF

cd "$consumer_dir"
go mod init example.com/burnblocks-consumer >/dev/null
go mod edit -replace=github.com/acevedomiguel/burnblocks="$repo_root"
# Resolve the module graph through the normal proxy. Only the build and run are
# offline, which is what shows the package itself reaches no network.
go mod tidy
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local go run .
