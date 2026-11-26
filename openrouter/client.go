package openrouter

import (
	"net/http"
	"time"

	"github.com/acevedomiguel/burnblocks/internal/openrouter/wire"
)

// OpenRouter client and transport vocabulary.
type (
	Client         = wire.Client
	Option         = wire.Option
	CreditSummary  = wire.CreditSummary
	KeyInfo        = wire.KeyInfo
	AnalyticsQuery = wire.AnalyticsQuery
	AnalyticsRow   = wire.AnalyticsRow
	APIError       = wire.APIError
)

// DefaultBaseURL is the production OpenRouter API endpoint.
const DefaultBaseURL = wire.DefaultBaseURL

// Sentinel errors allow callers to classify client failures with errors.Is.
var (
	ErrBadRequest           = wire.ErrBadRequest
	ErrUnauthorized         = wire.ErrUnauthorized
	ErrForbidden            = wire.ErrForbidden
	ErrNotFound             = wire.ErrNotFound
	ErrRateLimited          = wire.ErrRateLimited
	ErrServer               = wire.ErrServer
	ErrTransport            = wire.ErrTransport
	ErrMalformedResponse    = wire.ErrMalformedResponse
	ErrResponseTooLarge     = wire.ErrResponseTooLarge
	ErrMissingManagementKey = wire.ErrMissingManagementKey
	ErrMissingAPIKey        = wire.ErrMissingAPIKey
)

// NewClient validates configuration and returns a read-only OpenRouter client.
func NewClient(opts ...Option) (*Client, error) {
	return wire.NewClient(opts...)
}

// WithBaseURL overrides the API base URL. Non-loopback HTTP is refused.
func WithBaseURL(url string) Option {
	return wire.WithBaseURL(url)
}

// WithManagementKey sets the key used for credits and analytics.
func WithManagementKey(key string) Option {
	return wire.WithManagementKey(key)
}

// WithAPIKey sets the regular key used for the key endpoint.
func WithAPIKey(key string) Option {
	return wire.WithAPIKey(key)
}

// WithHTTPClient injects an HTTP client and its transport.
func WithHTTPClient(client *http.Client) Option {
	return wire.WithHTTPClient(client)
}

// WithVersion sets the User-Agent version component.
func WithVersion(version string) Option {
	return wire.WithVersion(version)
}

// WithSleep replaces the backoff sleeper.
func WithSleep(sleep func(time.Duration)) Option {
	return wire.WithSleep(sleep)
}

// WithClock replaces the clock used for rate-limit deadlines.
func WithClock(now func() time.Time) Option {
	return wire.WithClock(now)
}
