package wire

import (
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors classify every failure without exposing provider detail. They
// are safe to print: each carries only a status class and the endpoint path.
var (
	// ErrUnsupportedBaseURL rejects anything but an HTTPS base or an explicitly
	// injected loopback test base.
	ErrUnsupportedBaseURL = errors.New("deepseek: unsupported base URL")
	// ErrMissingKey rejects a client constructed without an API key.
	ErrMissingKey = errors.New("deepseek: an API key is required")
	// ErrUnauthorized maps 401: the key is invalid or expired.
	ErrUnauthorized = errors.New("deepseek: API key is invalid or expired")
	// ErrForbidden maps 403: the key lacks permission for the endpoint.
	ErrForbidden = errors.New("deepseek: permission denied")
	// ErrInsufficientBalance maps 402: the account cannot pay for a call. It is
	// never retried, because retrying cannot add funds.
	ErrInsufficientBalance = errors.New("deepseek: account balance is insufficient")
	// ErrNotFound maps 404: the endpoint is unavailable or changed.
	ErrNotFound = errors.New("deepseek: balance endpoint unavailable or changed")
	// ErrRateLimited maps 429: the server asked for a slower cadence.
	ErrRateLimited = errors.New("deepseek: rate limited")
	// ErrServer maps 5xx: a DeepSeek service error.
	ErrServer = errors.New("deepseek: DeepSeek service error")
	// ErrUnexpectedStatus maps any other non-2xx status.
	ErrUnexpectedStatus = errors.New("deepseek: unexpected response status")
	// ErrRedirect refuses to follow a redirect, so the bearer is never
	// forwarded to another host.
	ErrRedirect = errors.New("deepseek: redirect refused")
	// ErrTransport maps a transport or per-attempt timeout failure.
	ErrTransport = errors.New("deepseek: transport failure")
	// ErrMalformedResponse maps a body that is not one top-level JSON object
	// with only trailing whitespace.
	ErrMalformedResponse = errors.New("deepseek: malformed response")
	// ErrResponseTooLarge maps a body above the 4 MiB cap.
	ErrResponseTooLarge = errors.New("deepseek: response too large")
)

// APIError carries a safe classification for a failed read. It never holds a
// response body, header value, credential, or arbitrary URL.
type APIError struct {
	Status   int
	Endpoint string
	Kind     error
}

// Error formats the status class and endpoint path only. The sentinel already
// carries the package prefix, so the message is not doubled.
func (e *APIError) Error() string {
	if e.Status == 0 {
		return fmt.Sprintf("%s at %s", e.Kind.Error(), e.Endpoint)
	}
	return fmt.Sprintf("%s (%d %s) at %s", e.Kind.Error(), e.Status, http.StatusText(e.Status), e.Endpoint)
}

// Unwrap exposes the sentinel so errors.Is works on every failure.
func (e *APIError) Unwrap() error { return e.Kind }

// kindForStatus maps an HTTP status to its safe sentinel.
func kindForStatus(status int) error {
	switch status {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusPaymentRequired:
		return ErrInsufficientBalance
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusTooManyRequests:
		return ErrRateLimited
	}
	if status >= 500 {
		return ErrServer
	}
	return ErrUnexpectedStatus
}
