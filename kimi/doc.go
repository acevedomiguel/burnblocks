// Package kimi provides read-only access to Kimi Code membership quotas and
// estimated runway.
//
// The package is a library, not an application. Callers supply the token, HTTP
// client, clock, and any in-memory runway state explicitly; the package never
// reads the environment, the keyring, or a cache implicitly, and it never
// writes to stdout or stderr.
//
// It is read-only by contract. The only supported call is GET /usages on the
// explicitly selected Kimi Code host. It never sends inference requests, changes
// quota, purchases anything, or creates, rotates, or revokes a token.
//
// The behavior currently implemented inside KimiBurn moves here. KimiBurn,
// DashBurn, and the burnblocks command consume this package directly.
package kimi
