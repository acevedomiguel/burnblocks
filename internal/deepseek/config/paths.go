// Package config resolves the DeepSeek engine's on-disk locations. It holds no
// secrets and reads no environment beyond the standard user directories.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Paths holds the resolved location the DeepSeek engine reads and writes.
type Paths struct {
	ConfigDir string
}

// APIKeyFile is the permission-restricted fallback for the API key. It is used
// only when the environment and OS keyring do not supply one.
func (p Paths) APIKeyFile() string {
	return filepath.Join(p.ConfigDir, "api-key")
}

// DefaultPaths resolves a platform-appropriate directory through the standard
// library so an explicit XDG override is honoured. There is no cache tree: the
// engine stores no snapshot.
func DefaultPaths() (Paths, error) {
	configDir, err := userDir("XDG_CONFIG_HOME", os.UserConfigDir)
	if err != nil {
		return Paths{}, fmt.Errorf("config: resolve user config dir: %w", err)
	}
	return Paths{ConfigDir: filepath.Join(configDir, "deepseek")}, nil
}

// userDir prefers an explicit XDG directory when the user set one, and falls
// back to the platform-native location otherwise. On macOS the standard library
// ignores XDG, but users who set it expect it to win.
func userDir(envVar string, fallback func() (string, error)) (string, error) {
	if v := strings.TrimSpace(os.Getenv(envVar)); v != "" {
		return v, nil
	}
	return fallback()
}
