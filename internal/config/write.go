package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Write writes the configuration to the specified path.
// Creates the directory with 0700 permissions if it doesn't exist.
// Writes the file with 0600 permissions.
func Write(cfg *Config, path string) error {
	// Expand tilde in path
	path = expandHome(path)

	// Ensure directory exists with proper permissions
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory %s; %w", dir, err)
	}

	// Marshal config to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config; %w", err)
	}

	// Add header comment
	header := fmt.Sprintf("# Memorizer configuration\n# Generated: %s\n# Do not edit while daemon is running\n\n",
		time.Now().Format(time.RFC3339))
	content := []byte(header)
	content = append(content, data...)

	// Write file with secure permissions
	if err := os.WriteFile(path, content, 0600); err != nil {
		return fmt.Errorf("failed to write config file %s; %w", path, err)
	}

	return nil
}

// WriteDefault writes the configuration to the default config path.
func WriteDefault(cfg *Config) error {
	return Write(cfg, DefaultConfigPath())
}
