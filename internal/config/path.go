package config

import (
	"os"
	"path/filepath"
)

// DefaultConfigPath returns the default path for the config file.
func DefaultConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

// ConfigDir returns the default config directory path.
func ConfigDir() string {
	home := resolveHomeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "memorizer")
}

// EnsureConfigDirWithPerms creates the config directory with specified permissions.
// Use 0700 for secure directory permissions.
func EnsureConfigDirWithPerms(perms os.FileMode) error {
	return os.MkdirAll(ConfigDir(), perms)
}

// ConfigExists returns true if the config file exists at the default path.
func ConfigExists() bool {
	_, err := os.Stat(DefaultConfigPath())
	return err == nil
}

// ConfigExistsAt returns true if a config file exists at the specified path.
func ConfigExistsAt(path string) bool {
	path = expandHome(path)
	_, err := os.Stat(path)
	return err == nil
}
