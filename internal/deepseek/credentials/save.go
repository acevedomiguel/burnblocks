package credentials

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Save persists the API key to the keyring, falling back to a 0600 file only
// when the keyring is disabled or unavailable.
//
// Resolution prefers the keyring over the file, so a backend transition must
// never leave two different keys readable: a successful keyring save clears the
// old file, and a file fallback is refused while an older keyring entry still
// survives.
func (s *Store) Save(ctx context.Context, key string) error {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return ErrEmptySecret
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if s.keyringEnabled {
		if err := s.keyring.Set(ServiceName, AccountName, trimmed); err == nil {
			// The keyring now wins resolution, so a leftover fallback file would
			// resurrect the previous key if the keyring later went away.
			return s.removeStaleFile()
		}
		// The keyring could not take the key. Falling back to the file is safe
		// only when no older keyring entry survives, since that entry would keep
		// winning resolution over the file we are about to write.
		if s.keyringHoldsKey() {
			if err := s.keyring.Delete(ServiceName, AccountName); err != nil && !errors.Is(err, ErrKeyringNotFound) {
				return fmt.Errorf("credentials: the keyring still holds a previous key: %w", err)
			}
		}
	} else if s.keyring != nil {
		// The keyring is disabled by configuration, yet an entry from an earlier
		// run would still outrank the file. Clearing it is best effort: the file
		// save must not depend on keyring availability.
		_ = s.keyring.Delete(ServiceName, AccountName)
	}

	if s.apiKeyFile == "" {
		return ErrNoKeyFile
	}
	return writeSecretFile(s.apiKeyFile, trimmed)
}

// keyringHoldsKey reports whether the keyring currently stores a DeepSeek key.
// A keyring that cannot be read counts as holding none, which keeps the
// documented file fallback available when the keyring is simply unreachable.
func (s *Store) keyringHoldsKey() bool {
	if s.keyring == nil {
		return false
	}
	v, err := s.keyring.Get(ServiceName, AccountName)
	return err == nil && strings.TrimSpace(v) != ""
}

// removeStaleFile deletes a fallback file that a keyring save has superseded.
func (s *Store) removeStaleFile() error {
	if s.apiKeyFile == "" {
		return nil
	}
	if err := os.Remove(s.apiKeyFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("credentials: remove superseded key file %s: %w", s.apiKeyFile, err)
	}
	return nil
}

// Logout removes only Burn Blocks-owned DeepSeek credentials: the keyring entry
// and the fallback key file. It always attempts the keyring entry, even when the
// keyring is disabled, so flipping that setting cannot strand a stored key. It
// never revokes the key at the provider.
func (s *Store) Logout(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var failures []error
	if s.keyring != nil {
		err := s.keyring.Delete(ServiceName, AccountName)
		if err != nil && !errors.Is(err, ErrKeyringNotFound) && !errors.Is(err, ErrKeyringUnavailable) {
			failures = append(failures, fmt.Errorf("credentials: remove keyring entry: %w", err))
		}
	}
	if s.apiKeyFile != "" {
		if err := os.Remove(s.apiKeyFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, fmt.Errorf("credentials: remove %s: %w", s.apiKeyFile, err))
		}
	}
	return errors.Join(failures...)
}

// writeSecretFile creates the parent directory at 0700 and writes the secret to
// a fresh 0600 file. An existing file is replaced so a looser mode cannot
// survive.
func writeSecretFile(path, secret string) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("credentials: create directory: %w", err)
		}
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("credentials: replace %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(secret), 0o600); err != nil {
		return fmt.Errorf("credentials: write %s: %w", path, err)
	}
	return nil
}
