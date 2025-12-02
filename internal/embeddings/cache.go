package embeddings

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sync"
)

// Cache provides content-hash-based caching for embeddings
type Cache struct {
	dir    string
	logger *slog.Logger
	mu     sync.RWMutex
}

// NewCache creates a new embedding cache
func NewCache(dir string, logger *slog.Logger) (*Cache, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory; %w", err)
	}

	return &Cache{
		dir:    dir,
		logger: logger.With("component", "embedding-cache"),
	}, nil
}

// Get retrieves an embedding from cache by content hash
// Returns the embedding and true if found, nil and false otherwise
func (c *Cache) Get(hash string) ([]float32, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	path := c.cachePath(hash)
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			c.logger.Warn("failed to read embedding cache", "hash", hash, "error", err)
		}
		return nil, false
	}

	embedding, err := decodeEmbedding(data)
	if err != nil {
		c.logger.Warn("failed to decode cached embedding", "hash", hash, "error", err)
		return nil, false
	}

	c.logger.Debug("embedding cache hit", "hash", hash)
	return embedding, true
}

// Set stores an embedding in cache by content hash
func (c *Cache) Set(hash string, embedding []float32) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	path := c.cachePath(hash)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create cache subdirectory; %w", err)
	}

	data := encodeEmbedding(embedding)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write embedding cache; %w", err)
	}

	c.logger.Debug("embedding cached", "hash", hash, "dimensions", len(embedding))
	return nil
}

// Delete removes an embedding from cache
func (c *Cache) Delete(hash string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	path := c.cachePath(hash)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete embedding cache; %w", err)
	}
	return nil
}

// Clear removes all cached embeddings
func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	embeddingsDir := filepath.Join(c.dir, "embeddings")
	if err := os.RemoveAll(embeddingsDir); err != nil {
		return fmt.Errorf("failed to clear embedding cache; %w", err)
	}
	return nil
}

// cachePath returns the cache file path for a given hash
// Uses a two-level directory structure to avoid too many files in one directory
func (c *Cache) cachePath(hash string) string {
	if len(hash) < 4 {
		return filepath.Join(c.dir, "embeddings", hash+".emb")
	}
	return filepath.Join(c.dir, "embeddings", hash[:2], hash[2:4], hash+".emb")
}

// encodeEmbedding converts a float32 slice to bytes
// Format: [dimension_count (uint32)] [float32 values as little-endian bytes]
func encodeEmbedding(embedding []float32) []byte {
	data := make([]byte, 4+len(embedding)*4)
	binary.LittleEndian.PutUint32(data[:4], uint32(len(embedding)))
	for i, v := range embedding {
		binary.LittleEndian.PutUint32(data[4+i*4:], math.Float32bits(v))
	}
	return data
}

// decodeEmbedding converts bytes back to a float32 slice
func decodeEmbedding(data []byte) ([]float32, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("invalid embedding data: too short")
	}

	count := binary.LittleEndian.Uint32(data[:4])
	expectedLen := 4 + int(count)*4
	if len(data) != expectedLen {
		return nil, fmt.Errorf("invalid embedding data: expected %d bytes, got %d", expectedLen, len(data))
	}

	embedding := make([]float32, count)
	for i := range embedding {
		bits := binary.LittleEndian.Uint32(data[4+i*4:])
		embedding[i] = math.Float32frombits(bits)
	}
	return embedding, nil
}
