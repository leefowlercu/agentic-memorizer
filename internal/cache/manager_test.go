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

	// Write corrupted JSON
	fileHash := "sha256:corrupted"
	cachePath := manager.getCachePath(fileHash)
	if err := os.WriteFile(cachePath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write corrupted cache: %v", err)
	}

	_, err := manager.Get(fileHash)
	if err == nil {
		t.Error("Get() should return error for corrupted cache")
	}
}

func TestManager_IsStale(t *testing.T) {
	manager := &Manager{}

	tests := []struct {
		name        string
		cachedHash  string
		currentHash string
		want        bool
	}{
		{
			name:        "same hash",
			cachedHash:  "sha256:abc123",
			currentHash: "sha256:abc123",
			want:        false,
		},
		{
			name:        "different hash",
			cachedHash:  "sha256:abc123",
			currentHash: "sha256:def456",
			want:        true,
		},
		{
			name:        "empty hashes",
			cachedHash:  "",
			currentHash: "",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cached := &types.CachedAnalysis{
				FileHash: tt.cachedHash,
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
			got := manager.getCachePath(tt.fileHash)
			if got != tt.want {
				t.Errorf("getCachePath() = %q, want %q", got, tt.want)
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
