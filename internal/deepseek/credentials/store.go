// Package credentials resolves and stores Burn Blocks' single DeepSeek API key,
// and redacts secrets from anything a user can see. It is read-only with respect
// to DeepSeek: it never creates, rotates, revokes, or transmits a key itself.
package credentials

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Burn Blocks owns exactly one DeepSeek credential: the API key. It is stored in
// the OS keyring under ServiceName/AccountName or, as a fallback, in a
// permission-restricted file. The key is not read-only at the provider; Burn
// Blocks only ever reads with it.
const (
	// ServiceName is the keyring service namespace owned by Burn Blocks.
	ServiceName = "deepseek"
	// AccountName is the keyring entry under that service.
	AccountName = "api-key"
	// EnvAPIKey is the environment variable that always wins during resolution.
	EnvAPIKey = "DEEPSEEK_API_KEY"
)

// Sentinel errors surfaced by the credential store.
var (
	ErrKeyringUnavailable  = errors.New("credentials: the OS keyring is unavailable")
	ErrKeyringNotFound     = errors.New("credentials: the keyring holds no DeepSeek entry")
	ErrInsecurePermissions = errors.New("credentials: the API key file must be readable only by its owner (0600)")
	ErrEmptySecret         = errors.New("credentials: refusing to store an empty API key")
	ErrNoKeyFile           = errors.New("credentials: no API key file path is configured")
)

// Keyring is the subset of the go-keyring API that the store uses.
type Keyring interface {
	Get(service, user string) (string, error)
	Set(service, user, secret string) error
	Delete(service, user string) error
}

// Options configures a Store. Tests inject a fake keyring and a temp path.
type Options struct {
	Keyring        Keyring
	APIKeyFile     string
	KeyringEnabled bool
	Env            map[string]string
}

// Store resolves and persists the DeepSeek API key. Only Burn Blocks-owned
// surfaces are consulted: DEEPSEEK_API_KEY, the keyring entry
// deepseek/api-key, and the restricted fallback file.
type Store struct {
	keyring        Keyring
	apiKeyFile     string
	keyringEnabled bool

	// Env overrides the process environment; nil means os.Getenv.
	Env map[string]string
}

// NewStore builds a store from explicit options.
func NewStore(opts Options) *Store {
	return &Store{
		keyring:        opts.Keyring,
		apiKeyFile:     opts.APIKeyFile,
		keyringEnabled: opts.KeyringEnabled && opts.Keyring != nil,
		Env:            opts.Env,
	}
}

// Resolve returns the API key the process should use right now, or "" when none
// is configured. Resolution order is environment, keyring, fallback file.
func (s *Store) Resolve(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	// The environment always wins and is never persisted.
	if key := s.lookupEnv(EnvAPIKey); key != "" {
		return key, nil
	}

	if s.keyringEnabled {
		if v, err := s.keyring.Get(ServiceName, AccountName); err == nil {
			if key := strings.TrimSpace(v); key != "" {
				return key, nil
			}
		}
	}

	return readSecretFile(s.apiKeyFile)
}

// EnvironmentKey returns the trimmed DEEPSEEK_API_KEY value, or "". It lets a UI
// explain that the environment wins without persisting it.
func (s *Store) EnvironmentKey() string { return s.lookupEnv(EnvAPIKey) }

func (s *Store) lookupEnv(key string) string {
	if s.Env != nil {
		return strings.TrimSpace(s.Env[key])
	}
	return strings.TrimSpace(os.Getenv(key))
}

// readSecretFile enforces 0600 (no group or world access) before reading.
func readSecretFile(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("credentials: inspect %s: %w", path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("credentials: %s is a directory", path)
	}
	// Windows reports 0666 for every file, so the group/world check would reject
	// the fallback unconditionally. There the file's protection comes from the
	// directory ACL that writeSecretFile sets to 0700.
	if PermissionsAreEnforced {
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			return "", ErrInsecurePermissions
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("credentials: read %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}
