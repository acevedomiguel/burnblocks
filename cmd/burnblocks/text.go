package main

import (
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/acevedomiguel/burnblocks/kimi"
	"github.com/acevedomiguel/burnblocks/openrouter"
)

// notReported marks a value the provider did not send. It is never rendered as
// zero, matching the missing-versus-zero rule the engines enforce.
const notReported = "not reported"

// writeKimiText renders one concise Kimi Code status block. It reports ratios
// and never converts them to money, because Kimi has no dollar figure.
func writeKimiText(w io.Writer, snapshot kimi.Snapshot, apiBase string, cached bool, now time.Time) {
	fmt.Fprintf(w, "Kimi Code — %s\n", apiBase)

	for _, key := range kimi.CanonicalQuotaKeys {
		fmt.Fprintf(w, "\n  %s\n", key)
		quota, ok := snapshot.Quotas[key]
		if !ok {
			fmt.Fprintf(w, "    used      %s\n", notReported)
			continue
		}
		fmt.Fprintf(w, "    used      %s\n", kimiUsedCell(quota))
		fmt.Fprintf(w, "    resets    %s\n", resetCell(quota.ResetAt, now))
		if quota.ConflictReason != "" {
			fmt.Fprintf(w, "    conflict  %s\n", quota.ConflictReason)
		}
		fmt.Fprintf(w, "    runway    %s\n", kimiRunwayCell(snapshot.Runways[key]))
	}

	writeFooter(w, cached, now, kimiWarnings(snapshot.Warnings))
}

// writeOpenRouterText renders one concise OpenRouter status block. It reports
// US dollars and never mixes them with Kimi's ratios.
func writeOpenRouterText(w io.Writer, snapshot openrouter.Snapshot, cached bool, now time.Time) {
	fmt.Fprintf(w, "OpenRouter — %s\n\n", openrouter.DefaultBaseURL)

	fmt.Fprintf(w, "  credits    %s remaining of %s purchased (%s used)\n",
		credits(snapshot.Account.Remaining),
		credits(snapshot.Account.TotalPurchased),
		credits(snapshot.Account.TotalUsed))

	days := snapshot.Range.Days()
	fmt.Fprintf(w, "  last %-4s %s spent · %s tokens · %s requests",
		fmt.Sprintf("%dd", days), credits(totalSpend(snapshot)), tokens(snapshot.TotalTokens), count(snapshot.Requests))
	if snapshot.CacheHitRate != nil {
		fmt.Fprintf(w, " · %s cache hit", percent(*snapshot.CacheHitRate))
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "  runway     %s\n", openRouterRunwayCell(snapshot.Runway))
	fmt.Fprintf(w, "  key        %s\n", openRouterKeyCell(snapshot.Key))

	if models := openrouter.TopModels(snapshot.Models, 4); len(models) > 0 {
		rendered := make([]string, 0, len(models))
		for _, model := range models {
			rendered = append(rendered, model.Model+" "+credits(model.CreditSpend))
		}
		fmt.Fprintf(w, "  models     %s\n", strings.Join(rendered, " · "))
	}

	writeFooter(w, cached, now, openRouterWarnings(snapshot.Warnings))
}

// writeFooter prints the freshness stamp and any warnings. Diagnostics stay on
// the same stream as the report so a caller redirecting stdout keeps them
// together; the error path is what goes to stderr.
func writeFooter(w io.Writer, cached bool, now time.Time, warnings []warning) {
	fmt.Fprintf(w, "\n  updated    %s UTC", now.Format("2006-01-02 15:04"))
	if cached {
		fmt.Fprint(w, " · cached")
	}
	fmt.Fprintln(w)

	if len(warnings) == 0 {
		return
	}
	fmt.Fprintln(w, "\n  warnings")
	for _, item := range warnings {
		fmt.Fprintf(w, "    [%s] %s\n", item.section, item.message)
	}
}

// warning is the provider-neutral shape of a snapshot warning. Each provider
// package has its own Warning type, so the renderer normalizes them.
type warning struct {
	section string
	message string
}

func kimiWarnings(in []kimi.Warning) []warning {
	out := make([]warning, 0, len(in))
	for _, item := range in {
		out = append(out, warning{section: item.Section, message: item.Message})
	}
	return out
}

func openRouterWarnings(in []openrouter.Warning) []warning {
	out := make([]warning, 0, len(in))
	for _, item := range in {
		out = append(out, warning{section: item.Section, message: item.Message})
	}
	return out
}

// kimiUsedCell renders a quota's used amount from whichever representation the
// provider reported.
func kimiUsedCell(quota kimi.QuotaReading) string {
	if quota.UsedRatio != nil && isRatio(*quota.UsedRatio) {
		return percent(*quota.UsedRatio)
	}
	if quota.Used != nil && quota.Limit != nil {
		return fmt.Sprintf("%s of %s", number(*quota.Used), number(*quota.Limit))
	}
	if quota.Remaining != nil && quota.Limit != nil {
		return fmt.Sprintf("%s remaining of %s", number(*quota.Remaining), number(*quota.Limit))
	}
	return notReported
}

// kimiRunwayCell renders one quota's forecast. The status vocabulary is the
// engine's: "finite" is a real exhaustion estimate, "beyond_reset" means the
// quota will not run out before it resets, and Confidence only qualifies a
// finite estimate.
func kimiRunwayCell(est kimi.RunwayEstimate) string {
	seconds := func() *time.Duration {
		if est.Seconds == nil {
			return nil
		}
		d := time.Duration(*est.Seconds * float64(time.Second))
		return &d
	}

	switch est.Status {
	case "":
		return notReported
	case "unavailable":
		if est.Reason != "" {
			return "unavailable · " + est.Reason
		}
		return "unavailable"
	case "depleted":
		return "depleted · quota exhausted"
	case "no_consumption":
		return "no consumption observed"
	case "beyond_reset":
		if d := seconds(); d != nil {
			return "beyond this reset (resets in " + humanDuration(*d) + ")"
		}
		return "beyond this reset"
	case "finite":
		d := seconds()
		if d == nil {
			return "finite"
		}
		out := humanDuration(*d)
		if est.Confidence != "" && est.Confidence != "unavailable" {
			out += " · " + est.Confidence + " confidence"
		}
		if est.Basis != "" {
			out += " · " + est.Basis
		}
		if est.ExhaustedAt != nil {
			out += " · depletes " + est.ExhaustedAt.UTC().Format("2006-01-02 15:04")
		}
		return out
	default:
		return est.Status
	}
}

// openRouterRunwayCell renders the credit forecast.
func openRouterRunwayCell(est openrouter.RunwayEstimate) string {
	switch est.Basis {
	case openrouter.BasisUnavailable:
		return "unavailable"
	case openrouter.BasisNoRecentSpend:
		return "no recent spend"
	case openrouter.BasisBeyondHorizon:
		return "beyond horizon"
	case openrouter.BasisDepleted:
		return "depleted"
	}
	if est.DaysRemaining == nil {
		return string(est.Basis)
	}
	out := humanDuration(time.Duration(math.Floor(*est.DaysRemaining) * 24 * float64(time.Hour)))
	if est.LowConfidence {
		out += " · low confidence"
	}
	if est.DepletionDate != nil {
		out += " · depletes " + est.DepletionDate.Format("2006-01-02")
	}
	return out
}

// openRouterKeyCell renders the optional regular-key section. A nil key means
// no regular key is configured, which is not the same as a zero limit.
func openRouterKeyCell(key *openrouter.KeyCredits) string {
	if key == nil {
		return "no regular API key configured"
	}
	parts := make([]string, 0, 3)
	if key.Label != "" {
		parts = append(parts, fmt.Sprintf("%q", key.Label))
	}
	if key.Limit != nil {
		parts = append(parts, "limit "+credits(*key.Limit))
	} else {
		parts = append(parts, "unlimited")
	}
	if key.LimitRemaining != nil {
		parts = append(parts, "remaining "+credits(*key.LimitRemaining))
	}
	if len(parts) == 0 {
		return notReported
	}
	return strings.Join(parts, " · ")
}

// totalSpend sums the daily buckets, since the snapshot carries per-day figures.
func totalSpend(snapshot openrouter.Snapshot) float64 {
	var total float64
	for _, day := range snapshot.Daily {
		total += day.CreditSpend
	}
	return total
}

// resetCell renders a quota reset as a relative time, or a dash when the
// provider did not report one.
func resetCell(reset *time.Time, now time.Time) string {
	if reset == nil {
		return notReported
	}
	at := reset.UTC()
	if !at.After(now) {
		return "due now (" + at.Format("2006-01-02 15:04") + " UTC)"
	}
	return "in " + humanDuration(at.Sub(now)) + " (" + at.Format("2006-01-02 15:04") + " UTC)"
}

// isRatio reports whether v is a usable ratio.
func isRatio(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 1
}

// percent renders a 0..1 ratio as a whole percentage.
func percent(v float64) string { return fmt.Sprintf("%.0f%%", v*100) }

// credits renders a USD amount with two decimals and thousands separators.
func credits(v float64) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	whole := int64(v)
	cents := int64(math.Round((v - float64(whole)) * 100))
	if cents == 100 {
		whole++
		cents = 0
	}
	return fmt.Sprintf("%s$%s.%02d", sign, count(whole), cents)
}

// number renders a possibly fractional value without trailing zeros.
func number(v float64) string {
	if v == math.Trunc(v) {
		return count(int64(v))
	}
	return count(int64(math.Trunc(v))) + "." + strings.TrimRight(fmt.Sprintf("%.0f", (v-math.Trunc(v))*100), "0")
}

// tokens renders a token count with a K/M/B suffix.
func tokens(v int64) string {
	neg := ""
	if v < 0 {
		neg, v = "-", -v
	}
	switch {
	case v >= 1_000_000_000:
		return neg + trimZero(float64(v)/1e9) + "B"
	case v >= 1_000_000:
		return neg + trimZero(float64(v)/1e6) + "M"
	case v >= 1_000:
		return neg + trimZero(float64(v)/1e3) + "K"
	default:
		return neg + count(v)
	}
}

// count renders an integer with thousands separators.
func count(v int64) string {
	if v < 0 {
		return "-" + count(-v)
	}
	digits := fmt.Sprintf("%d", v)
	var out strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(r)
	}
	return out.String()
}

// humanDuration renders a duration compactly: "45m", "3h 12m", "46d".
func humanDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d >= 48*time.Hour:
		return fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
	case d >= time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	case d >= time.Minute:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
}

// trimZero renders one decimal place and drops a trailing ".0".
func trimZero(v float64) string {
	s := fmt.Sprintf("%.1f", v)
	return strings.TrimSuffix(s, ".0")
}
