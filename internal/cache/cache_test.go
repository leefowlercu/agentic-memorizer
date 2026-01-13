package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/providers"
)

func TestHashToPath(t *testing.T) {
	tests := []struct {
		name     string
		baseDir  string
		hash     string
		suffix   string
		expected string
	}{
		{
			name:     "normal hash",
			baseDir:  "/cache",
			hash:     "abcdef1234567890",
			suffix:   ".json",
			expected: filepath.Join("/cache", "ab", "cd", "abcdef1234567890.json"),
		},
		{
			name:     "hash with prefix",
			baseDir:  "/cache",
			hash:     "sha256:abcdef1234567890",
			suffix:   ".json",
			expected: filepath.Join("/cache", "ab", "cd", "abcdef1234567890.json"),
		},
		{
			name:     "short hash",
			baseDir:  "/cache",
			hash:     "abc",
			suffix:   ".json",
			expected: filepath.Join("/cache", "abc.json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hashToPath(tt.baseDir, tt.hash, tt.suffix)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestSemanticCache_GetSet(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		BaseDir: tmpDir,
		Version: 1,
	}

	cache, err := NewSemanticCache(config)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Test cache miss
	_, err = cache.Get("nonexistent")
	if err != ErrCacheMiss {
		t.Errorf("expected ErrCacheMiss, got %v", err)
	}

	// Test set and get
	result := &providers.SemanticResult{
		Summary: "test summary",
		Topics: []providers.Topic{
			{Name: "topic1", Confidence: 0.9},
			{Name: "topic2", Confidence: 0.8},
		},
		Version: 1,
	}

	hash := "abcdef1234567890"
	if err := cache.Set(hash, result); err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	got, err := cache.Get(hash)
	if err != nil {
		t.Fatalf("failed to get cache: %v", err)
	}

	if got.Summary != result.Summary {
		t.Errorf("expected summary %q, got %q", result.Summary, got.Summary)
	}
	if len(got.Topics) != len(result.Topics) {
		t.Errorf("expected %d topics, got %d", len(result.Topics), len(got.Topics))
	}
}

func TestSemanticCache_Has(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		BaseDir: tmpDir,
		Version: 1,
	}

	cache, err := NewSemanticCache(config)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	hash := "abcdef1234567890"

	// Should not exist initially
	if cache.Has(hash) {
		t.Error("expected cache to not have entry")
	}

	// Add entry
	result := &providers.SemanticResult{Summary: "test", Version: 1}
	_ = cache.Set(hash, result)

	// Should exist now
	if !cache.Has(hash) {
		t.Error("expected cache to have entry")
	}
}

func TestSemanticCache_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		BaseDir: tmpDir,
		Version: 1,
	}

	cache, err := NewSemanticCache(config)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	hash := "abcdef1234567890"
	result := &providers.SemanticResult{Summary: "test", Version: 1}
	_ = cache.Set(hash, result)

	// Delete
	if err := cache.Delete(hash); err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	// Should not exist
	if cache.Has(hash) {
		t.Error("expected cache entry to be deleted")
	}
}

func TestSemanticCache_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		BaseDir: tmpDir,
		Version: 1,
	}

	cache, err := NewSemanticCache(config)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Add entries
	for i := 0; i < 5; i++ {
		hash := filepath.Join("hash", string(rune('a'+i)))
		result := &providers.SemanticResult{Summary: "test", Version: 1}
		_ = cache.Set(hash, result)
	}

	// Clear
	if err := cache.Clear(); err != nil {
		t.Fatalf("failed to clear: %v", err)
	}

	// Verify directory still exists but is empty
	cacheDir := filepath.Join(tmpDir, "semantic")
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("failed to read cache dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty cache dir, got %d entries", len(entries))
	}
}

func TestSemanticCache_DifferentVersion(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cache with version 1
	config1 := CacheConfig{
		BaseDir: tmpDir,
		Version: 1,
	}
	cache1, _ := NewSemanticCache(config1)

	hash := "abcdef1234567890"
	result := &providers.SemanticResult{Summary: "test", Version: 1}
	_ = cache1.Set(hash, result)

	// Create cache with version 2
	config2 := CacheConfig{
		BaseDir: tmpDir,
		Version: 2,
	}
	cache2, _ := NewSemanticCache(config2)

	// Version is in file path, so different version = cache miss
	_, err := cache2.Get(hash)
	if err != ErrCacheMiss {
		t.Errorf("expected ErrCacheMiss for different version, got %v", err)
	}
}

func TestEmbeddingsCache_GetSet(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		BaseDir: tmpDir,
		Version: 1,
	}

	cache, err := NewEmbeddingsCache(config, "openai", "text-embedding-3-small")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Test cache miss
	_, err = cache.Get("nonexistent", 0)
	if err != ErrCacheMiss {
		t.Errorf("expected ErrCacheMiss, got %v", err)
	}

	// Test set and get
	result := &providers.EmbeddingsResult{
		Embedding:   []float32{0.1, 0.2, 0.3, 0.4, 0.5},
		Dimensions:  5,
		GeneratedAt: time.Now(),
		Version:     1,
	}

	hash := "abcdef1234567890"
	if err := cache.Set(hash, 0, result); err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	got, err := cache.Get(hash, 0)
	if err != nil {
		t.Fatalf("failed to get cache: %v", err)
	}

	if got.Dimensions != result.Dimensions {
		t.Errorf("expected dimensions %d, got %d", result.Dimensions, got.Dimensions)
	}
	if len(got.Embedding) != len(result.Embedding) {
		t.Errorf("expected %d embedding values, got %d", len(result.Embedding), len(got.Embedding))
	}
	for i := range result.Embedding {
		if got.Embedding[i] != result.Embedding[i] {
			t.Errorf("embedding[%d]: expected %f, got %f", i, result.Embedding[i], got.Embedding[i])
		}
	}
}

func TestEmbeddingsCache_ChunkIndex(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		BaseDir: tmpDir,
		Version: 1,
	}

	cache, err := NewEmbeddingsCache(config, "openai", "text-embedding-3-small")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	hash := "abcdef1234567890"

	// Store different chunks
	for i := 0; i < 3; i++ {
		result := &providers.EmbeddingsResult{
			Embedding:   []float32{float32(i)},
			Dimensions:  1,
			GeneratedAt: time.Now(),
			Version:     1,
		}
		if err := cache.Set(hash, i, result); err != nil {
			t.Fatalf("failed to set chunk %d: %v", i, err)
		}
	}

	// Verify each chunk
	for i := 0; i < 3; i++ {
		got, err := cache.Get(hash, i)
		if err != nil {
			t.Fatalf("failed to get chunk %d: %v", i, err)
		}
		if got.Embedding[0] != float32(i) {
			t.Errorf("chunk %d: expected embedding[0]=%f, got %f", i, float32(i), got.Embedding[0])
		}
	}
}

func TestEmbeddingsCache_Has(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		BaseDir: tmpDir,
		Version: 1,
	}

	cache, err := NewEmbeddingsCache(config, "openai", "text-embedding-3-small")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	hash := "abcdef1234567890"

	// Should not exist initially
	if cache.Has(hash, 0) {
		t.Error("expected cache to not have entry")
	}

	// Add entry
	result := &providers.EmbeddingsResult{
		Embedding:   []float32{0.1},
		Dimensions:  1,
		GeneratedAt: time.Now(),
		Version:     1,
	}
	_ = cache.Set(hash, 0, result)

	// Should exist now
	if !cache.Has(hash, 0) {
		t.Error("expected cache to have entry")
	}

	// Different chunk should not exist
	if cache.Has(hash, 1) {
		t.Error("expected cache to not have different chunk")
	}
}

func TestEmbeddingsCache_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		BaseDir: tmpDir,
		Version: 1,
	}

	cache, err := NewEmbeddingsCache(config, "openai", "text-embedding-3-small")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	hash := "abcdef1234567890"
	result := &providers.EmbeddingsResult{
		Embedding:   []float32{0.1},
		Dimensions:  1,
		GeneratedAt: time.Now(),
		Version:     1,
	}
	_ = cache.Set(hash, 0, result)

	// Delete
	if err := cache.Delete(hash, 0); err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	// Should not exist
	if cache.Has(hash, 0) {
		t.Error("expected cache entry to be deleted")
	}
}

func TestEmbeddingsCache_Stats(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		BaseDir: tmpDir,
		Version: 1,
	}

	cache, err := NewEmbeddingsCache(config, "openai", "text-embedding-3-small")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Initially empty
	stats := cache.Stats()
	if stats.EntryCount != 0 {
		t.Errorf("expected 0 entries, got %d", stats.EntryCount)
	}

	// Add entries
	for i := 0; i < 3; i++ {
		hash := string(rune('a' + i))
		result := &providers.EmbeddingsResult{
			Embedding:   []float32{0.1, 0.2, 0.3},
			Dimensions:  3,
			GeneratedAt: time.Now(),
			Version:     1,
		}
		_ = cache.Set(hash+hash+hash+hash, 0, result) // Make hash long enough for fan-out
	}

	stats = cache.Stats()
	if stats.EntryCount != 3 {
		t.Errorf("expected 3 entries, got %d", stats.EntryCount)
	}
	if stats.TotalSize == 0 {
		t.Error("expected non-zero total size")
	}
}
