package walker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileVisitor is called for each file found
type FileVisitor func(path string, info os.FileInfo) error

// Walk traverses a directory tree and calls visitor for each file
func Walk(root string, skipDirs []string, skipFiles []string, visitor FileVisitor) error {
	// Normalize root path
	root = filepath.Clean(root)

	// Convert skip directories to absolute paths
	skipPaths := make(map[string]bool)
	for _, dir := range skipDirs {
		absPath := filepath.Join(root, dir)
		skipPaths[absPath] = true
	}

	// Convert skip files to map for fast lookup
	skipFileNames := make(map[string]bool)
	for _, file := range skipFiles {
		skipFileNames[file] = true
	}

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Log error but continue walking
			fmt.Fprintf(os.Stderr, "Warning: error accessing %s: %v\n", path, err)
			return nil
		}

		// Skip the root directory itself
		if path == root {
			return nil
		}

		// Check if we should skip this directory
		if info.IsDir() {
			// Skip hidden directories (starting with .)
			if strings.HasPrefix(filepath.Base(path), ".") {
				return filepath.SkipDir
			}

			// Skip explicitly configured directories
			if skipPaths[path] {
				return filepath.SkipDir
			}

			return nil
		}

		// Skip hidden files (starting with .)
		if strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}

		// Skip explicitly configured files
		basename := filepath.Base(path)
		if skipFileNames[basename] {
			return nil
		}

		// Visit the file
		return visitor(path, info)
	})
}

// GetRelPath returns the relative path from root
func GetRelPath(root, path string) (string, error) {
	relPath, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("failed to get relative path: %w", err)
	}
	return relPath, nil
}
