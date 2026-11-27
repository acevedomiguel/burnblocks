// Package wire is Burn Blocks' bounded HTTP client for DeepSeek's read-only
// account balance endpoint. It calls exactly one endpoint, refuses redirects so
// the bearer is never forwarded, and maps failures to safe typed errors that
// never carry a response body or credential.
package wire

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/acevedomiguel/burnblocks/internal/deepseek/domain"
)

const (
	// DefaultBaseURL is the only production endpoint. Insecure HTTP is rejected
	// unless the host is loopback, which tests rely on.
	DefaultBaseURL = "https://api.deepseek.com"

	// endpointPath is the only path this client ever requests.
	endpointPath = "/user/balance"

	// defaultTimeout bounds one HTTP attempt.
	defaultTimeout = 15 * time.Second
	// maxResponseBytes is the largest accepted body, before the extra byte used
	// to detect an oversized body.
	maxResponseBytes = 4 << 20
	// maxRetries is the number of retries after the initial attempt.
	maxRetries = 2
	// backoffBase is the first exponential backoff step.
	backoffBase = 250 * time.Millisecond
	// backoffCap bounds one computed backoff step.
	backoffCap = 4 * time.Second
	// drainLimit bounds how much of a failed body is read for connection reuse.
	drainLimit = 4096
)

// Client reads DeepSeek balances from one selected base. It is safe for
// concurrent use.
type Client struct {
	baseURL    string
	apiKey     string
	version    string
	httpClient *http.Client
	timeout    time.Duration
	sleep      func(context.Context, time.Duration) error
	jitter     func() float64
	now        func() time.Time

	mu            sync.Mutex
	nextRefreshAt time.Time
}

var _ domain.BalanceSource = (*Client)(nil)

// Option configures a Client. Every seam that touches time or the network is
// injectable so tests stay deterministic.
type Option func(*Client)

// WithBaseURL selects the base URL. Production accepts HTTPS; a loopback URL is
// accepted solely for tests.
func WithBaseURL(base string) Option { return func(c *Client) { c.baseURL = base } }

// WithAPIKey sets the bearer key. It is never logged or echoed.
func WithAPIKey(key string) Option { return func(c *Client) { c.apiKey = key } }

// WithVersion sets the version reported in the User-Agent header.
func WithVersion(version string) Option { return func(c *Client) { c.version = version } }

// WithHTTPClient replaces the HTTP client. The client refuses redirects
// regardless of the supplied value, so the bearer is never forwarded.
func WithHTTPClient(client *http.Client) Option { return func(c *Client) { c.httpClient = client } }

// WithTimeout bounds one HTTP attempt.
func WithTimeout(timeout time.Duration) Option { return func(c *Client) { c.timeout = timeout } }

// WithSleep replaces the retry delay function. It receives the context so a
// cancelled read never sleeps.
func WithSleep(sleep func(context.Context, time.Duration) error) Option {
	return func(c *Client) { c.sleep = sleep }
}

// WithJitter replaces the backoff jitter source, which returns a fraction in
// [0,1].
func WithJitter(jitter func() float64) Option { return func(c *Client) { c.jitter = jitter } }

// WithClock replaces the clock used for observation and retry timestamps.
func WithClock(now func() time.Time) Option { return func(c *Client) { c.now = now } }

// NewClient builds a bounded client. It rejects an empty API key and any base
// URL that is neither HTTPS nor an injected loopback test base.
func NewClient(opts ...Option) (*Client, error) {
	c := &Client{
		baseURL: DefaultBaseURL,
		version: "dev",
		timeout: defaultTimeout,
		sleep:   sleepContext,
		jitter:  rand.Float64,
		now:     time.Now,
	}
	for _, opt := range opts {
		opt(c)
	}

	c.apiKey = strings.TrimSpace(c.apiKey)
	if c.apiKey == "" {
		return nil, ErrMissingKey
	}

	base, err := normalizeBaseURL(c.baseURL)
	if err != nil {
		return nil, err
	}
	c.baseURL = base

	client := c.httpClient
	if client == nil {
		client = &http.Client{}
	}
	// Copy so the caller's client is never mutated, and refuse redirects so a
	// bearer header can never be forwarded to another host.
	refusing := *client
	refusing.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	c.httpClient = &refusing
	return c, nil
}

// normalizeBaseURL validates the selected base. HTTPS is required; an insecure
// HTTP base is accepted only when the host is loopback, which tests inject. The
// error never echoes the supplied value, which may contain a secret-like query.
func normalizeBaseURL(base string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", ErrUnsupportedBaseURL
	}
	if u.Scheme == "https" {
		return strings.TrimRight(base, "/"), nil
	}
	if u.Scheme == "http" && isLoopback(u.Hostname()) {
		return strings.TrimRight(base, "/"), nil
	}
	return "", ErrUnsupportedBaseURL
}

// isLoopback reports whether host is localhost or a loopback IP literal. Only
// tests inject such a base.
func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// balanceURL is the single endpoint this client requests.
func (c *Client) balanceURL() string { return c.baseURL + endpointPath }

// NextRefreshAllowedAt reports the earliest instant another read may run. It is
// safe to call concurrently and returns the zero time when no constraint has
// been observed.
func (c *Client) NextRefreshAllowedAt() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.nextRefreshAt
}

// ReadBalance performs one bounded read of the balance endpoint. It retries at
// most twice after the initial attempt for transport failures and
// 429/502/503/504. Authentication failures and 402 insufficient balance are
// never retried.
func (c *Client) ReadBalance(ctx context.Context) (domain.BalanceReading, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return domain.BalanceReading{}, err
		}

		reading, header, err := c.attempt(ctx)
		now := c.now()
		if header != nil {
			c.recordNextRefresh(header, now)
		}
		if err == nil {
			return reading, nil
		}
		lastErr = err

		if ctxErr := ctx.Err(); ctxErr != nil {
			return domain.BalanceReading{}, ctxErr
		}
		var apiErr *APIError
		if !errors.As(err, &apiErr) || !retryable(apiErr) || attempt == maxRetries {
			return domain.BalanceReading{}, err
		}

		delay := c.backoff(attempt)
		if header != nil {
			if d, ok := retryAfterDelay(header.Get("Retry-After"), now); ok {
				delay = d
			}
		}
		if sleepErr := c.sleep(ctx, delay); sleepErr != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return domain.BalanceReading{}, ctxErr
			}
			return domain.BalanceReading{}, sleepErr
		}
	}
	return domain.BalanceReading{}, lastErr
}

// attempt performs exactly one HTTP request. On failure it returns the response
// headers so the retry loop can honour rate-limit constraints.
func (c *Client) attempt(ctx context.Context) (domain.BalanceReading, http.Header, error) {
	attemptCtx := ctx
	if c.timeout > 0 {
		var cancel context.CancelFunc
		attemptCtx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, c.balanceURL(), nil)
	if err != nil {
		return domain.BalanceReading{}, nil, &APIError{Endpoint: endpointPath, Kind: ErrTransport}
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "BurnBlocks/"+c.version)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return domain.BalanceReading{}, nil, ctxErr
		}
		return domain.BalanceReading{}, nil, &APIError{Endpoint: endpointPath, Kind: ErrTransport}
	}
	defer resp.Body.Close()
	header := resp.Header.Clone()

	switch {
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		drain(resp.Body)
		return domain.BalanceReading{}, header, &APIError{Status: resp.StatusCode, Endpoint: endpointPath, Kind: ErrRedirect}
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		drain(resp.Body)
		return domain.BalanceReading{}, header, &APIError{Status: resp.StatusCode, Endpoint: endpointPath, Kind: kindForStatus(resp.StatusCode)}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return domain.BalanceReading{}, header, ctxErr
		}
		return domain.BalanceReading{}, header, &APIError{Endpoint: endpointPath, Kind: ErrTransport}
	}
	if len(body) > maxResponseBytes {
		return domain.BalanceReading{}, header, &APIError{Endpoint: endpointPath, Kind: ErrResponseTooLarge}
	}
	raw, err := canonicalObject(body)
	if err != nil {
		return domain.BalanceReading{}, header, &APIError{Endpoint: endpointPath, Kind: ErrMalformedResponse}
	}
	reading, err := decodeBalance(raw, c.now())
	if err != nil {
		return domain.BalanceReading{}, header, &APIError{Endpoint: endpointPath, Kind: ErrMalformedResponse}
	}
	return reading, header, nil
}

// canonicalObject requires exactly one top-level JSON object and returns its
// bytes without surrounding whitespace. Trailing non-whitespace is rejected.
func canonicalObject(body []byte) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, ErrMalformedResponse
	}
	var raw json.RawMessage
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return nil, ErrMalformedResponse
	}
	return raw, nil
}

// drain reads a bounded amount of a discarded body so the connection can be
// reused. Its content is never inspected or reported.
func drain(body io.Reader) {
	_, _ = io.CopyN(io.Discard, body, drainLimit)
}

// retryable reports whether a failure is worth another attempt. 500 and 501 are
// deliberately excluded: only 429/502/503/504 are transient. A 402 insufficient
// balance is never retried.
func retryable(err *APIError) bool {
	switch {
	case errors.Is(err.Kind, ErrTransport):
		return true
	case err.Status == http.StatusTooManyRequests,
		err.Status == http.StatusBadGateway,
		err.Status == http.StatusServiceUnavailable,
		err.Status == http.StatusGatewayTimeout:
		return true
	}
	return false
}

// backoff returns the jittered delay for a retry attempt: the exponential step
// halved, plus up to the other half from the jitter source.
func (c *Client) backoff(attempt int) time.Duration {
	d := backoffBase
	for i := 0; i < attempt && d < backoffCap; i++ {
		d *= 2
	}
	if d > backoffCap {
		d = backoffCap
	}
	half := d / 2
	jitter := 0.0
	if c.jitter != nil {
		jitter = c.jitter()
	}
	if jitter < 0 {
		jitter = 0
	}
	if jitter > 1 {
		jitter = 1
	}
	return half + time.Duration(float64(half)*jitter)
}

// recordNextRefresh keeps the latest constraint observed. A later response can
// never move the allowed instant earlier.
func (c *Client) recordNextRefresh(header http.Header, now time.Time) {
	if v := header.Get("X-RateLimit-Reset"); v != "" {
		if at, ok := parseRateLimitReset(v, now); ok {
			c.noteNextRefresh(at)
		}
	}
	if v := header.Get("Retry-After"); v != "" {
		if d, ok := retryAfterDelay(v, now); ok {
			c.noteNextRefresh(now.Add(d))
		}
	}
}

func (c *Client) noteNextRefresh(at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if at.After(c.nextRefreshAt) {
		c.nextRefreshAt = at
	}
}

// parseRateLimitReset accepts unix seconds or milliseconds and ignores values
// that are not strictly in the future. No other format is invented.
func parseRateLimitReset(value string, now time.Time) (time.Time, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || n <= 0 {
		return time.Time{}, false
	}
	var at time.Time
	if n > 1e12 {
		at = time.UnixMilli(n).UTC()
	} else {
		at = time.Unix(n, 0).UTC()
	}
	if !at.After(now) {
		return time.Time{}, false
	}
	return at, true
}

// retryAfterDelay accepts integer seconds or an HTTP date. A date in the past
// means "retry now" and yields a zero delay. Seconds are parsed as int64 and
// saturate at the largest representable duration so an absurd value can never
// wrap negative and retry immediately.
func retryAfterDelay(value string, now time.Time) (time.Duration, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, false
	}
	seconds, err := strconv.ParseInt(trimmed, 10, 64)
	switch {
	case err == nil:
		if seconds < 0 {
			return 0, false
		}
		if seconds > math.MaxInt64/int64(time.Second) {
			return time.Duration(math.MaxInt64), true
		}
		return time.Duration(seconds) * time.Second, true
	case errors.Is(err, strconv.ErrRange) && !strings.HasPrefix(trimmed, "-"):
		// Positive digits beyond int64 are still an enormous delay, so they
		// saturate instead of becoming an invalid header that retries now.
		return time.Duration(math.MaxInt64), true
	}
	if at, err := http.ParseTime(trimmed); err == nil {
		d := at.Sub(now)
		if d < 0 {
			d = 0
		}
		return d, true
	}
	return 0, false
}

// sleepContext waits for d or until ctx is done, whichever comes first.
func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
