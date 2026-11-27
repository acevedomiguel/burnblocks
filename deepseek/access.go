package deepseek

import (
	"context"

	"github.com/acevedomiguel/burnblocks/internal/deepseek/credentials"
)

// Credential vocabulary. Burn Blocks owns exactly one DeepSeek secret: the API
// key. They are resolved from Burn Blocks-owned surfaces only.
type (
	// CredentialStore resolves and persists the DeepSeek API key.
	CredentialStore = credentials.Store
	// CredentialOptions configures a CredentialStore.
	CredentialOptions = credentials.Options
	// Keyring is the subset of the OS keyring API a CredentialStore uses.
	Keyring = credentials.Keyring
	// Redactor removes known secrets from text.
	Redactor = credentials.Redactor
)

const (
	// CredentialServiceName is the OS keyring service namespace.
	CredentialServiceName = credentials.ServiceName
	// CredentialAccountName is the keyring entry under that service.
	CredentialAccountName = credentials.AccountName
	// CredentialEnvVar is the environment variable consulted first.
	CredentialEnvVar = credentials.EnvAPIKey
	// PermissionsAreEnforced reports whether the OS exposes POSIX permission
	// bits, so a caller can explain why a loose fallback file is refused.
	PermissionsAreEnforced = credentials.PermissionsAreEnforced
)

// Credential sentinel errors.
var (
	ErrKeyringUnavailable  = credentials.ErrKeyringUnavailable
	ErrKeyringNotFound     = credentials.ErrKeyringNotFound
	ErrInsecurePermissions = credentials.ErrInsecurePermissions
	ErrEmptySecret         = credentials.ErrEmptySecret
	ErrNoKeyFile           = credentials.ErrNoKeyFile
)

// NewCredentialStore builds a credential store from explicit options.
func NewCredentialStore(opts CredentialOptions) *CredentialStore {
	return credentials.NewStore(opts)
}

// NewOSKeyring opens the platform OS keyring.
func NewOSKeyring() Keyring { return credentials.NewOSKeyring() }

// ResolveKey reads the API key from the store. Nothing is read implicitly; the
// caller decides when to resolve.
func ResolveKey(ctx context.Context, store *CredentialStore) (string, error) {
	return store.Resolve(ctx)
}

// NewRedactor builds a redactor that removes the supplied secrets.
func NewRedactor(secrets ...string) *Redactor { return credentials.NewRedactor(secrets...) }

// Redact removes generic secret shapes from s.
func Redact(s string) string { return credentials.Redact(s) }

// RedactAll removes generic secret shapes from every value.
func RedactAll(values []string) []string { return credentials.RedactAll(values) }

// LooksLikeSecret reports whether a value resembles a credential.
func LooksLikeSecret(s string) bool { return credentials.LooksLikeSecret(s) }

// IsBearer reports whether a value carries a bearer credential.
func IsBearer(value string) bool { return credentials.IsBearer(value) }
