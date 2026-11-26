package domain

import (
	"sort"
	"time"
)

// AggregateDaily maps raw rows onto exactly one bucket per UTC day in r.
// Missing days are zero-filled; duplicate rows for a day are summed.
func AggregateDaily(r TimeRange, rows []DailyUsage) []DailyUsage {
	buckets := r.DailyStarts()
	index := make(map[time.Time]int, len(buckets))
	for i, b := range buckets {
		index[b] = i
	}

	agg := make([]DailyUsage, len(buckets))
	for i, b := range buckets {
		agg[i] = DailyUsage{Day: b}
	}

	for _, row := range rows {
		day := startOfUTCDay(row.Day)
		i, ok := index[day]
		if !ok {
			continue
		}
		agg[i].CreditSpend += row.CreditSpend
		agg[i].Tokens += row.Tokens
		agg[i].Requests += row.Requests
		agg[i].TokensPrompt += row.TokensPrompt
		agg[i].TokensCompletion += row.TokensCompletion
		agg[i].CachedTokens += row.CachedTokens
		agg[i].ReasoningTokens += row.ReasoningTokens
	}

	return agg
}

// SumDaily totals spend, tokens, and active days across buckets.
func SumDaily(buckets []DailyUsage) DayTotal {
	var total DayTotal
	for _, b := range buckets {
		total.CreditSpend += b.CreditSpend
		total.Tokens += b.Tokens
		total.Requests += b.Requests
		total.TokensPrompt += b.TokensPrompt
		total.TokensCompletion += b.TokensCompletion
		total.CachedTokens += b.CachedTokens
		total.ReasoningTokens += b.ReasoningTokens
		if b.CreditSpend != 0 || b.Tokens != 0 {
			total.ActiveDays++
		}
	}
	return total
}

// CacheHitRate derives the prompt-cache hit ratio from a total. It returns nil
// when the API reported no prompt tokens, so the dashboard shows the row only
// when the number is meaningful.
func CacheHitRate(total DayTotal) *float64 {
	if total.TokensPrompt <= 0 {
		return nil
	}
	rate := float64(total.CachedTokens) / float64(total.TokensPrompt)
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	return &rate
}

// SortModels returns rows ordered by spend descending, ties broken by model
// name so rendering is deterministic. The input slice is not mutated.
func SortModels(models []ModelUsage) []ModelUsage {
	out := make([]ModelUsage, len(models))
	copy(out, models)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreditSpend != out[j].CreditSpend {
			return out[i].CreditSpend > out[j].CreditSpend
		}
		return out[i].Model < out[j].Model
	})
	return out
}

// TopModels limits rows to the top n by spend. The remainder is folded into a
// row named "Other (remaining)" so it can never be confused with a model that
// is literally called "Other".
func TopModels(models []ModelUsage, n int) []ModelUsage {
	if n <= 0 {
		return nil
	}
	sorted := SortModels(models)
	if len(sorted) <= n {
		return sorted
	}

	top := make([]ModelUsage, 0, n)
	top = append(top, sorted[:n-1]...)

	rest := ModelUsage{Model: "Other (remaining)"}
	for _, m := range sorted[n-1:] {
		rest.CreditSpend += m.CreditSpend
		rest.Tokens += m.Tokens
	}
	return append(top, rest)
}
