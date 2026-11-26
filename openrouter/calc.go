package openrouter

import (
	"time"

	"github.com/acevedomiguel/burnblocks/internal/openrouter/domain"
)

// AggregateDaily maps raw rows onto exactly one bucket per UTC day in r.
// Missing days are zero-filled; duplicate rows for a day are summed.
func AggregateDaily(r TimeRange, rows []DailyUsage) []DailyUsage {
	return domain.AggregateDaily(r, rows)
}

// SortModels returns rows ordered by spend descending, ties broken by model
// name so rendering is deterministic. The input slice is not mutated.
func SortModels(models []ModelUsage) []ModelUsage { return domain.SortModels(models) }

// TopModels limits rows to the top n by spend. The remainder is folded into a
// row named "Other (remaining)" so it can never be confused with a model that
// is literally called "Other".
func TopModels(models []ModelUsage, n int) []ModelUsage { return domain.TopModels(models, n) }

// DisplayRange returns the 30 daily buckets that include the current UTC day.
func DisplayRange(now time.Time) TimeRange { return domain.DisplayRange(now) }
