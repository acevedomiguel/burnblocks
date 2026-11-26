package kimi

import (
	"time"

	"github.com/acevedomiguel/burnblocks/internal/kimi/cache"
	"github.com/acevedomiguel/burnblocks/internal/kimi/config"
	"github.com/acevedomiguel/burnblocks/internal/kimi/credentials"
)

// TokenStore resolves and persists the Kimi Code token from Burn Blocks-owned
// surfaces only: KIMIBURN_TOKEN, the OS keyring entry kimiburn/token, and a
// permission-restricted fallback file. It is never consulted implicitly; the
// caller decides when to resolve or save.
type TokenStore = credentials.Store

// TokenStoreOptions configures a TokenStore. Production callers pass the token
// file from Paths; tests inject a fake keyring and a temporary path.
type TokenStoreOptions = credentials.Options

// Keyring is the subset of the OS keyring API a TokenStore uses.
type Keyring = credentials.Keyring

// TokenEnvVar is the environment variable that always wins during resolution.
const TokenEnvVar = credentials.EnvToken

// KeyringService and KeyringAccount name the Kimi Code credential entry.
const (
	KeyringService = credentials.ServiceName
	KeyringAccount = credentials.AccountName
)

// Credential sentinel errors.
var (
	ErrKeyringUnavailable  = credentials.ErrKeyringUnavailable
	ErrKeyringNotFound     = credentials.ErrKeyringNotFound
	ErrInsecurePermissions = credentials.ErrInsecurePermissions
	ErrEmptyToken          = credentials.ErrEmptyToken
	ErrNoTokenFile         = credentials.ErrNoTokenFile
)

// Redactor removes known secrets from text before it reaches a user-visible
// surface.
type Redactor = credentials.Redactor

// NewTokenStore builds a token store from explicit options.
func NewTokenStore(opts TokenStoreOptions) *TokenStore { return credentials.NewStore(opts) }

// NewOSKeyring opens the platform OS keyring.
func NewOSKeyring() Keyring { return credentials.NewOSKeyring() }

// NewRedactor builds a redactor that removes the supplied secrets.
func NewRedactor(secrets ...string) *Redactor { return credentials.NewRedactor(secrets...) }

// Redact removes generic secret shapes from s.
func Redact(s string) string { return credentials.Redact(s) }

// StateStore is the caller-owned cache boundary for StoredState.
type StateStore = cache.StateStore

// Cache sentinel errors.
var (
	ErrCacheMalformed          = cache.ErrMalformed
	ErrCacheUnsupportedVersion = cache.ErrUnsupportedVersion
)

// NewStateStore returns a file-backed StateStore rooted at path.
func NewStateStore(path string) StateStore { return cache.NewStore(path) }

// NewDisabledStateStore returns a StateStore that performs no I/O.
func NewDisabledStateStore() StateStore { return cache.NewDisabledStore() }

// Overrides are explicit configuration overrides applied after the file and
// environment.
type Overrides = config.Overrides

// Paths is the resolved configuration and cache layout.
type Paths = config.Paths

// DefaultPaths resolves the platform configuration and cache directories.
func DefaultPaths() (Paths, error) { return config.DefaultPaths() }

// LoadConfig resolves non-secret configuration from the given files, an
// explicit environment map, and optional overrides, in that order.
func LoadConfig(paths []string, env map[string]string, flags ...Overrides) (Config, error) {
	return config.Load(paths, env, flags...)
}

// IsAllowedBaseURL reports whether base is one of the documented Kimi Code hosts.
func IsAllowedBaseURL(base string) bool { return config.IsAllowedBaseURL(base) }

// FormatInterval renders a refresh interval in a parseable spelling.
func FormatInterval(d time.Duration) string { return config.FormatInterval(d) }

// SaveRefreshInterval persists an interval to the configuration file without
// touching other settings.
func SaveRefreshInterval(path string, interval time.Duration) error {
	return config.SaveRefreshInterval(path, interval)
}
