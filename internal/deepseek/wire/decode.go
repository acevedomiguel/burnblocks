package wire

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/acevedomiguel/burnblocks/internal/deepseek/domain"
)

// balanceEnvelope is the documented response shape. Every field is kept as a
// raw message so a present value can be told apart from an absent, null, or
// differently-typed one.
type balanceEnvelope struct {
	IsAvailable  *bool                        `json:"is_available"`
	BalanceInfos []map[string]json.RawMessage `json:"balance_infos"`
}

// decodeBalance turns one response object into a reading. A malformed amount is
// reported as a warning and left nil; it is never turned into zero. An absent
// amount is simply unavailable. "0.00" is a real zero.
func decodeBalance(raw json.RawMessage, observedAt time.Time) (domain.BalanceReading, error) {
	var env balanceEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return domain.BalanceReading{}, err
	}

	reading := domain.BalanceReading{
		ObservedAt: observedAt.UTC(),
		Available:  availability(env.IsAvailable),
	}

	currencies := make(map[domain.Currency]bool, len(env.BalanceInfos))
	for _, info := range env.BalanceInfos {
		currency, ok := rawString(info["currency"])
		if !ok || currency == "" {
			reading.Warnings = append(reading.Warnings, domain.Warning{
				Section: "Balances",
				Message: "a balance entry without a currency was ignored",
			})
			continue
		}
		// The currency comes from the provider. Only the documented values are
		// kept, so a malformed value is never echoed into a warning or into the
		// public JSON, where it would bypass redaction.
		unit := domain.Currency(currency)
		if unit != domain.CurrencyCNY && unit != domain.CurrencyUSD {
			reading.Warnings = append(reading.Warnings, domain.Warning{
				Section: "Balances",
				Message: "a balance entry with an unrecognised currency was ignored",
			})
			continue
		}
		currencies[unit] = true

		reading.Balances = append(reading.Balances, domain.Balance{
			Currency: unit,
			Total:    parseAmount(info["total_balance"], "total_balance", &reading.Warnings),
			Granted:  parseAmount(info["granted_balance"], "granted_balance", &reading.Warnings),
			ToppedUp: parseAmount(info["topped_up_balance"], "topped_up_balance", &reading.Warnings),
		})
	}

	// Each currency is a different unit. Keep them separate and say so, rather
	// than letting a reader assume the values were combined.
	if len(currencies) > 1 {
		reading.Warnings = append(reading.Warnings, domain.Warning{
			Section: "Balances",
			Message: "multiple currencies were reported; they are never totalled together",
		})
	}

	return reading, nil
}

// availability maps the tri-state readiness flag, keeping an absent flag
// distinct from a reported false.
func availability(flag *bool) domain.Availability {
	switch {
	case flag == nil:
		return domain.AvailabilityUnknown
	case *flag:
		return domain.AvailabilityAvailable
	default:
		return domain.AvailabilityUnavailable
	}
}

// rawString decodes a JSON string, reporting whether the field was a string at
// all. An absent field, null, or another type reports false.
func rawString(raw json.RawMessage) (string, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if len(trimmed) == 0 || trimmed == "null" {
		return "", false
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return "", false
	}
	return text, true
}

// parseAmount turns one provider amount into a value or nil. An absent field,
// null, or malformed value is unavailable and never zero; a malformed value
// additionally records a warning. A well-formed "0.00" is a real zero.
func parseAmount(raw json.RawMessage, label string, warnings *[]domain.Warning) *float64 {
	trimmed := strings.TrimSpace(string(raw))
	if len(trimmed) == 0 || trimmed == "null" {
		return nil
	}

	// The API documents amounts as strings. A JSON number is accepted too so a
	// provider change degrades to a warning rather than a dead endpoint.
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return finitePointer(text, label, warnings)
	}
	var number float64
	if err := json.Unmarshal(raw, &number); err == nil {
		if !isFinite(number) {
			appendMalformed(warnings, label)
			return nil
		}
		return &number
	}
	appendMalformed(warnings, label)
	return nil
}

// finitePointer parses a decimal string, rejecting anything that is not a
// finite number.
func finitePointer(text, label string, warnings *[]domain.Warning) *float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || !isFinite(value) {
		appendMalformed(warnings, label)
		return nil
	}
	return &value
}

func appendMalformed(warnings *[]domain.Warning, label string) {
	*warnings = append(*warnings, domain.Warning{
		Section: "Balances",
		Message: "a malformed " + label + " value was ignored",
	})
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
