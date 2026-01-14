package cache

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/leefowlercu/agentic-memorizer/internal/providers"
)

// SemanticCache caches semantic analysis results.
type SemanticCache struct {
	config SemanticCacheConfig
	mu     sync.RWMutex
	logger *slog.Logger
}

// NewSemanticCache creates a new semantic cache with the given configuration.
func NewSemanticCache(config SemanticCacheConfig) (*SemanticCache, error) {
	// Ensure cache directory exists
	cacheDir := filepath.Join(config.BaseDir, "semantic")
	if err := ensureDir(cacheDir); err != nil {
		return nil, fmt.Errorf("failed to create cache directory; %w", err)
	}

	// Default version if not set
	if config.Version == "" {
		config.Version = SemanticCacheVersion
	}

	return &SemanticCache{
		config: config,
		logger: slog.Default().With("component", "semantic-cache"),
	}, nil
}

// NewSemanticCacheWithDefaults creates a semantic cache with default settings.
func NewSemanticCacheWithDefaults() (*SemanticCache, error) {
	return NewSemanticCache(SemanticCacheConfig{
		BaseDir: GetCacheBaseDir(),
		Version: SemanticCacheVersion,
	})
}

// Get retrieves a cached semantic result by content hash.
// If the cache entry is corrupt or has a version mismatch, it is deleted (self-healing).
func (c *SemanticCache) Get(contentHash string) (*providers.SemanticResult, error) {
	c.mu.RLock()
	path := c.getPath(contentHash)
	data, err := os.ReadFile(path)
	c.mu.RUnlock()

	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("failed to read cache file; %w", err)
	}

	var result providers.SemanticResult
	if err := json.Unmarshal(data, &result); err != nil {
		// Self-healing: delete corrupt entry
		c.logger.Warn("deleting corrupt cache entry",
			"hash", contentHash,
			"error", err)
		_ = c.deleteInternal(path)
		return nil, ErrCacheMiss
	}

	// Version is stored as int in SemanticResult but we use semver string
	// We embed version info in filename, so if file exists with correct name, version matches
	return &result, nil
}

// deleteInternal removes a cache file without locking (caller must handle locking).
func (c *SemanticCache) deleteInternal(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// getPath returns the cache file path for a content hash.
func (c *SemanticCache) getPath(contentHash string) string {
	cacheDir := filepath.Join(c.config.BaseDir, "semantic")
	return hashToPath(cacheDir, contentHash, fmt.Sprintf("-v%s.json", c.config.Version))
}

// Set stores a semantic result in the cache.
func (c *SemanticCache) Set(contentHash string, result *providers.SemanticResult) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	path := c.getPath(contentHash)

	// Ensure directory exists
	if err := ensureDir(pathToDir(path)); err != nil {
		return fmt.Errorf("failed to create cache directory; %w", err)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result; %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file; %w", err)
	}

	return nil
}

// Delete removes a cached entry.
func (c *SemanticCache) Delete(contentHash string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	path := c.getPath(contentHash)

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete cache file; %w", err)
	}

	return nil
}

// Has checks if a cache entry exists.
func (c *SemanticCache) Has(contentHash string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	path := c.getPath(contentHash)

	_, err := os.Stat(path)
	return err == nil
}

// Clear removes all cached entries.
func (c *SemanticCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cacheDir := filepath.Join(c.config.BaseDir, "semantic")
	if err := os.RemoveAll(cacheDir); err != nil {
		return fmt.Errorf("failed to clear cache; %w", err)
	}

	return ensureDir(cacheDir)
}

// Stats returns cache statistics.
func (c *SemanticCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cacheDir := filepath.Join(c.config.BaseDir, "semantic")
	stats := CacheStats{}

	_ = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			stats.EntryCount++
			stats.TotalSize += info.Size()
		}
		return nil
	})

	return stats
}

// CacheStats contains cache statistics.
type CacheStats struct {
	EntryCount int64
	TotalSize  int64
}

// Version returns the cache version string.
func (c *SemanticCache) Version() string {
	return c.config.Version
}

// BaseDir returns the base cache directory.
func (c *SemanticCache) BaseDir() string {
	return c.config.BaseDir
}
