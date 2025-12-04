package walker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FileVisitor func(path string, info os.FileInfo) error

func Walk(root string, skipDirs []string, skipFiles []string, skipExtensions []string, visitor FileVisitor) error {
	root = filepath.Clean(root)

	skipPaths := make(map[string]bool)
	for _, dir := range skipDirs {
		absPath := filepath.Join(root, dir)
		skipPaths[absPath] = true
	}

	skipFileNames := make(map[string]bool)
	for _, file := range skipFiles {
		skipFileNames[file] = true
	}

	skipExts := make(map[string]bool)
	for _, ext := range skipExtensions {
		skipExts[ext] = true
	}

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: error accessing %s: %v\n", path, err)
			return nil
		}

		if path == root {
			return nil
		}

		if info.IsDir() {
			if strings.HasPrefix(filepath.Base(path), ".") {
				return filepath.SkipDir
			}

			if skipPaths[path] {
				return filepath.SkipDir
			}

			return nil
		}

		if strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}

		basename := filepath.Base(path)
		if skipFileNames[basename] {
			return nil
		}

		// Check skip extensions
		ext := filepath.Ext(path)
		if skipExts[ext] {
			return nil
		}

		return visitor(path, info)
	})
}

func GetRelPath(root, path string) (string, error) {
	relPath, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("failed to get relative path; %w", err)
	}
	return relPath, nil
}
