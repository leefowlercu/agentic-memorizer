package skip

import (
	"path/filepath"
	"strings"
)

// AlwaysSkipDirs contains directories that are always skipped regardless of SkipHidden setting.
// These directories should never be indexed as they contain:
// - .git: Version control data (huge and constantly changing)
// - .cache: Our own cache directory
// - .forgotten: Soft-deleted files (prevents re-indexing)
var AlwaysSkipDirs = []string{".git", ".cache", ".forgotten"}

// Config contains skip pattern configuration.
// This is passed to walker and watcher for consistent skip behavior.
type Config struct {
	SkipHidden     bool     // Skip hidden files/dirs (default: true)
	SkipDirs       []string // Directories to skip by name
	SkipFiles      []string // Files to skip by name
	SkipExtensions []string // Extensions to skip (e.g., ".zip")
}

// ShouldSkipDir checks if a directory should be skipped based on config.
// Always skips AlwaysSkipDirs regardless of SkipHidden setting.
func ShouldSkipDir(name string, cfg *Config) bool {
	// Always skip protected directories
	for _, skip := range AlwaysSkipDirs {
		if name == skip {
			return true
		}
	}

	// Skip hidden directories if configured (but allow like .github, .vscode when disabled)
	if cfg.SkipHidden && strings.HasPrefix(name, ".") {
		return true
	}

	// Check user-configured skip directories
	for _, skipDir := range cfg.SkipDirs {
		if name == skipDir {
			return true
		}
	}

	return false
}

// ShouldSkipFile checks if a file should be skipped based on config.
// Does not check directory path - only the file itself.
func ShouldSkipFile(name string, cfg *Config) bool {
	// Skip hidden files if configured
	if cfg.SkipHidden && strings.HasPrefix(name, ".") {
		return true
	}

	// Check user-configured skip files
	for _, skipFile := range cfg.SkipFiles {
		if name == skipFile {
			return true
		}
	}

	// Check skip extensions
	ext := filepath.Ext(name)
	for _, skipExt := range cfg.SkipExtensions {
		if ext == skipExt {
			return true
		}
	}

	return false
}

// ShouldSkip checks if any path (file or directory) should be skipped.
// For directories, use isDir=true. For files, use isDir=false.
func ShouldSkip(name string, isDir bool, cfg *Config) bool {
	if isDir {
		return ShouldSkipDir(name, cfg)
	}
	return ShouldSkipFile(name, cfg)
}
