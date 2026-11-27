package deepseek_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/acevedomiguel/burnblocks/deepseek"
)

// fakeSource is an in-process balance source: no network, no credentials.
type fakeSource struct {
	reading deepseek.BalanceReading
	err     error
}

func (f fakeSource) ReadBalance(context.Context) (deepseek.BalanceReading, error) {
	return f.reading, f.err
}

func (fakeSource) NextRefreshAllowedAt() time.Time { return time.Time{} }

func syntheticReading() deepseek.BalanceReading {
	return deepseek.BalanceReading{
		ObservedAt: time.Date(2026, 11, 27, 9, 30, 0, 0, time.UTC),
		Available:  deepseek.AvailabilityAvailable,
		Balances: []deepseek.Balance{
			{Currency: deepseek.CurrencyCNY, Total: ptr(123.45), Granted: ptr(3.45), ToppedUp: ptr(120)},
		},
	}
}

func ptr(v float64) *float64 { return &v }

func TestFetchRejectsAMissingSource(t *testing.T) {
	_, err := deepseek.Fetch(context.Background(), nil, deepseek.FetchOptions{})
	if !errors.Is(err, deepseek.ErrMissingSource) {
		t.Fatalf("err = %v, want ErrMissingSource", err)
	}
}

func TestFetchPropagatesTheReadError(t *testing.T) {
	want := errors.New("boom")
	_, err := deepseek.Fetch(context.Background(), fakeSource{err: want}, deepseek.FetchOptions{})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want the read error", err)
	}
}

func TestFetchDerivesASnapshotWithAnHonestUnavailableRunway(t *testing.T) {
	snap, err := deepseek.Fetch(context.Background(), fakeSource{reading: syntheticReading()}, deepseek.FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.SchemaVersion != deepseek.SnapshotSchemaVersion {
		t.Errorf("schema = %d", snap.SchemaVersion)
	}
	if snap.Runway.Status != deepseek.RunwayUnavailable || snap.Runway.Reason != deepseek.ReasonNoUsageHistory {
		t.Errorf("runway = %+v", snap.Runway)
	}
	if len(snap.Balances) != 1 || snap.Balances[0].Currency != deepseek.CurrencyCNY {
		t.Fatalf("balances = %+v", snap.Balances)
	}
	if snap.Balances[0].Total == nil || *snap.Balances[0].Total != 123.45 {
		t.Errorf("total = %v", snap.Balances[0].Total)
	}
}

// A source that omits an observation time falls back to the caller's clock.
func TestFetchStampsTheClockWhenTheReadingOmitsIt(t *testing.T) {
	now := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	reading := syntheticReading()
	reading.ObservedAt = time.Time{}
	snap, err := deepseek.Fetch(context.Background(), fakeSource{reading: reading}, deepseek.FetchOptions{Now: now})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !snap.GeneratedAt.Equal(now) {
		t.Errorf("generated at = %v, want the supplied clock", snap.GeneratedAt)
	}
}

// The public constructor wires a real client to a synthetic server with no
// external network.
func TestNewSourceReadsTheBalance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-synthetic-key" {
			t.Errorf("Authorization = %q", got)
		}
		_, _ = w.Write([]byte(`{"is_available":true,"balance_infos":[{"currency":"USD","total_balance":"42.00","granted_balance":"0.00","topped_up_balance":"42.00"}]}`))
	}))
	defer srv.Close()

	source, err := deepseek.NewSource(srv.URL, "sk-synthetic-key")
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	snap, err := deepseek.Fetch(context.Background(), source, deepseek.FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(snap.Balances) != 1 || snap.Balances[0].Currency != deepseek.CurrencyUSD {
		t.Fatalf("balances = %+v", snap.Balances)
	}
	if snap.Balances[0].Total == nil || *snap.Balances[0].Total != 42 {
		t.Errorf("total = %v", snap.Balances[0].Total)
	}
}

func TestNewSourceRejectsAMissingKey(t *testing.T) {
	if _, err := deepseek.NewSource(deepseek.DefaultBaseURL, ""); !errors.Is(err, deepseek.ErrMissingKey) {
		t.Fatalf("err = %v, want ErrMissingKey", err)
	}
}

func TestWriteJSONEmitsTheIndependentSchema(t *testing.T) {
	snap, err := deepseek.Fetch(context.Background(), fakeSource{reading: syntheticReading()}, deepseek.FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	var buf bytes.Buffer
	if err := deepseek.WriteJSON(&buf, snap); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	out := buf.Bytes()
	if len(out) == 0 || out[len(out)-1] != '\n' {
		t.Fatal("the document must end with a newline")
	}
	var doc struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.SchemaVersion != deepseek.SchemaVersion {
		t.Errorf("schema = %d, want %d", doc.SchemaVersion, deepseek.SchemaVersion)
	}
}

// Credentials resolve from an explicit environment map with no keyring and no
// real process environment.
func TestCredentialsResolveFromAnExplicitEnvironment(t *testing.T) {
	store := deepseek.NewCredentialStore(deepseek.CredentialOptions{
		KeyringEnabled: false,
		Env:            map[string]string{deepseek.CredentialEnvVar: "sk-synthetic-env"},
	})
	key, err := deepseek.ResolveKey(context.Background(), store)
	if err != nil {
		t.Fatalf("ResolveKey: %v", err)
	}
	if key != "sk-synthetic-env" {
		t.Errorf("key = %q", key)
	}
}

func TestRedactionRemovesAKeyShape(t *testing.T) {
	secret := "Authorization: Bearer sk-0123456789abcdef0123456789abcdef"
	if deepseek.Redact(secret) == secret {
		t.Error("redaction did not alter a secret-shaped value")
	}
	if strings.Contains(deepseek.Redact(secret), "0123456789abcdef") {
		t.Error("redaction left the key visible")
	}
}

func TestDefaultPathsResolves(t *testing.T) {
	if _, err := deepseek.DefaultPaths(); err != nil {
		t.Fatalf("DefaultPaths: %v", err)
	}
}
