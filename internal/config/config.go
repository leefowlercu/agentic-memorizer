package config

import (
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

// configMu protects configFilePath and currentConfig
var configMu sync.RWMutex

// configFilePath stores the path to the loaded config file
var configFilePath string

// currentConfig stores the loaded typed configuration
var currentConfig *Config

// Init initializes the configuration subsystem.
// It searches for configuration files in priority order:
//  1. Directory specified by MEMORIZER_CONFIG_DIR environment variable
//  2. ~/.config/memorizer/
//  3. Current working directory (.)
//
// If no config file is found, sensible defaults are used.
// If a config file exists but is invalid or unreadable, Init returns an error.
func Init() error {
	// T014: Setup Viper with config name and type
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// T027: Setup environment variable prefix
	viper.SetEnvPrefix("MEMORIZER")

	// T028: Setup dot-to-underscore key replacement for env vars
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// T029: Enable automatic environment variable binding
	viper.AutomaticEnv()

	// Register default values
	setDefaults()

	// T015: Check MEMORIZER_CONFIG_DIR environment variable first
	if envPath := os.Getenv("MEMORIZER_CONFIG_DIR"); envPath != "" {
		viper.AddConfigPath(envPath)
	}

	// T016: Add default config path (~/.config/memorizer/)
	if home := resolveHomeDir(); home != "" {
		viper.AddConfigPath(filepath.Join(home, ".config", "memorizer"))
	}

	// T017: Add current directory as fallback
	viper.AddConfigPath(".")

	// T018: Read config with proper error handling
	err := viper.ReadInConfig()
	if err != nil {
		// Check if it's a "file not found" error - that's acceptable
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// No config file found - use defaults + env var overrides (FR-012)
			// Unmarshal from viper to get defaults + env var overrides
			cfg := &Config{}
			if err := viper.Unmarshal(cfg); err != nil {
				return fmt.Errorf("failed to unmarshal config; %w", err)
			}
			configMu.Lock()
			configFilePath = ""
			currentConfig = cfg
			configMu.Unlock()
			return nil
		}

		// T019 & T020: Any other error (invalid YAML, permission denied) is fatal
		return fmt.Errorf("failed to read config; %w", err)
	}

	// Unmarshal to typed config
	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config; %w", err)
	}

	// Validate typed config
	if err := Validate(cfg); err != nil {
		return fmt.Errorf("config validation failed; %w", err)
	}

	// T021: Store the loaded config file path
	configMu.Lock()
	configFilePath = viper.ConfigFileUsed()
	currentConfig = cfg
	configMu.Unlock()

	// T052: Log config initialization
	slog.Debug("config initialized", "file", configFilePath)

	// T050: Setup SIGHUP signal handler for hot reload
	SetupSignalHandler()

	return nil
}

// InitWithDefaults initializes the configuration subsystem with defaults only.
// Use this in contexts where a config file is not required (e.g., initialize command).
func InitWithDefaults() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	viper.SetEnvPrefix("MEMORIZER")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	setDefaults()

	cfg := LoadWithDefaults()
	configMu.Lock()
	configFilePath = ""
	currentConfig = cfg
	configMu.Unlock()

	return nil
}

// ConfigFilePath returns the path to the loaded config file,
// or empty string if using defaults only.
func ConfigFilePath() string {
	configMu.RLock()
	defer configMu.RUnlock()
	return configFilePath
}

// Reset clears the configuration state for testing purposes.
func Reset() {
	viper.Reset()
	configMu.Lock()
	configFilePath = ""
	currentConfig = nil
	configMu.Unlock()
}

// Get returns the typed configuration.
// Returns nil if config has not been initialized.
func Get() *Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return currentConfig
}

// MustGet returns the typed configuration.
// Panics if config has not been initialized.
func MustGet() *Config {
	configMu.RLock()
	defer configMu.RUnlock()
	if currentConfig == nil {
		panic("config: not initialized; call Init() first")
	}
	return currentConfig
}

// ExpandPath expands a leading ~ in path to the user's home directory.
// Only expands "~" alone or "~/..." patterns. Patterns like "~user" are not expanded.
// Returns the path unchanged if it doesn't start with ~/ or if home dir cannot be determined.
// Use this when accessing path fields from config.Get() that may contain tildes.
func ExpandPath(path string) string {
	return expandHome(path)
}

// expandHome expands a leading ~ in path to the user's home directory.
// Only expands "~" alone or "~/..." patterns. Patterns like "~user" are not expanded.
// Returns the path unchanged if it doesn't start with ~/ or if home dir cannot be determined.
func expandHome(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}

	// Only expand "~" or "~/..."
	if len(path) > 1 && path[1] != '/' {
		return path
	}

	home := resolveHomeDir()
	if home == "" {
		return path
	}

	if len(path) == 1 {
		return home
	}

	return filepath.Join(home, path[2:])
}

func resolveHomeDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}

	u, err := user.Current()
	if err != nil {
		return ""
	}

	return u.HomeDir
}

// GetConfigPath returns the path where the config file should be located.
// If a config file is loaded, returns its path. Otherwise returns the default path.
func GetConfigPath() string {
	configMu.RLock()
	path := configFilePath
	configMu.RUnlock()

	if path != "" {
		return path
	}
	// Return default path
	home := resolveHomeDir()
	if home == "" {
		return filepath.Join(".config", "memorizer", "config.yaml")
	}
	return filepath.Join(home, ".config", "memorizer", "config.yaml")
}

// EnsureConfigDir creates the config directory if it doesn't exist.
// Uses 0700 permissions for security.
func EnsureConfigDir() error {
	return EnsureConfigDirWithPerms(0700)
}

// GetAllSettings returns all configuration settings as a map.
func GetAllSettings() map[string]any {
	return viper.AllSettings()
}

// Reload re-reads the configuration from disk.
// On failure, the previous configuration is retained and a config.reload_failed event is published.
// On success, a config.reloaded event is published with the list of changed sections.
func Reload() error {
	// Store current state in case reload fails
	currentSettings := viper.AllSettings()
	configMu.RLock()
	previousConfig := currentConfig
	configMu.RUnlock()

	err := viper.ReadInConfig()
	if err != nil {
		// Restore previous settings on failure
		for key, value := range currentSettings {
			viper.Set(key, value)
		}
		slog.Error("config reload failed; retaining previous values", "error", err)
		reloadErr := fmt.Errorf("failed to reload config; %w", err)
		publishConfigReloadFailed(reloadErr)
		return reloadErr
	}

	// Unmarshal to typed config
	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		// Restore previous state
		for key, value := range currentSettings {
			viper.Set(key, value)
		}
		configMu.Lock()
		currentConfig = previousConfig
		configMu.Unlock()
		slog.Error("config reload unmarshal failed; retaining previous values", "error", err)
		reloadErr := fmt.Errorf("failed to unmarshal config; %w", err)
		publishConfigReloadFailed(reloadErr)
		return reloadErr
	}

	// Validate typed config
	if err := Validate(cfg); err != nil {
		// Restore previous state
		for key, value := range currentSettings {
			viper.Set(key, value)
		}
		configMu.Lock()
		currentConfig = previousConfig
		configMu.Unlock()
		slog.Error("config reload validation failed; retaining previous values", "error", err)
		reloadErr := fmt.Errorf("config validation failed; %w", err)
		publishConfigReloadFailed(reloadErr)
		return reloadErr
	}

	// Publish success event before updating currentConfig so we can compare
	publishConfigReloaded(previousConfig, cfg)

	configMu.Lock()
	currentConfig = cfg
	configMu.Unlock()

	slog.Info("config reloaded", "file", viper.ConfigFileUsed())
	return nil
}
