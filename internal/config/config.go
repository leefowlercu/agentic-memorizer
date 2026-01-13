package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// configFilePath stores the path to the loaded config file
var configFilePath string

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
	if home := os.Getenv("HOME"); home != "" {
		viper.AddConfigPath(filepath.Join(home, ".config", "memorizer"))
	}

	// T017: Add current directory as fallback
	viper.AddConfigPath(".")

	// T018: Read config with proper error handling
	err := viper.ReadInConfig()
	if err != nil {
		// Check if it's a "file not found" error - that's acceptable
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// No config file found - use defaults (FR-012)
			configFilePath = ""
			return nil
		}

		// T019 & T020: Any other error (invalid YAML, permission denied) is fatal
		return fmt.Errorf("failed to read config; %w", err)
	}

	// T021: Store the loaded config file path
	configFilePath = viper.ConfigFileUsed()

	// T052: Log config initialization
	slog.Info("config initialized", "file", configFilePath)

	// T050: Setup SIGHUP signal handler for hot reload
	SetupSignalHandler()

	return nil
}

// ConfigFilePath returns the path to the loaded config file,
// or empty string if using defaults only.
func ConfigFilePath() string {
	return configFilePath
}

// Reset clears the configuration state for testing purposes.
func Reset() {
	viper.Reset()
	configFilePath = ""
}

// GetString returns the string value for the given key.
// Returns empty string if key is not found.
func GetString(key string) string {
	return viper.GetString(key)
}

// GetInt returns the integer value for the given key.
// Returns 0 if key is not found or value cannot be converted to int.
func GetInt(key string) int {
	return viper.GetInt(key)
}

// GetBool returns the boolean value for the given key.
// Returns false if key is not found or value cannot be converted to bool.
func GetBool(key string) bool {
	return viper.GetBool(key)
}

// SetDefault sets a default value for the given key.
func SetDefault(key string, value any) {
	viper.SetDefault(key, value)
}

// Set sets a value for the given key, overriding defaults and config file values.
// Primarily used for testing.
func Set(key string, value any) {
	viper.Set(key, value)
}

// GetPath returns the string value for the given key with ~ expanded to $HOME.
// Returns empty string if key is not found.
func GetPath(key string) string {
	return expandHome(viper.GetString(key))
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

	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	if len(path) == 1 {
		return home
	}

	return filepath.Join(home, path[2:])
}

// GetConfigPath returns the path where the config file should be located.
// If a config file is loaded, returns its path. Otherwise returns the default path.
func GetConfigPath() string {
	if configFilePath != "" {
		return configFilePath
	}
	// Return default path
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "memorizer", "config.yaml")
}

// EnsureConfigDir creates the config directory if it doesn't exist.
func EnsureConfigDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory; %w", err)
	}
	configDir := filepath.Join(home, ".config", "memorizer")
	return os.MkdirAll(configDir, 0755)
}

// GetAllSettings returns all configuration settings as a map.
func GetAllSettings() map[string]any {
	return viper.AllSettings()
}

// Reload re-reads the configuration from disk.
// On failure, the previous configuration is retained.
func Reload() error {
	// Store current settings in case reload fails
	currentSettings := viper.AllSettings()

	err := viper.ReadInConfig()
	if err != nil {
		// Restore previous settings on failure
		for key, value := range currentSettings {
			viper.Set(key, value)
		}
		slog.Error("config reload failed; retaining previous values", "error", err)
		return fmt.Errorf("failed to reload config; %w", err)
	}

	slog.Info("config reloaded", "file", viper.ConfigFileUsed())
	return nil
}
