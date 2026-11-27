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
// when the keyring is disabled, or when it cannot take the write but proves it
// holds no older key.
//
// Resolution prefers the keyring over the file, so a backend transition must
// never leave two different keys readable: a successful keyring save clears the
// old file, and a file fallback is refused while an older keyring entry could
// still survive. A stale entry would keep winning resolution over the file once
// the keyring recovered, silently ignoring the new key.
func (s *Store) Save(ctx context.Context, key string) error {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return ErrEmptySecret
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if !s.keyringEnabled {
		// The keyring is disabled by configuration, yet an entry from an earlier
		// run would still outrank the file. Clearing it is best effort: the file
		// save must not depend on keyring availability.
		if s.keyring != nil {
			_ = s.keyring.Delete(ServiceName, AccountName)
		}
		if s.apiKeyFile == "" {
			return ErrNoKeyFile
		}
		return writeSecretFile(s.apiKeyFile, trimmed)
	}

	if err := s.keyring.Set(ServiceName, AccountName, trimmed); err == nil {
		// The keyring now wins resolution, so a leftover fallback file would
		// resurrect the previous key if the keyring later went away.
		return s.removeStaleFile()
	}

	// The keyring could not take the key. A file fallback is safe only when no
	// older keyring entry survives, since that entry would keep winning
	// resolution over the file. Clear any entry, including one we cannot read:
	// the delete itself proves the keyring accepted the removal. If even the
	// delete fails, the keyring state is unknown, so refuse rather than risk
	// leaving a stale key that would shadow the file once the keyring recovers.
	if err := s.keyring.Delete(ServiceName, AccountName); err != nil && !errors.Is(err, ErrKeyringNotFound) {
		return fmt.Errorf("credentials: the OS keyring could not be cleared; refusing to fall back to the key file: %w", err)
	}

	if s.apiKeyFile == "" {
		return ErrNoKeyFile
	}
	return writeSecretFile(s.apiKeyFile, trimmed)
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
		// Not-found means there is nothing to remove, which is the expected
		// state. Any other failure, including an unreachable keyring, means a
		// stored key may survive, so it is reported instead of being reported as
		// success.
		err := s.keyring.Delete(ServiceName, AccountName)
		if err != nil && !errors.Is(err, ErrKeyringNotFound) {
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
