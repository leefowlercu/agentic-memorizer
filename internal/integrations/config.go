package integrations

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// ConfigFile represents a JSON configuration file.
type ConfigFile struct {
	Path    string
	Content map[string]any
	Format  string // "json", "toml"
}

// ReadJSONConfig reads a JSON configuration file.
func ReadJSONConfig(path string) (*ConfigFile, error) {
	expandedPath := expandPath(path)

	data, err := os.ReadFile(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &ConfigFile{
				Path:    expandedPath,
				Content: make(map[string]any),
				Format:  "json",
			}, nil
		}
		return nil, fmt.Errorf("failed to read config; %w", err)
	}

	var content map[string]any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &content); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config; %w", err)
		}
	}

	if content == nil {
		content = make(map[string]any)
	}

	return &ConfigFile{
		Path:    expandedPath,
		Content: content,
		Format:  "json",
	}, nil
}

// WriteJSONConfig writes a JSON configuration file with indentation.
func WriteJSONConfig(path string, content map[string]any) error {
	expandedPath := expandPath(path)

	// Ensure parent directory exists
	dir := filepath.Dir(expandedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory; %w", err)
	}

	data, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON config; %w", err)
	}

	// Add trailing newline
	data = append(data, '\n')

	if err := os.WriteFile(expandedPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config; %w", err)
	}

	return nil
}

// BackupConfig creates a backup of a configuration file.
func BackupConfig(path string) (string, error) {
	expandedPath := expandPath(path)

	// Check if file exists
	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		return "", nil // Nothing to backup
	}

	// Generate backup path with timestamp
	timestamp := time.Now().Format("20060102-150405")
	ext := filepath.Ext(expandedPath)
	base := expandedPath[:len(expandedPath)-len(ext)]
	backupPath := fmt.Sprintf("%s.%s.backup%s", base, timestamp, ext)

	// Copy file
	src, err := os.Open(expandedPath)
	if err != nil {
		return "", fmt.Errorf("failed to open config for backup; %w", err)
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return "", fmt.Errorf("failed to create backup file; %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("failed to copy config to backup; %w", err)
	}

	return backupPath, nil
}

// RestoreBackup restores a configuration from backup.
func RestoreBackup(backupPath string, originalPath string) error {
	expandedBackup := expandPath(backupPath)
	expandedOriginal := expandPath(originalPath)

	src, err := os.Open(expandedBackup)
	if err != nil {
		return fmt.Errorf("failed to open backup file; %w", err)
	}
	defer src.Close()

	dst, err := os.Create(expandedOriginal)
	if err != nil {
		return fmt.Errorf("failed to create restored config; %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to restore from backup; %w", err)
	}

	return nil
}

// expandPath expands ~ to home directory.
func expandPath(path string) string {
	if len(path) == 0 {
		return path
	}

	if path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[1:])
		}
	}

	return path
}

// GetMapSection retrieves a nested map from a config, handling type variations.
func GetMapSection(content map[string]any, key string) (map[string]any, bool) {
	val, exists := content[key]
	if !exists {
		return nil, false
	}

	// Try map[string]any first
	if m, ok := val.(map[string]any); ok {
		return m, true
	}

	// Try map[string]interface{} (from JSON unmarshal)
	if m, ok := val.(map[string]interface{}); ok {
		result := make(map[string]any, len(m))
		for k, v := range m {
			result[k] = v
		}
		// Update the original to use the converted map
		content[key] = result
		return result, true
	}

	return nil, false
}

// ConfigExists checks if a configuration file exists.
func ConfigExists(path string) bool {
	expandedPath := expandPath(path)
	_, err := os.Stat(expandedPath)
	return err == nil
}

// FindBinary searches for a binary in PATH or common locations.
func FindBinary(name string) (string, bool) {
	// Check if it's in PATH
	path, err := os.Executable()
	if err == nil && filepath.Base(path) == name {
		return path, true
	}

	// Check common binary locations
	commonPaths := []string{
		"/usr/local/bin",
		"/usr/bin",
		"/opt/homebrew/bin",
	}

	home, _ := os.UserHomeDir()
	if home != "" {
		commonPaths = append(commonPaths, filepath.Join(home, ".local/bin"))
		commonPaths = append(commonPaths, filepath.Join(home, "bin"))
	}

	for _, dir := range commonPaths {
		fullPath := filepath.Join(dir, name)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, true
		}
	}

	// Check PATH environment variable
	pathEnv := os.Getenv("PATH")
	if pathEnv != "" {
		for _, dir := range filepath.SplitList(pathEnv) {
			fullPath := filepath.Join(dir, name)
			if _, err := os.Stat(fullPath); err == nil {
				return fullPath, true
			}
		}
	}

	return "", false
}
