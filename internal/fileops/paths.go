package fileops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IsInDirectory checks if the given path is within the specified directory.
// Both paths are cleaned and resolved to absolute paths before comparison.
// Returns true if path is inside dir (or is dir itself), false otherwise.
func IsInDirectory(path, dir string) (bool, error) {
	// Clean and resolve both paths
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false, fmt.Errorf("failed to resolve path; %w", err)
	}

	absDir, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return false, fmt.Errorf("failed to resolve directory; %w", err)
	}

	// Ensure directory path ends with separator for proper prefix matching
	if !strings.HasSuffix(absDir, string(filepath.Separator)) {
		absDir += string(filepath.Separator)
	}

	// Check if path starts with directory
	// Also check if they're exactly equal (path IS the directory)
	return strings.HasPrefix(absPath+string(filepath.Separator), absDir), nil
}

// ValidatePath checks if a path is safe for use.
// It rejects paths containing:
//   - Parent directory references (..)
//   - Null bytes
//   - Absolute paths when relative is expected
func ValidatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// Check for null bytes
	if strings.ContainsRune(path, '\x00') {
		return fmt.Errorf("path contains null byte")
	}

	// Check for parent directory references BEFORE cleaning
	// (cleaning would normalize "subdir/../file" to "file")
	if strings.Contains(path, "..") {
		return fmt.Errorf("path contains parent directory reference (..)")
	}

	return nil
}

// ValidateSubdirectory checks if a subdirectory path is safe for use within
// a parent directory. This is more restrictive than ValidatePath.
func ValidateSubdirectory(subdir string) error {
	if subdir == "" {
		return nil // Empty subdir is allowed (means root)
	}

	if err := ValidatePath(subdir); err != nil {
		return err
	}

	// Additional checks for subdirectories
	if filepath.IsAbs(subdir) {
		return fmt.Errorf("subdirectory must be relative, not absolute")
	}

	return nil
}

// EnsureDir creates a directory and all necessary parents if they don't exist.
// If the directory already exists, this is a no-op.
func EnsureDir(path string) error {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return nil // Already exists as directory
		}
		return fmt.Errorf("path exists but is not a directory; %s", path)
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat path; %w", err)
	}

	// Create directory with parents
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory; %w", err)
	}

	return nil
}

// RelativePath returns the relative path from base to target.
// If target is not within base, returns an error.
func RelativePath(base, target string) (string, error) {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base; %w", err)
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("failed to resolve target; %w", err)
	}

	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return "", fmt.Errorf("failed to compute relative path; %w", err)
	}

	// Check that the relative path doesn't escape base
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("target is outside base directory")
	}

	return rel, nil
}

// ExpandHome expands ~ to the user's home directory.
func ExpandHome(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory; %w", err)
	}

	if path == "~" {
		return home, nil
	}

	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:]), nil
	}

	// ~user syntax not supported
	return "", fmt.Errorf("~user syntax not supported")
}

// PathExists checks if a path exists (file or directory).
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsFile checks if path exists and is a regular file.
func IsFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

// IsDir checks if path exists and is a directory.
func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
