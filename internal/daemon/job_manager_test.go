package daemon

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/registry"
	"github.com/leefowlercu/agentic-memorizer/internal/walker"
)

// mockWalker implements walker.Walker for testing.
type mockWalker struct {
	mu              sync.Mutex
	walkCalls       []string
	walkAllCalls    int
	walkIncCalls    int
	stats           walker.WalkerStats
	discoveredPaths map[string]struct{}
	walkErr         error
}

func newMockWalker() *mockWalker {
	return &mockWalker{
		discoveredPaths: make(map[string]struct{}),
		stats:           walker.WalkerStats{FilesDiscovered: 5, DirsTraversed: 3},
	}
}

func (m *mockWalker) Walk(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.walkCalls = append(m.walkCalls, path)
	return m.walkErr
}

func (m *mockWalker) WalkAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.walkAllCalls++
	return m.walkErr
}

func (m *mockWalker) WalkIncremental(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.walkErr
}

func (m *mockWalker) WalkAllIncremental(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.walkIncCalls++
	return m.walkErr
}

func (m *mockWalker) Stats() walker.WalkerStats {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stats
}

func (m *mockWalker) DrainDiscoveredPaths() map[string]struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.discoveredPaths
}

// mockRegistry implements registry.Registry for testing.
type mockRegistry struct {
	mu            sync.Mutex
	paths         []registry.RememberedPath
	validateErr   error
	removedPaths  []string
	validateCalls int
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		paths: []registry.RememberedPath{
			{Path: "/test/path1"},
			{Path: "/test/path2"},
		},
	}
}

func (m *mockRegistry) AddPath(ctx context.Context, path string, config *registry.PathConfig) error {
	return nil
}

func (m *mockRegistry) RemovePath(ctx context.Context, path string) error {
	return nil
}

func (m *mockRegistry) GetPath(ctx context.Context, path string) (*registry.RememberedPath, error) {
	return nil, nil
}

func (m *mockRegistry) ListPaths(ctx context.Context) ([]registry.RememberedPath, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.paths, nil
}

func (m *mockRegistry) UpdatePathConfig(ctx context.Context, path string, config *registry.PathConfig) error {
	return nil
}

func (m *mockRegistry) UpdatePathLastWalk(ctx context.Context, path string, lastWalk time.Time) error {
	return nil
}

func (m *mockRegistry) FindContainingPath(ctx context.Context, filePath string) (*registry.RememberedPath, error) {
	return nil, nil
}

func (m *mockRegistry) GetEffectiveConfig(ctx context.Context, filePath string) (*registry.PathConfig, error) {
	return nil, nil
}

func (m *mockRegistry) GetFileState(ctx context.Context, path string) (*registry.FileState, error) {
	return nil, nil
}

func (m *mockRegistry) UpdateFileState(ctx context.Context, state *registry.FileState) error {
	return nil
}

func (m *mockRegistry) DeleteFileState(ctx context.Context, path string) error {
	return nil
}

func (m *mockRegistry) ListFileStates(ctx context.Context, parentPath string) ([]registry.FileState, error) {
	return nil, nil
}

func (m *mockRegistry) DeleteFileStatesForPath(ctx context.Context, parentPath string) error {
	return nil
}

func (m *mockRegistry) UpdateMetadataState(ctx context.Context, path string, contentHash string, metadataHash string, size int64, modTime time.Time) error {
	return nil
}

func (m *mockRegistry) UpdateSemanticState(ctx context.Context, path string, analysisVersion string, err error) error {
	return nil
}

func (m *mockRegistry) UpdateEmbeddingsState(ctx context.Context, path string, err error) error {
	return nil
}

func (m *mockRegistry) ClearAnalysisState(ctx context.Context, path string) error {
	return nil
}

func (m *mockRegistry) ListFilesNeedingMetadata(ctx context.Context, parentPath string) ([]registry.FileState, error) {
	return nil, nil
}

func (m *mockRegistry) ListFilesNeedingSemantic(ctx context.Context, parentPath string, maxRetries int) ([]registry.FileState, error) {
	return nil, nil
}

func (m *mockRegistry) ListFilesNeedingEmbeddings(ctx context.Context, parentPath string, maxRetries int) ([]registry.FileState, error) {
	return nil, nil
}

func (m *mockRegistry) CheckPathHealth(ctx context.Context) ([]registry.PathStatus, error) {
	return nil, nil
}

func (m *mockRegistry) ValidateAndCleanPaths(ctx context.Context) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validateCalls++
	if m.validateErr != nil {
		return nil, m.validateErr
	}
	return m.removedPaths, nil
}

func (m *mockRegistry) Close() error {
	return nil
}

func (m *mockRegistry) CountFileStates(ctx context.Context, parentPath string) (int, error) {
	return 0, nil
}

func (m *mockRegistry) CountAnalyzedFiles(ctx context.Context, parentPath string) (int, error) {
	return 0, nil
}

func TestNewJobManager(t *testing.T) {
	w := newMockWalker()
	r := newMockRegistry()
	bag := &ComponentBag{}
	hc := NewComponentHealthCollector(bag)

	m := NewJobManager(nil, w, nil, r, hc)

	if m == nil {
		t.Fatal("expected non-nil job manager")
	}
	if m.walker == nil {
		t.Error("expected walker to be set")
	}
	if m.registry == nil {
		t.Error("expected registry to be set")
	}
	if m.healthCollector != hc {
		t.Error("expected health collector to be set")
	}
	if m.jobRunner == nil {
		t.Error("expected job runner to be created")
	}
}

func TestJobManager_Rebuild_Full(t *testing.T) {
	w := newMockWalker()
	r := newMockRegistry()
	bag := &ComponentBag{}
	hc := NewComponentHealthCollector(bag)

	m := NewJobManager(nil, w, nil, r, hc, WithJobManagerLogger(slog.Default()))

	ctx := context.Background()
	result, err := m.Rebuild(ctx, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Status != "completed" {
		t.Errorf("expected status 'completed', got %s", result.Status)
	}
	if result.FilesQueued != 5 {
		t.Errorf("expected files_queued 5, got %d", result.FilesQueued)
	}
	if result.DirsProcessed != 3 {
		t.Errorf("expected dirs_processed 3, got %d", result.DirsProcessed)
	}

	if w.walkAllCalls != 1 {
		t.Errorf("expected 1 WalkAll call, got %d", w.walkAllCalls)
	}
	if w.walkIncCalls != 0 {
		t.Errorf("expected 0 WalkAllIncremental calls, got %d", w.walkIncCalls)
	}
}

func TestJobManager_Rebuild_Incremental(t *testing.T) {
	w := newMockWalker()
	r := newMockRegistry()
	bag := &ComponentBag{}
	hc := NewComponentHealthCollector(bag)

	m := NewJobManager(nil, w, nil, r, hc)

	ctx := context.Background()
	result, err := m.Rebuild(ctx, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if w.walkAllCalls != 0 {
		t.Errorf("expected 0 WalkAll calls, got %d", w.walkAllCalls)
	}
	if w.walkIncCalls != 1 {
		t.Errorf("expected 1 WalkAllIncremental call, got %d", w.walkIncCalls)
	}
}

func TestJobManager_Rebuild_NilWalker(t *testing.T) {
	r := newMockRegistry()
	bag := &ComponentBag{}
	hc := NewComponentHealthCollector(bag)

	m := NewJobManager(nil, nil, nil, r, hc)

	ctx := context.Background()
	_, err := m.Rebuild(ctx, true)
	if err == nil {
		t.Fatal("expected error for nil walker")
	}
	if err.Error() != "walker not initialized" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestJobManager_Rebuild_WalkError(t *testing.T) {
	w := newMockWalker()
	w.walkErr = errors.New("simulated walk error")
	r := newMockRegistry()
	bag := &ComponentBag{}
	hc := NewComponentHealthCollector(bag)

	m := NewJobManager(nil, w, nil, r, hc)

	ctx := context.Background()
	_, err := m.Rebuild(ctx, true)
	if err == nil {
		t.Fatal("expected error from walk failure")
	}
}

func TestJobManager_RebuildWithRecord(t *testing.T) {
	w := newMockWalker()
	r := newMockRegistry()
	bag := &ComponentBag{}
	hc := NewComponentHealthCollector(bag)

	m := NewJobManager(nil, w, nil, r, hc)

	ctx := context.Background()
	result, err := m.RebuildWithRecord(ctx, true, "job.test_rebuild")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Check job result was recorded
	jr, ok := hc.GetJobResult("job.test_rebuild")
	if !ok {
		t.Fatal("expected job result to be recorded")
	}
	if jr.Status != RunSuccess {
		t.Errorf("expected job status success, got %v", jr.Status)
	}
}

func TestJobManager_RebuildWithRecord_Failure(t *testing.T) {
	w := newMockWalker()
	w.walkErr = errors.New("simulated error")
	r := newMockRegistry()
	bag := &ComponentBag{}
	hc := NewComponentHealthCollector(bag)

	m := NewJobManager(nil, w, nil, r, hc)

	ctx := context.Background()
	_, err := m.RebuildWithRecord(ctx, true, "job.test_rebuild_fail")
	if err == nil {
		t.Fatal("expected error")
	}

	// Check job result was recorded with failure
	jr, ok := hc.GetJobResult("job.test_rebuild_fail")
	if !ok {
		t.Fatal("expected job result to be recorded even on failure")
	}
	if jr.Status != RunFailed {
		t.Errorf("expected job status failed, got %v", jr.Status)
	}
}

func TestJobManager_ValidateAndCleanPaths(t *testing.T) {
	r := newMockRegistry()
	r.removedPaths = []string{"/test/removed"}
	bag := &ComponentBag{}
	hc := NewComponentHealthCollector(bag)

	m := NewJobManager(nil, nil, nil, r, hc)

	ctx := context.Background()
	removed := m.ValidateAndCleanPaths(ctx)

	if len(removed) != 1 {
		t.Errorf("expected 1 removed path, got %d", len(removed))
	}
	if r.validateCalls != 1 {
		t.Errorf("expected 1 validate call, got %d", r.validateCalls)
	}
}

func TestJobManager_ValidateAndCleanPaths_NilRegistry(t *testing.T) {
	bag := &ComponentBag{}
	hc := NewComponentHealthCollector(bag)

	m := NewJobManager(nil, nil, nil, nil, hc)

	ctx := context.Background()
	removed := m.ValidateAndCleanPaths(ctx)

	if removed != nil {
		t.Errorf("expected nil removed paths, got %v", removed)
	}
}

func TestJobManager_ValidateAndCleanPaths_Error(t *testing.T) {
	r := newMockRegistry()
	r.validateErr = errors.New("validation error")
	bag := &ComponentBag{}
	hc := NewComponentHealthCollector(bag)

	m := NewJobManager(nil, nil, nil, r, hc, WithJobManagerLogger(slog.Default()))

	ctx := context.Background()
	removed := m.ValidateAndCleanPaths(ctx)

	if removed != nil {
		t.Errorf("expected nil removed paths on error, got %v", removed)
	}
}

func TestJobManager_InitialWalk(t *testing.T) {
	w := newMockWalker()
	r := newMockRegistry()
	bag := &ComponentBag{}
	hc := NewComponentHealthCollector(bag)

	m := NewJobManager(nil, w, nil, r, hc, WithJobManagerLogger(slog.Default()))

	ctx := context.Background()
	result, err := m.InitialWalk(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Should use full walk
	if w.walkAllCalls != 1 {
		t.Errorf("expected 1 WalkAll call for initial walk, got %d", w.walkAllCalls)
	}

	// Should record job result
	jr, ok := hc.GetJobResult("job.initial_walk")
	if !ok {
		t.Fatal("expected initial walk job result to be recorded")
	}
	if jr.Status != RunSuccess {
		t.Errorf("expected success, got %v", jr.Status)
	}
}

func TestJobManager_WalkPath(t *testing.T) {
	w := newMockWalker()
	bag := &ComponentBag{}
	hc := NewComponentHealthCollector(bag)

	m := NewJobManager(nil, w, nil, nil, hc)

	ctx := context.Background()
	err := m.WalkPath(ctx, "/test/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(w.walkCalls) != 1 {
		t.Errorf("expected 1 Walk call, got %d", len(w.walkCalls))
	}
	if w.walkCalls[0] != "/test/path" {
		t.Errorf("expected path '/test/path', got %s", w.walkCalls[0])
	}
}

func TestJobManager_WalkPath_NilWalker(t *testing.T) {
	bag := &ComponentBag{}
	hc := NewComponentHealthCollector(bag)

	m := NewJobManager(nil, nil, nil, nil, hc)

	ctx := context.Background()
	err := m.WalkPath(ctx, "/test/path")
	if err != nil {
		t.Errorf("expected nil error for nil walker, got %v", err)
	}
}

func TestJobManager_StartStopPeriodicRebuild(t *testing.T) {
	w := newMockWalker()
	r := newMockRegistry()
	bag := &ComponentBag{}
	hc := NewComponentHealthCollector(bag)

	m := NewJobManager(nil, w, nil, r, hc, WithJobManagerLogger(slog.Default()))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use a very short interval for testing
	m.StartPeriodicRebuild(ctx, 50*time.Millisecond)

	// Wait for at least one periodic rebuild
	time.Sleep(80 * time.Millisecond)

	m.StopPeriodicRebuild()

	// Should have done at least one incremental walk
	w.mu.Lock()
	incCalls := w.walkIncCalls
	w.mu.Unlock()

	if incCalls < 1 {
		t.Errorf("expected at least 1 incremental walk from periodic rebuild, got %d", incCalls)
	}
}

func TestJobManager_StopPeriodicRebuild_NotStarted(t *testing.T) {
	bag := &ComponentBag{}
	hc := NewComponentHealthCollector(bag)

	m := NewJobManager(nil, nil, nil, nil, hc, WithJobManagerLogger(slog.Default()))

	// Should not panic
	m.StopPeriodicRebuild()
}

func TestJobManager_WithJobManagerLogger(t *testing.T) {
	bag := &ComponentBag{}
	hc := NewComponentHealthCollector(bag)

	m := NewJobManager(nil, nil, nil, nil, hc, WithJobManagerLogger(slog.Default()))

	if m == nil {
		t.Fatal("expected non-nil job manager")
	}
}
