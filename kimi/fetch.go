package kimi

import (
	"context"
	"errors"

	"github.com/acevedomiguel/burnblocks/internal/kimi/projection"
	"github.com/acevedomiguel/burnblocks/internal/kimi/runway"
)

// ErrMissingSource rejects a fetch without a usage source.
var ErrMissingSource = errors.New("kimiburn: usage source is required")

// BuildSnapshot derives a snapshot from one successful reading and folds the
// reading into state. It is the normalization and runway step shared by Fetch
// and by callers that perform their own read, such as an application that
// substitutes a clock only when the provider omitted an observation time.
//
// A non-nil state is updated in place; a nil state uses an ephemeral cold start
// that is not retained. BuildSnapshot never loads or saves a cache, and callers
// must serialize access to shared state.
func BuildSnapshot(reading UsageReading, state *StoredState) Snapshot {
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
	return snapshot
}

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

	return BuildSnapshot(reading, state), nil
}
