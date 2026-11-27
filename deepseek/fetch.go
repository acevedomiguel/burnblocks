package deepseek

import (
	"context"
	"errors"
	"time"

	"github.com/acevedomiguel/burnblocks/internal/deepseek/projection"
)

// ErrMissingSource rejects a fetch without a balance source.
var ErrMissingSource = errors.New("deepseek: balance source is required")

// FetchOptions configures one full snapshot fetch.
type FetchOptions struct {
	// Now is the reference instant recorded when a source returns a reading
	// without an observation time. It is normalised to UTC.
	Now time.Time
}

// BuildSnapshot converts one successful reading into a snapshot with an honest
// unavailable forecast. DeepSeek publishes no spend history, so no runway can be
// derived from the API.
func BuildSnapshot(reading BalanceReading) Snapshot {
	return projection.NewSnapshot(reading)
}

// Fetch performs one read-only balance check and derives a snapshot. It never
// loads or saves a cache and never reads a credential implicitly.
func Fetch(ctx context.Context, source BalanceSource, opts FetchOptions) (Snapshot, error) {
	if source == nil {
		return Snapshot{}, ErrMissingSource
	}
	reading, err := source.ReadBalance(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	if reading.ObservedAt.IsZero() {
		reading.ObservedAt = opts.Now
	}
	return BuildSnapshot(reading), nil
}
