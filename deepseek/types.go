package deepseek

import (
	"github.com/acevedomiguel/burnblocks/internal/deepseek/config"
	"github.com/acevedomiguel/burnblocks/internal/deepseek/domain"
	"github.com/acevedomiguel/burnblocks/internal/deepseek/projection"
	"github.com/acevedomiguel/burnblocks/internal/deepseek/wire"
)

// Provider-neutral vocabulary. These aliases keep the public surface stable
// while the implementation stays behind internal packages.
type (
	// BalanceSource reads DeepSeek account balances.
	BalanceSource = domain.BalanceSource
	// BalanceReading is one successful provider reading.
	BalanceReading = domain.BalanceReading
	// Balance is one currency's balances. A nil amount means the provider did
	// not report a usable value, never that the value is zero.
	Balance = domain.Balance
	// Snapshot is one decoded reading and its derived forecast.
	Snapshot = domain.Snapshot
	// Availability is DeepSeek's readiness flag for calling models.
	Availability = domain.Availability
	// Currency is the unit of one balance.
	Currency = domain.Currency
	// RunwayEstimate is the forecast section.
	RunwayEstimate = domain.RunwayEstimate
	// RunwayStatus is the forecast state.
	RunwayStatus = domain.RunwayStatus
	// RunwayReason explains an unavailable forecast.
	RunwayReason = domain.RunwayReason
	// Warning is a stable section and a message that contains no secrets.
	Warning = domain.Warning
	// Paths is the resolved on-disk location the engine reads and writes.
	Paths = config.Paths
	// APIError safely classifies a failed provider read without containing
	// response bodies, header values, arbitrary URLs, or credentials.
	APIError = wire.APIError
)

const (
	// DefaultBaseURL is the production DeepSeek API base.
	DefaultBaseURL = wire.DefaultBaseURL
	// SchemaVersion is the DeepSeek public JSON schema version. It is versioned
	// independently of every other provider.
	SchemaVersion = projection.PublicSchemaVersion
	// SnapshotSchemaVersion versions the internal snapshot representation.
	SnapshotSchemaVersion = domain.SnapshotSchemaVersion

	// AvailabilityUnknown means the provider did not report the readiness flag.
	AvailabilityUnknown = domain.AvailabilityUnknown
	// AvailabilityAvailable means the account can currently make API calls.
	AvailabilityAvailable = domain.AvailabilityAvailable
	// AvailabilityUnavailable means the account cannot currently make API calls.
	AvailabilityUnavailable = domain.AvailabilityUnavailable

	// CurrencyCNY is the Chinese yuan.
	CurrencyCNY = domain.CurrencyCNY
	// CurrencyUSD is the United States dollar.
	CurrencyUSD = domain.CurrencyUSD

	// RunwayUnavailable means no forecast can be derived.
	RunwayUnavailable = domain.RunwayUnavailable
	// ReasonNoUsageHistory means DeepSeek publishes no spend history.
	ReasonNoUsageHistory = domain.ReasonNoUsageHistory
)

// Sentinel errors let callers classify failures with errors.Is.
var (
	// ErrUnsupportedBaseURL reports a base that is not HTTPS or loopback.
	ErrUnsupportedBaseURL = wire.ErrUnsupportedBaseURL
	// ErrMissingKey reports construction without a caller-supplied API key.
	ErrMissingKey = wire.ErrMissingKey
	// ErrUnauthorized reports an invalid or expired key.
	ErrUnauthorized = wire.ErrUnauthorized
	// ErrForbidden reports insufficient key permission.
	ErrForbidden = wire.ErrForbidden
	// ErrInsufficientBalance reports a 402: the account cannot pay for a call.
	ErrInsufficientBalance = wire.ErrInsufficientBalance
	// ErrNotFound reports an unavailable or changed balance endpoint.
	ErrNotFound = wire.ErrNotFound
	// ErrRateLimited reports a provider rate-limit response.
	ErrRateLimited = wire.ErrRateLimited
	// ErrServer reports a DeepSeek service failure.
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

// DefaultPaths resolves the platform configuration directory.
func DefaultPaths() (Paths, error) { return config.DefaultPaths() }
