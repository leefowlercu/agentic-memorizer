package embeddings

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestEncodeDecodeEmbedding(t *testing.T) {
	tests := []struct {
		name      string
		embedding []float32
	}{
		{
			name:      "empty embedding",
			embedding: []float32{},
		},
		{
			name:      "single value",
			embedding: []float32{1.5},
		},
		{
			name:      "typical embedding",
			embedding: []float32{0.1, -0.2, 0.3, -0.4, 0.5},
		},
		{
			name:      "high dimension embedding",
			embedding: make([]float32, 1536),
		},
		{
			name:      "special values",
			embedding: []float32{0.0, -0.0, 1e-10, -1e-10, 1e10, -1e10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize high dimension embedding with values
			if len(tt.embedding) == 1536 {
				for i := range tt.embedding {
					tt.embedding[i] = float32(i) / 1536.0
				}
			}

			encoded := encodeEmbedding(tt.embedding)
			decoded, err := decodeEmbedding(encoded)

			if err != nil {
				t.Fatalf("decodeEmbedding failed: %v", err)
			}

			if !reflect.DeepEqual(decoded, tt.embedding) {
				t.Errorf("round-trip failed: got %v, want %v", decoded, tt.embedding)
			}
		})
	}
}

func TestDecodeEmbeddingErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "too short",
			data: []byte{1, 2, 3},
		},
		{
			name: "wrong length",
			data: []byte{2, 0, 0, 0, 1, 2, 3, 4}, // Claims 2 floats but only has 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeEmbedding(tt.data)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestCacheOperations(t *testing.T) {
	// Create temp directory for cache
	tempDir, err := os.MkdirTemp("", "embedding-cache-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cache, err := NewCache(tempDir, "openai", nil)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	testHash := "abc123def456"
	testEmbedding := []float32{0.1, 0.2, 0.3, 0.4, 0.5}

	// Test cache miss
	t.Run("cache miss", func(t *testing.T) {
		_, found := cache.Get(testHash)
		if found {
			t.Error("expected cache miss, got hit")
		}
	})

	// Test set and get
	t.Run("set and get", func(t *testing.T) {
		if err := cache.Set(testHash, testEmbedding); err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		retrieved, found := cache.Get(testHash)
		if !found {
			t.Fatal("expected cache hit, got miss")
		}

		if !reflect.DeepEqual(retrieved, testEmbedding) {
			t.Errorf("got %v, want %v", retrieved, testEmbedding)
		}
	})

	// Test delete
	t.Run("delete", func(t *testing.T) {
		if err := cache.Delete(testHash); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		_, found := cache.Get(testHash)
		if found {
			t.Error("expected cache miss after delete, got hit")
		}
	})

	// Test clear
	t.Run("clear", func(t *testing.T) {
		// Add multiple entries
		for i := 0; i < 5; i++ {
			hash := filepath.Join("hash", string(rune('a'+i)))
			if err := cache.Set(hash, testEmbedding); err != nil {
				t.Fatalf("Set failed: %v", err)
			}
		}

		if err := cache.Clear(); err != nil {
			t.Fatalf("Clear failed: %v", err)
		}

		// Verify all entries are gone
		for i := 0; i < 5; i++ {
			hash := filepath.Join("hash", string(rune('a'+i)))
			_, found := cache.Get(hash)
			if found {
				t.Errorf("expected cache miss for %s after clear, got hit", hash)
			}
		}
	})
}

func TestCachePath(t *testing.T) {
	cache := &Cache{dir: "/tmp/cache", provider: "openai"}

	tests := []struct {
		hash     string
		expected string
	}{
		{
			hash:     "ab",
			expected: "/tmp/cache/embeddings/openai/ab.emb",
		},
		{
			hash:     "abc",
			expected: "/tmp/cache/embeddings/openai/abc.emb",
		},
		{
			hash:     "abcd1234",
			expected: "/tmp/cache/embeddings/openai/ab/cd/abcd1234.emb",
		},
		{
			// sha256: prefix should be skipped for sharding, using "41" and "d6" from hash value
			hash:     "sha256:41d63309faf26bf95fba3b553dba9fd793effe0116d9237a012499587f8ce94b",
			expected: "/tmp/cache/embeddings/openai/41/d6/sha256:41d63309faf26bf95fba3b553dba9fd793effe0116d9237a012499587f8ce94b.emb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.hash, func(t *testing.T) {
			got := cache.cachePath(tt.hash)
			if got != tt.expected {
				t.Errorf("got %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestCacheProviderSegregation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "embedding-cache-segregation-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create caches for different providers
	openaiCache, err := NewCache(tempDir, "openai", nil)
	if err != nil {
		t.Fatalf("failed to create openai cache: %v", err)
	}

	voyageCache, err := NewCache(tempDir, "voyage", nil)
	if err != nil {
		t.Fatalf("failed to create voyage cache: %v", err)
	}

	testHash := "abc123def456"
	openaiEmbedding := []float32{0.1, 0.2, 0.3}
	voyageEmbedding := []float32{0.4, 0.5, 0.6}

	// Store different embeddings for same hash in different providers
	if err := openaiCache.Set(testHash, openaiEmbedding); err != nil {
		t.Fatalf("Set openai failed: %v", err)
	}
	if err := voyageCache.Set(testHash, voyageEmbedding); err != nil {
		t.Fatalf("Set voyage failed: %v", err)
	}

	// Verify each cache returns its own embedding
	gotOpenAI, found := openaiCache.Get(testHash)
	if !found {
		t.Fatal("expected openai cache hit")
	}
	if !reflect.DeepEqual(gotOpenAI, openaiEmbedding) {
		t.Errorf("openai cache: got %v, want %v", gotOpenAI, openaiEmbedding)
	}

	gotVoyage, found := voyageCache.Get(testHash)
	if !found {
		t.Fatal("expected voyage cache hit")
	}
	if !reflect.DeepEqual(gotVoyage, voyageEmbedding) {
		t.Errorf("voyage cache: got %v, want %v", gotVoyage, voyageEmbedding)
	}

	// Clear openai cache should not affect voyage cache
	if err := openaiCache.Clear(); err != nil {
		t.Fatalf("Clear openai failed: %v", err)
	}

	_, found = openaiCache.Get(testHash)
	if found {
		t.Error("expected openai cache miss after clear")
	}

	gotVoyage, found = voyageCache.Get(testHash)
	if !found {
		t.Fatal("expected voyage cache hit after openai clear")
	}
	if !reflect.DeepEqual(gotVoyage, voyageEmbedding) {
		t.Errorf("voyage cache after openai clear: got %v, want %v", gotVoyage, voyageEmbedding)
	}
}
