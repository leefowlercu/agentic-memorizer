package registry

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpen(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	reg, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to open registry: %v", err)
	}
	defer reg.Close()

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestOpen_CreatesDirectory(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "subdir", "nested", "test.db")

	reg, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to open registry: %v", err)
	}
	defer reg.Close()

	// Verify directory structure was created
	dir := filepath.Dir(dbPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("database directory was not created")
	}
}

func TestAddPath(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()
	testPath := "/test/project"

	err := reg.AddPath(ctx, testPath, nil)
	if err != nil {
		t.Fatalf("failed to add path: %v", err)
	}

	// Verify path was added
	rp, err := reg.GetPath(ctx, testPath)
	if err != nil {
		t.Fatalf("failed to get path: %v", err)
	}
	if rp.Path != testPath {
		t.Errorf("expected path %q, got %q", testPath, rp.Path)
	}
}

func TestAddPath_WithConfig(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()
	testPath := "/test/project"
	config := &PathConfig{
		SkipExtensions:  []string{".exe", ".dll"},
		SkipDirectories: []string{"node_modules", ".git"},
		SkipHidden:      true,
	}

	err := reg.AddPath(ctx, testPath, config)
	if err != nil {
		t.Fatalf("failed to add path: %v", err)
	}

	// Verify config was stored
	rp, err := reg.GetPath(ctx, testPath)
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
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()
	testPath := "/test/project"

	// Add path first time
	err := reg.AddPath(ctx, testPath, nil)
	if err != nil {
		t.Fatalf("failed to add path: %v", err)
	}

	// Add same path again
	err = reg.AddPath(ctx, testPath, nil)
	if err != ErrPathExists {
		t.Errorf("expected ErrPathExists, got %v", err)
	}
}

func TestRemovePath(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()
	testPath := "/test/project"

	// Add then remove
	reg.AddPath(ctx, testPath, nil)
	err := reg.RemovePath(ctx, testPath)
	if err != nil {
		t.Fatalf("failed to remove path: %v", err)
	}

	// Verify path was removed
	_, err = reg.GetPath(ctx, testPath)
	if err != ErrPathNotFound {
		t.Errorf("expected ErrPathNotFound, got %v", err)
	}
}

func TestRemovePath_NotFound(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()
	err := reg.RemovePath(ctx, "/nonexistent")
	if err != ErrPathNotFound {
		t.Errorf("expected ErrPathNotFound, got %v", err)
	}
}

func TestListPaths(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()

	// Add multiple paths
	paths := []string{"/test/a", "/test/b", "/test/c"}
	for _, p := range paths {
		if err := reg.AddPath(ctx, p, nil); err != nil {
			t.Fatalf("failed to add path %q: %v", p, err)
		}
	}

	// List paths
	result, err := reg.ListPaths(ctx)
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

func TestListPaths_Empty(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()
	result, err := reg.ListPaths(ctx)
	if err != nil {
		t.Fatalf("failed to list paths: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 paths, got %d", len(result))
	}
}

func TestUpdatePathConfig(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()
	testPath := "/test/project"

	// Add path with initial config
	initialConfig := &PathConfig{SkipHidden: false}
	reg.AddPath(ctx, testPath, initialConfig)

	// Update config
	newConfig := &PathConfig{
		SkipHidden:     true,
		SkipExtensions: []string{".log"},
	}
	err := reg.UpdatePathConfig(ctx, testPath, newConfig)
	if err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// Verify update
	rp, _ := reg.GetPath(ctx, testPath)
	if !rp.Config.SkipHidden {
		t.Error("expected SkipHidden to be true after update")
	}
	if len(rp.Config.SkipExtensions) != 1 {
		t.Errorf("expected 1 skip extension, got %d", len(rp.Config.SkipExtensions))
	}
}

func TestUpdatePathConfig_NotFound(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()
	err := reg.UpdatePathConfig(ctx, "/nonexistent", &PathConfig{})
	if err != ErrPathNotFound {
		t.Errorf("expected ErrPathNotFound, got %v", err)
	}
}

func TestUpdatePathLastWalk(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()
	testPath := "/test/project"
	reg.AddPath(ctx, testPath, nil)

	// Update last walk
	walkTime := time.Now().Truncate(time.Second)
	err := reg.UpdatePathLastWalk(ctx, testPath, walkTime)
	if err != nil {
		t.Fatalf("failed to update last walk: %v", err)
	}

	// Verify update
	rp, _ := reg.GetPath(ctx, testPath)
	if rp.LastWalkAt == nil {
		t.Fatal("expected LastWalkAt to be set")
	}
	if !rp.LastWalkAt.Truncate(time.Second).Equal(walkTime) {
		t.Errorf("expected LastWalkAt %v, got %v", walkTime, *rp.LastWalkAt)
	}
}

func TestFindContainingPath(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()

	// Add some remembered paths
	reg.AddPath(ctx, "/projects", nil)
	reg.AddPath(ctx, "/projects/myapp", nil)
	reg.AddPath(ctx, "/documents", nil)

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
			wantPath: "/projects/myapp", // Should find closest ancestor
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
			rp, err := reg.FindContainingPath(ctx, tt.filePath)
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

func TestGetEffectiveConfig(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()

	config := &PathConfig{
		SkipExtensions: []string{".exe"},
		SkipHidden:     true,
	}
	reg.AddPath(ctx, "/projects/myapp", config)

	// Get effective config for file in remembered path
	effectiveConfig, err := reg.GetEffectiveConfig(ctx, "/projects/myapp/src/main.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if effectiveConfig == nil {
		t.Fatal("expected config to be set")
	}
	if !effectiveConfig.SkipHidden {
		t.Error("expected SkipHidden to be true")
	}
}

func TestFileState_CRUD(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

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

	err := reg.UpdateFileState(ctx, state)
	if err != nil {
		t.Fatalf("failed to create file state: %v", err)
	}

	// Read file state
	result, err := reg.GetFileState(ctx, testPath)
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
	err = reg.UpdateFileState(ctx, state)
	if err != nil {
		t.Fatalf("failed to update file state: %v", err)
	}

	result, _ = reg.GetFileState(ctx, testPath)
	if result.ContentHash != "xyz789" {
		t.Errorf("expected content hash xyz789, got %s", result.ContentHash)
	}
	if result.Size != 2048 {
		t.Errorf("expected size 2048, got %d", result.Size)
	}

	// Delete file state
	err = reg.DeleteFileState(ctx, testPath)
	if err != nil {
		t.Fatalf("failed to delete file state: %v", err)
	}

	_, err = reg.GetFileState(ctx, testPath)
	if err != ErrPathNotFound {
		t.Errorf("expected ErrPathNotFound, got %v", err)
	}
}

func TestFileState_WithAnalysis(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()
	testPath := "/test/file.go"
	modTime := time.Now().Truncate(time.Second)
	analyzedAt := time.Now().Add(-time.Hour).Truncate(time.Second)

	state := &FileState{
		Path:            testPath,
		ContentHash:     "abc123",
		MetadataHash:    "def456",
		Size:            1024,
		ModTime:         modTime,
		LastAnalyzedAt:  &analyzedAt,
		AnalysisVersion: "1.0.0",
	}

	err := reg.UpdateFileState(ctx, state)
	if err != nil {
		t.Fatalf("failed to create file state: %v", err)
	}

	result, err := reg.GetFileState(ctx, testPath)
	if err != nil {
		t.Fatalf("failed to get file state: %v", err)
	}

	if result.LastAnalyzedAt == nil {
		t.Fatal("expected LastAnalyzedAt to be set")
	}
	if result.AnalysisVersion != "1.0.0" {
		t.Errorf("expected analysis version 1.0.0, got %s", result.AnalysisVersion)
	}
}

func TestListFileStates(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

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
		reg.UpdateFileState(ctx, state)
	}

	// Add file state under different path
	otherState := &FileState{
		Path:         "/documents/report.pdf",
		ContentHash:  "other",
		MetadataHash: "other",
		Size:         200,
		ModTime:      modTime,
	}
	reg.UpdateFileState(ctx, otherState)

	// List file states for parent path
	result, err := reg.ListFileStates(ctx, "/projects/myapp")
	if err != nil {
		t.Fatalf("failed to list file states: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 file states, got %d", len(result))
	}
}

func TestDeleteFileStatesForPath(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

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
		reg.UpdateFileState(ctx, state)
	}

	// Delete file states for /projects/myapp
	err := reg.DeleteFileStatesForPath(ctx, "/projects/myapp")
	if err != nil {
		t.Fatalf("failed to delete file states: %v", err)
	}

	// Verify myapp files are deleted
	result, _ := reg.ListFileStates(ctx, "/projects/myapp")
	if len(result) != 0 {
		t.Errorf("expected 0 file states for myapp, got %d", len(result))
	}

	// Verify documents file still exists
	_, err = reg.GetFileState(ctx, "/documents/c.pdf")
	if err != nil {
		t.Errorf("documents file should still exist: %v", err)
	}
}

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

func TestPathConfig_JSON(t *testing.T) {
	config := &PathConfig{
		SkipExtensions:     []string{".exe", ".dll"},
		SkipDirectories:    []string{"node_modules"},
		SkipFiles:          []string{".DS_Store"},
		SkipHidden:         true,
		IncludeExtensions:  []string{".env"},
		IncludeDirectories: []string{".github"},
		IncludeFiles:       []string{".gitignore"},
		IncludeHidden:      true,
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

func TestMigrations_Idempotent(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	// Open registry (runs migrations)
	reg1, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to open registry first time: %v", err)
	}
	reg1.Close()

	// Open again (should not fail on existing schema)
	reg2, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to open registry second time: %v", err)
	}
	reg2.Close()
}

func TestPathCleaning(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()

	// Add path with trailing slash
	err := reg.AddPath(ctx, "/test/project/", nil)
	if err != nil {
		t.Fatalf("failed to add path: %v", err)
	}

	// Get path without trailing slash
	rp, err := reg.GetPath(ctx, "/test/project")
	if err != nil {
		t.Fatalf("failed to get path: %v", err)
	}

	if rp.Path != "/test/project" {
		t.Errorf("expected cleaned path /test/project, got %s", rp.Path)
	}
}

func TestUpdateMetadataState(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()
	testPath := "/test/file.go"
	modTime := time.Now().Truncate(time.Second)

	// Insert initial metadata state
	err := reg.UpdateMetadataState(ctx, testPath, "contenthash123", "metahash456", 1024, modTime)
	if err != nil {
		t.Fatalf("failed to update metadata state: %v", err)
	}

	// Verify state
	state, err := reg.GetFileState(ctx, testPath)
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
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()
	testPath := "/test/file.go"
	modTime := time.Now().Truncate(time.Second)

	// First create the file state
	err := reg.UpdateMetadataState(ctx, testPath, "hash", "meta", 100, modTime)
	if err != nil {
		t.Fatalf("failed to setup file state: %v", err)
	}

	// Update semantic state as success
	err = reg.UpdateSemanticState(ctx, testPath, "1.0.0", nil)
	if err != nil {
		t.Fatalf("failed to update semantic state: %v", err)
	}

	// Verify state
	state, err := reg.GetFileState(ctx, testPath)
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
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()
	testPath := "/test/file.go"
	modTime := time.Now().Truncate(time.Second)

	// First create the file state
	err := reg.UpdateMetadataState(ctx, testPath, "hash", "meta", 100, modTime)
	if err != nil {
		t.Fatalf("failed to setup file state: %v", err)
	}

	// Update semantic state as failure
	analysisErr := errors.New("API rate limit exceeded")
	err = reg.UpdateSemanticState(ctx, testPath, "1.0.0", analysisErr)
	if err != nil {
		t.Fatalf("failed to update semantic state: %v", err)
	}

	// Verify state
	state, err := reg.GetFileState(ctx, testPath)
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
	err = reg.UpdateSemanticState(ctx, testPath, "1.0.0", analysisErr)
	if err != nil {
		t.Fatalf("failed to update semantic state again: %v", err)
	}

	state, _ = reg.GetFileState(ctx, testPath)
	if state.SemanticRetryCount != 2 {
		t.Errorf("expected retry count 2, got %d", state.SemanticRetryCount)
	}
}

func TestUpdateEmbeddingsState(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()
	testPath := "/test/file.go"
	modTime := time.Now().Truncate(time.Second)

	// Setup file state
	err := reg.UpdateMetadataState(ctx, testPath, "hash", "meta", 100, modTime)
	if err != nil {
		t.Fatalf("failed to setup file state: %v", err)
	}

	// Update embeddings state as success
	err = reg.UpdateEmbeddingsState(ctx, testPath, nil)
	if err != nil {
		t.Fatalf("failed to update embeddings state: %v", err)
	}

	state, _ := reg.GetFileState(ctx, testPath)
	if state.EmbeddingsAnalyzedAt == nil {
		t.Error("expected EmbeddingsAnalyzedAt to be set")
	}
	if state.EmbeddingsError != nil {
		t.Error("expected EmbeddingsError to be nil")
	}
}

func TestClearAnalysisState(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()
	testPath := "/test/file.go"
	modTime := time.Now().Truncate(time.Second)

	// Setup file state with analysis
	err := reg.UpdateMetadataState(ctx, testPath, "hash", "meta", 100, modTime)
	if err != nil {
		t.Fatalf("failed to setup file state: %v", err)
	}

	err = reg.UpdateSemanticState(ctx, testPath, "1.0.0", nil)
	if err != nil {
		t.Fatalf("failed to setup semantic state: %v", err)
	}

	err = reg.UpdateEmbeddingsState(ctx, testPath, nil)
	if err != nil {
		t.Fatalf("failed to setup embeddings state: %v", err)
	}

	// Clear analysis state
	err = reg.ClearAnalysisState(ctx, testPath)
	if err != nil {
		t.Fatalf("failed to clear analysis state: %v", err)
	}

	// Verify all analysis timestamps are cleared
	state, _ := reg.GetFileState(ctx, testPath)
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
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()
	modTime := time.Now().Truncate(time.Second)

	// Create files with different states
	// File needing metadata
	reg.UpdateFileState(ctx, &FileState{
		Path:         "/test/needs-metadata.go",
		ContentHash:  "hash1",
		MetadataHash: "meta1",
		Size:         100,
		ModTime:      modTime,
	})

	// File with metadata but needing semantic
	reg.UpdateMetadataState(ctx, "/test/needs-semantic.go", "hash2", "meta2", 200, modTime)

	// File with semantic but needing embeddings
	reg.UpdateMetadataState(ctx, "/test/needs-embeddings.go", "hash3", "meta3", 300, modTime)
	reg.UpdateSemanticState(ctx, "/test/needs-embeddings.go", "1.0.0", nil)

	// Fully analyzed file
	reg.UpdateMetadataState(ctx, "/test/complete.go", "hash4", "meta4", 400, modTime)
	reg.UpdateSemanticState(ctx, "/test/complete.go", "1.0.0", nil)
	reg.UpdateEmbeddingsState(ctx, "/test/complete.go", nil)

	// Test ListFilesNeedingMetadata
	needsMetadata, err := reg.ListFilesNeedingMetadata(ctx, "/test")
	if err != nil {
		t.Fatalf("ListFilesNeedingMetadata failed: %v", err)
	}
	if len(needsMetadata) != 1 {
		t.Errorf("expected 1 file needing metadata, got %d", len(needsMetadata))
	}

	// Test ListFilesNeedingSemantic
	needsSemantic, err := reg.ListFilesNeedingSemantic(ctx, "/test", 3)
	if err != nil {
		t.Fatalf("ListFilesNeedingSemantic failed: %v", err)
	}
	if len(needsSemantic) != 1 {
		t.Errorf("expected 1 file needing semantic, got %d", len(needsSemantic))
	}

	// Test ListFilesNeedingEmbeddings
	needsEmbeddings, err := reg.ListFilesNeedingEmbeddings(ctx, "/test", 3)
	if err != nil {
		t.Fatalf("ListFilesNeedingEmbeddings failed: %v", err)
	}
	if len(needsEmbeddings) != 1 {
		t.Errorf("expected 1 file needing embeddings, got %d", len(needsEmbeddings))
	}
}

func TestListFilesNeedingSemantic_RespectsMaxRetries(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()
	modTime := time.Now().Truncate(time.Second)

	// Create file with metadata
	reg.UpdateMetadataState(ctx, "/test/file.go", "hash", "meta", 100, modTime)

	// Fail semantic analysis 3 times
	for i := 0; i < 3; i++ {
		reg.UpdateSemanticState(ctx, "/test/file.go", "1.0.0", errors.New("error"))
	}

	// With maxRetries=3, file should be excluded
	needsSemantic, _ := reg.ListFilesNeedingSemantic(ctx, "/test", 3)
	if len(needsSemantic) != 0 {
		t.Errorf("expected 0 files (maxRetries exceeded), got %d", len(needsSemantic))
	}

	// With maxRetries=5, file should be included
	needsSemantic, _ = reg.ListFilesNeedingSemantic(ctx, "/test", 5)
	if len(needsSemantic) != 1 {
		t.Errorf("expected 1 file, got %d", len(needsSemantic))
	}
}

// Tests for path health checking

func TestPathStatusConstants(t *testing.T) {
	// Verify the path status constants have expected values
	tests := []struct {
		constant string
		expected string
	}{
		{PathStatusOK, "ok"},
		{PathStatusMissing, "missing"},
		{PathStatusDenied, "denied"},
		{PathStatusError, "error"},
	}

	for _, tt := range tests {
		if tt.constant != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, tt.constant)
		}
	}
}

func TestCheckPathHealth_EmptyRegistry(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()

	// Check health on empty registry
	statuses, err := reg.CheckPathHealth(ctx)
	if err != nil {
		t.Fatalf("CheckPathHealth failed: %v", err)
	}

	if len(statuses) != 0 {
		t.Errorf("expected 0 statuses for empty registry, got %d", len(statuses))
	}
}

func TestCheckPathHealth_ExistingPath(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()

	// Create a real directory to test against
	testDir := t.TempDir()

	// Add the existing path
	err := reg.AddPath(ctx, testDir, nil)
	if err != nil {
		t.Fatalf("failed to add path: %v", err)
	}

	// Check health
	statuses, err := reg.CheckPathHealth(ctx)
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
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()

	// Add a path that doesn't exist on the filesystem
	missingPath := "/nonexistent/path/that/does/not/exist"
	err := reg.AddPath(ctx, missingPath, nil)
	if err != nil {
		t.Fatalf("failed to add path: %v", err)
	}

	// Check health
	statuses, err := reg.CheckPathHealth(ctx)
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

func TestCheckPathHealth_MultiplePaths(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()

	// Create a real directory
	existingDir := t.TempDir()

	// Add multiple paths with different statuses
	reg.AddPath(ctx, existingDir, nil)
	reg.AddPath(ctx, "/nonexistent/path1", nil)
	reg.AddPath(ctx, "/nonexistent/path2", nil)

	// Check health
	statuses, err := reg.CheckPathHealth(ctx)
	if err != nil {
		t.Fatalf("CheckPathHealth failed: %v", err)
	}

	if len(statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(statuses))
	}

	// Count statuses by type
	statusCounts := make(map[string]int)
	for _, s := range statuses {
		statusCounts[s.Status]++
	}

	if statusCounts[PathStatusOK] != 1 {
		t.Errorf("expected 1 OK status, got %d", statusCounts[PathStatusOK])
	}
	if statusCounts[PathStatusMissing] != 2 {
		t.Errorf("expected 2 missing statuses, got %d", statusCounts[PathStatusMissing])
	}
}

func TestValidateAndCleanPaths_RemovesMissing(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()

	// Create a real directory
	existingDir := t.TempDir()

	// Add paths
	reg.AddPath(ctx, existingDir, nil)
	reg.AddPath(ctx, "/nonexistent/path", nil)

	// Validate and clean
	removed, err := reg.ValidateAndCleanPaths(ctx)
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
	paths, _ := reg.ListPaths(ctx)
	if len(paths) != 1 {
		t.Errorf("expected 1 remaining path, got %d", len(paths))
	}
	if paths[0].Path != existingDir {
		t.Errorf("expected remaining path %q, got %q", existingDir, paths[0].Path)
	}
}

func TestValidateAndCleanPaths_CleansFileState(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()
	modTime := time.Now().Truncate(time.Second)

	// Add a path that doesn't exist
	missingPath := "/nonexistent/project"
	reg.AddPath(ctx, missingPath, nil)

	// Add file states under that path
	for _, f := range []string{
		"/nonexistent/project/file1.go",
		"/nonexistent/project/file2.go",
		"/nonexistent/project/subdir/file3.go",
	} {
		reg.UpdateFileState(ctx, &FileState{
			Path:         f,
			ContentHash:  "hash",
			MetadataHash: "meta",
			Size:         100,
			ModTime:      modTime,
		})
	}

	// Verify file states exist
	states, _ := reg.ListFileStates(ctx, missingPath)
	if len(states) != 3 {
		t.Fatalf("expected 3 file states before cleanup, got %d", len(states))
	}

	// Validate and clean
	_, err := reg.ValidateAndCleanPaths(ctx)
	if err != nil {
		t.Fatalf("ValidateAndCleanPaths failed: %v", err)
	}

	// Verify file states are cleaned up
	states, _ = reg.ListFileStates(ctx, missingPath)
	if len(states) != 0 {
		t.Errorf("expected 0 file states after cleanup, got %d", len(states))
	}
}

func TestValidateAndCleanPaths_PreservesNonMissing(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()
	modTime := time.Now().Truncate(time.Second)

	// Create a real directory
	existingDir := t.TempDir()

	// Add paths
	reg.AddPath(ctx, existingDir, nil)

	// Add file states under existing path
	existingFile := filepath.Join(existingDir, "file.go")
	reg.UpdateFileState(ctx, &FileState{
		Path:         existingFile,
		ContentHash:  "hash",
		MetadataHash: "meta",
		Size:         100,
		ModTime:      modTime,
	})

	// Validate and clean
	removed, err := reg.ValidateAndCleanPaths(ctx)
	if err != nil {
		t.Fatalf("ValidateAndCleanPaths failed: %v", err)
	}

	// No paths should be removed
	if len(removed) != 0 {
		t.Errorf("expected 0 removed paths, got %d", len(removed))
	}

	// Verify path and file state still exist
	paths, _ := reg.ListPaths(ctx)
	if len(paths) != 1 {
		t.Errorf("expected 1 path, got %d", len(paths))
	}

	states, _ := reg.ListFileStates(ctx, existingDir)
	if len(states) != 1 {
		t.Errorf("expected 1 file state, got %d", len(states))
	}
}

func TestValidateAndCleanPaths_EmptyRegistry(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()

	// Validate and clean empty registry
	removed, err := reg.ValidateAndCleanPaths(ctx)
	if err != nil {
		t.Fatalf("ValidateAndCleanPaths failed: %v", err)
	}

	if len(removed) != 0 {
		t.Errorf("expected 0 removed paths, got %d", len(removed))
	}
}

func TestValidateAndCleanPaths_AllMissing(t *testing.T) {
	reg := newTestRegistry(t)
	defer reg.Close()

	ctx := context.Background()

	// Add only missing paths
	reg.AddPath(ctx, "/nonexistent/path1", nil)
	reg.AddPath(ctx, "/nonexistent/path2", nil)

	// Validate and clean
	removed, err := reg.ValidateAndCleanPaths(ctx)
	if err != nil {
		t.Fatalf("ValidateAndCleanPaths failed: %v", err)
	}

	if len(removed) != 2 {
		t.Errorf("expected 2 removed paths, got %d", len(removed))
	}

	// Verify registry is empty
	paths, _ := reg.ListPaths(ctx)
	if len(paths) != 0 {
		t.Errorf("expected 0 remaining paths, got %d", len(paths))
	}
}

// Helper functions

func newTestRegistry(t *testing.T) *SQLiteRegistry {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	reg, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to open registry: %v", err)
	}

	return reg
}

func boolPtr(b bool) *bool {
	return &b
}
