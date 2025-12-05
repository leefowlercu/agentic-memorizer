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

func (m *Manager) Get(fileHash string) (*types.CachedAnalysis, error) {
	// Try current version first
	currentPath := m.getCachePath(fileHash)
	if cached, err := m.readCacheFile(currentPath); err == nil && cached != nil {
		return cached, nil
	}

	// Try legacy format (no version suffix)
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

	cachePath := m.getCachePath(cached.FileHash)

	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache; %w", err)
	}

	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file; %w", err)
	}

	return nil
}

// getCachePath returns the versioned cache path for a file hash.
// Format: {hash[:16]}-v{schema}-{metadata}-{semantic}.json
func (m *Manager) getCachePath(fileHash string) string {
	filename := fmt.Sprintf("%s-v%d-%d-%d.json",
		fileHash[:16],
		CacheSchemaVersion,
		CacheMetadataVersion,
		CacheSemanticVersion,
	)
	return filepath.Join(m.cacheDir, "summaries", filename)
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
		if !entry.IsDir() {
			path := filepath.Join(summariesDir, entry.Name())
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove cache file; %w", err)
			}
		}
	}

	return nil
}

// GetStats returns statistics about the cache contents including version distribution.
func (m *Manager) GetStats() (*CacheStats, error) {
	stats := &CacheStats{
		VersionCounts: make(map[string]int),
	}

	summariesDir := filepath.Join(m.cacheDir, "summaries")
	entries, err := os.ReadDir(summariesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return stats, nil // Empty cache is valid
		}
		return nil, fmt.Errorf("failed to read cache directory; %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		stats.TotalEntries++

		// Get file size
		info, err := entry.Info()
		if err == nil {
			stats.TotalSize += info.Size()
		}

		// Parse version from filename
		version := parseVersionFromFilename(entry.Name())
		stats.VersionCounts[version]++

		if version == "v0.0.0" {
			stats.LegacyEntries++
		}
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
// Returns the number of entries removed.
func (m *Manager) ClearOldVersions() (int, error) {
	summariesDir := filepath.Join(m.cacheDir, "summaries")
	entries, err := os.ReadDir(summariesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // Empty cache is valid
		}
		return 0, fmt.Errorf("failed to read cache directory; %w", err)
	}

	currentVersion := CacheVersion()
	removed := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		version := parseVersionFromFilename(entry.Name())

		// Remove if not current version
		if version != currentVersion {
			path := filepath.Join(summariesDir, entry.Name())
			if err := os.Remove(path); err != nil {
				return removed, fmt.Errorf("failed to remove cache file %s; %w", entry.Name(), err)
			}
			removed++
		}
	}

	return removed, nil
}

// GetCacheDir returns the cache directory path.
func (m *Manager) GetCacheDir() string {
	return m.cacheDir
}
