package storage

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Helper functions

func newTestStorage(t *testing.T) *Storage {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	storage, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to open storage: %v", err)
	}
	t.Cleanup(func() {
		storage.Close()
	})

	return storage
}

func boolPtr(b bool) *bool {
	return &b
}

// Storage tests

func TestOpen(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to open storage: %v", err)
	}
	defer s.Close()

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestOpen_CreatesDirectory(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "subdir", "nested", "test.db")

	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to open storage: %v", err)
	}
	defer s.Close()

	// Verify directory structure was created
	dir := filepath.Dir(dbPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("database directory was not created")
	}
}

func TestMigrations_Idempotent(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	// Open storage (runs migrations)
	s1, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to open storage first time: %v", err)
	}
	s1.Close()

	// Open again (should not fail on existing schema)
	s2, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to open storage second time: %v", err)
	}
	s2.Close()
}

func TestGetSchemaVersion(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	version, err := s.GetSchemaVersion(ctx)
	if err != nil {
		t.Fatalf("failed to get schema version: %v", err)
	}

	// Should be at latest version after all migrations run
	if version < 1 {
		t.Errorf("expected version >= 1, got %d", version)
	}
}

// Remembered paths tests

func TestAddPath(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()
	testPath := "/test/project"

	err := s.AddPath(ctx, testPath, nil)
	if err != nil {
		t.Fatalf("failed to add path: %v", err)
	}

	// Verify path was added
	rp, err := s.GetPath(ctx, testPath)
	if err != nil {
		t.Fatalf("failed to get path: %v", err)
	}
	if rp.Path != testPath {
		t.Errorf("expected path %q, got %q", testPath, rp.Path)
	}
}

func TestAddPath_WithConfig(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()
	testPath := "/test/project"
	config := &PathConfig{
		SkipExtensions:  []string{".exe", ".dll"},
		SkipDirectories: []string{"node_modules", ".git"},
		SkipHidden:      true,
	}

	err := s.AddPath(ctx, testPath, config)
	if err != nil {
		t.Fatalf("failed to add path: %v", err)
	}

	// Verify config was stored
	rp, err := s.GetPath(ctx, testPath)
	if err != nil {
		t.Fatalf("failed to get path: %v", err)
	}

	if rp.Config == nil {
		t.Fatal("expected config to be set")
	}
	if len(rp.Config.SkipExtensions) != 2 {
		t.Errorf("expected 2 skip extensions, got %d", len(rp.Config.SkipExtensions))
	}
	if !rp.Config.SkipHidden {
		t.Error("expected SkipHidden to be true")
	}
}

func TestAddPath_Duplicate(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()
	testPath := "/test/project"

	// Add path first time
	err := s.AddPath(ctx, testPath, nil)
	if err != nil {
		t.Fatalf("failed to add path: %v", err)
	}

	// Add same path again
	err = s.AddPath(ctx, testPath, nil)
	if err != ErrPathExists {
		t.Errorf("expected ErrPathExists, got %v", err)
	}
}

func TestRemovePath(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()
	testPath := "/test/project"

	// Add then remove
	s.AddPath(ctx, testPath, nil)
	err := s.RemovePath(ctx, testPath)
	if err != nil {
		t.Fatalf("failed to remove path: %v", err)
	}

	// Verify path was removed
	_, err = s.GetPath(ctx, testPath)
	if err != ErrPathNotFound {
		t.Errorf("expected ErrPathNotFound, got %v", err)
	}
}

func TestRemovePath_NotFound(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()
	err := s.RemovePath(ctx, "/nonexistent")
	if err != ErrPathNotFound {
		t.Errorf("expected ErrPathNotFound, got %v", err)
	}
}

func TestListPaths(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	// Add multiple paths
	paths := []string{"/test/a", "/test/b", "/test/c"}
	for _, p := range paths {
		if err := s.AddPath(ctx, p, nil); err != nil {
			t.Fatalf("failed to add path %q: %v", p, err)
		}
	}

	// List paths
	result, err := s.ListPaths(ctx)
	if err != nil {
		t.Fatalf("failed to list paths: %v", err)
	}

	if len(result) != len(paths) {
		t.Errorf("expected %d paths, got %d", len(paths), len(result))
	}

	// Verify order (should be sorted by path)
	for i, rp := range result {
		if rp.Path != paths[i] {
			t.Errorf("expected path %q at index %d, got %q", paths[i], i, rp.Path)
		}
	}
}

func TestUpdatePathConfig(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()
	testPath := "/test/project"

	// Add path with initial config
	initialConfig := &PathConfig{SkipHidden: false}
	s.AddPath(ctx, testPath, initialConfig)

	// Update config
	newConfig := &PathConfig{
		SkipHidden:     true,
		SkipExtensions: []string{".log"},
	}
	err := s.UpdatePathConfig(ctx, testPath, newConfig)
	if err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// Verify update
	rp, _ := s.GetPath(ctx, testPath)
	if !rp.Config.SkipHidden {
		t.Error("expected SkipHidden to be true after update")
	}
	if len(rp.Config.SkipExtensions) != 1 {
		t.Errorf("expected 1 skip extension, got %d", len(rp.Config.SkipExtensions))
	}
}

func TestUpdatePathLastWalk(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()
	testPath := "/test/project"
	s.AddPath(ctx, testPath, nil)

	// Update last walk
	walkTime := time.Now().Truncate(time.Second)
	err := s.UpdatePathLastWalk(ctx, testPath, walkTime)
	if err != nil {
		t.Fatalf("failed to update last walk: %v", err)
	}

	// Verify update
	rp, _ := s.GetPath(ctx, testPath)
	if rp.LastWalkAt == nil {
		t.Fatal("expected LastWalkAt to be set")
	}
	if !rp.LastWalkAt.Truncate(time.Second).Equal(walkTime) {
		t.Errorf("expected LastWalkAt %v, got %v", walkTime, *rp.LastWalkAt)
	}
}

func TestFindContainingPath(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	// Add some remembered paths
	s.AddPath(ctx, "/projects", nil)
	s.AddPath(ctx, "/projects/myapp", nil)
	s.AddPath(ctx, "/documents", nil)

	tests := []struct {
		name     string
		filePath string
		wantPath string
		wantErr  error
	}{
		{
			name:     "file directly in remembered path",
			filePath: "/projects/file.txt",
			wantPath: "/projects",
		},
		{
			name:     "file in nested remembered path",
			filePath: "/projects/myapp/src/main.go",
			wantPath: "/projects/myapp",
		},
		{
			name:     "file in non-nested remembered path",
			filePath: "/documents/report.pdf",
			wantPath: "/documents",
		},
		{
			name:     "file not in any remembered path",
			filePath: "/other/file.txt",
			wantErr:  ErrPathNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rp, err := s.FindContainingPath(ctx, tt.filePath)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rp.Path != tt.wantPath {
				t.Errorf("expected path %q, got %q", tt.wantPath, rp.Path)
			}
		})
	}
}

// File state tests

func TestFileState_CRUD(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()
	testPath := "/test/file.go"
	modTime := time.Now().Truncate(time.Second)

	// Create file state
	state := &FileState{
		Path:         testPath,
		ContentHash:  "abc123",
		MetadataHash: "def456",
		Size:         1024,
		ModTime:      modTime,
	}

	err := s.UpdateFileState(ctx, state)
	if err != nil {
		t.Fatalf("failed to create file state: %v", err)
	}

	// Read file state
	result, err := s.GetFileState(ctx, testPath)
	if err != nil {
		t.Fatalf("failed to get file state: %v", err)
	}

	if result.ContentHash != "abc123" {
		t.Errorf("expected content hash abc123, got %s", result.ContentHash)
	}
	if result.Size != 1024 {
		t.Errorf("expected size 1024, got %d", result.Size)
	}

	// Update file state
	state.ContentHash = "xyz789"
	state.Size = 2048
	err = s.UpdateFileState(ctx, state)
	if err != nil {
		t.Fatalf("failed to update file state: %v", err)
	}

	result, _ = s.GetFileState(ctx, testPath)
	if result.ContentHash != "xyz789" {
		t.Errorf("expected content hash xyz789, got %s", result.ContentHash)
	}
	if result.Size != 2048 {
		t.Errorf("expected size 2048, got %d", result.Size)
	}

	// Delete file state
	err = s.DeleteFileState(ctx, testPath)
	if err != nil {
		t.Fatalf("failed to delete file state: %v", err)
	}

	_, err = s.GetFileState(ctx, testPath)
	if err != ErrPathNotFound {
		t.Errorf("expected ErrPathNotFound, got %v", err)
	}
}

func TestListFileStates(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()
	modTime := time.Now().Truncate(time.Second)

	// Add file states under a parent path
	files := []string{
		"/projects/myapp/src/main.go",
		"/projects/myapp/src/utils.go",
		"/projects/myapp/README.md",
	}
	for _, f := range files {
		state := &FileState{
			Path:         f,
			ContentHash:  "hash",
			MetadataHash: "meta",
			Size:         100,
			ModTime:      modTime,
		}
		s.UpdateFileState(ctx, state)
	}

	// Add file state under different path
	otherState := &FileState{
		Path:         "/documents/report.pdf",
		ContentHash:  "other",
		MetadataHash: "other",
		Size:         200,
		ModTime:      modTime,
	}
	s.UpdateFileState(ctx, otherState)

	// List file states for parent path
	result, err := s.ListFileStates(ctx, "/projects/myapp")
	if err != nil {
		t.Fatalf("failed to list file states: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 file states, got %d", len(result))
	}
}

func TestDeleteFileStatesForPath(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()
	modTime := time.Now().Truncate(time.Second)

	// Add file states
	for _, f := range []string{"/projects/myapp/a.go", "/projects/myapp/b.go", "/documents/c.pdf"} {
		state := &FileState{
			Path:         f,
			ContentHash:  "hash",
			MetadataHash: "meta",
			Size:         100,
			ModTime:      modTime,
		}
		s.UpdateFileState(ctx, state)
	}

	// Delete file states for /projects/myapp
	err := s.DeleteFileStatesForPath(ctx, "/projects/myapp")
	if err != nil {
		t.Fatalf("failed to delete file states: %v", err)
	}

	// Verify myapp files are deleted
	result, _ := s.ListFileStates(ctx, "/projects/myapp")
	if len(result) != 0 {
		t.Errorf("expected 0 file states for myapp, got %d", len(result))
	}

	// Verify documents file still exists
	_, err = s.GetFileState(ctx, "/documents/c.pdf")
	if err != nil {
		t.Errorf("documents file should still exist: %v", err)
	}
}

func TestUpdateMetadataState(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()
	testPath := "/test/file.go"
	modTime := time.Now().Truncate(time.Second)

	// Insert initial metadata state
	err := s.UpdateMetadataState(ctx, testPath, "contenthash123", "metahash456", 1024, modTime)
	if err != nil {
		t.Fatalf("failed to update metadata state: %v", err)
	}

	// Verify state
	state, err := s.GetFileState(ctx, testPath)
	if err != nil {
		t.Fatalf("failed to get file state: %v", err)
	}

	if state.ContentHash != "contenthash123" {
		t.Errorf("expected content hash 'contenthash123', got %q", state.ContentHash)
	}
	if state.MetadataHash != "metahash456" {
		t.Errorf("expected metadata hash 'metahash456', got %q", state.MetadataHash)
	}
	if state.Size != 1024 {
		t.Errorf("expected size 1024, got %d", state.Size)
	}
	if state.MetadataAnalyzedAt == nil {
		t.Error("expected MetadataAnalyzedAt to be set")
	}
}

func TestUpdateSemanticState_Success(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()
	testPath := "/test/file.go"
	modTime := time.Now().Truncate(time.Second)

	// First create the file state
	err := s.UpdateMetadataState(ctx, testPath, "hash", "meta", 100, modTime)
	if err != nil {
		t.Fatalf("failed to setup file state: %v", err)
	}

	// Update semantic state as success
	err = s.UpdateSemanticState(ctx, testPath, "1.0.0", nil)
	if err != nil {
		t.Fatalf("failed to update semantic state: %v", err)
	}

	// Verify state
	state, err := s.GetFileState(ctx, testPath)
	if err != nil {
		t.Fatalf("failed to get file state: %v", err)
	}

	if state.SemanticAnalyzedAt == nil {
		t.Error("expected SemanticAnalyzedAt to be set")
	}
	if state.AnalysisVersion != "1.0.0" {
		t.Errorf("expected analysis version '1.0.0', got %q", state.AnalysisVersion)
	}
	if state.SemanticError != nil {
		t.Error("expected SemanticError to be nil")
	}
	if state.SemanticRetryCount != 0 {
		t.Errorf("expected retry count 0, got %d", state.SemanticRetryCount)
	}
}

func TestUpdateSemanticState_Failure(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()
	testPath := "/test/file.go"
	modTime := time.Now().Truncate(time.Second)

	// First create the file state
	err := s.UpdateMetadataState(ctx, testPath, "hash", "meta", 100, modTime)
	if err != nil {
		t.Fatalf("failed to setup file state: %v", err)
	}

	// Update semantic state as failure
	analysisErr := errors.New("API rate limit exceeded")
	err = s.UpdateSemanticState(ctx, testPath, "1.0.0", analysisErr)
	if err != nil {
		t.Fatalf("failed to update semantic state: %v", err)
	}

	// Verify state
	state, err := s.GetFileState(ctx, testPath)
	if err != nil {
		t.Fatalf("failed to get file state: %v", err)
	}

	if state.SemanticAnalyzedAt != nil {
		t.Error("expected SemanticAnalyzedAt to be nil on failure")
	}
	if state.SemanticError == nil {
		t.Fatal("expected SemanticError to be set")
	}
	if *state.SemanticError != "API rate limit exceeded" {
		t.Errorf("expected error 'API rate limit exceeded', got %q", *state.SemanticError)
	}
	if state.SemanticRetryCount != 1 {
		t.Errorf("expected retry count 1, got %d", state.SemanticRetryCount)
	}

	// Update again to verify retry count increments
	err = s.UpdateSemanticState(ctx, testPath, "1.0.0", analysisErr)
	if err != nil {
		t.Fatalf("failed to update semantic state again: %v", err)
	}

	state, _ = s.GetFileState(ctx, testPath)
	if state.SemanticRetryCount != 2 {
		t.Errorf("expected retry count 2, got %d", state.SemanticRetryCount)
	}
}

func TestClearAnalysisState(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()
	testPath := "/test/file.go"
	modTime := time.Now().Truncate(time.Second)

	// Setup file state with analysis
	err := s.UpdateMetadataState(ctx, testPath, "hash", "meta", 100, modTime)
	if err != nil {
		t.Fatalf("failed to setup file state: %v", err)
	}

	err = s.UpdateSemanticState(ctx, testPath, "1.0.0", nil)
	if err != nil {
		t.Fatalf("failed to setup semantic state: %v", err)
	}

	err = s.UpdateEmbeddingsState(ctx, testPath, nil)
	if err != nil {
		t.Fatalf("failed to setup embeddings state: %v", err)
	}

	// Clear analysis state
	err = s.ClearAnalysisState(ctx, testPath)
	if err != nil {
		t.Fatalf("failed to clear analysis state: %v", err)
	}

	// Verify all analysis timestamps are cleared
	state, _ := s.GetFileState(ctx, testPath)
	if state.LastAnalyzedAt != nil {
		t.Error("expected LastAnalyzedAt to be nil")
	}
	if state.MetadataAnalyzedAt != nil {
		t.Error("expected MetadataAnalyzedAt to be nil")
	}
	if state.SemanticAnalyzedAt != nil {
		t.Error("expected SemanticAnalyzedAt to be nil")
	}
	if state.EmbeddingsAnalyzedAt != nil {
		t.Error("expected EmbeddingsAnalyzedAt to be nil")
	}
	if state.SemanticRetryCount != 0 {
		t.Errorf("expected semantic retry count 0, got %d", state.SemanticRetryCount)
	}
	if state.EmbeddingsRetryCount != 0 {
		t.Errorf("expected embeddings retry count 0, got %d", state.EmbeddingsRetryCount)
	}
}

func TestListFilesNeedingAnalysis(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()
	modTime := time.Now().Truncate(time.Second)

	// Create files with different states
	// File needing metadata
	s.UpdateFileState(ctx, &FileState{
		Path:         "/test/needs-metadata.go",
		ContentHash:  "hash1",
		MetadataHash: "meta1",
		Size:         100,
		ModTime:      modTime,
	})

	// File with metadata but needing semantic
	s.UpdateMetadataState(ctx, "/test/needs-semantic.go", "hash2", "meta2", 200, modTime)

	// File with semantic but needing embeddings
	s.UpdateMetadataState(ctx, "/test/needs-embeddings.go", "hash3", "meta3", 300, modTime)
	s.UpdateSemanticState(ctx, "/test/needs-embeddings.go", "1.0.0", nil)

	// Fully analyzed file
	s.UpdateMetadataState(ctx, "/test/complete.go", "hash4", "meta4", 400, modTime)
	s.UpdateSemanticState(ctx, "/test/complete.go", "1.0.0", nil)
	s.UpdateEmbeddingsState(ctx, "/test/complete.go", nil)

	// Test ListFilesNeedingMetadata
	needsMetadata, err := s.ListFilesNeedingMetadata(ctx, "/test")
	if err != nil {
		t.Fatalf("ListFilesNeedingMetadata failed: %v", err)
	}
	if len(needsMetadata) != 1 {
		t.Errorf("expected 1 file needing metadata, got %d", len(needsMetadata))
	}

	// Test ListFilesNeedingSemantic
	needsSemantic, err := s.ListFilesNeedingSemantic(ctx, "/test", 3)
	if err != nil {
		t.Fatalf("ListFilesNeedingSemantic failed: %v", err)
	}
	if len(needsSemantic) != 1 {
		t.Errorf("expected 1 file needing semantic, got %d", len(needsSemantic))
	}

	// Test ListFilesNeedingEmbeddings
	needsEmbeddings, err := s.ListFilesNeedingEmbeddings(ctx, "/test", 3)
	if err != nil {
		t.Fatalf("ListFilesNeedingEmbeddings failed: %v", err)
	}
	if len(needsEmbeddings) != 1 {
		t.Errorf("expected 1 file needing embeddings, got %d", len(needsEmbeddings))
	}
}

// PathConfig tests

func TestPathConfig_JSON(t *testing.T) {
	config := &PathConfig{
		SkipExtensions:     []string{".exe", ".dll"},
		SkipDirectories:    []string{"node_modules"},
		SkipFiles:          []string{".DS_Store"},
		SkipHidden:         true,
		IncludeExtensions:  []string{".env"},
		IncludeDirectories: []string{".github"},
		IncludeFiles:       []string{".gitignore"},
		UseVision:          boolPtr(false),
	}

	// Marshal to JSON
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal back
	var result PathConfig
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify fields
	if len(result.SkipExtensions) != 2 {
		t.Errorf("expected 2 skip extensions, got %d", len(result.SkipExtensions))
	}
	if !result.SkipHidden {
		t.Error("expected SkipHidden to be true")
	}
	if result.UseVision == nil || *result.UseVision != false {
		t.Error("expected UseVision to be false")
	}
}

func TestPathConfig_Clone(t *testing.T) {
	original := &PathConfig{
		SkipExtensions:     []string{".exe", ".dll"},
		SkipDirectories:    []string{"node_modules"},
		SkipFiles:          []string{".DS_Store"},
		SkipHidden:         true,
		IncludeExtensions:  []string{".env"},
		IncludeDirectories: []string{".github"},
		IncludeFiles:       []string{".gitignore"},
		UseVision:          boolPtr(true),
	}

	clone := original.Clone()

	// Verify clone is not the same pointer
	if clone == original {
		t.Error("Clone() returned same pointer")
	}

	// Verify values are equal
	if clone.SkipHidden != original.SkipHidden {
		t.Errorf("SkipHidden = %v, want %v", clone.SkipHidden, original.SkipHidden)
	}
	if len(clone.SkipExtensions) != len(original.SkipExtensions) {
		t.Errorf("SkipExtensions length = %d, want %d", len(clone.SkipExtensions), len(original.SkipExtensions))
	}
	if *clone.UseVision != *original.UseVision {
		t.Errorf("UseVision = %v, want %v", *clone.UseVision, *original.UseVision)
	}

	// Modify clone and verify original unchanged
	clone.SkipExtensions[0] = ".changed"
	if original.SkipExtensions[0] != ".exe" {
		t.Errorf("original was modified, SkipExtensions[0] = %s, want .exe", original.SkipExtensions[0])
	}
}

func TestPathConfig_Clone_Nil(t *testing.T) {
	var original *PathConfig = nil
	clone := original.Clone()

	if clone != nil {
		t.Error("Clone() of nil should return nil")
	}
}

// Critical events queue tests

func TestCriticalEventQueue_EnqueueDequeue(t *testing.T) {
	s := newTestStorage(t)
	queue := NewCriticalEventQueue(s, 100)
	defer queue.Close()

	ctx := context.Background()

	// Enqueue an event
	event := CriticalEvent{
		Type:      "test.event",
		Timestamp: time.Now(),
		Payload:   map[string]string{"key": "value"},
	}
	err := queue.Enqueue(event)
	if err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	// Check length
	length, err := queue.Len()
	if err != nil {
		t.Fatalf("failed to get length: %v", err)
	}
	if length != 1 {
		t.Errorf("expected length 1, got %d", length)
	}

	// Dequeue
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := queue.Dequeue(ctx)
	if err != nil {
		t.Fatalf("failed to dequeue: %v", err)
	}

	if result.Type != "test.event" {
		t.Errorf("expected type 'test.event', got %q", result.Type)
	}

	// Check length after dequeue
	length, err = queue.Len()
	if err != nil {
		t.Fatalf("failed to get length: %v", err)
	}
	if length != 0 {
		t.Errorf("expected length 0, got %d", length)
	}
}

func TestCriticalEventQueue_Cap(t *testing.T) {
	s := newTestStorage(t)
	queue := NewCriticalEventQueue(s, 5)
	defer queue.Close()

	// Enqueue more than cap
	for i := 0; i < 7; i++ {
		event := CriticalEvent{
			Type:      "test.event",
			Timestamp: time.Now(),
			Payload:   i,
		}
		err := queue.Enqueue(event)
		if err != nil {
			t.Fatalf("failed to enqueue %d: %v", i, err)
		}
	}

	// Check length is capped
	length, err := queue.Len()
	if err != nil {
		t.Fatalf("failed to get length: %v", err)
	}
	if length != 5 {
		t.Errorf("expected length 5 (cap), got %d", length)
	}

	// Verify cap method
	if queue.Cap() != 5 {
		t.Errorf("expected cap 5, got %d", queue.Cap())
	}
}

// FileState model tests

func TestFileState_IsStale(t *testing.T) {
	modTime := time.Now()
	state := &FileState{
		Size:    1024,
		ModTime: modTime,
	}

	tests := []struct {
		name    string
		size    int64
		modTime time.Time
		want    bool
	}{
		{
			name:    "same size and mod time",
			size:    1024,
			modTime: modTime,
			want:    false,
		},
		{
			name:    "different size",
			size:    2048,
			modTime: modTime,
			want:    true,
		},
		{
			name:    "different mod time",
			size:    1024,
			modTime: modTime.Add(time.Second),
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := state.IsStale(tt.size, tt.modTime); got != tt.want {
				t.Errorf("IsStale() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFileState_NeedsAnalysis(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		lastAnalyzedAt *time.Time
		version        string
		currentVersion string
		want           bool
	}{
		{
			name:           "never analyzed",
			lastAnalyzedAt: nil,
			version:        "",
			currentVersion: "1.0.0",
			want:           true,
		},
		{
			name:           "same version",
			lastAnalyzedAt: &now,
			version:        "1.0.0",
			currentVersion: "1.0.0",
			want:           false,
		},
		{
			name:           "different version",
			lastAnalyzedAt: &now,
			version:        "1.0.0",
			currentVersion: "1.1.0",
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &FileState{
				LastAnalyzedAt:  tt.lastAnalyzedAt,
				AnalysisVersion: tt.version,
			}
			if got := state.NeedsAnalysis(tt.currentVersion); got != tt.want {
				t.Errorf("NeedsAnalysis() = %v, want %v", got, tt.want)
			}
		})
	}
}

// QueueStats tests

func TestQueueStats_Total(t *testing.T) {
	stats := &QueueStats{
		Pending:   10,
		Inflight:  5,
		Completed: 100,
		Failed:    2,
	}

	if total := stats.Total(); total != 117 {
		t.Errorf("Total() = %d, want 117", total)
	}
}

// Path health tests

func TestCheckPathHealth_ExistingPath(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	// Create a real directory to test against
	testDir := t.TempDir()

	// Add the existing path
	err := s.AddPath(ctx, testDir, nil)
	if err != nil {
		t.Fatalf("failed to add path: %v", err)
	}

	// Check health
	statuses, err := s.CheckPathHealth(ctx)
	if err != nil {
		t.Fatalf("CheckPathHealth failed: %v", err)
	}

	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}

	if statuses[0].Status != PathStatusOK {
		t.Errorf("expected status %q, got %q", PathStatusOK, statuses[0].Status)
	}
	if statuses[0].Error != nil {
		t.Errorf("expected no error, got %v", statuses[0].Error)
	}
}

func TestCheckPathHealth_MissingPath(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	// Add a path that doesn't exist on the filesystem
	missingPath := "/nonexistent/path/that/does/not/exist"
	err := s.AddPath(ctx, missingPath, nil)
	if err != nil {
		t.Fatalf("failed to add path: %v", err)
	}

	// Check health
	statuses, err := s.CheckPathHealth(ctx)
	if err != nil {
		t.Fatalf("CheckPathHealth failed: %v", err)
	}

	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}

	if statuses[0].Status != PathStatusMissing {
		t.Errorf("expected status %q, got %q", PathStatusMissing, statuses[0].Status)
	}
	if statuses[0].Error == nil {
		t.Error("expected error to be set")
	}
}

func TestValidateAndCleanPaths_RemovesMissing(t *testing.T) {
	s := newTestStorage(t)
	ctx := context.Background()

	// Create a real directory
	existingDir := t.TempDir()

	// Add paths
	s.AddPath(ctx, existingDir, nil)
	s.AddPath(ctx, "/nonexistent/path", nil)

	// Validate and clean
	removed, err := s.ValidateAndCleanPaths(ctx)
	if err != nil {
		t.Fatalf("ValidateAndCleanPaths failed: %v", err)
	}

	if len(removed) != 1 {
		t.Fatalf("expected 1 removed path, got %d", len(removed))
	}
	if removed[0] != "/nonexistent/path" {
		t.Errorf("expected removed path '/nonexistent/path', got %q", removed[0])
	}

	// Verify only existing path remains
	paths, _ := s.ListPaths(ctx)
	if len(paths) != 1 {
		t.Errorf("expected 1 remaining path, got %d", len(paths))
	}
	if paths[0].Path != existingDir {
		t.Errorf("expected remaining path %q, got %q", existingDir, paths[0].Path)
	}
}
