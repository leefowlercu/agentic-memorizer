package cache

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Cache version constants.
const (
	// SemanticCacheVersion is the current semantic analysis cache version (semver).
	// Increment major for wholesale changes, minor for structure changes, patch for prompt changes.
	SemanticCacheVersion = "1.0.0"

	// EmbeddingsCacheVersion is the current embeddings cache version (integer).
	EmbeddingsCacheVersion = 1
)

var (
	// ErrCacheMiss is returned when an entry is not found in the cache.
	ErrCacheMiss = errors.New("cache miss")

	// ErrVersionMismatch is returned when the cached version doesn't match.
	ErrVersionMismatch = errors.New("version mismatch")

	// ErrCorruptCache is returned when a cache entry is corrupted.
	ErrCorruptCache = errors.New("corrupt cache entry")
)

// SemanticCacheConfig contains configuration for the semantic cache.
type SemanticCacheConfig struct {
	// BaseDir is the base directory for cache storage.
	BaseDir string

	// MaxSize is the maximum total cache size in bytes (0 = unlimited).
	MaxSize int64

	// Version is the semantic cache version (semver format).
	Version string
}

// EmbeddingsCacheConfig contains configuration for the embeddings cache.
type EmbeddingsCacheConfig struct {
	// BaseDir is the base directory for cache storage.
	BaseDir string

	// MaxSize is the maximum total cache size in bytes (0 = unlimited).
	MaxSize int64

	// Version is the embeddings cache version (integer).
	Version int

	// Provider is the embeddings provider name.
	Provider string

	// Model is the embeddings model name.
	Model string
}

// CacheConfig contains configuration for the cache (deprecated, use specific configs).
type CacheConfig struct {
	// BaseDir is the base directory for cache storage.
	BaseDir string

	// MaxSize is the maximum total cache size in bytes (0 = unlimited).
	MaxSize int64

	// Version is the current analysis/embedding version.
	Version int
}

// GetCacheBaseDir returns the base directory for cache storage.
// It follows XDG Base Directory Specification with fallback to config directory.
//
// Resolution order:
// 1. $XDG_CACHE_HOME/memorizer (if XDG_CACHE_HOME is set)
// 2. $MEMORIZER_CONFIG_DIR/cache (if MEMORIZER_CONFIG_DIR is set)
// 3. ~/.config/memorizer/cache (default fallback)
func GetCacheBaseDir() string {
	// Check XDG_CACHE_HOME first
	if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
		return filepath.Join(xdgCache, "memorizer")
	}

	// Check MEMORIZER_CONFIG_DIR
	if configDir := os.Getenv("MEMORIZER_CONFIG_DIR"); configDir != "" {
		return filepath.Join(configDir, "cache")
	}

	// Default fallback to ~/.config/memorizer/cache
	home, err := os.UserHomeDir()
	if err != nil {
		// Last resort fallback
		return filepath.Join(".", ".cache", "memorizer")
	}

	return filepath.Join(home, ".config", "memorizer", "cache")
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
