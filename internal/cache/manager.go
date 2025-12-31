package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// CacheStats provides statistics about the cache contents
type CacheStats struct {
	TotalEntries  int            `json:"total_entries"`
	LegacyEntries int            `json:"legacy_entries"`
	TotalSize     int64          `json:"total_size_bytes"`
	VersionCounts map[string]int `json:"version_counts"`
}

type Manager struct {
	cacheDir string
}

func NewManager(cacheDir string) (*Manager, error) {
	summariesDir := filepath.Join(cacheDir, "summaries")
	if err := os.MkdirAll(summariesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory; %w", err)
	}

	return &Manager{
		cacheDir: cacheDir,
	}, nil
}

func (m *Manager) Get(fileHash, provider string) (*types.CachedAnalysis, error) {
	// Try current version with provider subdirectory
	currentPath := m.getCachePath(fileHash, provider)
	if cached, err := m.readCacheFile(currentPath); err == nil && cached != nil {
		return cached, nil
	}

	// Try legacy format (no version suffix, no provider subdirectory)
	legacyPath := m.getLegacyCachePath(fileHash)
	if cached, err := m.readCacheFile(legacyPath); err == nil && cached != nil {
		// Mark as legacy (version 0.0.0) - fields default to 0 in JSON if not present
		return cached, nil
	}

	return nil, nil // Cache miss
}

// readCacheFile reads and unmarshals a cache file from the given path
func (m *Manager) readCacheFile(path string) (*types.CachedAnalysis, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read cache file; %w", err)
	}

	var cached types.CachedAnalysis
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache; %w", err)
	}

	return &cached, nil
}

func (m *Manager) Set(cached *types.CachedAnalysis) error {
	// Ensure version fields are set
	cached.SchemaVersion = CacheSchemaVersion
	cached.MetadataVersion = CacheMetadataVersion
	cached.SemanticVersion = CacheSemanticVersion

	// Ensure provider is set (required for subdirectory routing)
	if cached.Provider == "" {
		return fmt.Errorf("provider field is required in CachedAnalysis")
	}

	cachePath := m.getCachePath(cached.FileHash, cached.Provider)

	// Ensure provider subdirectory exists
	providerDir := filepath.Dir(cachePath)
	if err := os.MkdirAll(providerDir, 0755); err != nil {
		return fmt.Errorf("failed to create provider cache directory; %w", err)
	}

	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache; %w", err)
	}

	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file; %w", err)
	}

	return nil
}

// getCachePath returns the versioned cache path for a file hash with provider subdirectory.
// Uses two-level sharding to prevent filesystem performance degradation.
// Format: summaries/{provider}/{shard1}/{shard2}/{hash[:16]}-v{schema}-{metadata}-{semantic}.json
func (m *Manager) getCachePath(fileHash, provider string) string {
	filename := fmt.Sprintf("%s-v%d-%d-%d.json",
		fileHash[:16],
		CacheSchemaVersion,
		CacheMetadataVersion,
		CacheSemanticVersion,
	)
	basePath := filepath.Join(m.cacheDir, "summaries", provider)
	return ShardPath(basePath, fileHash, filename)
}

// getLegacyCachePath returns the legacy (unversioned) cache path for a file hash.
// Format: {hash[:16]}.json
func (m *Manager) getLegacyCachePath(fileHash string) string {
	filename := fileHash[:16] + ".json"
	return filepath.Join(m.cacheDir, "summaries", filename)
}

func HashFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file; %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to hash file; %w", err)
	}

	hashBytes := hasher.Sum(nil)
	return "sha256:" + hex.EncodeToString(hashBytes), nil
}

// IsStale checks if a cache entry is stale and needs re-analysis.
// Returns true if either:
//   - Content hash doesn't match (file was modified)
//   - Cache version is outdated (needs re-analysis with current logic)
func (m *Manager) IsStale(cached *types.CachedAnalysis, currentHash string) bool {
	// Content hash mismatch = stale
	if cached.FileHash != currentHash {
		return true
	}

	// Version mismatch = stale
	if IsStaleVersion(cached) {
		return true
	}

	return false
}

func (m *Manager) Clear() error {
	summariesDir := filepath.Join(m.cacheDir, "summaries")
	entries, err := os.ReadDir(summariesDir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory; %w", err)
	}

	for _, entry := range entries {
		path := filepath.Join(summariesDir, entry.Name())

		// If it's a provider subdirectory, remove it recursively
		if entry.IsDir() {
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("failed to remove provider cache directory; %w", err)
			}
		} else {
			// Legacy cache file (no provider subdirectory)
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove cache file; %w", err)
			}
		}
	}

	return nil
}

// GetStats returns statistics about the cache contents including version distribution.
// Recursively walks all subdirectories (provider and shard directories).
func (m *Manager) GetStats() (*CacheStats, error) {
	stats := &CacheStats{
		VersionCounts: make(map[string]int),
	}

	summariesDir := filepath.Join(m.cacheDir, "summaries")

	err := filepath.WalkDir(summariesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // Empty cache is valid
			}
			return nil // Skip unreadable entries
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Only process JSON files
		if !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}

		stats.TotalEntries++

		// Get file size
		info, err := d.Info()
		if err == nil {
			stats.TotalSize += info.Size()
		}

		// Parse version from filename
		version := parseVersionFromFilename(d.Name())
		stats.VersionCounts[version]++

		if version == "v0.0.0" {
			stats.LegacyEntries++
		}

		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to walk cache directory; %w", err)
	}

	return stats, nil
}

// parseVersionFromFilename extracts version string from cache filename.
// Returns "v0.0.0" for legacy format (no version suffix).
func parseVersionFromFilename(filename string) string {
	// New format: {hash}-v{schema}-{metadata}-{semantic}.json
	// Example: sha256:abc12345-v1-1-1.json
	if strings.Contains(filename, "-v") {
		// Extract version part after "-v"
		parts := strings.Split(filename, "-v")
		if len(parts) == 2 {
			versionPart := strings.TrimSuffix(parts[1], ".json")
			// Convert "1-1-1" to "v1.1.1"
			versionPart = strings.ReplaceAll(versionPart, "-", ".")
			return "v" + versionPart
		}
	}

	// Legacy format: {hash}.json
	return "v0.0.0"
}

// ClearOldVersions removes all cache entries that are not the current version.
// Recursively walks all subdirectories (provider and shard directories).
// Returns the number of entries removed.
func (m *Manager) ClearOldVersions() (int, error) {
	summariesDir := filepath.Join(m.cacheDir, "summaries")
	currentVersion := CacheVersion()
	removed := 0

	err := filepath.WalkDir(summariesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // Empty cache is valid
			}
			return nil // Skip unreadable entries
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Only process JSON files
		if !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}

		version := parseVersionFromFilename(d.Name())

		// Remove if not current version
		if version != currentVersion {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove cache file %s; %w", d.Name(), err)
			}
			removed++
		}

		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return removed, err
	}

	return removed, nil
}

// GetCacheDir returns the cache directory path.
func (m *Manager) GetCacheDir() string {
	return m.cacheDir
}
