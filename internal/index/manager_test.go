package index

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

func TestManager_WriteAndLoad(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "index-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	indexPath := filepath.Join(tmpDir, "index.json")
	mgr := NewManager(indexPath)

	// Create test index
	testIndex := &types.Index{
		Generated: time.Now(),
		Root:      "/test/root",
		Entries: []types.IndexEntry{
			{
				Metadata: types.FileMetadata{
					FileInfo: types.FileInfo{
						Path:    "/test/root/file1.txt",
						RelPath: "file1.txt",
						Hash:    "abc123",
						Size:    100,
					},
				},
			},
		},
		Stats: types.IndexStats{
			TotalFiles: 1,
		},
	}

	metadata := BuildMetadata{
		BuildDurationMs: 100,
		FilesProcessed:  1,
		CacheHits:       0,
		APICalls:        1,
	}

	// Set and write index
	mgr.SetIndex(testIndex, metadata)
	if err := mgr.WriteAtomic("test-v1.0"); err != nil {
		t.Fatalf("Failed to write index: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Fatal("Index file was not created")
	}

	// Load index
	computed, err := mgr.LoadComputed()
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}

	// Verify loaded data
	if computed.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", computed.Version)
	}
	if computed.DaemonVersion != "test-v1.0" {
		t.Errorf("Expected daemon version test-v1.0, got %s", computed.DaemonVersion)
	}
	if computed.Index.Stats.TotalFiles != 1 {
		t.Errorf("Expected 1 file, got %d", computed.Index.Stats.TotalFiles)
	}
	if len(computed.Index.Entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(computed.Index.Entries))
	}
}

func TestManager_AtomicWrite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "index-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	indexPath := filepath.Join(tmpDir, "index.json")
	mgr := NewManager(indexPath)

	testIndex := &types.Index{
		Generated: time.Now(),
		Root:      "/test",
		Entries:   []types.IndexEntry{},
		Stats:     types.IndexStats{},
	}

	mgr.SetIndex(testIndex, BuildMetadata{})

	// Write multiple times to test atomicity
	for i := 0; i < 5; i++ {
		if err := mgr.WriteAtomic("test-version"); err != nil {
			t.Fatalf("Write %d failed: %v", i, err)
		}

		// Verify no temp file left behind
		tmpPath := indexPath + ".tmp"
		if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
			t.Errorf("Temp file still exists after write %d", i)
		}
	}
}

func TestManager_UpdateSingle(t *testing.T) {
	mgr := NewManager("/tmp/test.json")

	// Initialize with an index
	initialIndex := &types.Index{
		Generated: time.Now(),
		Root:      "/test",
		Entries: []types.IndexEntry{
			{
				Metadata: types.FileMetadata{
					FileInfo: types.FileInfo{
						Path:    "/test/file1.txt",
						RelPath: "file1.txt",
						Hash:    "hash1",
					},
				},
			},
		},
		Stats: types.IndexStats{
			TotalFiles: 1,
		},
	}

	mgr.SetIndex(initialIndex, BuildMetadata{})

	tests := []struct {
		name          string
		entry         types.IndexEntry
		expectedFiles int
		expectUpdate  bool
	}{
		{
			name: "update existing file",
			entry: types.IndexEntry{
				Metadata: types.FileMetadata{
					FileInfo: types.FileInfo{
						Path:    "/test/file1.txt",
						RelPath: "file1.txt",
						Hash:    "hash1-updated",
					},
				},
			},
			expectedFiles: 1,
			expectUpdate:  true,
		},
		{
			name: "add new file",
			entry: types.IndexEntry{
				Metadata: types.FileMetadata{
					FileInfo: types.FileInfo{
						Path:    "/test/file2.txt",
						RelPath: "file2.txt",
						Hash:    "hash2",
					},
				},
			},
			expectedFiles: 2,
			expectUpdate:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateInfo := UpdateInfo{
				WasAnalyzed: false,
				WasCached:   false,
				HadError:    false,
			}
			_, err := mgr.UpdateSingle(tt.entry, updateInfo)
			if err != nil {
				t.Fatalf("UpdateSingle failed: %v", err)
			}

			current := mgr.GetCurrent()
			if current.Stats.TotalFiles != tt.expectedFiles {
				t.Errorf("Expected %d files, got %d", tt.expectedFiles, current.Stats.TotalFiles)
			}

			// Verify the entry exists
			found := false
			for _, e := range current.Entries {
				if e.Metadata.Path == tt.entry.Metadata.Path {
					found = true
					if e.Metadata.Hash != tt.entry.Metadata.Hash {
						t.Errorf("Expected hash %s, got %s", tt.entry.Metadata.Hash, e.Metadata.Hash)
					}
					break
				}
			}
			if !found {
				t.Errorf("Entry not found in index: %s", tt.entry.Metadata.Path)
			}
		})
	}
}

func TestManager_RemoveFile(t *testing.T) {
	mgr := NewManager("/tmp/test.json")

	// Initialize with multiple files
	initialIndex := &types.Index{
		Generated: time.Now(),
		Root:      "/test",
		Entries: []types.IndexEntry{
			{
				Metadata: types.FileMetadata{
					FileInfo: types.FileInfo{Path: "/test/file1.txt"},
				},
			},
			{
				Metadata: types.FileMetadata{
					FileInfo: types.FileInfo{Path: "/test/file2.txt"},
				},
			},
		},
		Stats: types.IndexStats{
			TotalFiles: 2,
		},
	}

	mgr.SetIndex(initialIndex, BuildMetadata{})

	// Remove first file
	result, err := mgr.RemoveFile("/test/file1.txt")
	if err != nil {
		t.Fatalf("RemoveFile failed: %v", err)
	}
	if !result.Removed {
		t.Error("Expected Removed to be true")
	}

	current := mgr.GetCurrent()
	if current.Stats.TotalFiles != 1 {
		t.Errorf("Expected 1 file, got %d", current.Stats.TotalFiles)
	}
	if len(current.Entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(current.Entries))
	}

	// Verify correct file was removed
	if current.Entries[0].Metadata.Path != "/test/file2.txt" {
		t.Errorf("Wrong file remaining: %s", current.Entries[0].Metadata.Path)
	}

	// Try to remove non-existent file
	result, err = mgr.RemoveFile("/test/nonexistent.txt")
	if err == nil {
		t.Error("Expected error when removing non-existent file")
	}
	if result.Removed {
		t.Error("Expected Removed to be false for non-existent file")
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "index-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	indexPath := filepath.Join(tmpDir, "index.json")
	mgr := NewManager(indexPath)

	// Initialize index
	initialIndex := &types.Index{
		Generated: time.Now(),
		Root:      "/test",
		Entries:   []types.IndexEntry{},
		Stats:     types.IndexStats{},
	}
	mgr.SetIndex(initialIndex, BuildMetadata{})

	// Concurrent readers and writers
	var wg sync.WaitGroup
	numReaders := 10
	numWriters := 5

	// Start readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = mgr.GetCurrent()
			}
		}()
	}

	// Start writers
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				entry := types.IndexEntry{
					Metadata: types.FileMetadata{
						FileInfo: types.FileInfo{
							Path: filepath.Join("/test", "concurrent", "file.txt"),
						},
					},
				}
				updateInfo := UpdateInfo{
					WasAnalyzed: false,
					WasCached:   false,
					HadError:    false,
				}
				_, _ = mgr.UpdateSingle(entry, updateInfo)
			}
		}(i)
	}

	wg.Wait()

	// Verify no panics occurred and index is still valid
	current := mgr.GetCurrent()
	if current == nil {
		t.Error("Index is nil after concurrent access")
	}
}

func TestManager_LoadNonExistentFile(t *testing.T) {
	mgr := NewManager("/nonexistent/path/index.json")

	_, err := mgr.LoadComputed()
	if err == nil {
		t.Error("Expected error when loading non-existent file")
	}
}

func TestManager_WriteWithoutIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "index-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	indexPath := filepath.Join(tmpDir, "index.json")
	mgr := NewManager(indexPath)

	// Try to write without setting an index
	err = mgr.WriteAtomic("test-version")
	if err == nil {
		t.Error("Expected error when writing without an index")
	}
}
