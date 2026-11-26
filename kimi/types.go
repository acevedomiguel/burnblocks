package kimi

import (
	"github.com/acevedomiguel/burnblocks/internal/kimi/config"
	"github.com/acevedomiguel/burnblocks/internal/kimi/domain"
	"github.com/acevedomiguel/burnblocks/internal/kimi/wire"
)

// UsageSource reads provider-neutral usage data.
type UsageSource = domain.UsageSource

// UsageReading is one successful provider reading.
type UsageReading = domain.UsageReading

// QuotaKey names one canonical quota window.
type QuotaKey = domain.QuotaKey

// QuotaReading contains normalized values for one quota. Nil pointers mean the
// provider did not report a value, not that the value is zero.
type QuotaReading = domain.QuotaReading

// QuotaSource identifies the representation used to normalize a quota.
type QuotaSource = domain.QuotaSource

// ConflictReason identifies irreconcilable provider representations.
type ConflictReason = domain.ConflictReason

// Snapshot is one decoded reading and its derived runway estimates.
type Snapshot = domain.Snapshot

// RunwayEstimate describes a local, reset-bounded forecast.
type RunwayEstimate = domain.RunwayEstimate

// Sample is one observation in a quota's current reset period.
type Sample = domain.Sample

// StoredState contains caller-owned snapshots and runway history. It may hold
// private raw account data and is never persisted implicitly by this package.
type StoredState = domain.StoredState

// Warning is a stable section and a redacted human-readable message.
type Warning = domain.Warning

// Config is Kimiburn's non-secret configuration vocabulary.
type Config = config.Config

// APIError safely classifies a failed provider read without containing response
// bodies, header values, arbitrary URLs, or credentials.
type APIError = wire.APIError

const (
	// DefaultBaseURL is the primary Kimi Code API base.
	DefaultBaseURL = config.DefaultBaseURL
	// OverseasBaseURL is the explicitly selected overseas Kimi Code API base.
	OverseasBaseURL = config.OverseasBaseURL
	// MinRefreshInterval is the minimum positive automatic refresh interval.
	MinRefreshInterval = config.MinRefreshInterval
	// SnapshotSchemaVersion versions the cached snapshot representation.
	SnapshotSchemaVersion = domain.SnapshotSchemaVersion
	// CacheSchemaVersion versions the caller-owned StoredState envelope.
	CacheSchemaVersion = domain.CacheSchemaVersion

	// QuotaLimit5h is the rolling five-hour quota.
	QuotaLimit5h = domain.QuotaLimit5h
	// QuotaLimit7d is the legacy rolling seven-day quota.
	QuotaLimit7d = domain.QuotaLimit7d
	// QuotaLimitMonthTotal is the shared monthly membership quota.
	QuotaLimitMonthTotal = domain.QuotaLimitMonthTotal
	// QuotaLimitMonthCode is the Code allocation inside the shared monthly pool.
	QuotaLimitMonthCode = domain.QuotaLimitMonthCode

	// SourceRatio marks values decoded from a reported usage ratio.
	SourceRatio = domain.SourceRatio
	// SourceLegacyCounts marks values decoded from legacy absolute counts.
	SourceLegacyCounts = domain.SourceLegacyCounts
	// ConflictSources marks irreconcilable ratio and legacy representations.
	ConflictSources = domain.ConflictSources
)

var (
	// ErrUnsupportedBaseURL rejects unsupported provider hosts and malformed bases.
	ErrUnsupportedBaseURL = wire.ErrUnsupportedBaseURL
	// ErrMissingToken rejects construction without a caller-supplied token.
	ErrMissingToken = wire.ErrMissingToken
	// ErrUnauthorized reports an invalid or expired token.
	ErrUnauthorized = wire.ErrUnauthorized
	// ErrForbidden reports insufficient token permission.
	ErrForbidden = wire.ErrForbidden
	// ErrNotFound reports an unavailable or changed usage endpoint.
	ErrNotFound = wire.ErrNotFound
	// ErrRateLimited reports a provider rate-limit response.
	ErrRateLimited = wire.ErrRateLimited
	// ErrServer reports a Kimi service failure.
	ErrServer = wire.ErrServer
	// ErrUnexpectedStatus reports an otherwise unclassified non-2xx response.
	ErrUnexpectedStatus = wire.ErrUnexpectedStatus
	// ErrRedirect reports a refused redirect.
	ErrRedirect = wire.ErrRedirect
	// ErrTransport reports a transport or per-attempt timeout failure.
	ErrTransport = wire.ErrTransport
	// ErrMalformedResponse reports a response that is not one JSON object.
	ErrMalformedResponse = wire.ErrMalformedResponse
	// ErrResponseTooLarge reports a response above the client size limit.
	ErrResponseTooLarge = wire.ErrResponseTooLarge
)

// DefaultConfig returns the built-in non-secret configuration.
func DefaultConfig() Config { return config.Defaults() }

// EmptyStoredState returns usable caller-owned cold-start state.
func EmptyStoredState() StoredState { return domain.EmptyStoredState() }
