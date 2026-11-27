package projection

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/acevedomiguel/burnblocks/internal/deepseek/domain"
)

func ptr(v float64) *float64 { return &v }

func syntheticSnapshot() domain.Snapshot {
	return domain.Snapshot{
		SchemaVersion: domain.SnapshotSchemaVersion,
		GeneratedAt:   time.Date(2026, 11, 27, 9, 30, 0, 0, time.UTC),
		Available:     domain.AvailabilityAvailable,
		Balances: []domain.Balance{
			{Currency: domain.CurrencyCNY, Total: ptr(123.45), Granted: ptr(3.45), ToppedUp: ptr(120)},
			{Currency: domain.CurrencyUSD, Total: ptr(0)},
		},
		Runway:   domain.RunwayEstimate{Status: domain.RunwayUnavailable, Reason: domain.ReasonNoUsageHistory},
		Warnings: []domain.Warning{{Section: "Balances", Message: "a note"}},
	}
}

func TestMarshalJSONEndsWithANewlineAndVersionsIndependently(t *testing.T) {
	data, err := MarshalJSON(syntheticSnapshot())
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatal("the document must end with a newline")
	}
	var doc struct {
		SchemaVersion int    `json:"schema_version"`
		GeneratedAt   string `json:"generated_at"`
		Available     string `json:"available"`
		Runway        struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		} `json:"runway"`
		Warnings []domain.Warning `json:"warnings"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.SchemaVersion != PublicSchemaVersion {
		t.Errorf("schema_version = %d, want %d", doc.SchemaVersion, PublicSchemaVersion)
	}
	if PublicSchemaVersion != 1 {
		t.Errorf("PublicSchemaVersion = %d, want the independent version 1", PublicSchemaVersion)
	}
	if doc.GeneratedAt != "2026-11-27T09:30:00Z" {
		t.Errorf("generated_at = %q", doc.GeneratedAt)
	}
	if doc.Available != "available" {
		t.Errorf("available = %q", doc.Available)
	}
	if doc.Runway.Status != "unavailable" || doc.Runway.Reason != "no_usage_history" {
		t.Errorf("runway = %+v", doc.Runway)
	}
	if len(doc.Warnings) != 1 {
		t.Errorf("warnings = %v", doc.Warnings)
	}
}

func TestMarshalJSONKeepsCurrenciesSeparateAndZeroDistinctFromMissing(t *testing.T) {
	data, err := MarshalJSON(syntheticSnapshot())
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	var doc struct {
		Balances []struct {
			Currency string   `json:"currency"`
			Total    *float64 `json:"total"`
			Granted  *float64 `json:"granted"`
			ToppedUp *float64 `json:"topped_up"`
		} `json:"balances"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Balances) != 2 {
		t.Fatalf("balances = %d, want two separate currencies", len(doc.Balances))
	}
	if doc.Balances[0].Currency != "CNY" || doc.Balances[1].Currency != "USD" {
		t.Errorf("currencies = %q, %q", doc.Balances[0].Currency, doc.Balances[1].Currency)
	}
	// A reported zero stays 0, while a value the provider did not report is null.
	if doc.Balances[1].Total == nil || *doc.Balances[1].Total != 0 {
		t.Errorf("the USD zero must survive as 0, got %v", doc.Balances[1].Total)
	}
	if doc.Balances[1].Granted != nil {
		t.Errorf("a missing granted amount must be null, got %v", *doc.Balances[1].Granted)
	}
}

func TestBuildJSONDefaultsAnEmptySnapshotHonestly(t *testing.T) {
	doc := BuildJSON(domain.Snapshot{})
	if doc.Available != string(domain.AvailabilityUnknown) {
		t.Errorf("available = %q, want unknown", doc.Available)
	}
	if doc.Runway.Status != string(domain.RunwayUnavailable) {
		t.Errorf("runway status = %q, want unavailable", doc.Runway.Status)
	}
	if doc.Warnings == nil {
		t.Error("warnings must be an empty array, never null")
	}
	if doc.Balances == nil {
		t.Error("balances must be an empty array, never null")
	}
}

func TestWriteJSONWritesTheMarshalledDocument(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, syntheticSnapshot()); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	data, err := MarshalJSON(syntheticSnapshot())
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), data) {
		t.Error("WriteJSON and MarshalJSON must agree")
	}
}

func TestNewSnapshotAlwaysReportsUnavailableRunway(t *testing.T) {
	reading := domain.BalanceReading{
		ObservedAt: time.Date(2026, 11, 27, 9, 30, 0, 0, time.UTC),
		Available:  domain.AvailabilityUnavailable,
		Balances:   []domain.Balance{{Currency: domain.CurrencyUSD, Total: ptr(5)}},
	}
	snap := NewSnapshot(reading)
	if snap.SchemaVersion != domain.SnapshotSchemaVersion {
		t.Errorf("schema = %d", snap.SchemaVersion)
	}
	if snap.Runway.Status != domain.RunwayUnavailable || snap.Runway.Reason != domain.ReasonNoUsageHistory {
		t.Errorf("runway = %+v, want unavailable with no usage history", snap.Runway)
	}
	if !snap.GeneratedAt.Equal(reading.ObservedAt) {
		t.Errorf("generated at = %v", snap.GeneratedAt)
	}
}
