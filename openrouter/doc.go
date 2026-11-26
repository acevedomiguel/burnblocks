// Package openrouter provides read-only access to OpenRouter account credits,
// regular-key limits, spend analytics, and estimated credit runway.
//
// The package is a library, not an application. Callers supply the credentials,
// HTTP client, clock, and previous snapshot explicitly; the package never reads
// the environment, the keyring, or a cache implicitly, and it never writes to
// stdout or stderr.
//
// It is read-only by contract. It calls GET /credits, GET /key,
// GET /analytics/meta, and the read-only POST /analytics/query. It never sends
// inference requests, purchases credit, or creates, rotates, or revokes a key.
//
// The behavior currently implemented inside RouteBurn moves here. RouteBurn,
// DashBurn, and the burnblocks command consume this package directly.
package openrouter
