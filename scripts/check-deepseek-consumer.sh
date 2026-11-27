#!/usr/bin/env bash
# Exercises the public deepseek package from a separate module with synthetic
# data and no external network. It proves the published surface a consumer such
# as DashBurn or the burnblocks CLI depends on: the read-only balance boundary,
# snapshot fetch, the independent schema-1 JSON, credentials, paths, and
# redaction.
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
	"time"

	"github.com/acevedomiguel/burnblocks/deepseek"
)

// fakeSource is an in-process BalanceSource: no network, no credentials.
type fakeSource struct{ now time.Time }

func (f fakeSource) ReadBalance(context.Context) (deepseek.BalanceReading, error) {
	return deepseek.BalanceReading{
		ObservedAt: f.now,
		Available:  deepseek.AvailabilityAvailable,
		Balances: []deepseek.Balance{
			{Currency: deepseek.CurrencyCNY, Total: f64(123.45), Granted: f64(3.45), ToppedUp: f64(120)},
			{Currency: deepseek.CurrencyUSD, Total: f64(0)},
		},
	}, nil
}

func (fakeSource) NextRefreshAllowedAt() time.Time { return time.Time{} }

func main() {
	now := time.Date(2026, 11, 27, 9, 30, 0, 0, time.UTC)

	snap, err := deepseek.Fetch(context.Background(), fakeSource{now: now}, deepseek.FetchOptions{Now: now})
	if err != nil {
		panic(err)
	}
	if snap.SchemaVersion != deepseek.SnapshotSchemaVersion {
		panic(fmt.Sprintf("snapshot schema = %d", snap.SchemaVersion))
	}
	// The forecast is honestly unavailable: DeepSeek publishes no usage history.
	if snap.Runway.Status != deepseek.RunwayUnavailable || snap.Runway.Reason != deepseek.ReasonNoUsageHistory {
		panic(fmt.Sprintf("runway = %+v", snap.Runway))
	}
	// Currencies are reported separately and never totalled together.
	if len(snap.Balances) != 2 || snap.Balances[0].Currency != deepseek.CurrencyCNY || snap.Balances[1].Currency != deepseek.CurrencyUSD {
		panic(fmt.Sprintf("balances = %+v", snap.Balances))
	}

	// The independent schema-1 public projection with a trailing newline.
	var buf bytes.Buffer
	if err := deepseek.WriteJSON(&buf, snap); err != nil {
		panic(err)
	}
	out := buf.Bytes()
	if len(out) == 0 || out[len(out)-1] != '\n' {
		panic("projection must end with a newline")
	}
	var doc struct {
		SchemaVersion int `json:"schema_version"`
		Balances      []struct {
			Currency string   `json:"currency"`
			Granted  *float64 `json:"granted"`
		} `json:"balances"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		panic(err)
	}
	if doc.SchemaVersion != deepseek.SchemaVersion {
		panic(fmt.Sprintf("public schema = %d", doc.SchemaVersion))
	}
	// A reported zero survives; a value the provider did not report is null.
	if doc.Balances[1].Granted != nil {
		panic("a missing granted amount must project as null")
	}
	if doc.Balances[0].Granted == nil || *doc.Balances[0].Granted != 3.45 {
		panic("a reported granted amount must survive")
	}

	// Credential resolution from an explicit environment map, no keyring and no
	// real process environment.
	store := deepseek.NewCredentialStore(deepseek.CredentialOptions{
		KeyringEnabled: false,
		Env:            map[string]string{deepseek.CredentialEnvVar: "sk-synthetic-key"},
	})
	key, err := deepseek.ResolveKey(context.Background(), store)
	if err != nil {
		panic(err)
	}
	if key != "sk-synthetic-key" {
		panic("explicit environment key was not resolved")
	}

	// Redaction scrubs a key-shaped value.
	if deepseek.Redact("Authorization: Bearer sk-0123456789abcdef0123456789abcdef") == "Authorization: Bearer sk-0123456789abcdef0123456789abcdef" {
		panic("redaction did not alter a secret-shaped value")
	}

	// Constructing a source performs no I/O until ReadBalance is called, and an
	// empty base selects the documented default.
	source, err := deepseek.NewSource("", "sk-synthetic-key", deepseek.WithVersion("test-version"))
	if err != nil {
		panic(err)
	}
	if source == nil {
		panic("a source must be returned")
	}
	if deepseek.DefaultBaseURL != "https://api.deepseek.com" {
		panic("unexpected default base URL")
	}

	// The platform path surface the CLI relies on.
	if _, err := deepseek.DefaultPaths(); err != nil {
		panic(err)
	}
	_ = deepseek.PermissionsAreEnforced
	_ = deepseek.BuildSnapshot(deepseek.BalanceReading{})

	fmt.Println("external deepseek consumer passed")
}

func f64(v float64) *float64 { return &v }
EOF

cd "$consumer_dir"
go mod init example.com/burnblocks-consumer >/dev/null
go mod edit -replace=github.com/acevedomiguel/burnblocks="$repo_root"
# Resolve the module graph through the normal proxy. Only the build and run are
# offline, which is what shows the package itself reaches no network.
go mod tidy
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local go run .
