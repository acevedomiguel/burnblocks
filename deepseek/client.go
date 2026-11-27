package deepseek

import (
	"net/http"
	"time"

	"github.com/acevedomiguel/burnblocks/internal/deepseek/wire"
)

type sourceOptions struct {
	httpClient *http.Client
	now        func() time.Time
	version    string
	timeout    time.Duration
}

// SourceOption configures a balance source without changing its
// provider-neutral boundary.
type SourceOption func(*sourceOptions)

// WithHTTPClient injects the HTTP client used by a source. The source always
// refuses redirects, including when the supplied client permits them. Nil keeps
// the default client.
func WithHTTPClient(client *http.Client) SourceOption {
	return func(options *sourceOptions) { options.httpClient = client }
}

// WithClock injects the clock used for observation and retry timestamps. Nil
// keeps the system clock.
func WithClock(now func() time.Time) SourceOption {
	return func(options *sourceOptions) { options.now = now }
}

// WithVersion sets the version reported in the client's User-Agent so an
// application can identify its own release. An empty version keeps the module
// default.
func WithVersion(version string) SourceOption {
	return func(options *sourceOptions) { options.version = version }
}

// WithTimeout bounds one HTTP attempt. A non-positive value keeps the documented
// default.
func WithTimeout(timeout time.Duration) SourceOption {
	return func(options *sourceOptions) { options.timeout = timeout }
}

// NewSource constructs a bounded DeepSeek balance source from an explicitly
// selected base and caller-supplied key. An empty base selects DefaultBaseURL.
// It performs no credential discovery and no I/O until ReadBalance is called.
func NewSource(baseURL, apiKey string, opts ...SourceOption) (BalanceSource, error) {
	var options sourceOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	if baseURL == "" {
		baseURL = wire.DefaultBaseURL
	}
	clientOptions := []wire.Option{wire.WithBaseURL(baseURL), wire.WithAPIKey(apiKey)}
	if options.httpClient != nil {
		clientOptions = append(clientOptions, wire.WithHTTPClient(options.httpClient))
	}
	if options.now != nil {
		clientOptions = append(clientOptions, wire.WithClock(options.now))
	}
	if options.version != "" {
		clientOptions = append(clientOptions, wire.WithVersion(options.version))
	}
	if options.timeout > 0 {
		clientOptions = append(clientOptions, wire.WithTimeout(options.timeout))
	}
	return wire.NewClient(clientOptions...)
}
