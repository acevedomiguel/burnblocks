// Package kimi provides read-only, headless access to Kimi Code membership
// quotas and Burn Blocks' local runway estimates.
//
// Callers supply credentials, HTTP dependencies, output writers, and optional
// in-memory state. The package never discovers environment credentials, opens a
// keyring or cache, or writes to process output streams. Credential, config, and
// cache access is available through the explicit helpers in this package; the
// caller decides when to consult them.
//
// It is read-only by contract. The only supported call is GET /usages on the
// explicitly selected Kimi Code host. Raw provider data can contain private
// account information and must be kept out of logs and fixtures.
package kimi
