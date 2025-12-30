package fileops

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// conflictSuffixPattern matches existing conflict suffixes like "-1", "-2", etc.
var conflictSuffixPattern = regexp.MustCompile(`-(\d+)$`)

// ResolveConflict finds a non-conflicting name for the given path by appending
// a -N suffix (where N is an incrementing integer starting from 1).
//
// For files, the suffix is inserted before the extension:
//   - file.md → file-1.md → file-2.md
//   - archive.tar.gz → archive-1.tar.gz → archive-2.tar.gz
//
// For directories:
//   - mydir → mydir-1 → mydir-2
//
// If the path doesn't exist, it returns the original path unchanged.
func ResolveConflict(path string) (string, error) {
	// Check if path exists - if not, no conflict
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path, nil
	}

	// Check if it's a directory
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to stat path; %w", err)
	}

	isDir := info.IsDir()

	// Get base name and directory
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	// For directories, simple suffix append
	if isDir {
		return resolveConflictWithSuffix(dir, base, "")
	}

	// For files, handle extensions
	// Find the primary extension (the first one from the right that's commonly doubled)
	name, ext := splitNameAndExtensions(base)

	return resolveConflictWithSuffix(dir, name, ext)
}

// resolveConflictWithSuffix finds a non-conflicting name by incrementing suffix
func resolveConflictWithSuffix(dir, name, ext string) (string, error) {
	// Strip any existing conflict suffix from name
	baseName := conflictSuffixPattern.ReplaceAllString(name, "")

	// Start from 1 and find first available
	for i := 1; i <= 10000; i++ { // Reasonable upper limit
		candidate := filepath.Join(dir, fmt.Sprintf("%s-%d%s", baseName, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("could not find available name after 10000 attempts")
}

// splitNameAndExtensions splits a filename into base name and all extensions.
// It handles compound extensions like .tar.gz, .tar.bz2, etc.
//
// Examples:
//   - "file.md" → ("file", ".md")
//   - "archive.tar.gz" → ("archive", ".tar.gz")
//   - "no-extension" → ("no-extension", "")
//   - ".hidden" → (".hidden", "")
//   - ".hidden.txt" → (".hidden", ".txt")
func splitNameAndExtensions(filename string) (name, ext string) {
	// Known compound extensions (order matters - check longer first)
	compoundExts := []string{
		".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst",
	}

	lower := strings.ToLower(filename)
	for _, compound := range compoundExts {
		if strings.HasSuffix(lower, compound) {
			name = filename[:len(filename)-len(compound)]
			ext = filename[len(filename)-len(compound):]
			return
		}
	}

	// Handle hidden files (starting with .)
	if strings.HasPrefix(filename, ".") {
		// Check if there's another dot after the first one
		afterDot := filename[1:]
		if idx := strings.LastIndex(afterDot, "."); idx > 0 {
			name = filename[:idx+1]
			ext = filename[idx+1:]
			return
		}
		// No extension, entire thing is the name
		return filename, ""
	}

	// Standard single extension
	ext = filepath.Ext(filename)
	if ext != "" {
		name = filename[:len(filename)-len(ext)]
	} else {
		name = filename
	}

	return
}

// HasConflictSuffix checks if a path has a conflict suffix like -1, -2, etc.
func HasConflictSuffix(path string) bool {
	base := filepath.Base(path)
	name, _ := splitNameAndExtensions(base)
	return conflictSuffixPattern.MatchString(name)
}

// GetConflictNumber extracts the conflict number from a path, or 0 if none.
func GetConflictNumber(path string) int {
	base := filepath.Base(path)
	name, _ := splitNameAndExtensions(base)

	matches := conflictSuffixPattern.FindStringSubmatch(name)
	if len(matches) < 2 {
		return 0
	}

	n, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}

	return n
}
