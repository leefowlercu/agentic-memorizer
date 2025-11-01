package integrations

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExpandPath expands ~ to the user's home directory
func ExpandPath(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	if path == "~" {
		return homeDir, nil
	}

	return filepath.Join(homeDir, path[2:]), nil
}

// FileExists checks if a file exists at the given path
func FileExists(path string) bool {
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return false
	}

	info, err := os.Stat(expandedPath)
	if err != nil {
		return false
	}

	return !info.IsDir()
}

// DirExists checks if a directory exists at the given path
func DirExists(path string) bool {
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return false
	}

	info, err := os.Stat(expandedPath)
	if err != nil {
		return false
	}

	return info.IsDir()
}

// EnsureDir creates a directory if it doesn't exist
func EnsureDir(path string) error {
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}

	if err := os.MkdirAll(expandedPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}

// ValidateOutputFormat checks if the given format string is valid
func ValidateOutputFormat(format string) error {
	outputFormat := OutputFormat(format)
	if !outputFormat.IsValid() {
		return fmt.Errorf("invalid output format %q: must be one of xml, markdown, json", format)
	}
	return nil
}

// ParseOutputFormat parses a string into an OutputFormat
func ParseOutputFormat(format string) (OutputFormat, error) {
	outputFormat := OutputFormat(format)
	if !outputFormat.IsValid() {
		return "", fmt.Errorf("invalid output format %q: must be one of xml, markdown, json", format)
	}
	return outputFormat, nil
}

// GetDefaultOutputFormat returns the default output format
func GetDefaultOutputFormat() OutputFormat {
	return FormatXML
}

// FormatSize formats bytes into human-readable size
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// IsExecutable checks if a file is executable
func IsExecutable(path string) bool {
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return false
	}

	info, err := os.Stat(expandedPath)
	if err != nil {
		return false
	}

	// Check if file has execute permission
	mode := info.Mode()
	return mode&0111 != 0
}

// ValidateBinaryPath validates that a binary path exists and is executable
func ValidateBinaryPath(path string) error {
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}

	if !FileExists(expandedPath) {
		return fmt.Errorf("binary not found at %s", expandedPath)
	}

	if !IsExecutable(expandedPath) {
		return fmt.Errorf("binary at %s is not executable", expandedPath)
	}

	return nil
}
