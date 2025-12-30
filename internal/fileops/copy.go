package fileops

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopyItem represents a single copy operation.
type CopyItem struct {
	Src string // Source path (file or directory)
	Dst string // Destination path
}

// CopyResult contains the result of a copy operation.
type CopyResult struct {
	Src       string // Original source path
	Dst       string // Final destination path (may differ from requested if conflict resolved)
	Size      int64  // Total bytes copied
	IsDir     bool   // Whether source was a directory
	FileCount int    // Number of files copied (1 for single file, N for directory)
	Renamed   bool   // Whether destination was renamed due to conflict
}

// Copy copies a file or directory from src to dst.
// If force is false and dst exists, the destination is renamed with -N suffix.
// If force is true, existing files are overwritten.
func Copy(src, dst string, force bool) (CopyResult, error) {
	result := CopyResult{Src: src}

	// Check source exists
	srcInfo, err := os.Stat(src)
	if err != nil {
		return result, fmt.Errorf("source not found; %w", err)
	}

	result.IsDir = srcInfo.IsDir()

	// Handle destination conflict
	finalDst := dst
	if !force {
		if PathExists(dst) {
			resolved, err := ResolveConflict(dst)
			if err != nil {
				return result, fmt.Errorf("failed to resolve conflict; %w", err)
			}
			finalDst = resolved
			result.Renamed = true
		}
	}
	result.Dst = finalDst

	// Perform copy
	if result.IsDir {
		size, count, err := copyDir(src, finalDst, force)
		if err != nil {
			return result, err
		}
		result.Size = size
		result.FileCount = count
	} else {
		size, err := copyFile(src, finalDst, force)
		if err != nil {
			return result, err
		}
		result.Size = size
		result.FileCount = 1
	}

	return result, nil
}

// CopyBatch copies multiple files/directories.
// It continues on failure and returns all results and errors.
// Errors are indexed to match the corresponding CopyItem.
func CopyBatch(items []CopyItem, force bool) ([]CopyResult, []error) {
	results := make([]CopyResult, len(items))
	errors := make([]error, len(items))

	for i, item := range items {
		result, err := Copy(item.Src, item.Dst, force)
		results[i] = result
		errors[i] = err
	}

	return results, errors
}

// copyFile copies a single file from src to dst.
// If force is true, dst is overwritten if it exists.
func copyFile(src, dst string, force bool) (int64, error) {
	// Open source
	srcFile, err := os.Open(src)
	if err != nil {
		return 0, fmt.Errorf("failed to open source; %w", err)
	}
	defer srcFile.Close()

	// Get source info for permissions
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to stat source; %w", err)
	}

	// Ensure destination directory exists
	dstDir := filepath.Dir(dst)
	if err := EnsureDir(dstDir); err != nil {
		return 0, fmt.Errorf("failed to create destination directory; %w", err)
	}

	// Create destination file
	flags := os.O_CREATE | os.O_WRONLY
	if force {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL // Fail if exists
	}

	dstFile, err := os.OpenFile(dst, flags, srcInfo.Mode().Perm())
	if err != nil {
		return 0, fmt.Errorf("failed to create destination; %w", err)
	}
	defer dstFile.Close()

	// Copy content
	written, err := io.Copy(dstFile, srcFile)
	if err != nil {
		return written, fmt.Errorf("failed to copy content; %w", err)
	}

	return written, nil
}

// copyDir recursively copies a directory from src to dst.
func copyDir(src, dst string, force bool) (int64, int, error) {
	// Get source info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to stat source; %w", err)
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode().Perm()); err != nil {
		return 0, 0, fmt.Errorf("failed to create destination directory; %w", err)
	}

	var totalSize int64
	var fileCount int

	// Walk source directory
	err = filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Calculate relative path from source root
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path; %w", err)
		}

		// Calculate destination path
		dstPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			// Create directory
			info, err := d.Info()
			if err != nil {
				return fmt.Errorf("failed to get dir info; %w", err)
			}
			if err := os.MkdirAll(dstPath, info.Mode().Perm()); err != nil {
				return fmt.Errorf("failed to create directory; %w", err)
			}
		} else {
			// Copy file
			size, err := copyFile(path, dstPath, force)
			if err != nil {
				return err
			}
			totalSize += size
			fileCount++
		}

		return nil
	})

	if err != nil {
		return totalSize, fileCount, fmt.Errorf("failed to copy directory; %w", err)
	}

	return totalSize, fileCount, nil
}

// CopyToDir copies src to a file with the same name inside dstDir.
// This is a convenience function for the common case of copying a file
// into a directory while preserving its name.
func CopyToDir(src, dstDir string, force bool) (CopyResult, error) {
	dst := filepath.Join(dstDir, filepath.Base(src))
	return Copy(src, dst, force)
}
