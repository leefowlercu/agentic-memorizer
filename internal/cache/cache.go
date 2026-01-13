package cache

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrCacheMiss is returned when an entry is not found in the cache.
	ErrCacheMiss = errors.New("cache miss")

	// ErrVersionMismatch is returned when the cached version doesn't match.
	ErrVersionMismatch = errors.New("version mismatch")
)

// CacheConfig contains configuration for the cache.
type CacheConfig struct {
	// BaseDir is the base directory for cache storage.
	BaseDir string

	// MaxSize is the maximum total cache size in bytes (0 = unlimited).
	MaxSize int64

	// Version is the current analysis/embedding version.
	Version int
}

// ensureDir creates a directory if it doesn't exist.
func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// hashToPath converts a content hash to a cache file path with fan-out.
// Uses 2-level directory fan-out: xx/yy/full_hash
func hashToPath(baseDir, hash, suffix string) string {
	// Remove algorithm prefix if present (e.g., "sha256:")
	cleanHash := hash
	if idx := strings.Index(hash, ":"); idx != -1 {
		cleanHash = hash[idx+1:]
	}

	// Ensure hash is long enough
	if len(cleanHash) < 4 {
		return filepath.Join(baseDir, cleanHash+suffix)
	}

	// 2-level fan-out
	level1 := cleanHash[:2]
	level2 := cleanHash[2:4]

	return filepath.Join(baseDir, level1, level2, cleanHash+suffix)
}

// pathToDir returns the directory part of a cache path.
func pathToDir(path string) string {
	return filepath.Dir(path)
}
