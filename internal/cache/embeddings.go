package cache

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/providers"
)

const (
	embCacheMagic = 0x454D4231 // "EMB1"
)

// EmbeddingsCache caches embedding vectors in binary format.
type EmbeddingsCache struct {
	config EmbeddingsCacheConfig
	mu     sync.RWMutex
	logger *slog.Logger
}

// NewEmbeddingsCache creates a new embeddings cache.
func NewEmbeddingsCache(config EmbeddingsCacheConfig) (*EmbeddingsCache, error) {
	cacheDir := filepath.Join(config.BaseDir, "embeddings", config.Provider, config.Model)
	if err := ensureDir(cacheDir); err != nil {
		return nil, fmt.Errorf("failed to create cache directory; %w", err)
	}

	// Default version if not set
	if config.Version == 0 {
		config.Version = EmbeddingsCacheVersion
	}

	return &EmbeddingsCache{
		config: config,
		logger: slog.Default().With("component", "embeddings-cache"),
	}, nil
}

// NewEmbeddingsCacheWithDefaults creates an embeddings cache with default settings.
func NewEmbeddingsCacheWithDefaults(provider, model string) (*EmbeddingsCache, error) {
	return NewEmbeddingsCache(EmbeddingsCacheConfig{
		BaseDir:  GetCacheBaseDir(),
		Version:  EmbeddingsCacheVersion,
		Provider: provider,
		Model:    model,
	})
}

// Get retrieves a cached embedding by content hash and chunk index.
// If the cache entry is corrupt, it is deleted (self-healing).
func (c *EmbeddingsCache) Get(contentHash string, chunkIndex int) (*providers.EmbeddingsResult, error) {
	c.mu.RLock()
	path := c.getPath(contentHash, chunkIndex)

	file, err := os.Open(path)
	if err != nil {
		c.mu.RUnlock()
		if os.IsNotExist(err) {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("failed to open cache file; %w", err)
	}

	result, err := readEmbeddingFile(file)
	file.Close()
	c.mu.RUnlock()

	if err != nil {
		// Self-healing: delete corrupt entry
		c.logger.Warn("deleting corrupt cache entry",
			"hash", contentHash,
			"chunk", chunkIndex,
			"error", err)
		_ = c.deleteInternal(path)
		return nil, ErrCacheMiss
	}

	return result, nil
}

// deleteInternal removes a cache file without locking (caller must handle locking).
func (c *EmbeddingsCache) deleteInternal(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Set stores an embedding in the cache.
func (c *EmbeddingsCache) Set(contentHash string, chunkIndex int, result *providers.EmbeddingsResult) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	path := c.getPath(contentHash, chunkIndex)

	// Ensure directory exists
	if err := ensureDir(pathToDir(path)); err != nil {
		return fmt.Errorf("failed to create cache directory; %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create cache file; %w", err)
	}
	defer file.Close()

	return writeEmbeddingFile(file, result)
}

// Delete removes a cached entry.
func (c *EmbeddingsCache) Delete(contentHash string, chunkIndex int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	path := c.getPath(contentHash, chunkIndex)

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete cache file; %w", err)
	}

	return nil
}

// Has checks if a cache entry exists.
func (c *EmbeddingsCache) Has(contentHash string, chunkIndex int) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	path := c.getPath(contentHash, chunkIndex)
	_, err := os.Stat(path)
	return err == nil
}

// Clear removes all cached entries for this provider/model.
func (c *EmbeddingsCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cacheDir := c.getCacheDir()
	if err := os.RemoveAll(cacheDir); err != nil {
		return fmt.Errorf("failed to clear cache; %w", err)
	}

	return ensureDir(cacheDir)
}

// Stats returns cache statistics.
func (c *EmbeddingsCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cacheDir := c.getCacheDir()
	stats := CacheStats{}

	_ = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Skip files we can't access
		}
		if !info.IsDir() {
			stats.EntryCount++
			stats.TotalSize += info.Size()
		}
		return nil
	})

	return stats
}

// getCacheDir returns the cache directory for this provider/model.
func (c *EmbeddingsCache) getCacheDir() string {
	return filepath.Join(c.config.BaseDir, "embeddings", c.config.Provider, c.config.Model)
}

// getPath returns the cache file path for a content hash and chunk.
func (c *EmbeddingsCache) getPath(contentHash string, chunkIndex int) string {
	cacheDir := c.getCacheDir()
	suffix := fmt.Sprintf("-chunk-%d-v%d.emb", chunkIndex, c.config.Version)
	return hashToPath(cacheDir, contentHash, suffix)
}

// Version returns the cache version.
func (c *EmbeddingsCache) Version() int {
	return c.config.Version
}

// BaseDir returns the base cache directory.
func (c *EmbeddingsCache) BaseDir() string {
	return c.config.BaseDir
}

// Provider returns the provider name.
func (c *EmbeddingsCache) Provider() string {
	return c.config.Provider
}

// Model returns the model name.
func (c *EmbeddingsCache) Model() string {
	return c.config.Model
}

// embeddingHeader is the binary file header.
type embeddingHeader struct {
	Magic      uint32
	Version    uint16
	Dimensions uint16
	Timestamp  int64
}

// writeEmbeddingFile writes an embedding to a binary file.
func writeEmbeddingFile(w io.Writer, result *providers.EmbeddingsResult) error {
	// Write header
	header := embeddingHeader{
		Magic:      embCacheMagic,
		Version:    uint16(EmbeddingsCacheVersion),
		Dimensions: uint16(result.Dimensions),
		Timestamp:  result.GeneratedAt.Unix(),
	}

	if err := binary.Write(w, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("failed to write header; %w", err)
	}

	// Write embedding data
	for _, v := range result.Embedding {
		if err := binary.Write(w, binary.LittleEndian, v); err != nil {
			return fmt.Errorf("failed to write embedding; %w", err)
		}
	}

	return nil
}

// readEmbeddingFile reads an embedding from a binary file.
func readEmbeddingFile(r io.Reader) (*providers.EmbeddingsResult, error) {
	// Read header
	var header embeddingHeader
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("failed to read header; %w", err)
	}

	// Verify magic
	if header.Magic != embCacheMagic {
		return nil, fmt.Errorf("invalid cache file magic")
	}

	// Read embedding data
	embedding := make([]float32, header.Dimensions)
	for i := range embedding {
		if err := binary.Read(r, binary.LittleEndian, &embedding[i]); err != nil {
			return nil, fmt.Errorf("failed to read embedding; %w", err)
		}
	}

	return &providers.EmbeddingsResult{
		Embedding:   embedding,
		Dimensions:  int(header.Dimensions),
		GeneratedAt: time.Unix(header.Timestamp, 0),
		Version:     int(header.Version),
	}, nil
}
