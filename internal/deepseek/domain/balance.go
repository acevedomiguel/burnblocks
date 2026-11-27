// Package domain holds the DeepSeek engine's provider-neutral types. It never
// imports a wire client, so the public package and its consumers depend on this
// package only.
package domain

import (
	"context"
	"time"
)

// SnapshotSchemaVersion versions the internal snapshot representation. It is
// independent of the public JSON schema version and of every other provider's
// numbering.
const SnapshotSchemaVersion = 1

// Availability is DeepSeek's readiness flag for calling models. It is not a
// forecast: it says whether the account can currently make API calls, not how
// long the balance lasts. The unknown value preserves a flag the provider did
// not report.
type Availability string

const (
	// AvailabilityUnknown means the provider did not report the flag.
	AvailabilityUnknown Availability = "unknown"
	// AvailabilityAvailable means the provider reported the account as usable.
	AvailabilityAvailable Availability = "available"
	// AvailabilityUnavailable means the provider reported the account as not
	// usable for API calls.
	AvailabilityUnavailable Availability = "unavailable"
)

// Currency is the unit of one balance. Two entries with different currencies
// are never added together, and a DeepSeek amount is never added to another
// provider's amount.
type Currency string

const (
	// CurrencyCNY is the Chinese yuan.
	CurrencyCNY Currency = "CNY"
	// CurrencyUSD is the United States dollar.
	CurrencyUSD Currency = "USD"
)

// Balance is one currency's reported balances. Each amount is a separate
// pointer: nil means the provider did not report a usable value, which is never
// the same as a reported zero. Granted balance is spent before topped-up
// balance, so the split is the only consumption signal the API offers.
type Balance struct {
	Currency Currency
	Total    *float64
	Granted  *float64
	ToppedUp *float64
}

// BalanceSource reads DeepSeek account balances. NextRefreshAllowedAt reports
// the earliest instant another read may run, or the zero time when no
// constraint was observed.
type BalanceSource interface {
	ReadBalance(ctx context.Context) (BalanceReading, error)
	NextRefreshAllowedAt() time.Time
}

// BalanceReading is one successful provider reading. Amounts that were absent
// or malformed stay nil; Warnings records the malformed ones. Balances keeps
// every currency the provider returned, in the order it returned them.
type BalanceReading struct {
	ObservedAt time.Time
	Available  Availability
	Balances   []Balance
	Warnings   []Warning
}

// RunwayStatus mirrors the other providers' runway vocabulary.
type RunwayStatus string

const (
	// RunwayUnavailable means no forecast can be derived.
	RunwayUnavailable RunwayStatus = "unavailable"
)

// RunwayReason explains why no forecast is available.
type RunwayReason string

const (
	// ReasonNoUsageHistory means DeepSeek publishes no spend history to estimate
	// from. Burn Blocks will not invent one.
	ReasonNoUsageHistory RunwayReason = "no_usage_history"
)

// RunwayEstimate is the forecast. DeepSeek exposes a single read-only balance
// call and no usage endpoint, so the only honest status today is unavailable.
type RunwayEstimate struct {
	Status RunwayStatus
	Reason RunwayReason
}

// Snapshot is one decoded reading plus its (currently unavailable) forecast.
type Snapshot struct {
	SchemaVersion int
	GeneratedAt   time.Time
	Available     Availability
	Balances      []Balance
	Runway        RunwayEstimate
	Warnings      []Warning
}

// Warning is a stable section name plus a human-readable message that contains
// no secrets.
type Warning struct {
	Section string `json:"section"`
	Message string `json:"message"`
}
