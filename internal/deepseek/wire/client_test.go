package wire

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// These fixtures are hand-authored and synthetic. They never contain real
// account data.
const (
	syntheticSingleCurrency = `{"is_available":true,"balance_infos":[{"currency":"CNY","total_balance":"123.45","granted_balance":"3.45","topped_up_balance":"120.00"}]}`
	syntheticTwoCurrencies  = `{"is_available":true,"balance_infos":[{"currency":"CNY","total_balance":"123.45","granted_balance":"3.45","topped_up_balance":"120.00"},{"currency":"USD","total_balance":"50.00","granted_balance":"0.00","topped_up_balance":"50.00"}]}`
)

// recorder is a no-wait sleeper that records each delay a retry asked for.
type recorder struct {
	delays []time.Duration
}

func (r *recorder) sleep(_ context.Context, d time.Duration) error {
	r.delays = append(r.delays, d)
	return nil
}

func newClient(t *testing.T, baseURL string, opts ...Option) *Client {
	t.Helper()
	options := append([]Option{WithBaseURL(baseURL), WithAPIKey("sk-synthetic-key"), WithSleep(func(context.Context, time.Duration) error { return nil })}, opts...)
	client, err := NewClient(options...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func handlerFunc(t *testing.T, body string, status int, count *int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if count != nil {
			atomic.AddInt32(count, 1)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-synthetic-key" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.URL.Path; got != endpointPath {
			t.Errorf("path = %q, want %q", got, endpointPath)
		}
		if status != http.StatusOK {
			http.Error(w, "no", status)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func ptrValue(t *testing.T, p *float64) float64 {
	t.Helper()
	if p == nil {
		t.Fatal("amount is nil, want a value")
	}
	return *p
}

func TestReadBalanceParsesStringsAndZero(t *testing.T) {
	srv := handlerFunc(t, syntheticSingleCurrency, http.StatusOK, nil)
	reading, err := newClient(t, srv.URL).ReadBalance(context.Background())
	if err != nil {
		t.Fatalf("ReadBalance: %v", err)
	}
	if reading.Available != "available" {
		t.Errorf("availability = %q, want available", reading.Available)
	}
	if len(reading.Balances) != 1 {
		t.Fatalf("balances = %d, want 1", len(reading.Balances))
	}
	balance := reading.Balances[0]
	if balance.Currency != "CNY" {
		t.Errorf("currency = %q", balance.Currency)
	}
	if got := ptrValue(t, balance.Total); got != 123.45 {
		t.Errorf("total = %v, want 123.45", got)
	}
	if got := ptrValue(t, balance.Granted); got != 3.45 {
		t.Errorf("granted = %v, want 3.45", got)
	}
	if got := ptrValue(t, balance.ToppedUp); got != 120.00 {
		t.Errorf("topped up = %v, want 120.00", got)
	}
	if len(reading.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", reading.Warnings)
	}
}

// A reported "0.00" is a real zero, not a missing value.
func TestReadBalanceKeepsARealZero(t *testing.T) {
	body := `{"is_available":false,"balance_infos":[{"currency":"USD","total_balance":"0.00","granted_balance":"0.00","topped_up_balance":"0.00"}]}`
	srv := handlerFunc(t, body, http.StatusOK, nil)
	reading, err := newClient(t, srv.URL).ReadBalance(context.Background())
	if err != nil {
		t.Fatalf("ReadBalance: %v", err)
	}
	if reading.Available != "unavailable" {
		t.Errorf("availability = %q, want unavailable", reading.Available)
	}
	balance := reading.Balances[0]
	if balance.Total == nil || *balance.Total != 0 {
		t.Errorf("total = %v, want a real zero", balance.Total)
	}
}

// A malformed or absent amount is unavailable, never zero.
func TestReadBalanceTreatsMalformedAndAbsentAsUnavailable(t *testing.T) {
	body := `{"is_available":true,"balance_infos":[{"currency":"CNY","total_balance":"not-a-number","granted_balance":"10.00"}]}`
	srv := handlerFunc(t, body, http.StatusOK, nil)
	reading, err := newClient(t, srv.URL).ReadBalance(context.Background())
	if err != nil {
		t.Fatalf("ReadBalance: %v", err)
	}
	balance := reading.Balances[0]
	if balance.Total != nil {
		t.Errorf("malformed total = %v, want nil", *balance.Total)
	}
	if balance.ToppedUp != nil {
		t.Errorf("absent topped up = %v, want nil", *balance.ToppedUp)
	}
	if len(reading.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one malformed-value warning", reading.Warnings)
	}
	if !strings.Contains(reading.Warnings[0].Message, "malformed") {
		t.Errorf("warning = %q", reading.Warnings[0].Message)
	}
}

// An absent readiness flag stays unknown rather than becoming false.
func TestReadBalanceAbsentFlagIsUnknown(t *testing.T) {
	body := `{"balance_infos":[{"currency":"CNY","total_balance":"1.00"}]}`
	srv := handlerFunc(t, body, http.StatusOK, nil)
	reading, err := newClient(t, srv.URL).ReadBalance(context.Background())
	if err != nil {
		t.Fatalf("ReadBalance: %v", err)
	}
	if reading.Available != "unknown" {
		t.Errorf("availability = %q, want unknown", reading.Available)
	}
}

// Two currencies are reported separately and never totalled.
func TestReadBalanceKeepsCurrenciesSeparate(t *testing.T) {
	srv := handlerFunc(t, syntheticTwoCurrencies, http.StatusOK, nil)
	reading, err := newClient(t, srv.URL).ReadBalance(context.Background())
	if err != nil {
		t.Fatalf("ReadBalance: %v", err)
	}
	if len(reading.Balances) != 2 {
		t.Fatalf("balances = %d, want 2 separate entries", len(reading.Balances))
	}
	if reading.Balances[0].Currency != "CNY" || reading.Balances[1].Currency != "USD" {
		t.Errorf("currencies = %q, %q", reading.Balances[0].Currency, reading.Balances[1].Currency)
	}
	var found bool
	for _, w := range reading.Warnings {
		if strings.Contains(w.Message, "never totalled") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a warning that currencies are never totalled: %v", reading.Warnings)
	}
}

// An entry without a currency cannot be attributed and is dropped with a
// warning rather than folded into another currency.
func TestReadBalanceDropsAnEntryWithoutACurrency(t *testing.T) {
	body := `{"is_available":true,"balance_infos":[{"total_balance":"9.99"},{"currency":"USD","total_balance":"1.00"}]}`
	srv := handlerFunc(t, body, http.StatusOK, nil)
	reading, err := newClient(t, srv.URL).ReadBalance(context.Background())
	if err != nil {
		t.Fatalf("ReadBalance: %v", err)
	}
	if len(reading.Balances) != 1 || reading.Balances[0].Currency != "USD" {
		t.Fatalf("balances = %+v, want only the USD entry", reading.Balances)
	}
	if len(reading.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one dropped-entry warning", reading.Warnings)
	}
}

// A JSON number instead of the documented string is tolerated.
func TestReadBalanceAcceptsANumber(t *testing.T) {
	body := `{"is_available":true,"balance_infos":[{"currency":"USD","total_balance":12.5}]}`
	srv := handlerFunc(t, body, http.StatusOK, nil)
	reading, err := newClient(t, srv.URL).ReadBalance(context.Background())
	if err != nil {
		t.Fatalf("ReadBalance: %v", err)
	}
	if got := ptrValue(t, reading.Balances[0].Total); got != 12.5 {
		t.Errorf("total = %v, want 12.5", got)
	}
	if len(reading.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", reading.Warnings)
	}
}

func TestReadBalanceStatusClassification(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, ErrUnauthorized},
		{http.StatusPaymentRequired, ErrInsufficientBalance},
		{http.StatusForbidden, ErrForbidden},
		{http.StatusNotFound, ErrNotFound},
		{http.StatusInternalServerError, ErrServer},
		{http.StatusBadGateway, ErrServer},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			srv := handlerFunc(t, "", tc.status, nil)
			_, err := newClient(t, srv.URL).ReadBalance(context.Background())
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// A 402 is terminal: retrying cannot add funds.
func TestReadBalanceDoesNotRetryInsufficientBalance(t *testing.T) {
	var count int32
	srv := handlerFunc(t, "", http.StatusPaymentRequired, &count)
	_, err := newClient(t, srv.URL).ReadBalance(context.Background())
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("err = %v", err)
	}
	if got := atomic.LoadInt32(&count); got != 1 {
		t.Errorf("requests = %d, want exactly 1 (no retry)", got)
	}
}

// A 429 is retried and its Retry-After delay is honoured.
func TestReadBalanceHonoursRetryAfter(t *testing.T) {
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&count, 1) == 1 {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(syntheticSingleCurrency))
	}))
	defer srv.Close()

	rec := &recorder{}
	client := newClient(t, srv.URL, WithSleep(rec.sleep))
	reading, err := client.ReadBalance(context.Background())
	if err != nil {
		t.Fatalf("ReadBalance: %v", err)
	}
	if len(reading.Balances) != 1 {
		t.Fatalf("balances = %d, want the recovered read", len(reading.Balances))
	}
	if got := atomic.LoadInt32(&count); got != 2 {
		t.Errorf("requests = %d, want 2", got)
	}
	if len(rec.delays) != 1 || rec.delays[0] != 7*time.Second {
		t.Errorf("delays = %v, want [7s]", rec.delays)
	}
	if at := client.NextRefreshAllowedAt(); at.IsZero() {
		t.Error("Retry-After must record the next allowed refresh")
	}
}

// A transport failure is retried the bounded number of times.
func TestReadBalanceRetriesTransportFailure(t *testing.T) {
	calls := 0
	client, err := NewClient(
		WithBaseURL("http://127.0.0.1:1"),
		WithAPIKey("sk-synthetic-key"),
		WithHTTPClient(&http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
			calls++
			return nil, errors.New("boom")
		})}),
		WithSleep(func(context.Context, time.Duration) error { return nil }),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = client.ReadBalance(context.Background())
	if !errors.Is(err, ErrTransport) {
		t.Fatalf("err = %v, want ErrTransport", err)
	}
	if calls != maxRetries+1 {
		t.Errorf("calls = %d, want %d", calls, maxRetries+1)
	}
}

func TestReadBalanceRejectsMalformedAndOversizedBodies(t *testing.T) {
	t.Run("malformed", func(t *testing.T) {
		srv := handlerFunc(t, `["not","an","object"]`, http.StatusOK, nil)
		_, err := newClient(t, srv.URL).ReadBalance(context.Background())
		if !errors.Is(err, ErrMalformedResponse) {
			t.Fatalf("err = %v, want ErrMalformedResponse", err)
		}
	})
	t.Run("oversized", func(t *testing.T) {
		srv := handlerFunc(t, strings.Repeat("a", maxResponseBytes+2), http.StatusOK, nil)
		_, err := newClient(t, srv.URL).ReadBalance(context.Background())
		if !errors.Is(err, ErrResponseTooLarge) {
			t.Fatalf("err = %v, want ErrResponseTooLarge", err)
		}
	})
}

// The client never follows a redirect, so the bearer cannot be forwarded.
func TestReadBalanceRefusesRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.invalid/elsewhere", http.StatusFound)
	}))
	defer srv.Close()
	_, err := newClient(t, srv.URL).ReadBalance(context.Background())
	if !errors.Is(err, ErrRedirect) {
		t.Fatalf("err = %v, want ErrRedirect", err)
	}
}

func TestNewClientValidation(t *testing.T) {
	t.Run("missing key", func(t *testing.T) {
		if _, err := NewClient(WithBaseURL("https://api.deepseek.com")); !errors.Is(err, ErrMissingKey) {
			t.Fatalf("err = %v, want ErrMissingKey", err)
		}
	})
	t.Run("plain http outside loopback", func(t *testing.T) {
		_, err := NewClient(WithBaseURL("http://api.deepseek.com"), WithAPIKey("sk-synthetic-key"))
		if !errors.Is(err, ErrUnsupportedBaseURL) {
			t.Fatalf("err = %v, want ErrUnsupportedBaseURL", err)
		}
	})
	t.Run("loopback http is allowed", func(t *testing.T) {
		if _, err := NewClient(WithBaseURL("http://127.0.0.1:8080"), WithAPIKey("sk-synthetic-key")); err != nil {
			t.Fatalf("loopback base must be allowed for tests: %v", err)
		}
	})
}

// A base that is not a bare origin would move the request off the fixed
// endpoint, so it must be refused rather than silently mangled into the URL.
func TestNewClientRejectsNonOriginBases(t *testing.T) {
	bases := []string{
		"https://api.deepseek.com?x=1",
		"https://api.deepseek.com#fragment",
		"https://user:pass@api.deepseek.com",
		"https://api.deepseek.com/proxy",
		"https://",
		"https://api.deepseek.com?",
		"https://api.deepseek.com#",
		"https://api.deepseek.com/?",
		"https://api.deepseek.com/#",
	}
	for _, base := range bases {
		t.Run(base, func(t *testing.T) {
			_, err := NewClient(WithBaseURL(base), WithAPIKey("sk-synthetic-key"))
			if !errors.Is(err, ErrUnsupportedBaseURL) {
				t.Fatalf("err = %v, want ErrUnsupportedBaseURL for %q", err, base)
			}
		})
	}
}

func TestDecodeBalanceIgnoresUnknownFields(t *testing.T) {
	body := `{"is_available":true,"future_field":123,"balance_infos":[{"currency":"CNY","total_balance":"1.00","extra":"x"}]}`
	reading, err := decodeBalance([]byte(body), time.Unix(0, 0))
	if err != nil {
		t.Fatalf("decodeBalance: %v", err)
	}
	if len(reading.Balances) != 1 || reading.Balances[0].Total == nil {
		t.Fatalf("reading = %+v", reading)
	}
}

// An unrecognised currency is provider-controlled text. It must be dropped with
// a static warning, never echoed into a warning or the projected balances, so a
// secret-shaped value cannot reach a serialized document unredacted.
func TestReadBalanceDropsAnUnrecognisedCurrencyWithoutEchoingIt(t *testing.T) {
	const secret = "sk-0123456789abcdef0123456789abcdef"
	body := `{"is_available":true,"balance_infos":[{"currency":"` + secret + `","total_balance":"5.00"},{"currency":"CNY","total_balance":"1.00"}]}`
	srv := handlerFunc(t, body, http.StatusOK, nil)
	reading, err := newClient(t, srv.URL).ReadBalance(context.Background())
	if err != nil {
		t.Fatalf("ReadBalance: %v", err)
	}
	if len(reading.Balances) != 1 || reading.Balances[0].Currency != "CNY" {
		t.Fatalf("balances = %+v, want only the documented CNY entry", reading.Balances)
	}
	var found bool
	for _, w := range reading.Warnings {
		if strings.Contains(w.Message, secret) || strings.Contains(w.Message, "sk-") {
			t.Errorf("warning echoed provider text: %q", w.Message)
		}
		if strings.Contains(w.Message, "unrecognised currency") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an unrecognised-currency warning: %v", reading.Warnings)
	}
}

// transportFunc is an http.RoundTripper driven by a function.
type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
