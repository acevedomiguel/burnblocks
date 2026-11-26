package openrouter

import (
	"context"
	"time"

	"github.com/acevedomiguel/burnblocks/internal/openrouter/cache"
	"github.com/acevedomiguel/burnblocks/internal/openrouter/config"
	"github.com/acevedomiguel/burnblocks/internal/openrouter/credentials"
)

// Configuration vocabulary. Callers pass an explicit environment map and file
// list; nothing is read implicitly.
type (
	// Config is RouteBurn's non-secret configuration.
	Config = config.Config
	// Overrides are explicit CLI flags, which win over the file and environment.
	Overrides = config.Overrides
	// Paths is the resolved configuration and credential layout.
	Paths = config.Paths
)

const (
	// MinRefreshInterval is the minimum positive automatic refresh interval.
	MinRefreshInterval = config.MinRefreshInterval
	// SupportedRangeDays is the fixed analytics window, in UTC days.
	SupportedRangeDays = config.SupportedRangeDays
)

// DefaultConfig returns the built-in non-secret configuration.
func DefaultConfig() Config { return config.Defaults() }

// LoadConfig resolves configuration from the given files, an explicit
// environment map, and optional overrides, in that order.
func LoadConfig(paths []string, env map[string]string, flags ...Overrides) (Config, error) {
	return config.Load(paths, env, flags...)
}

// DefaultPaths resolves the platform configuration and credential directories.
func DefaultPaths() (Paths, error) { return config.DefaultPaths() }

// FormatInterval renders a refresh interval in a parseable spelling.
func FormatInterval(d time.Duration) string { return config.FormatInterval(d) }

// SaveRefreshInterval persists an interval to the configuration file without
// touching other settings.
func SaveRefreshInterval(path string, interval time.Duration) error {
	return config.SaveRefreshInterval(path, interval)
}

// Credential vocabulary. RouteBurn owns exactly two secrets: the management key
// and the optional regular API key. They are never interchangeable.
type (
	// CredentialStore resolves and persists both credentials from
	// RouteBurn-owned surfaces only.
	CredentialStore = credentials.Store
	// CredentialOptions configures a CredentialStore.
	CredentialOptions = credentials.Options
	// CredentialKind names one credential slot.
	CredentialKind = credentials.Kind
	// Creds is the resolved pair; an empty slot means it is not configured.
	Creds = credentials.Creds
	// Keyring is the subset of the OS keyring API a CredentialStore uses.
	Keyring = credentials.Keyring
	// Redactor removes known secrets from text.
	Redactor = credentials.Redactor
)

const (
	// CredentialManagement is the account-wide management key slot.
	CredentialManagement = credentials.KindManagement
	// CredentialAPIKey is the optional regular API key slot.
	CredentialAPIKey = credentials.KindAPIKey
	// CredentialServiceName names the OS keyring service.
	CredentialServiceName = credentials.ServiceName
)

// Environment variables consulted first, in this order. OPENROUTER_API_KEY is a
// compatibility fallback for the regular key slot only and never satisfies the
// management credential.
const (
	EnvManagementKey = credentials.EnvManagementKey
	EnvAPIKey        = credentials.EnvAPIKey
	EnvOpenRouterKey = credentials.EnvOpenRouterKey
)

// Credential sentinel errors.
var (
	ErrKeyringUnavailable  = credentials.ErrKeyringUnavailable
	ErrInsecurePermissions = credentials.ErrInsecurePermissions
	ErrNoManagementKey     = credentials.ErrNoManagementKey
	ErrEmptySecret         = credentials.ErrEmptySecret
)

// NewCredentialStore builds a credential store from explicit options.
func NewCredentialStore(opts CredentialOptions) *CredentialStore { return credentials.NewStore(opts) }

// NewOSKeyring opens the platform OS keyring.
func NewOSKeyring() Keyring { return credentials.NewOSKeyring() }

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

// Resolve reads both credential slots without touching the supplied context's
// error state until the caller checks the result.
func ResolveCreds(ctx context.Context, store *CredentialStore) (Creds, error) {
	return store.Resolve(ctx)
}

// CacheStore is a file-backed, versioned, aggregate-only snapshot cache. A
// cached runway value is never presented as live.
type CacheStore = cache.Store

// Cache sentinel errors.
var (
	ErrNoSnapshot       = cache.ErrNoSnapshot
	ErrCacheUnsupported = cache.ErrUnsupportedVersion
)

// NewCacheStore returns a cache rooted at path.
func NewCacheStore(path string) *CacheStore { return cache.NewStore(path) }
