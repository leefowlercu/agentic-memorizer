package walker

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/leefowlercu/agentic-memorizer/internal/skip"
)

type FileVisitor func(path string, info os.FileInfo) error

// Walk traverses a directory tree, calling visitor for each file.
// Uses skip.Config to determine which files and directories to skip.
func Walk(root string, cfg *skip.Config, visitor FileVisitor) error {
	root = filepath.Clean(root)

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: error accessing %s: %v\n", path, err)
			return nil
		}

		if path == root {
			return nil
		}

		basename := filepath.Base(path)

		if info.IsDir() {
			if skip.ShouldSkipDir(basename, cfg) {
				return filepath.SkipDir
			}
			return nil
		}

		if skip.ShouldSkipFile(basename, cfg) {
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
