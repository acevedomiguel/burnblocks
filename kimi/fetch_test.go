package kimi_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	kimi "github.com/acevedomiguel/burnblocks/kimi"
)

type fakeUsageSource struct {
	reading kimi.UsageReading
	err     error
	calls   int
}

func (s *fakeUsageSource) ReadUsage(context.Context) (kimi.UsageReading, error) {
	s.calls++
	return s.reading, s.err
}

func (*fakeUsageSource) NextRefreshAllowedAt() time.Time { return time.Time{} }

func syntheticLibraryReading(now time.Time) kimi.UsageReading {
	used := 0.25
	reset := now.Add(4 * time.Hour)
	return kimi.UsageReading{
		ObservedAt: now,
		Quotas: map[kimi.QuotaKey]kimi.QuotaReading{
			kimi.QuotaLimit5h: {
				Source: kimi.SourceRatio, UsedRatio: &used, ResetAt: &reset,
				Raw: json.RawMessage(`{"used_ratio":0.25}`),
			},
		},
		Raw:      json.RawMessage(`{"usages":{"limit_5h":{"used_ratio":0.25}},"future_provider_field":"synthetic-value"}`),
		Warnings: []kimi.Warning{},
	}
}

func TestFetchSuccessUpdatesCallerState(t *testing.T) {
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	source := &fakeUsageSource{reading: syntheticLibraryReading(now)}
	state := kimi.EmptyStoredState()
	snapshot, err := kimi.Fetch(context.Background(), source, &state)
	if err != nil {
		t.Fatal(err)
	}
	if source.calls != 1 {
		t.Fatalf("ReadUsage calls=%d, want 1", source.calls)
	}
	if string(snapshot.Raw) != string(source.reading.Raw) {
		t.Fatalf("raw=%s, want %s", snapshot.Raw, source.reading.Raw)
	}
	runway := snapshot.Runways[kimi.QuotaLimit5h]
	if runway.Basis != "window_average" || runway.Status != "finite" {
		t.Fatalf("runway=%+v, want finite window_average", runway)
	}
	if len(state.Samples[kimi.QuotaLimit5h]) != 1 {
		t.Fatalf("samples=%+v, want one 5h sample", state.Samples)
	}
	if state.CacheVersion != kimi.CacheSchemaVersion || state.Snapshot == nil || !reflect.DeepEqual(*state.Snapshot, snapshot) {
		t.Fatalf("stored state=%+v, snapshot=%+v", state, snapshot)
	}
}

func TestFetchNilStateUsesColdStart(t *testing.T) {
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	source := &fakeUsageSource{reading: syntheticLibraryReading(now)}
	snapshot, err := kimi.Fetch(context.Background(), source, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.GeneratedAt.Equal(now) || snapshot.Runways[kimi.QuotaLimit5h].Basis != "window_average" {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestFetchFailureLeavesStateUnchanged(t *testing.T) {
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	previous := kimi.Snapshot{SchemaVersion: 1, GeneratedAt: now.Add(-time.Minute), Raw: json.RawMessage(`{"previous":true}`)}
	state := kimi.StoredState{
		CacheVersion: 1,
		Snapshot:     &previous,
		Samples: map[kimi.QuotaKey][]kimi.Sample{
			kimi.QuotaLimit5h: {{ObservedAt: now.Add(-time.Minute), UsedRatio: 0.2, ResetAt: now.Add(3 * time.Hour)}},
		},
	}
	beforeSnapshot := previous
	before := kimi.StoredState{
		CacheVersion: 1,
		Snapshot:     &beforeSnapshot,
		Samples: map[kimi.QuotaKey][]kimi.Sample{
			kimi.QuotaLimit5h: {{ObservedAt: now.Add(-time.Minute), UsedRatio: 0.2, ResetAt: now.Add(3 * time.Hour)}},
		},
	}
	readErr := errors.New("synthetic read failure")
	source := &fakeUsageSource{err: readErr}
	snapshot, err := kimi.Fetch(context.Background(), source, &state)
	if !errors.Is(err, readErr) {
		t.Fatalf("Fetch error=%v, want original read error", err)
	}
	if !reflect.DeepEqual(state, before) {
		t.Fatalf("state changed after failure\ngot:  %+v\nwant: %+v", state, before)
	}
	if !reflect.DeepEqual(snapshot, kimi.Snapshot{}) {
		t.Fatalf("snapshot=%+v, want zero value", snapshot)
	}
}

func TestFetchRejectsNilSource(t *testing.T) {
	snapshot, err := kimi.Fetch(context.Background(), nil, nil)
	if !errors.Is(err, kimi.ErrMissingSource) {
		t.Fatalf("Fetch error=%v, want ErrMissingSource", err)
	}
	if !reflect.DeepEqual(snapshot, kimi.Snapshot{}) {
		t.Fatalf("snapshot=%+v, want zero value", snapshot)
	}
}
