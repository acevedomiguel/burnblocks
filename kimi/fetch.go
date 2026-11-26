package kimi

import (
	"context"
	"errors"

	"github.com/acevedomiguel/burnblocks/internal/kimi/projection"
	"github.com/acevedomiguel/burnblocks/internal/kimi/runway"
)

// ErrMissingSource rejects a fetch without a usage source.
var ErrMissingSource = errors.New("kimiburn: usage source is required")

// Fetch performs one headless read and derives a snapshot using the same
// normalization and runway logic as the CLI. Non-nil state is updated only
// after a successful read; nil state uses an ephemeral cold start. Fetch never
// loads or saves a cache. Callers must serialize access to shared state.
func Fetch(ctx context.Context, source UsageSource, state *StoredState) (Snapshot, error) {
	if source == nil {
		return Snapshot{}, ErrMissingSource
	}
	reading, err := source.ReadUsage(ctx)
	if err != nil {
		return Snapshot{}, err
	}

	snapshot := projection.NewSnapshot(reading)
	next := EmptyStoredState()
	if state != nil {
		next = *state
	}
	if next.Samples == nil {
		next.Samples = make(map[QuotaKey][]Sample)
	}
	runway.RecordReading(next.Samples, reading)
	snapshot.Runways = runway.CalculateRunways(next.Samples, reading)
	next.CacheVersion = CacheSchemaVersion
	next.Snapshot = &snapshot
	if state != nil {
		*state = next
	}
	return snapshot, nil
}
