package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/leefowlercu/agentic-memorizer/internal/providers"
)

// SemanticCache caches semantic analysis results.
type SemanticCache struct {
	config CacheConfig
	mu     sync.RWMutex
}

// NewSemanticCache creates a new semantic cache.
func NewSemanticCache(config CacheConfig) (*SemanticCache, error) {
	// Ensure cache directory exists
	cacheDir := filepath.Join(config.BaseDir, "semantic")
	if err := ensureDir(cacheDir); err != nil {
		return nil, fmt.Errorf("failed to create cache directory; %w", err)
	}

	return &SemanticCache{
		config: config,
	}, nil
}

// Get retrieves a cached semantic result by content hash.
func (c *SemanticCache) Get(contentHash string) (*providers.SemanticResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cacheDir := filepath.Join(c.config.BaseDir, "semantic")
	path := hashToPath(cacheDir, contentHash, fmt.Sprintf("-v%d.json", c.config.Version))

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("failed to read cache file; %w", err)
	}

	var result providers.SemanticResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache data; %w", err)
	}

	// Verify version
	if result.Version != c.config.Version {
		return nil, ErrVersionMismatch
	}

	return &result, nil
}

// Set stores a semantic result in the cache.
func (c *SemanticCache) Set(contentHash string, result *providers.SemanticResult) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cacheDir := filepath.Join(c.config.BaseDir, "semantic")
	path := hashToPath(cacheDir, contentHash, fmt.Sprintf("-v%d.json", c.config.Version))

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

	cacheDir := filepath.Join(c.config.BaseDir, "semantic")
	path := hashToPath(cacheDir, contentHash, fmt.Sprintf("-v%d.json", c.config.Version))

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete cache file; %w", err)
	}

	return nil
}

// Has checks if a cache entry exists.
func (c *SemanticCache) Has(contentHash string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cacheDir := filepath.Join(c.config.BaseDir, "semantic")
	path := hashToPath(cacheDir, contentHash, fmt.Sprintf("-v%d.json", c.config.Version))

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
