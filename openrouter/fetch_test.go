package openrouter_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	openrouter "github.com/acevedomiguel/burnblocks/openrouter"
)

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

func fixedTime() time.Time {
	return time.Date(2026, 11, 21, 15, 42, 0, 0, time.UTC)
}

func TestFetchUsesPublicAPI(t *testing.T) {
	snap, err := openrouter.Fetch(context.Background(), fakeAPI{}, openrouter.FetchOptions{Now: fixedTime()})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got := len(snap.Daily); got != 30 {
		t.Fatalf("daily buckets = %d, want 30", got)
	}
	if snap.Account.Remaining != 42.1 {
		t.Errorf("remaining = %v, want 42.1", snap.Account.Remaining)
	}
	if snap.Key != nil {
		t.Errorf("key = %+v, want nil for a missing regular key", snap.Key)
	}
}

func TestNewClientDelegatesOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/credits":
			if got := r.Header.Get("Authorization"); got != "Bearer synthetic-management" {
				t.Errorf("management authorization = %q", got)
			}
			if got := r.Header.Get("User-Agent"); got != "RouteBurn/test" {
				t.Errorf("user agent = %q", got)
			}
			fmt.Fprint(w, `{"data":{"total_credits":100,"total_usage":57.9}}`)
		case "/key":
			if got := r.Header.Get("Authorization"); got != "Bearer synthetic-regular" {
				t.Errorf("regular authorization = %q", got)
			}
			fmt.Fprint(w, `{"data":{"label":"synthetic","limit":null,"limit_remaining":null,"limit_reset":null,"usage_daily":1,"usage_weekly":2,"usage_monthly":3}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := openrouter.NewClient(
		openrouter.WithBaseURL(server.URL),
		openrouter.WithManagementKey("synthetic-management"),
		openrouter.WithAPIKey("synthetic-regular"),
		openrouter.WithHTTPClient(server.Client()),
		openrouter.WithVersion("test"),
		openrouter.WithSleep(func(time.Duration) {}),
		openrouter.WithClock(fixedTime),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	credits, err := client.Credits(context.Background())
	if err != nil {
		t.Fatalf("Credits: %v", err)
	}
	if credits.Remaining != 42.1 {
		t.Errorf("remaining = %v, want 42.1", credits.Remaining)
	}
	key, err := client.Key(context.Background())
	if err != nil {
		t.Fatalf("Key: %v", err)
	}
	if key.Label != "synthetic" || key.Limit != nil || key.LimitRemaining != nil {
		t.Errorf("key = %+v", key)
	}
}
