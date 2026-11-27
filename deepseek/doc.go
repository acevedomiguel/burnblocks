// Package deepseek provides read-only access to a DeepSeek account balance.
//
// The package is a library, not an application. Callers supply the credential,
// HTTP client, clock, and base URL explicitly; the package never reads the
// environment, the keyring, configuration, or a cache implicitly, and it never
// writes to stdout or stderr. Credential and path access is available through
// the explicit helpers in this package; the caller decides when to consult them.
//
// It is read-only by contract. It calls only GET /user/balance. DeepSeek has no
// read-only keys, so the same key can also call models; Burn Blocks never does,
// and the package never creates, rotates, or revokes a key.
//
// A balance is denominated in one currency. DeepSeek amounts are never summed
// with each other across currencies, and never with another provider's dollars.
package deepseek
