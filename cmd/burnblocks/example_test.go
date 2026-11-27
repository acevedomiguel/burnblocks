package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/acevedomiguel/burnblocks/kimi"
	"github.com/acevedomiguel/burnblocks/openrouter"
)

// The examples on this file are runnable: `go test ./cmd/burnblocks` executes
// them and checks their output against the // Output blocks, so the documented
// CLI behaviour cannot drift. Every dependency is injected and every value is
// synthetic, so nothing here reaches the network, the keyring, or a real
// credential. The same invocations appear in README.md's Usage section.

// exampleDir creates a throwaway directory for one example. Examples have no
// *testing.T, so this mirrors t.TempDir.
func exampleDir() (string, func()) {
	dir, err := os.MkdirTemp("", "burnblocks-example")
	if err != nil {
		panic(err)
	}
	return dir, func() { _ = os.RemoveAll(dir) }
}

// exampleAnalytics is six synthetic days of OpenRouter spend, used so the
// example report and its runway are non-empty.
func exampleAnalytics() []openrouter.AnalyticsRow {
	rows := make([]openrouter.AnalyticsRow, 0, 6)
	for i := 1; i <= 6; i++ {
		rows = append(rows, openrouter.AnalyticsRow{
			Day:          kimiNow().AddDate(0, 0, -i),
			CreditSpend:  0.5,
			Tokens:       250_000,
			RequestCount: 12,
		})
	}
	return rows
}

// mustOK runs one example command and fails the example with the diagnostic if
// it does not exit 0, so a regression reads as an exit code rather than only an
// output diff.
func mustOK(args []string, d deps, stdout, stderr *bytes.Buffer) {
	if code := runWithDeps(args, stdout, stderr, d); code != exitOK {
		panic(fmt.Sprintf("%v exited %d: %s", args, code, stderr.String()))
	}
}

// Example_help prints the flag list and the exit-code table.
func Example_help() {
	var stdout, stderr bytes.Buffer
	_ = runWithDeps([]string{"--help"}, &stdout, &stderr, deps{})
	fmt.Print(stdout.String())

	// Output:
	// burnblocks reports Kimi Code and OpenRouter usage.
	//
	// Usage:
	//   burnblocks kimi [--json] [--config <path>] [--no-cache]
	//   burnblocks openrouter [--json] [--config <path>] [--no-cache]
	//   burnblocks --version
	//   burnblocks --help
	//
	// Flags:
	//   --json            Emit one provider JSON snapshot on stdout
	//   --config <path>   Use an explicit configuration file
	//   --no-cache        Do not read or write the local cache
	//
	// Each provider keeps its own versioned JSON schema and its own unit: Kimi reports
	// ratios and OpenRouter reports US dollars. Never total them together.
	//
	// Exit codes:
	//   0 success   1 runtime or API failure   2 usage error   3 missing credentials
}

// Example_kimiText shows the default text report: one block per canonical quota,
// with Kimi's ratios and its per-quota reset and runway.
func Example_kimiText() {
	dir, cleanup := exampleDir()
	defer cleanup()
	d := kimiDepsFor(dir, fakeKimiSource{reading: syntheticKimiReading(kimiNow())}, &memoryState{}, map[string]string{kimi.TokenEnvVar: "synthetic-token"})

	var stdout, stderr bytes.Buffer
	mustOK([]string{"kimi"}, d, &stdout, &stderr)
	fmt.Print(stdout.String())

	// Output:
	// Kimi Code — https://api.kimi.com/coding/v1
	//
	//   limit_5h
	//     used      25%
	//     resets    in 4h 0m (2026-11-26 16:00 UTC)
	//     runway    3h 0m · window_average · depletes 2026-11-26 15:00
	//
	//   limit_7d
	//     used      not reported
	//
	//   limit_month_total
	//     used      not reported
	//
	//   limit_month_code
	//     used      not reported
	//
	//   updated    2026-11-26 12:00 UTC
}

// Example_kimiJSON shows --json: Kimi's own schema-1 document on stdout.
func Example_kimiJSON() {
	dir, cleanup := exampleDir()
	defer cleanup()
	d := kimiDepsFor(dir, fakeKimiSource{reading: syntheticKimiReading(kimiNow())}, &memoryState{}, map[string]string{kimi.TokenEnvVar: "synthetic-token"})

	var stdout, stderr bytes.Buffer
	mustOK([]string{"kimi", "--json"}, d, &stdout, &stderr)
	fmt.Print(stdout.String())

	// Output:
	// {
	//   "schema_version": 1,
	//   "generated_at": "2026-11-26T12:00:00Z",
	//   "cached": false,
	//   "cache_age_seconds": 0,
	//   "source": {
	//     "api_base": "https://api.kimi.com/coding/v1",
	//     "endpoint": "/usages"
	//   },
	//   "quotas": {
	//     "limit_5h": {
	//       "source": "ratio",
	//       "used_ratio": 0.25,
	//       "remaining_ratio": 0.75,
	//       "limit": null,
	//       "used": null,
	//       "remaining": null,
	//       "reset_at": "2026-11-26T16:00:00Z",
	//       "reset_in_seconds": 14400,
	//       "runway": {
	//         "status": "finite",
	//         "reason": null,
	//         "seconds": 10800,
	//         "exhausted_at": "2026-11-26T15:00:00Z",
	//         "burn_ratio_per_second": 0.00006944444444444444,
	//         "burn_percent_per_hour": 25,
	//         "sample_count": 1,
	//         "window_seconds": 0,
	//         "confidence": "unavailable",
	//         "basis": "window_average"
	//       }
	//     },
	//     "limit_7d": null,
	//     "limit_month_code": null,
	//     "limit_month_total": null
	//   },
	//   "goods_version": null,
	//   "booster_wallet": null,
	//   "raw": null,
	//   "warnings": []
	// }
}

// Example_openrouterText shows the default text report. OpenRouter reports US
// dollars, which are never mixed with Kimi's ratios.
func Example_openrouterText() {
	dir, cleanup := exampleDir()
	defer cleanup()
	api := fakeOpenRouterAPI{
		credits: openrouter.CreditSummary{TotalPurchased: 100, TotalUsed: 12.5, Remaining: 87.5},
		keyErr:  openrouter.ErrMissingAPIKey,
		rows:    exampleAnalytics(),
	}
	d := openRouterDepsFor(dir, api, map[string]string{openrouter.EnvManagementKey: "synthetic-management-key"})

	var stdout, stderr bytes.Buffer
	mustOK([]string{"openrouter"}, d, &stdout, &stderr)
	fmt.Print(stdout.String())

	// Output:
	// OpenRouter — https://openrouter.ai/api/v1
	//
	//   credits    $87.50 remaining of $100.00 purchased ($12.50 used)
	//   last 30d  $3.00 spent · 1.5M tokens · 72 requests
	//   runway     875d 0h · low confidence · depletes 2029-04-19
	//   key        no regular API key configured
	//
	//   updated    2026-11-26 12:00 UTC
}

// Example_openrouterJSON shows --json: OpenRouter's own schema-2 document.
func Example_openrouterJSON() {
	dir, cleanup := exampleDir()
	defer cleanup()
	api := fakeOpenRouterAPI{
		credits: openrouter.CreditSummary{TotalPurchased: 100, TotalUsed: 12.5, Remaining: 87.5},
		keyErr:  openrouter.ErrMissingAPIKey,
		rows:    exampleAnalytics(),
	}
	d := openRouterDepsFor(dir, api, map[string]string{openrouter.EnvManagementKey: "synthetic-management-key"})

	var stdout, stderr bytes.Buffer
	mustOK([]string{"openrouter", "--json"}, d, &stdout, &stderr)
	fmt.Print(stdout.String())

	// Output:
	// {
	//   "schema_version": 2,
	//   "generated_at": "2026-11-26T12:00:00Z",
	//   "cached": false,
	//   "account": {
	//     "total_credits": 100,
	//     "total_usage": 12.5,
	//     "remaining": 87.5
	//   },
	//   "last_30_days": {
	//     "credit_spend": 3,
	//     "tokens": 1500000,
	//     "requests": 72,
	//     "tokens_prompt": 0,
	//     "tokens_completion": 0,
	//     "cached_tokens": 0,
	//     "reasoning_tokens": 0,
	//     "cache_hit_rate": null
	//   },
	//   "runway": {
	//     "window_days": 30,
	//     "daily_burn": 0.1,
	//     "days_remaining": 875,
	//     "depletion_date": "2029-04-19",
	//     "basis": "finite",
	//     "low_confidence": true
	//   },
	//   "key": null,
	//   "warnings": []
	// }
}

// Example_noCache shows --no-cache: with no cache there is nothing to fall back
// to, so a failed read exits non-zero and prints the reason on stderr. It also
// asserts that the cache was not touched at all.
func Example_noCache() {
	dir, cleanup := exampleDir()
	defer cleanup()
	state := &memoryState{}
	d := kimiDepsFor(dir, fakeKimiSource{err: errors.New("provider unavailable")}, state, map[string]string{kimi.TokenEnvVar: "synthetic-token"})

	var stdout, stderr bytes.Buffer
	code := runWithDeps([]string{"kimi", "--no-cache"}, &stdout, &stderr, d)
	if state.loads != 0 || state.saves != 0 {
		panic(fmt.Sprintf("--no-cache touched the cache: %d loads, %d saves", state.loads, state.saves))
	}

	fmt.Printf("exit code: %d\n", code)
	fmt.Print(stderr.String())

	// Output:
	// exit code: 1
	// burnblocks: provider unavailable
}

// Example_missingCredentials shows exit code 3 when no credential is configured.
func Example_missingCredentials() {
	dir, cleanup := exampleDir()
	defer cleanup()
	d := kimiDepsFor(dir, fakeKimiSource{}, &memoryState{}, map[string]string{})

	var stdout, stderr bytes.Buffer
	code := runWithDeps([]string{"kimi"}, &stdout, &stderr, d)

	fmt.Printf("exit code: %d\n", code)
	fmt.Print(stderr.String())

	// Output:
	// exit code: 3
	// burnblocks: no Kimi Code token is configured. Set KIMIBURN_TOKEN or run `kimiburn setup`.
}
