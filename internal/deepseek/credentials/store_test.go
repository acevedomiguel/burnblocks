package credentials

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeKeyring is an in-memory keyring so tests never touch the real one.
type fakeKeyring struct {
	values    map[string]string
	fail      bool
	setErr    error
	deleteErr error
}

func newFakeKeyring() *fakeKeyring { return &fakeKeyring{values: map[string]string{}} }

func (f *fakeKeyring) Get(service, user string) (string, error) {
	if f.fail {
		return "", ErrKeyringUnavailable
	}
	v, ok := f.values[service+"/"+user]
	if !ok {
		return "", ErrKeyringNotFound
	}
	return v, nil
}

func (f *fakeKeyring) Set(service, user, secret string) error {
	if f.fail {
		return ErrKeyringUnavailable
	}
	if f.setErr != nil {
		return f.setErr
	}
	f.values[service+"/"+user] = secret
	return nil
}

func (f *fakeKeyring) Delete(service, user string) error {
	if f.fail {
		return ErrKeyringUnavailable
	}
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.values, service+"/"+user)
	return nil
}

func newStore(t *testing.T, kr Keyring) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	s := NewStore(Options{
		Keyring:        kr,
		APIKeyFile:     filepath.Join(dir, "api-key"),
		KeyringEnabled: kr != nil,
	})
	return s, dir
}

func TestResolveReadsNothingWhenNothingIsConfigured(t *testing.T) {
	s, _ := newStore(t, nil)
	key, err := s.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if key != "" {
		t.Errorf("key = %q, want empty", key)
	}
}

func TestEnvironmentKeyWinsAndIsNotPersisted(t *testing.T) {
	kr := newFakeKeyring()
	s, dir := newStore(t, kr)
	s.Env = map[string]string{EnvAPIKey: "sk-synthetic-env"}

	key, err := s.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if key != "sk-synthetic-env" {
		t.Errorf("key = %q", key)
	}
	if len(kr.values) != 0 {
		t.Errorf("environment key must not be written to the keyring: %v", kr.values)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("environment key must not create files: %v", entries)
	}
	if got := s.EnvironmentKey(); got != "sk-synthetic-env" {
		t.Errorf("EnvironmentKey = %q", got)
	}
}

func TestKeyringIsPreferredOverTheFile(t *testing.T) {
	kr := newFakeKeyring()
	s, dir := newStore(t, kr)
	if err := kr.Set(ServiceName, AccountName, "sk-synthetic-keyring"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "api-key"), []byte("sk-synthetic-file"), 0o600); err != nil {
		t.Fatal(err)
	}

	key, err := s.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if key != "sk-synthetic-keyring" {
		t.Errorf("key = %q, want the keyring value", key)
	}
}

func TestFileFallbackRequiresRestrictivePermissions(t *testing.T) {
	if !PermissionsAreEnforced {
		t.Skip("the OS does not expose POSIX permission bits")
	}
	s, dir := newStore(t, nil)
	path := filepath.Join(dir, "api-key")
	if err := os.WriteFile(path, []byte("sk-synthetic-file"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := s.Resolve(context.Background())
	if !errors.Is(err, ErrInsecurePermissions) {
		t.Fatalf("err = %v, want ErrInsecurePermissions", err)
	}
	if strings.Contains(err.Error(), "sk-synthetic-file") {
		t.Errorf("error must not leak the key: %v", err)
	}
}

func TestFileFallbackWorksWhenTheKeyringFails(t *testing.T) {
	kr := newFakeKeyring()
	kr.fail = true
	s, dir := newStore(t, kr)
	if err := os.WriteFile(filepath.Join(dir, "api-key"), []byte(" sk-synthetic-file \n"), 0o600); err != nil {
		t.Fatal(err)
	}

	key, err := s.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if key != "sk-synthetic-file" {
		t.Errorf("key = %q, want the trimmed file value", key)
	}
}

func TestSaveRefusesAnEmptyKey(t *testing.T) {
	s, _ := newStore(t, nil)
	if err := s.Save(context.Background(), "   "); !errors.Is(err, ErrEmptySecret) {
		t.Fatalf("err = %v, want ErrEmptySecret", err)
	}
}

func TestSaveWritesARestrictiveFileWhenTheKeyringIsDisabled(t *testing.T) {
	s, dir := newStore(t, nil)
	if err := s.Save(context.Background(), "sk-synthetic-new"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path := filepath.Join(dir, "api-key")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if PermissionsAreEnforced {
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("permissions = %o, want 600", perm)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "sk-synthetic-new" {
		t.Errorf("file = %q", data)
	}
}

// A successful keyring write clears the fallback file, so an unreachable keyring
// later cannot resurrect the replaced key.
func TestSaveClearsTheStaleFileAfterAKeyringWrite(t *testing.T) {
	kr := newFakeKeyring()
	s, dir := newStore(t, kr)
	path := filepath.Join(dir, "api-key")
	if err := os.WriteFile(path, []byte("sk-synthetic-old"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := s.Save(context.Background(), "sk-synthetic-new"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the stale file must be removed: %v", err)
	}

	key, err := s.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if key != "sk-synthetic-new" {
		t.Fatalf("key = %q, want the saved key", key)
	}

	kr.fail = true
	key, err = s.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if key != "" {
		t.Errorf("key = %q, want no stale key after the fallback was removed", key)
	}
}

// When the keyring cannot take the write, the key goes to the file and the stale
// keyring entry is cleared so it cannot shadow the file.
func TestSaveFallsBackToTheFileAndClearsTheKeyring(t *testing.T) {
	kr := newFakeKeyring()
	kr.setErr = errors.New("keyring: transient failure")
	kr.values[ServiceName+"/"+AccountName] = "sk-synthetic-old"
	s, dir := newStore(t, kr)

	if err := s.Save(context.Background(), "sk-synthetic-new"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, ok := kr.values[ServiceName+"/"+AccountName]; ok {
		t.Error("the stale keyring entry must be cleared")
	}
	data, err := os.ReadFile(filepath.Join(dir, "api-key"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "sk-synthetic-new" {
		t.Errorf("file = %q", data)
	}
}

func TestLogoutRemovesOwnedCredentialsOnly(t *testing.T) {
	kr := newFakeKeyring()
	s, dir := newStore(t, kr)
	if err := kr.Set(ServiceName, AccountName, "sk-synthetic-key"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "api-key")
	if err := os.WriteFile(path, []byte("sk-synthetic-file"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := s.Logout(context.Background()); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, ok := kr.values[ServiceName+"/"+AccountName]; ok {
		t.Error("the keyring entry must be removed")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the fallback file must be removed: %v", err)
	}
}

// A keyring that cannot be reached is a genuine failure: logout must report it
// while still removing the fallback file rather than silently claiming success
// even though the stored key may survive.
func TestLogoutReportsUnavailableKeyringButRemovesFile(t *testing.T) {
	kr := newFakeKeyring()
	kr.fail = true
	s, dir := newStore(t, kr)
	path := filepath.Join(dir, "api-key")
	if err := os.WriteFile(path, []byte("sk-synthetic-file"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := s.Logout(context.Background())
	if !errors.Is(err, ErrKeyringUnavailable) {
		t.Fatalf("Logout err = %v, want ErrKeyringUnavailable", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("logout must still remove the fallback file: %v", statErr)
	}
}

// A keyring that cannot take the write and cannot be cleared leaves the keyring
// state unknown. Save must refuse rather than write a file that a surviving
// keyring entry would later shadow.
func TestSaveRefusesFallbackWhenTheKeyringCannotBeCleared(t *testing.T) {
	kr := newFakeKeyring()
	kr.fail = true
	s, dir := newStore(t, kr)

	err := s.Save(context.Background(), "sk-synthetic-new")
	if !errors.Is(err, ErrKeyringUnavailable) {
		t.Fatalf("Save err = %v, want ErrKeyringUnavailable", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "api-key")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("Save must not write a fallback file when the keyring state is unknown: %v", statErr)
	}
	if len(kr.values) != 0 {
		t.Errorf("keyring must be untouched: %v", kr.values)
	}
}

func TestRedactHidesDeepSeekKeyShapes(t *testing.T) {
	// Synthetic, not a real key.
	secret := "Authorization: Bearer sk-0123456789abcdef0123456789abcdef"
	if got := Redact(secret); got == secret {
		t.Errorf("Redact left a secret-shaped value unchanged: %q", got)
	}
	if !LooksLikeSecret(secret) {
		t.Error("LooksLikeSecret must recognise a bearer key")
	}

	redactor := NewRedactor("sk-synthetic-known-value-not-a-prefix")
	if got := redactor.Redact("prefix sk-synthetic-known-value-not-a-prefix suffix"); strings.Contains(got, "known-value") {
		t.Errorf("Redactor left a known secret: %q", got)
	}
	if !IsBearer("bearer sk-synthetic") {
		t.Error("IsBearer must match case-insensitively")
	}
}
