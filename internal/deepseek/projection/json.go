// Package projection builds DeepSeek snapshots and renders the provider's own
// versioned JSON document.
package projection

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/acevedomiguel/burnblocks/internal/deepseek/domain"
)

// PublicSchemaVersion is the DeepSeek engine's public JSON schema version. It is
// versioned independently of the other providers and of the internal snapshot
// representation.
const PublicSchemaVersion = 1

// NewSnapshot converts one successful reading into the internal snapshot. The
// forecast is always honestly unavailable: DeepSeek publishes no spend history,
// and a readiness flag is not a burn rate.
func NewSnapshot(reading domain.BalanceReading) domain.Snapshot {
	return domain.Snapshot{
		SchemaVersion: domain.SnapshotSchemaVersion,
		GeneratedAt:   reading.ObservedAt.UTC(),
		Available:     reading.Available,
		Balances:      cloneBalances(reading.Balances),
		Runway: domain.RunwayEstimate{
			Status: domain.RunwayUnavailable,
			Reason: domain.ReasonNoUsageHistory,
		},
		Warnings: cloneWarnings(reading.Warnings),
	}
}

// Document is the versioned machine-readable contract.
type Document struct {
	SchemaVersion int              `json:"schema_version"`
	GeneratedAt   string           `json:"generated_at"`
	Available     string           `json:"available"`
	Balances      []JSONBalance    `json:"balances"`
	Runway        JSONRunway       `json:"runway"`
	Warnings      []domain.Warning `json:"warnings"`
}

// JSONBalance is one currency's balances. A null amount means the provider did
// not report a usable value; it never means zero.
type JSONBalance struct {
	Currency string   `json:"currency"`
	Total    *float64 `json:"total"`
	Granted  *float64 `json:"granted"`
	ToppedUp *float64 `json:"topped_up"`
}

// JSONRunway is the forecast section. Status is always "unavailable" today.
type JSONRunway struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// BuildJSON converts a snapshot into the stable contract.
func BuildJSON(snap domain.Snapshot) Document {
	doc := Document{
		SchemaVersion: PublicSchemaVersion,
		GeneratedAt:   snap.GeneratedAt.UTC().Format(time.RFC3339),
		Available:     string(snap.Available),
		Balances:      make([]JSONBalance, 0, len(snap.Balances)),
		Runway: JSONRunway{
			Status: string(snap.Runway.Status),
			Reason: string(snap.Runway.Reason),
		},
		Warnings: cloneWarnings(snap.Warnings),
	}
	if doc.Available == "" {
		doc.Available = string(domain.AvailabilityUnknown)
	}
	if doc.Runway.Status == "" {
		doc.Runway.Status = string(domain.RunwayUnavailable)
	}
	for _, balance := range snap.Balances {
		doc.Balances = append(doc.Balances, JSONBalance{
			Currency: string(balance.Currency),
			Total:    balance.Total,
			Granted:  balance.Granted,
			ToppedUp: balance.ToppedUp,
		})
	}
	if doc.Warnings == nil {
		doc.Warnings = []domain.Warning{}
	}
	return doc
}

// MarshalJSON renders the contract as indented JSON with a trailing newline.
func MarshalJSON(snap domain.Snapshot) ([]byte, error) {
	data, err := json.MarshalIndent(BuildJSON(snap), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("json: encode projection: %w", err)
	}
	return append(data, '\n'), nil
}

// WriteJSON writes the indented document and a trailing newline to w. It
// marshals before writing so a projection error cannot leave partial output.
func WriteJSON(w io.Writer, snap domain.Snapshot) error {
	data, err := MarshalJSON(snap)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("json: write projection: %w", err)
	}
	return nil
}

// cloneBalances copies a reading so a caller cannot mutate a snapshot through
// the slice it still holds.
func cloneBalances(balances []domain.Balance) []domain.Balance {
	if balances == nil {
		return nil
	}
	out := make([]domain.Balance, len(balances))
	copy(out, balances)
	return out
}

// cloneWarnings returns a non-nil copy so the contract always emits an array,
// never null.
func cloneWarnings(warnings []domain.Warning) []domain.Warning {
	return append([]domain.Warning{}, warnings...)
}
