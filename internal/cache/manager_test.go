package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()

	manager, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if manager == nil {
		t.Fatal("NewManager() returned nil manager")
	}

	if manager.cacheDir != tmpDir {
		t.Errorf("NewManager() cacheDir = %q, want %q", manager.cacheDir, tmpDir)
	}

	// Check that summaries directory was created
	summariesDir := filepath.Join(tmpDir, "summaries")
	if _, err := os.Stat(summariesDir); os.IsNotExist(err) {
		t.Error("NewManager() did not create summaries directory")
	}
}

func TestNewManager_InvalidDirectory(t *testing.T) {
	invalidDir := "/root/no-permission/cache"

	_, err := NewManager(invalidDir)
	if err == nil {
		t.Error("NewManager() should return error for invalid directory")
	}
}

func TestManager_SetAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)

	semantic := &types.SemanticAnalysis{
		Summary:      "Test summary",
		Tags:         []string{"tag1", "tag2"},
		KeyTopics:    []string{"topic1"},
		DocumentType: "test-document",
		Confidence:   0.95,
	}
	cached := &types.CachedAnalysis{
		FileHash: "sha256:abc123def456",
		Semantic: semantic,
	}

	// Set cache
	err := manager.Set(cached)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Get cache
	retrieved, err := manager.Get(cached.FileHash)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if retrieved == nil {
		t.Fatal("Get() returned nil")
	}

	if retrieved.FileHash != cached.FileHash {
		t.Errorf("FileHash = %q, want %q", retrieved.FileHash, cached.FileHash)
	}

	if retrieved.Semantic.Summary != cached.Semantic.Summary {
		t.Errorf("Summary = %q, want %q", retrieved.Semantic.Summary, cached.Semantic.Summary)
	}
}

func TestManager_Get_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)

	retrieved, err := manager.Get("sha256:nonexistent")
	if err != nil {
		t.Fatalf("Get() error = %v, should return nil for non-existent", err)
	}

	if retrieved != nil {
		t.Error("Get() should return nil for non-existent cache")
	}
}

func TestManager_Get_CorruptedCache(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)

	// Write corrupted JSON to both versioned and legacy paths
	fileHash := "sha256:corrupted12345"
	versionedPath := manager.getCachePath(fileHash)
	legacyPath := manager.getLegacyCachePath(fileHash)

	if err := os.WriteFile(versionedPath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write corrupted versioned cache: %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write corrupted legacy cache: %v", err)
	}

	// Get returns nil on corrupted files (readCacheFile returns nil, nil on parse error)
	// This matches the behavior where we try to parse and fallback gracefully
	cached, _ := manager.Get(fileHash)
	if cached != nil {
		t.Error("Get() should return nil for corrupted cache")
	}
}

func TestManager_Get_CorruptedCacheDirectRead(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)

	// Write corrupted JSON to versioned path
	fileHash := "sha256:corrupted12345"
	versionedPath := manager.getCachePath(fileHash)

	if err := os.WriteFile(versionedPath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write corrupted cache: %v", err)
	}

	// Direct read should return error for corrupted cache
	_, err := manager.readCacheFile(versionedPath)
	if err == nil {
		t.Error("readCacheFile() should return error for corrupted cache")
	}
}

func TestManager_IsStale(t *testing.T) {
	manager := &Manager{}

	tests := []struct {
		name            string
		cachedHash      string
		currentHash     string
		schemaVersion   int
		metadataVersion int
		semanticVersion int
		want            bool
	}{
		{
			name:            "same hash, current version",
			cachedHash:      "sha256:abc123",
			currentHash:     "sha256:abc123",
			schemaVersion:   CacheSchemaVersion,
			metadataVersion: CacheMetadataVersion,
			semanticVersion: CacheSemanticVersion,
			want:            false,
		},
		{
			name:            "different hash",
			cachedHash:      "sha256:abc123",
			currentHash:     "sha256:def456",
			schemaVersion:   CacheSchemaVersion,
			metadataVersion: CacheMetadataVersion,
			semanticVersion: CacheSemanticVersion,
			want:            true,
		},
		{
			name:            "same hash, legacy version (0.0.0)",
			cachedHash:      "sha256:abc123",
			currentHash:     "sha256:abc123",
			schemaVersion:   0,
			metadataVersion: 0,
			semanticVersion: 0,
			want:            true, // Legacy entries are always stale
		},
		{
			name:            "same hash, old metadata version",
			cachedHash:      "sha256:abc123",
			currentHash:     "sha256:abc123",
			schemaVersion:   CacheSchemaVersion,
			metadataVersion: CacheMetadataVersion - 1,
			semanticVersion: CacheSemanticVersion,
			want:            true,
		},
		{
			name:            "same hash, old semantic version",
			cachedHash:      "sha256:abc123",
			currentHash:     "sha256:abc123",
			schemaVersion:   CacheSchemaVersion,
			metadataVersion: CacheMetadataVersion,
			semanticVersion: CacheSemanticVersion - 1,
			want:            true,
		},
		{
			name:            "empty hashes, current version",
			cachedHash:      "",
			currentHash:     "",
			schemaVersion:   CacheSchemaVersion,
			metadataVersion: CacheMetadataVersion,
			semanticVersion: CacheSemanticVersion,
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cached := &types.CachedAnalysis{
				FileHash:        tt.cachedHash,
				SchemaVersion:   tt.schemaVersion,
				MetadataVersion: tt.metadataVersion,
				SemanticVersion: tt.semanticVersion,
			}

			got := manager.IsStale(cached, tt.currentHash)
			if got != tt.want {
				t.Errorf("IsStale() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHashFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("Hello, World!")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	hash1, err := HashFile(testFile)
	if err != nil {
		t.Fatalf("HashFile() error = %v", err)
	}

	if hash1 == "" {
		t.Error("HashFile() returned empty hash")
	}

	if hash1[:7] != "sha256:" {
		t.Errorf("HashFile() hash should start with 'sha256:', got %q", hash1)
	}

	// Hash same file again - should get same hash
	hash2, err := HashFile(testFile)
	if err != nil {
		t.Fatalf("HashFile() error on second call = %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("HashFile() inconsistent: first=%q, second=%q", hash1, hash2)
	}

	// Modify file - should get different hash
	if err := os.WriteFile(testFile, []byte("Modified content"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	hash3, err := HashFile(testFile)
	if err != nil {
		t.Fatalf("HashFile() error after modification = %v", err)
	}

	if hash1 == hash3 {
		t.Error("HashFile() should return different hash for modified file")
	}
}

func TestHashFile_NonExistent(t *testing.T) {
	_, err := HashFile("/nonexistent/file.txt")
	if err == nil {
		t.Error("HashFile() should return error for non-existent file")
	}
}

func TestManager_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)

	// Add some cached entries
	for i := 0; i < 5; i++ {
		semantic := &types.SemanticAnalysis{
			Summary: "Test",
		}
		cached := &types.CachedAnalysis{
			FileHash: fmt.Sprintf("sha256:test%016d", i),
			Semantic: semantic,
		}
		if err := manager.Set(cached); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
	}

	// Verify files exist
	summariesDir := filepath.Join(tmpDir, "summaries")
	entriesBefore, _ := os.ReadDir(summariesDir)
	if len(entriesBefore) == 0 {
		t.Fatal("No cache files were created")
	}

	// Clear cache
	err := manager.Clear()
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	// Verify files are gone
	entriesAfter, _ := os.ReadDir(summariesDir)
	if len(entriesAfter) != 0 {
		t.Errorf("Clear() left %d files, want 0", len(entriesAfter))
	}
}

func TestManager_Clear_EmptyCache(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)

	// Clear empty cache should not error
	err := manager.Clear()
	if err != nil {
		t.Errorf("Clear() error on empty cache = %v", err)
	}
}

func TestManager_Clear_WithSubdirectories(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)

	// Create a subdirectory in summaries (should be ignored by Clear)
	summariesDir := filepath.Join(tmpDir, "summaries")
	subDir := filepath.Join(summariesDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	// Add a cache file
	semantic := &types.SemanticAnalysis{
		Summary: "Test",
	}
	cached := &types.CachedAnalysis{
		FileHash: "sha256:test0000000000",
		Semantic: semantic,
	}
	if err := manager.Set(cached); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Clear should remove files but not directories
	err := manager.Clear()
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	// Check subdirectory still exists
	if _, err := os.Stat(subDir); os.IsNotExist(err) {
		t.Error("Clear() should not remove subdirectories")
	}

	// Check cache file is gone
	entries, _ := os.ReadDir(summariesDir)
	fileCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			fileCount++
		}
	}
	if fileCount != 0 {
		t.Errorf("Clear() left %d files, want 0", fileCount)
	}
}

func TestManager_getCachePath(t *testing.T) {
	manager := &Manager{
		cacheDir: "/test/cache",
	}

	tests := []struct {
		name     string
		fileHash string
		want     string
	}{
		{
			name:     "standard hash",
			fileHash: "sha256:abcdef1234567890",
			want:     fmt.Sprintf("/test/cache/summaries/sha256:abcdef123-v%d-%d-%d.json", CacheSchemaVersion, CacheMetadataVersion, CacheSemanticVersion),
		},
		{
			name:     "long hash",
			fileHash: "sha256:0123456789abcdef0123456789abcdef",
			want:     fmt.Sprintf("/test/cache/summaries/sha256:012345678-v%d-%d-%d.json", CacheSchemaVersion, CacheMetadataVersion, CacheSemanticVersion),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := manager.getCachePath(tt.fileHash)
			if got != tt.want {
				t.Errorf("getCachePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestManager_getLegacyCachePath(t *testing.T) {
	manager := &Manager{
		cacheDir: "/test/cache",
	}

	tests := []struct {
		name     string
		fileHash string
		want     string
	}{
		{
			name:     "standard hash",
			fileHash: "sha256:abcdef1234567890",
			want:     "/test/cache/summaries/sha256:abcdef123.json",
		},
		{
			name:     "long hash",
			fileHash: "sha256:0123456789abcdef0123456789abcdef",
			want:     "/test/cache/summaries/sha256:012345678.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := manager.getLegacyCachePath(tt.fileHash)
			if got != tt.want {
				t.Errorf("getLegacyCachePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHashFile_DifferentContent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two files with different content
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	hash1, _ := HashFile(file1)
	hash2, _ := HashFile(file2)

	if hash1 == hash2 {
		t.Error("HashFile() should return different hashes for different content")
	}
}

func TestHashFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()

	emptyFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	hash, err := HashFile(emptyFile)
	if err != nil {
		t.Fatalf("HashFile() error on empty file = %v", err)
	}

	if hash == "" {
		t.Error("HashFile() should return hash for empty file")
	}

	// SHA256 of empty content should be consistent
	hash2, _ := HashFile(emptyFile)
	if hash != hash2 {
		t.Error("HashFile() should be consistent for empty files")
	}
}

func TestParseVersionFromFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "legacy format",
			filename: "sha256:abc12345.json",
			want:     "v0.0.0",
		},
		{
			name:     "current version format",
			filename: "sha256:abc12345-v1-1-1.json",
			want:     "v1.1.1",
		},
		{
			name:     "higher version format",
			filename: "sha256:abc12345-v2-3-4.json",
			want:     "v2.3.4",
		},
		{
			name:     "no json extension",
			filename: "sha256:abc12345-v1-1-1",
			want:     "v1.1.1",
		},
		{
			name:     "non-cache file",
			filename: "other-file.txt",
			want:     "v0.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseVersionFromFilename(tt.filename)
			if got != tt.want {
				t.Errorf("parseVersionFromFilename(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestManager_GetStats(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)

	// Empty cache
	stats, err := manager.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if stats.TotalEntries != 0 {
		t.Errorf("GetStats() TotalEntries = %d, want 0", stats.TotalEntries)
	}

	// Add some cache entries
	semantic := &types.SemanticAnalysis{Summary: "Test"}

	// Add versioned entries via Set() - use unique 16-char prefixes
	hashes := []string{
		"sha256:aaaa111111111111",
		"sha256:bbbb222222222222",
		"sha256:cccc333333333333",
	}
	for _, hash := range hashes {
		cached := &types.CachedAnalysis{
			FileHash: hash,
			Semantic: semantic,
		}
		if err := manager.Set(cached); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
	}

	// Manually add legacy entries - use unique 16-char prefixes
	summariesDir := filepath.Join(tmpDir, "summaries")
	legacyHashes := []string{"sha256:legaaa00", "sha256:legbbb00"}
	for _, hash := range legacyHashes {
		legacyPath := filepath.Join(summariesDir, hash+".json")
		legacyData := []byte(`{"file_hash": "legacy"}`)
		if err := os.WriteFile(legacyPath, legacyData, 0644); err != nil {
			t.Fatalf("Failed to write legacy cache: %v", err)
		}
	}

	// Get stats
	stats, err = manager.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	if stats.TotalEntries != 5 {
		t.Errorf("GetStats() TotalEntries = %d, want 5", stats.TotalEntries)
	}

	if stats.LegacyEntries != 2 {
		t.Errorf("GetStats() LegacyEntries = %d, want 2", stats.LegacyEntries)
	}

	if stats.TotalSize == 0 {
		t.Error("GetStats() TotalSize should be > 0")
	}

	currentVersion := CacheVersion()
	if stats.VersionCounts[currentVersion] != 3 {
		t.Errorf("GetStats() VersionCounts[%s] = %d, want 3", currentVersion, stats.VersionCounts[currentVersion])
	}

	if stats.VersionCounts["v0.0.0"] != 2 {
		t.Errorf("GetStats() VersionCounts[v0.0.0] = %d, want 2", stats.VersionCounts["v0.0.0"])
	}
}

func TestManager_GetStats_EmptyCache(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)

	stats, err := manager.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	if stats.TotalEntries != 0 {
		t.Errorf("GetStats() TotalEntries = %d, want 0", stats.TotalEntries)
	}

	if stats.LegacyEntries != 0 {
		t.Errorf("GetStats() LegacyEntries = %d, want 0", stats.LegacyEntries)
	}

	if stats.TotalSize != 0 {
		t.Errorf("GetStats() TotalSize = %d, want 0", stats.TotalSize)
	}

	if len(stats.VersionCounts) != 0 {
		t.Errorf("GetStats() VersionCounts should be empty, got %v", stats.VersionCounts)
	}
}

func TestManager_ClearOldVersions(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	summariesDir := filepath.Join(tmpDir, "summaries")

	semantic := &types.SemanticAnalysis{Summary: "Test"}

	// Add current version entries - use unique 16-char prefixes
	currentHashes := []string{
		"sha256:curr111111111111",
		"sha256:curr222222222222",
		"sha256:curr333333333333",
	}
	for _, hash := range currentHashes {
		cached := &types.CachedAnalysis{
			FileHash: hash,
			Semantic: semantic,
		}
		if err := manager.Set(cached); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
	}

	// Manually add legacy entries - use unique prefixes
	legacyPaths := []string{"sha256:lega1111.json", "sha256:lega2222.json"}
	for _, filename := range legacyPaths {
		legacyPath := filepath.Join(summariesDir, filename)
		legacyData := []byte(`{"file_hash": "legacy"}`)
		if err := os.WriteFile(legacyPath, legacyData, 0644); err != nil {
			t.Fatalf("Failed to write legacy cache: %v", err)
		}
	}

	// Manually add old versioned entries - use unique prefixes
	oldVersionPaths := []string{"sha256:oldv1111-v0-0-1.json", "sha256:oldv2222-v0-0-1.json"}
	for _, filename := range oldVersionPaths {
		oldVersionPath := filepath.Join(summariesDir, filename)
		oldData := []byte(`{"file_hash": "old", "schema_version": 0, "metadata_version": 0, "semantic_version": 1}`)
		if err := os.WriteFile(oldVersionPath, oldData, 0644); err != nil {
			t.Fatalf("Failed to write old version cache: %v", err)
		}
	}

	// Verify initial state
	stats, _ := manager.GetStats()
	if stats.TotalEntries != 7 {
		t.Fatalf("Expected 7 entries, got %d", stats.TotalEntries)
	}

	// Clear old versions
	removed, err := manager.ClearOldVersions()
	if err != nil {
		t.Fatalf("ClearOldVersions() error = %v", err)
	}

	if removed != 4 {
		t.Errorf("ClearOldVersions() removed = %d, want 4 (2 legacy + 2 old versioned)", removed)
	}

	// Verify final state
	stats, _ = manager.GetStats()
	if stats.TotalEntries != 3 {
		t.Errorf("After ClearOldVersions(), TotalEntries = %d, want 3", stats.TotalEntries)
	}

	if stats.LegacyEntries != 0 {
		t.Errorf("After ClearOldVersions(), LegacyEntries = %d, want 0", stats.LegacyEntries)
	}

	currentVersion := CacheVersion()
	if stats.VersionCounts[currentVersion] != 3 {
		t.Errorf("After ClearOldVersions(), VersionCounts[%s] = %d, want 3", currentVersion, stats.VersionCounts[currentVersion])
	}
}

func TestManager_ClearOldVersions_EmptyCache(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)

	removed, err := manager.ClearOldVersions()
	if err != nil {
		t.Fatalf("ClearOldVersions() error = %v", err)
	}

	if removed != 0 {
		t.Errorf("ClearOldVersions() on empty cache removed = %d, want 0", removed)
	}
}

func TestManager_ClearOldVersions_OnlyCurrentVersion(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)

	semantic := &types.SemanticAnalysis{Summary: "Test"}

	// Add only current version entries - use unique 16-char prefixes
	hashes := []string{
		"sha256:only111111111111",
		"sha256:only222222222222",
		"sha256:only333333333333",
	}
	for _, hash := range hashes {
		cached := &types.CachedAnalysis{
			FileHash: hash,
			Semantic: semantic,
		}
		if err := manager.Set(cached); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
	}

	removed, err := manager.ClearOldVersions()
	if err != nil {
		t.Fatalf("ClearOldVersions() error = %v", err)
	}

	if removed != 0 {
		t.Errorf("ClearOldVersions() should not remove current version entries, removed = %d", removed)
	}

	stats, _ := manager.GetStats()
	if stats.TotalEntries != 3 {
		t.Errorf("After ClearOldVersions(), TotalEntries = %d, want 3", stats.TotalEntries)
	}
}

func TestManager_Get_FallbackToLegacy(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	summariesDir := filepath.Join(tmpDir, "summaries")

	// Create a legacy cache entry directly
	fileHash := "sha256:legacy12345678"
	legacyPath := manager.getLegacyCachePath(fileHash)
	legacyData := []byte(`{"file_hash": "sha256:legacy12345678", "semantic": {"summary": "Legacy entry"}}`)
	if err := os.WriteFile(legacyPath, legacyData, 0644); err != nil {
		t.Fatalf("Failed to write legacy cache: %v", err)
	}

	// Verify no versioned entry exists
	versionedPath := manager.getCachePath(fileHash)
	if _, err := os.Stat(versionedPath); !os.IsNotExist(err) {
		t.Fatal("Versioned cache entry should not exist")
	}

	// Get should fallback to legacy
	cached, err := manager.Get(fileHash)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if cached == nil {
		t.Fatal("Get() should return legacy entry")
	}

	if cached.Semantic.Summary != "Legacy entry" {
		t.Errorf("Get() cached.Semantic.Summary = %q, want %q", cached.Semantic.Summary, "Legacy entry")
	}

	// Legacy entry should have version 0.0.0
	if cached.SchemaVersion != 0 || cached.MetadataVersion != 0 || cached.SemanticVersion != 0 {
		t.Errorf("Legacy entry should have version 0.0.0, got %d.%d.%d",
			cached.SchemaVersion, cached.MetadataVersion, cached.SemanticVersion)
	}

	// Verify summaries directory still exists and contains the legacy file
	entries, _ := os.ReadDir(summariesDir)
	found := false
	for _, entry := range entries {
		if entry.Name() == filepath.Base(legacyPath) {
			found = true
			break
		}
	}
	if !found {
		t.Error("Legacy file should still exist in summaries directory")
	}
}

func TestManager_SetPopulatesVersionFields(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)

	// Create entry without version fields
	cached := &types.CachedAnalysis{
		FileHash: "sha256:test123456789",
		Semantic: &types.SemanticAnalysis{Summary: "Test"},
	}

	// Set should populate version fields
	if err := manager.Set(cached); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Verify version fields are set in the cached object
	if cached.SchemaVersion != CacheSchemaVersion {
		t.Errorf("Set() SchemaVersion = %d, want %d", cached.SchemaVersion, CacheSchemaVersion)
	}
	if cached.MetadataVersion != CacheMetadataVersion {
		t.Errorf("Set() MetadataVersion = %d, want %d", cached.MetadataVersion, CacheMetadataVersion)
	}
	if cached.SemanticVersion != CacheSemanticVersion {
		t.Errorf("Set() SemanticVersion = %d, want %d", cached.SemanticVersion, CacheSemanticVersion)
	}

	// Verify by reading back
	retrieved, err := manager.Get(cached.FileHash)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if retrieved.SchemaVersion != CacheSchemaVersion {
		t.Errorf("Retrieved SchemaVersion = %d, want %d", retrieved.SchemaVersion, CacheSchemaVersion)
	}
}

func TestManager_GetCacheDir(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)

	if manager.GetCacheDir() != tmpDir {
		t.Errorf("GetCacheDir() = %q, want %q", manager.GetCacheDir(), tmpDir)
	}
}
