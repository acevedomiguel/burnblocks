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

	"github.com/acevedomiguel/burnblocks/openrouter"
)

// fakeOpenRouterAPI is an in-process read-only API: no network, no credentials.
type fakeOpenRouterAPI struct {
	credits    openrouter.CreditSummary
	creditsErr error
	key        openrouter.KeyInfo
	keyErr     error
	rows       []openrouter.AnalyticsRow
	rowsErr    error
}

func (f fakeOpenRouterAPI) Credits(context.Context) (openrouter.CreditSummary, error) {
	return f.credits, f.creditsErr
}

func (f fakeOpenRouterAPI) Key(context.Context) (openrouter.KeyInfo, error) {
	return f.key, f.keyErr
}

func (f fakeOpenRouterAPI) QueryAnalytics(context.Context, openrouter.AnalyticsQuery) ([]openrouter.AnalyticsRow, error) {
	return f.rows, f.rowsErr
}

func (f fakeOpenRouterAPI) NextRefreshAllowedAt() time.Time { return time.Time{} }

// openRouterTestDeps builds hermetic dependencies: a synthetic management key
// from the environment map, an injected API, and a temporary configuration and
// cache layout.
func openRouterTestDeps(t *testing.T, api openrouter.API) deps {
	t.Helper()
	dir := t.TempDir()
	return deps{
		environment:   map[string]string{openrouter.EnvManagementKey: "synthetic-management-key"},
		openRouterAPI: func(_, _, _, _ string) (openrouter.API, error) { return api, nil },
		openRouterPaths: func() (openrouter.Paths, error) {
			return openrouter.Paths{
				ConfigDir:  dir,
				ConfigFile: filepath.Join(dir, "config.yaml"),
				CacheDir:   dir,
				CacheFile:  filepath.Join(dir, "snapshot.json"),
			}, nil
		},
		now: kimiNow,
	}
}

func TestOpenRouterMissingManagementKeyExitsCredentials(t *testing.T) {
	d := openRouterTestDeps(t, fakeOpenRouterAPI{})
	d.environment = map[string]string{}

	var stdout, stderr bytes.Buffer
	code := runWithDeps([]string{"openrouter"}, &stdout, &stderr, d)

	if code != exitCredentials {
		t.Fatalf("exit = %d, want %d", code, exitCredentials)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "no OpenRouter management key") {
		t.Errorf("stderr = %q, want a missing-credential diagnostic", stderr.String())
	}
}

// The regular API key alone must never satisfy the management slot.
func TestOpenRouterRegularKeyAloneIsNotEnough(t *testing.T) {
	d := openRouterTestDeps(t, fakeOpenRouterAPI{})
	d.environment = map[string]string{openrouter.EnvOpenRouterKey: "synthetic-regular-key"}

	var stdout, stderr bytes.Buffer
	code := runWithDeps([]string{"openrouter"}, &stdout, &stderr, d)

	if code != exitCredentials {
		t.Fatalf("exit = %d, want %d", code, exitCredentials)
	}
}

func TestOpenRouterJSONEmitsSchemaTwo(t *testing.T) {
	api := fakeOpenRouterAPI{
		credits: openrouter.CreditSummary{TotalPurchased: 100, TotalUsed: 57.9, Remaining: 42.1},
		keyErr:  openrouter.ErrMissingAPIKey,
	}
	d := openRouterTestDeps(t, api)

	var stdout, stderr bytes.Buffer
	code := runWithDeps([]string{"openrouter", "--json"}, &stdout, &stderr, d)

	if code != exitOK {
		t.Fatalf("exit = %d (stderr %q)", code, stderr.String())
	}
	var doc struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, stdout.String())
	}
	if doc.SchemaVersion != openrouter.SchemaVersion {
		t.Errorf("schema_version = %d, want %d", doc.SchemaVersion, openrouter.SchemaVersion)
	}
	if !strings.HasSuffix(stdout.String(), "\n") {
		t.Error("JSON output must end with a newline")
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty on success", stderr.String())
	}
}

func TestOpenRouterTextReportsCreditsAndDollars(t *testing.T) {
	api := fakeOpenRouterAPI{
		credits: openrouter.CreditSummary{TotalPurchased: 100, TotalUsed: 57.9, Remaining: 42.1},
		keyErr:  openrouter.ErrMissingAPIKey,
	}
	d := openRouterTestDeps(t, api)

	var stdout, stderr bytes.Buffer
	code := runWithDeps([]string{"openrouter"}, &stdout, &stderr, d)

	if code != exitOK {
		t.Fatalf("exit = %d (stderr %q)", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"OpenRouter", "$42.10", "$100.00", "updated"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	// Kimi reports ratios; OpenRouter never renders a percentage for money.
	if strings.Contains(out, "%") {
		t.Errorf("OpenRouter output must not render a percentage:\n%s", out)
	}
}

// Credits are required; a failing analytics read degrades to a warning.
func TestOpenRouterAnalyticsFailureStillReportsCredits(t *testing.T) {
	api := fakeOpenRouterAPI{
		credits: openrouter.CreditSummary{TotalPurchased: 100, TotalUsed: 57.9, Remaining: 42.1},
		keyErr:  openrouter.ErrMissingAPIKey,
		rowsErr: errors.New("synthetic analytics failure"),
	}
	d := openRouterTestDeps(t, api)

	var stdout, stderr bytes.Buffer
	code := runWithDeps([]string{"openrouter"}, &stdout, &stderr, d)

	if code != exitOK {
		t.Fatalf("exit = %d, want %d with a degraded analytics read (stderr %q)", code, exitOK, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "$42.10") {
		t.Errorf("credits must still be reported:\n%s", out)
	}
	if !strings.Contains(out, "synthetic analytics failure") {
		t.Errorf("the analytics failure must be warned about:\n%s", out)
	}
}

func TestOpenRouterCreditsFailureWithoutCacheExitsFailure(t *testing.T) {
	api := fakeOpenRouterAPI{creditsErr: errors.New("synthetic credits failure")}
	d := openRouterTestDeps(t, api)

	var stdout, stderr bytes.Buffer
	code := runWithDeps([]string{"openrouter"}, &stdout, &stderr, d)

	if code != exitFailure {
		t.Fatalf("exit = %d, want %d", code, exitFailure)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "synthetic credits failure") {
		t.Errorf("stderr = %q, want the provider failure", stderr.String())
	}
}

func TestOpenRouterUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"unknown flag", []string{"openrouter", "--bogus"}},
		{"positional argument", []string{"openrouter", "extra"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := openRouterTestDeps(t, fakeOpenRouterAPI{})
			var stdout, stderr bytes.Buffer
			if code := runWithDeps(tc.args, &stdout, &stderr, d); code != exitUsage {
				t.Fatalf("exit = %d, want %d", code, exitUsage)
			}
			if !strings.Contains(stderr.String(), "Usage:") {
				t.Errorf("stderr = %q, want the usage block", stderr.String())
			}
		})
	}
}
