package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// Manager handles caching of semantic analysis results
type Manager struct {
	cacheDir string
}

// NewManager creates a new cache manager
func NewManager(cacheDir string) (*Manager, error) {
	// Create cache directories
	summariesDir := filepath.Join(cacheDir, "summaries")
	if err := os.MkdirAll(summariesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Manager{
		cacheDir: cacheDir,
	}, nil
}

// Get retrieves a cached analysis by file hash
func (m *Manager) Get(fileHash string) (*types.CachedAnalysis, error) {
	cachePath := m.getCachePath(fileHash)

	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Not cached
		}
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var cached types.CachedAnalysis
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache: %w", err)
	}

	return &cached, nil
}

// Set stores an analysis in the cache
func (m *Manager) Set(cached *types.CachedAnalysis) error {
	cachePath := m.getCachePath(cached.FileHash)

	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// getCachePath returns the cache file path for a given file hash
func (m *Manager) getCachePath(fileHash string) string {
	// Use first 16 chars of hash as filename
	filename := fileHash[:16] + ".json"
	return filepath.Join(m.cacheDir, "summaries", filename)
}

// HashFile computes the SHA-256 hash of a file
func HashFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	hashBytes := hasher.Sum(nil)
	return "sha256:" + hex.EncodeToString(hashBytes), nil
}

// IsStale checks if cached analysis is stale (file has been modified)
func (m *Manager) IsStale(cached *types.CachedAnalysis, currentHash string) bool {
	return cached.FileHash != currentHash
}

// Clear removes all cached analyses
func (m *Manager) Clear() error {
	summariesDir := filepath.Join(m.cacheDir, "summaries")
	entries, err := os.ReadDir(summariesDir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			path := filepath.Join(summariesDir, entry.Name())
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove cache file: %w", err)
			}
		}
	}

	return nil
}
