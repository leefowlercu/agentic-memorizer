package fileops

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// MoveItem represents a single move operation.
type MoveItem struct {
	Src string // Source path (file or directory)
	Dst string // Destination path
}

// MoveResult contains the result of a move operation.
type MoveResult struct {
	Src             string // Original source path
	Dst             string // Final destination path (may differ if conflict resolved)
	IsDir           bool   // Whether source was a directory
	FileCount       int    // Number of files moved (1 for single file, N for directory)
	Renamed         bool   // Whether destination was renamed due to conflict
	CrossFilesystem bool   // Whether move required copy+delete (cross-filesystem)
}

// Move moves a file or directory from src to dst.
// If dst exists, it is renamed with -N suffix to avoid conflict.
// If src and dst are on different filesystems, this falls back to copy+delete.
func Move(src, dst string) (MoveResult, error) {
	result := MoveResult{Src: src}

	// Check source exists
	srcInfo, err := os.Stat(src)
	if err != nil {
		return result, fmt.Errorf("source not found; %w", err)
	}

	result.IsDir = srcInfo.IsDir()

	// Count files if directory
	if result.IsDir {
		count, err := countFiles(src)
		if err != nil {
			return result, fmt.Errorf("failed to count files; %w", err)
		}
		result.FileCount = count
	} else {
		result.FileCount = 1
	}

	// Handle destination conflict
	finalDst := dst
	if PathExists(dst) {
		resolved, err := ResolveConflict(dst)
		if err != nil {
			return result, fmt.Errorf("failed to resolve conflict; %w", err)
		}
		finalDst = resolved
		result.Renamed = true
	}
	result.Dst = finalDst

	// Ensure destination directory exists
	dstDir := filepath.Dir(finalDst)
	if err := EnsureDir(dstDir); err != nil {
		return result, fmt.Errorf("failed to create destination directory; %w", err)
	}

	// Try direct rename first (works within same filesystem)
	err = os.Rename(src, finalDst)
	if err == nil {
		return result, nil
	}

	// Check if this is a cross-filesystem error
	if !isCrossFilesystemError(err) {
		return result, fmt.Errorf("failed to move; %w", err)
	}

	// Fall back to copy + delete for cross-filesystem moves
	result.CrossFilesystem = true

	// Copy to destination
	_, err = Copy(src, finalDst, true) // force=true since we already checked dst
	if err != nil {
		return result, fmt.Errorf("cross-filesystem copy failed; %w", err)
	}

	// Delete source
	if result.IsDir {
		if err := os.RemoveAll(src); err != nil {
			// Copy succeeded but delete failed - leave copy, report error
			return result, fmt.Errorf("source deletion failed after copy; %w", err)
		}
	} else {
		if err := os.Remove(src); err != nil {
			return result, fmt.Errorf("source deletion failed after copy; %w", err)
		}
	}

	return result, nil
}

// MoveBatch moves multiple files/directories.
// It continues on failure and returns all results and errors.
// Errors are indexed to match the corresponding MoveItem.
func MoveBatch(items []MoveItem) ([]MoveResult, []error) {
	results := make([]MoveResult, len(items))
	errors := make([]error, len(items))

	for i, item := range items {
		result, err := Move(item.Src, item.Dst)
		results[i] = result
		errors[i] = err
	}

	return results, errors
}

// MoveToDir moves src to a file with the same name inside dstDir.
// This is a convenience function for the common case of moving a file
// into a directory while preserving its name.
func MoveToDir(src, dstDir string) (MoveResult, error) {
	dst := filepath.Join(dstDir, filepath.Base(src))
	return Move(src, dst)
}

// countFiles counts the number of regular files in a directory tree.
func countFiles(dir string) (int, error) {
	count := 0
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	return count, err
}

// isCrossFilesystemError checks if an error is due to cross-filesystem rename.
func isCrossFilesystemError(err error) bool {
	if linkErr, ok := err.(*os.LinkError); ok {
		// EXDEV is the error code for cross-filesystem move
		if errno, ok := linkErr.Err.(syscall.Errno); ok {
			return errno == syscall.EXDEV
		}
	}
	return false
}
