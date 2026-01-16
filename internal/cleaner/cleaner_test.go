package cleaner

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

// mockRegistry implements registry.Registry for testing.
type mockRegistry struct {
	mu                    sync.Mutex
	fileStates            map[string]registry.FileState
	deletedPaths          []string
	bulkDeletedPaths      []string
	deleteError           error
	bulkDeleteError       error
	listStatesError       error
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		fileStates: make(map[string]registry.FileState),
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
	return nil, nil
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
	m.mu.Lock()
	defer m.mu.Unlock()
	if state, ok := m.fileStates[path]; ok {
		return &state, nil
	}
	return nil, registry.ErrPathNotFound
}

func (m *mockRegistry) UpdateFileState(ctx context.Context, state *registry.FileState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fileStates[state.Path] = *state
	return nil
}

func (m *mockRegistry) DeleteFileState(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deleteError != nil {
		return m.deleteError
	}
	m.deletedPaths = append(m.deletedPaths, path)
	delete(m.fileStates, path)
	return nil
}

func (m *mockRegistry) ListFileStates(ctx context.Context, parentPath string) ([]registry.FileState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listStatesError != nil {
		return nil, m.listStatesError
	}
	var states []registry.FileState
	for _, state := range m.fileStates {
		states = append(states, state)
	}
	return states, nil
}

func (m *mockRegistry) DeleteFileStatesForPath(ctx context.Context, parentPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.bulkDeleteError != nil {
		return m.bulkDeleteError
	}
	m.bulkDeletedPaths = append(m.bulkDeletedPaths, parentPath)
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

func (m *mockRegistry) Close() error {
	return nil
}

func (m *mockRegistry) CheckPathHealth(ctx context.Context) ([]registry.PathStatus, error) {
	return nil, nil
}

func (m *mockRegistry) ValidateAndCleanPaths(ctx context.Context) ([]string, error) {
	return nil, nil
}

// mockGraph implements graph.Graph for testing.
type mockGraph struct {
	mu                        sync.Mutex
	deletedPaths              []string
	deletedDirectories        []string
	deletedFilesUnderPaths    []string
	deletedDirsUnderPaths     []string
	deleteError               error
	deleteDirectoryError      error
	deleteFilesUnderError     error
	deleteDirsUnderError      error
}

func newMockGraph() *mockGraph {
	return &mockGraph{}
}

func (m *mockGraph) Name() string {
	return "mock-graph"
}

func (m *mockGraph) Start(ctx context.Context) error {
	return nil
}

func (m *mockGraph) Stop(ctx context.Context) error {
	return nil
}

func (m *mockGraph) UpsertFile(ctx context.Context, file *graph.FileNode) error {
	return nil
}

func (m *mockGraph) DeleteFile(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deleteError != nil {
		return m.deleteError
	}
	m.deletedPaths = append(m.deletedPaths, path)
	return nil
}

func (m *mockGraph) GetFile(ctx context.Context, path string) (*graph.FileNode, error) {
	return nil, nil
}

func (m *mockGraph) UpsertDirectory(ctx context.Context, dir *graph.DirectoryNode) error {
	return nil
}

func (m *mockGraph) DeleteDirectory(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deleteDirectoryError != nil {
		return m.deleteDirectoryError
	}
	m.deletedDirectories = append(m.deletedDirectories, path)
	return nil
}

func (m *mockGraph) DeleteFilesUnderPath(ctx context.Context, parentPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deleteFilesUnderError != nil {
		return m.deleteFilesUnderError
	}
	m.deletedFilesUnderPaths = append(m.deletedFilesUnderPaths, parentPath)
	return nil
}

func (m *mockGraph) DeleteDirectoriesUnderPath(ctx context.Context, parentPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deleteDirsUnderError != nil {
		return m.deleteDirsUnderError
	}
	m.deletedDirsUnderPaths = append(m.deletedDirsUnderPaths, parentPath)
	return nil
}

func (m *mockGraph) UpsertChunkWithMetadata(ctx context.Context, chunk *graph.ChunkNode, meta *chunkers.ChunkMetadata) error {
	return nil
}

func (m *mockGraph) UpsertChunkEmbedding(ctx context.Context, chunkID string, emb *graph.ChunkEmbeddingNode) error {
	return nil
}

func (m *mockGraph) DeleteChunkEmbeddings(ctx context.Context, chunkID string, provider, model string) error {
	return nil
}

func (m *mockGraph) DeleteChunks(ctx context.Context, filePath string) error {
	return nil
}

func (m *mockGraph) SetFileTags(ctx context.Context, path string, tags []string) error {
	return nil
}

func (m *mockGraph) SetFileTopics(ctx context.Context, path string, topics []graph.Topic) error {
	return nil
}

func (m *mockGraph) SetFileEntities(ctx context.Context, path string, entities []graph.Entity) error {
	return nil
}

func (m *mockGraph) SetFileReferences(ctx context.Context, path string, refs []graph.Reference) error {
	return nil
}

func (m *mockGraph) Query(ctx context.Context, cypher string) (*graph.QueryResult, error) {
	return nil, nil
}

func (m *mockGraph) HasEmbedding(ctx context.Context, contentHash string, version int) (bool, error) {
	return false, nil
}

func (m *mockGraph) ExportSnapshot(ctx context.Context) (*graph.GraphSnapshot, error) {
	return nil, nil
}

func (m *mockGraph) GetFileWithRelations(ctx context.Context, path string) (*graph.FileWithRelations, error) {
	return nil, nil
}

func (m *mockGraph) SearchSimilarChunks(ctx context.Context, embedding []float32, k int) ([]graph.ChunkNode, error) {
	return nil, nil
}

func (m *mockGraph) IsConnected() bool {
	return true
}

func TestCleaner_New(t *testing.T) {
	reg := newMockRegistry()
	bus := events.NewBus()
	defer bus.Close()

	c := New(reg, nil, bus)
	if c == nil {
		t.Fatal("expected non-nil cleaner")
	}
	if c.registry != reg {
		t.Error("expected registry to be set")
	}
	if c.bus != bus {
		t.Error("expected bus to be set")
	}
}

func TestCleaner_DeletePath(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	// Add a file state
	reg.fileStates["/test/file.go"] = registry.FileState{Path: "/test/file.go"}

	c := New(reg, g, bus)

	err := c.DeletePath(context.Background(), "/test/file.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify registry deletion
	reg.mu.Lock()
	if len(reg.deletedPaths) != 1 || reg.deletedPaths[0] != "/test/file.go" {
		t.Errorf("expected registry delete to be called with /test/file.go, got %v", reg.deletedPaths)
	}
	reg.mu.Unlock()

	// Verify graph deletion
	g.mu.Lock()
	if len(g.deletedPaths) != 1 || g.deletedPaths[0] != "/test/file.go" {
		t.Errorf("expected graph delete to be called with /test/file.go, got %v", g.deletedPaths)
	}
	g.mu.Unlock()
}

func TestCleaner_DeletePath_GraphNil(t *testing.T) {
	reg := newMockRegistry()
	bus := events.NewBus()
	defer bus.Close()

	reg.fileStates["/test/file.go"] = registry.FileState{Path: "/test/file.go"}

	// Create cleaner with nil graph
	c := New(reg, nil, bus)

	err := c.DeletePath(context.Background(), "/test/file.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify registry was still deleted
	reg.mu.Lock()
	if len(reg.deletedPaths) != 1 {
		t.Errorf("expected registry delete to be called, got %d calls", len(reg.deletedPaths))
	}
	reg.mu.Unlock()
}

func TestCleaner_DeletePath_GraphError(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	reg.fileStates["/test/file.go"] = registry.FileState{Path: "/test/file.go"}
	g.deleteError = errors.New("graph connection failed")

	c := New(reg, g, bus)

	// Should not return error even if graph fails
	err := c.DeletePath(context.Background(), "/test/file.go")
	if err != nil {
		t.Fatalf("expected no error even with graph failure, got: %v", err)
	}

	// Verify registry was still deleted
	reg.mu.Lock()
	if len(reg.deletedPaths) != 1 {
		t.Errorf("expected registry delete to be called, got %d calls", len(reg.deletedPaths))
	}
	reg.mu.Unlock()
}

func TestCleaner_Reconcile(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	// Add some file states
	reg.fileStates["/test/file1.go"] = registry.FileState{Path: "/test/file1.go"}
	reg.fileStates["/test/file2.go"] = registry.FileState{Path: "/test/file2.go"}
	reg.fileStates["/test/file3.go"] = registry.FileState{Path: "/test/file3.go"}

	c := New(reg, g, bus)

	// Only file1 and file2 were discovered (file3 is stale)
	discoveredPaths := map[string]struct{}{
		"/test/file1.go": {},
		"/test/file2.go": {},
	}

	result, err := c.Reconcile(context.Background(), "/test", discoveredPaths)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FilesChecked != 3 {
		t.Errorf("expected FilesChecked=3, got %d", result.FilesChecked)
	}
	if result.StaleFound != 1 {
		t.Errorf("expected StaleFound=1, got %d", result.StaleFound)
	}
	if result.StaleRemoved != 1 {
		t.Errorf("expected StaleRemoved=1, got %d", result.StaleRemoved)
	}
	if result.Errors != 0 {
		t.Errorf("expected Errors=0, got %d", result.Errors)
	}

	// Verify file3 was deleted from registry and graph
	reg.mu.Lock()
	deleted := false
	for _, p := range reg.deletedPaths {
		if p == "/test/file3.go" {
			deleted = true
			break
		}
	}
	reg.mu.Unlock()
	if !deleted {
		t.Error("expected file3.go to be deleted from registry")
	}

	g.mu.Lock()
	deleted = false
	for _, p := range g.deletedPaths {
		if p == "/test/file3.go" {
			deleted = true
			break
		}
	}
	g.mu.Unlock()
	if !deleted {
		t.Error("expected file3.go to be deleted from graph")
	}
}

func TestCleaner_Reconcile_NoStale(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	// Add file states
	reg.fileStates["/test/file1.go"] = registry.FileState{Path: "/test/file1.go"}
	reg.fileStates["/test/file2.go"] = registry.FileState{Path: "/test/file2.go"}

	c := New(reg, g, bus)

	// All files were discovered
	discoveredPaths := map[string]struct{}{
		"/test/file1.go": {},
		"/test/file2.go": {},
	}

	result, err := c.Reconcile(context.Background(), "/test", discoveredPaths)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FilesChecked != 2 {
		t.Errorf("expected FilesChecked=2, got %d", result.FilesChecked)
	}
	if result.StaleFound != 0 {
		t.Errorf("expected StaleFound=0, got %d", result.StaleFound)
	}
	if result.StaleRemoved != 0 {
		t.Errorf("expected StaleRemoved=0, got %d", result.StaleRemoved)
	}

	// Verify nothing was deleted
	reg.mu.Lock()
	if len(reg.deletedPaths) != 0 {
		t.Errorf("expected no deletions, got %d", len(reg.deletedPaths))
	}
	reg.mu.Unlock()
}

func TestCleaner_Reconcile_ListStatesError(t *testing.T) {
	reg := newMockRegistry()
	bus := events.NewBus()
	defer bus.Close()

	reg.listStatesError = errors.New("database error")

	c := New(reg, nil, bus)

	_, err := c.Reconcile(context.Background(), "/test", map[string]struct{}{})
	if err == nil {
		t.Fatal("expected error when listing file states fails")
	}
}

func TestCleaner_Start_SubscribesToEvents(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	reg.fileStates["/test/deleted.go"] = registry.FileState{Path: "/test/deleted.go"}

	c := New(reg, g, bus)

	err := c.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer c.Stop()

	// Publish a PathDeleted event
	event := events.NewEvent(events.PathDeleted, events.FileEvent{
		Path: "/test/deleted.go",
	})
	bus.Publish(context.Background(), event)

	// Wait for event processing
	time.Sleep(100 * time.Millisecond)

	// Verify file was deleted
	reg.mu.Lock()
	found := false
	for _, p := range reg.deletedPaths {
		if p == "/test/deleted.go" {
			found = true
			break
		}
	}
	reg.mu.Unlock()

	if !found {
		t.Error("expected PathDeleted event to trigger deletion")
	}
}

func TestCleaner_HandleDelete_InvalidPayload(t *testing.T) {
	reg := newMockRegistry()
	bus := events.NewBus()
	defer bus.Close()

	c := New(reg, nil, bus)

	err := c.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer c.Stop()

	// Publish event with wrong payload type
	event := events.Event{
		Type:      events.PathDeleted,
		Timestamp: time.Now(),
		Payload:   "wrong type", // Should be events.FileEvent
	}
	bus.Publish(context.Background(), event)

	// Wait for event processing
	time.Sleep(50 * time.Millisecond)

	// Verify no deletion occurred
	reg.mu.Lock()
	if len(reg.deletedPaths) != 0 {
		t.Errorf("expected no deletions with invalid payload, got %d", len(reg.deletedPaths))
	}
	reg.mu.Unlock()
}

func TestCleaner_Stop_Unsubscribes(t *testing.T) {
	reg := newMockRegistry()
	bus := events.NewBus()
	defer bus.Close()

	c := New(reg, nil, bus)

	err := c.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Stop the cleaner
	err = c.Stop()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Add a file state after stop
	reg.fileStates["/test/file.go"] = registry.FileState{Path: "/test/file.go"}

	// Publish event after stop
	event := events.NewEvent(events.PathDeleted, events.FileEvent{
		Path: "/test/file.go",
	})
	bus.Publish(context.Background(), event)

	// Wait for any potential event processing
	time.Sleep(50 * time.Millisecond)

	// Verify no deletion occurred (unsubscribed)
	reg.mu.Lock()
	if len(reg.deletedPaths) != 0 {
		t.Errorf("expected no deletions after stop, got %d", len(reg.deletedPaths))
	}
	reg.mu.Unlock()
}

func TestCleaner_ConcurrentDeletes(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	// Add many file states
	for i := 0; i < 100; i++ {
		path := "/test/file" + string(rune('0'+i%10)) + string(rune('0'+i/10)) + ".go"
		reg.fileStates[path] = registry.FileState{Path: path}
	}

	c := New(reg, g, bus)

	// Delete files concurrently
	var wg sync.WaitGroup
	var deletedCount atomic.Int32

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			path := "/test/file" + string(rune('0'+idx%10)) + string(rune('0'+idx/10)) + ".go"
			if err := c.DeletePath(context.Background(), path); err == nil {
				deletedCount.Add(1)
			}
		}(i)
	}

	wg.Wait()

	if deletedCount.Load() != 100 {
		t.Errorf("expected 100 successful deletes, got %d", deletedCount.Load())
	}
}

// Edge case tests

func TestCleaner_Start_Idempotent(t *testing.T) {
	reg := newMockRegistry()
	bus := events.NewBus()
	defer bus.Close()

	c := New(reg, nil, bus)

	// First start should succeed
	err := c.Start(context.Background())
	if err != nil {
		t.Fatalf("first start failed: %v", err)
	}
	defer c.Stop()

	// Second start should return ErrAlreadyStarted
	err = c.Start(context.Background())
	if err != ErrAlreadyStarted {
		t.Errorf("expected ErrAlreadyStarted, got %v", err)
	}

	// Verify IsStarted returns true
	if !c.IsStarted() {
		t.Error("expected IsStarted() to return true")
	}
}

func TestCleaner_Reconcile_EmptyDiscoveredPathsNoMassDeletion(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	// Add file states to registry
	reg.fileStates["/test/file1.go"] = registry.FileState{Path: "/test/file1.go"}
	reg.fileStates["/test/file2.go"] = registry.FileState{Path: "/test/file2.go"}
	reg.fileStates["/test/file3.go"] = registry.FileState{Path: "/test/file3.go"}

	c := New(reg, g, bus)

	// Reconcile with empty discovered paths - should skip to prevent mass deletion
	emptyDiscovered := map[string]struct{}{}

	result, err := c.Reconcile(context.Background(), "/test", emptyDiscovered)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify reconciliation was skipped
	if !result.Skipped {
		t.Error("expected Skipped=true when discovered is empty but file_state has entries")
	}
	if result.FilesChecked != 3 {
		t.Errorf("expected FilesChecked=3, got %d", result.FilesChecked)
	}
	if result.StaleFound != 0 {
		t.Errorf("expected StaleFound=0 (skipped), got %d", result.StaleFound)
	}
	if result.StaleRemoved != 0 {
		t.Errorf("expected StaleRemoved=0 (skipped), got %d", result.StaleRemoved)
	}

	// Verify nothing was deleted
	reg.mu.Lock()
	if len(reg.deletedPaths) != 0 {
		t.Errorf("expected no deletions when reconciliation skipped, got %d", len(reg.deletedPaths))
	}
	reg.mu.Unlock()
}

func TestCleaner_Reconcile_RespectsContextCancellation(t *testing.T) {
	reg := newMockRegistry()
	bus := events.NewBus()
	defer bus.Close()

	// Add many file states (more than 100 to trigger context check)
	for i := 0; i < 250; i++ {
		path := "/test/file" + string(rune('a'+i/26)) + string(rune('a'+i%26)) + ".go"
		reg.fileStates[path] = registry.FileState{Path: path}
	}

	c := New(reg, nil, bus)

	// Create already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Reconcile should return context error
	discoveredPaths := map[string]struct{}{
		"/test/different.go": {},
	}

	_, err := c.Reconcile(ctx, "/test", discoveredPaths)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestCleaner_HandleDelete_EmptyPath(t *testing.T) {
	reg := newMockRegistry()
	bus := events.NewBus()
	defer bus.Close()

	c := New(reg, nil, bus)

	err := c.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer c.Stop()

	// Publish event with empty path
	event := events.NewEvent(events.PathDeleted, events.FileEvent{
		Path: "", // Empty path should be ignored
	})
	bus.Publish(context.Background(), event)

	// Wait for event processing
	time.Sleep(50 * time.Millisecond)

	// Verify no deletion occurred (empty path was ignored)
	reg.mu.Lock()
	if len(reg.deletedPaths) != 0 {
		t.Errorf("expected no deletions for empty path, got %d", len(reg.deletedPaths))
	}
	reg.mu.Unlock()
}

// slowMockRegistry delays DeleteFileState for testing graceful shutdown.
type slowMockRegistry struct {
	*mockRegistry
	deleteDelay time.Duration
	deleteCount atomic.Int32
}

func newSlowMockRegistry(delay time.Duration) *slowMockRegistry {
	return &slowMockRegistry{
		mockRegistry: newMockRegistry(),
		deleteDelay:  delay,
	}
}

func (m *slowMockRegistry) DeleteFileState(ctx context.Context, path string) error {
	time.Sleep(m.deleteDelay)
	m.deleteCount.Add(1)
	return m.mockRegistry.DeleteFileState(ctx, path)
}

func TestCleaner_Stop_WaitsForInflightOperations(t *testing.T) {
	// Use a slow registry to simulate in-flight operation
	reg := newSlowMockRegistry(200 * time.Millisecond)
	bus := events.NewBus()
	defer bus.Close()

	reg.fileStates["/test/file.go"] = registry.FileState{Path: "/test/file.go"}

	c := New(reg, nil, bus)

	err := c.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Publish delete event
	event := events.NewEvent(events.PathDeleted, events.FileEvent{
		Path: "/test/file.go",
	})
	bus.Publish(context.Background(), event)

	// Small delay to ensure event handler started
	time.Sleep(50 * time.Millisecond)

	// Stop should wait for in-flight operation
	stopStart := time.Now()
	err = c.Stop()
	stopDuration := time.Since(stopStart)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Stop should have waited at least some time for the in-flight operation
	// (200ms delete delay minus 50ms we already waited)
	if stopDuration < 100*time.Millisecond {
		t.Errorf("expected Stop() to wait for in-flight operation, but returned in %v", stopDuration)
	}

	// Verify operation completed
	if reg.deleteCount.Load() != 1 {
		t.Errorf("expected 1 delete to complete, got %d", reg.deleteCount.Load())
	}
}

func TestCleaner_Stop_CalledTwice(t *testing.T) {
	reg := newMockRegistry()
	bus := events.NewBus()
	defer bus.Close()

	c := New(reg, nil, bus)

	err := c.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First stop should succeed
	err = c.Stop()
	if err != nil {
		t.Fatalf("first stop failed: %v", err)
	}

	// Second stop should also succeed (idempotent)
	err = c.Stop()
	if err != nil {
		t.Fatalf("second stop failed: %v", err)
	}

	// Verify IsStarted returns false
	if c.IsStarted() {
		t.Error("expected IsStarted() to return false after stop")
	}
}

func TestCleaner_Stop_BeforeStart(t *testing.T) {
	reg := newMockRegistry()
	bus := events.NewBus()
	defer bus.Close()

	c := New(reg, nil, bus)

	// Stop without starting should succeed (no-op)
	err := c.Stop()
	if err != nil {
		t.Fatalf("stop before start failed: %v", err)
	}

	// Verify IsStarted returns false
	if c.IsStarted() {
		t.Error("expected IsStarted() to return false")
	}

	// Should still be able to start after
	err = c.Start(context.Background())
	if err != nil {
		t.Fatalf("start after stop failed: %v", err)
	}
	defer c.Stop()

	if !c.IsStarted() {
		t.Error("expected IsStarted() to return true after start")
	}
}

func TestCleaner_Reconcile_NilDiscoveredPaths(t *testing.T) {
	reg := newMockRegistry()
	bus := events.NewBus()
	defer bus.Close()

	// Add file states to registry
	reg.fileStates["/test/file1.go"] = registry.FileState{Path: "/test/file1.go"}
	reg.fileStates["/test/file2.go"] = registry.FileState{Path: "/test/file2.go"}

	c := New(reg, nil, bus)

	// Reconcile with nil discovered paths - should skip (safeguard)
	// nil map has len() == 0, so safeguard should trigger
	result, err := c.Reconcile(context.Background(), "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify reconciliation was skipped
	if !result.Skipped {
		t.Error("expected Skipped=true when discovered is nil but file_state has entries")
	}

	// Verify nothing was deleted
	reg.mu.Lock()
	if len(reg.deletedPaths) != 0 {
		t.Errorf("expected no deletions when reconciliation skipped, got %d", len(reg.deletedPaths))
	}
	reg.mu.Unlock()
}

func TestCleaner_DeletePath_RegistryNotFoundError(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	// Don't add file to registry - DeleteFileState will return ErrPathNotFound
	c := New(reg, g, bus)

	// Should succeed even when file not in registry
	err := c.DeletePath(context.Background(), "/test/nonexistent.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Graph should still be called
	g.mu.Lock()
	if len(g.deletedPaths) != 1 || g.deletedPaths[0] != "/test/nonexistent.go" {
		t.Errorf("expected graph delete to be called, got %v", g.deletedPaths)
	}
	g.mu.Unlock()
}

func TestCleaner_DeletePath_Directory(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	c := New(reg, g, bus)

	// Delete a directory path (simulates directory deletion event)
	err := c.DeletePath(context.Background(), "/test/mydir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all graph cleanup methods were called
	g.mu.Lock()
	defer g.mu.Unlock()

	// DeleteFile should be called (tries both file and directory)
	if len(g.deletedPaths) != 1 || g.deletedPaths[0] != "/test/mydir" {
		t.Errorf("expected DeleteFile called with /test/mydir, got %v", g.deletedPaths)
	}

	// DeleteDirectory should be called
	if len(g.deletedDirectories) != 1 || g.deletedDirectories[0] != "/test/mydir" {
		t.Errorf("expected DeleteDirectory called with /test/mydir, got %v", g.deletedDirectories)
	}

	// DeleteFilesUnderPath should be called for child cleanup
	if len(g.deletedFilesUnderPaths) != 1 || g.deletedFilesUnderPaths[0] != "/test/mydir" {
		t.Errorf("expected DeleteFilesUnderPath called with /test/mydir, got %v", g.deletedFilesUnderPaths)
	}

	// DeleteDirectoriesUnderPath should be called for child cleanup
	if len(g.deletedDirsUnderPaths) != 1 || g.deletedDirsUnderPaths[0] != "/test/mydir" {
		t.Errorf("expected DeleteDirectoriesUnderPath called with /test/mydir, got %v", g.deletedDirsUnderPaths)
	}

	// Verify registry bulk delete was called
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if len(reg.bulkDeletedPaths) != 1 || reg.bulkDeletedPaths[0] != "/test/mydir" {
		t.Errorf("expected DeleteFileStatesForPath called with /test/mydir, got %v", reg.bulkDeletedPaths)
	}
}

func TestCleaner_DeletePath_EmptyDirectory(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	c := New(reg, g, bus)

	// Delete an empty directory (no children in registry or graph)
	err := c.DeletePath(context.Background(), "/test/emptydir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All cleanup methods should still be called (they just won't find anything)
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.deletedDirectories) != 1 || g.deletedDirectories[0] != "/test/emptydir" {
		t.Errorf("expected DeleteDirectory called for empty dir, got %v", g.deletedDirectories)
	}
	if len(g.deletedFilesUnderPaths) != 1 {
		t.Errorf("expected DeleteFilesUnderPath called for empty dir, got %v", g.deletedFilesUnderPaths)
	}
	if len(g.deletedDirsUnderPaths) != 1 {
		t.Errorf("expected DeleteDirectoriesUnderPath called for empty dir, got %v", g.deletedDirsUnderPaths)
	}
}

func TestCleaner_DeletePath_NestedDirectories(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	c := New(reg, g, bus)

	// Delete a parent directory that would have nested children
	// The cleaner uses prefix matching, so deleting /test/parent
	// should trigger cleanup for /test/parent/child1, /test/parent/child2, etc.
	err := c.DeletePath(context.Background(), "/test/parent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// Verify prefix-based cleanup methods were called
	if len(g.deletedFilesUnderPaths) != 1 || g.deletedFilesUnderPaths[0] != "/test/parent" {
		t.Errorf("expected DeleteFilesUnderPath with /test/parent, got %v", g.deletedFilesUnderPaths)
	}
	if len(g.deletedDirsUnderPaths) != 1 || g.deletedDirsUnderPaths[0] != "/test/parent" {
		t.Errorf("expected DeleteDirectoriesUnderPath with /test/parent, got %v", g.deletedDirsUnderPaths)
	}

	// Verify registry bulk delete was called for child file states
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if len(reg.bulkDeletedPaths) != 1 || reg.bulkDeletedPaths[0] != "/test/parent" {
		t.Errorf("expected DeleteFileStatesForPath with /test/parent, got %v", reg.bulkDeletedPaths)
	}
}

func TestCleaner_DeletePath_AllMethodsCalledInOrder(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	// Add a file state to test single file deletion path
	reg.fileStates["/test/path"] = registry.FileState{Path: "/test/path"}

	c := New(reg, g, bus)

	err := c.DeletePath(context.Background(), "/test/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all methods were called (the "try both" approach)
	reg.mu.Lock()
	// Single file delete
	if len(reg.deletedPaths) != 1 {
		t.Errorf("expected 1 single file delete, got %d", len(reg.deletedPaths))
	}
	// Bulk delete for children
	if len(reg.bulkDeletedPaths) != 1 {
		t.Errorf("expected 1 bulk delete, got %d", len(reg.bulkDeletedPaths))
	}
	reg.mu.Unlock()

	g.mu.Lock()
	// DeleteFile
	if len(g.deletedPaths) != 1 {
		t.Errorf("expected 1 DeleteFile call, got %d", len(g.deletedPaths))
	}
	// DeleteDirectory
	if len(g.deletedDirectories) != 1 {
		t.Errorf("expected 1 DeleteDirectory call, got %d", len(g.deletedDirectories))
	}
	// DeleteFilesUnderPath
	if len(g.deletedFilesUnderPaths) != 1 {
		t.Errorf("expected 1 DeleteFilesUnderPath call, got %d", len(g.deletedFilesUnderPaths))
	}
	// DeleteDirectoriesUnderPath
	if len(g.deletedDirsUnderPaths) != 1 {
		t.Errorf("expected 1 DeleteDirectoriesUnderPath call, got %d", len(g.deletedDirsUnderPaths))
	}
	g.mu.Unlock()
}

func TestCleaner_DeletePath_GraphErrorsContinue(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	// Set errors on all graph operations
	g.deleteError = errors.New("delete file error")
	g.deleteDirectoryError = errors.New("delete directory error")
	g.deleteFilesUnderError = errors.New("delete files under error")
	g.deleteDirsUnderError = errors.New("delete dirs under error")

	c := New(reg, g, bus)

	// Should not return error even if all graph ops fail
	err := c.DeletePath(context.Background(), "/test/path")
	if err != nil {
		t.Fatalf("expected no error with graph failures, got: %v", err)
	}

	// Verify registry operations still occurred
	reg.mu.Lock()
	if len(reg.bulkDeletedPaths) != 1 {
		t.Errorf("expected registry bulk delete to be called despite graph errors")
	}
	reg.mu.Unlock()
}

// Edge case tests

func TestCleaner_DeletePath_TrailingSlash(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	c := New(reg, g, bus)

	// Path with trailing slash should work (filesystem events may include trailing slashes)
	err := c.DeletePath(context.Background(), "/test/dir/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify cleanup methods were called with the path as provided
	// (normalization, if needed, should happen at the caller level or in graph/registry)
	g.mu.Lock()
	if len(g.deletedPaths) != 1 {
		t.Errorf("expected DeleteFile to be called, got %d calls", len(g.deletedPaths))
	}
	if len(g.deletedDirectories) != 1 {
		t.Errorf("expected DeleteDirectory to be called, got %d calls", len(g.deletedDirectories))
	}
	if len(g.deletedFilesUnderPaths) != 1 {
		t.Errorf("expected DeleteFilesUnderPath to be called, got %d calls", len(g.deletedFilesUnderPaths))
	}
	g.mu.Unlock()
}

func TestCleaner_DeletePath_SpecialCharacters(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	c := New(reg, g, bus)

	testCases := []struct {
		name string
		path string
	}{
		{"single_quote", "/test/it's a file.txt"},
		{"backslash", "/test/path\\with\\backslashes"},
		{"unicode", "/test/文件/données.txt"},
		{"spaces", "/test/path with spaces/file.txt"},
		{"brackets", "/test/[special]/file(1).txt"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset mock state
			g.mu.Lock()
			g.deletedPaths = nil
			g.deletedDirectories = nil
			g.deletedFilesUnderPaths = nil
			g.deletedDirsUnderPaths = nil
			g.mu.Unlock()

			err := c.DeletePath(context.Background(), tc.path)
			if err != nil {
				t.Fatalf("unexpected error for path %q: %v", tc.path, err)
			}

			// Verify methods were called
			g.mu.Lock()
			if len(g.deletedPaths) != 1 {
				t.Errorf("expected DeleteFile to be called for %q", tc.path)
			}
			g.mu.Unlock()
		})
	}
}

func TestCleaner_DeletePath_EmptyPathDirect(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	c := New(reg, g, bus)

	// Empty path passed directly to DeletePath (not via event handler)
	// The cleaner currently doesn't validate this in DeletePath itself,
	// only in handlePathDeleted. Operations will be called with empty string.
	err := c.DeletePath(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Methods are called even with empty path (validation is at event handler level)
	g.mu.Lock()
	if len(g.deletedPaths) != 1 || g.deletedPaths[0] != "" {
		t.Errorf("expected DeleteFile to be called with empty path, got %v", g.deletedPaths)
	}
	g.mu.Unlock()
}

func TestCleaner_DeletePath_RootPath(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	c := New(reg, g, bus)

	// Root path "/" - this is a dangerous edge case as it could delete everything
	// The cleaner doesn't prevent this; it's the caller's responsibility
	err := c.DeletePath(context.Background(), "/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All cleanup methods should be called
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.deletedDirectories) != 1 || g.deletedDirectories[0] != "/" {
		t.Errorf("expected DeleteDirectory to be called with /, got %v", g.deletedDirectories)
	}

	// Note: DeleteFilesUnderPath("/") would match all files starting with "/"
	// which is every absolute path. This is dangerous but documented behavior.
	if len(g.deletedFilesUnderPaths) != 1 || g.deletedFilesUnderPaths[0] != "/" {
		t.Errorf("expected DeleteFilesUnderPath to be called with /, got %v", g.deletedFilesUnderPaths)
	}
}

func TestCleaner_DeletePath_ConcurrentSamePath(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	c := New(reg, g, bus)

	// Delete the same path concurrently - should be idempotent
	const goroutines = 10
	const path = "/test/concurrent/file.go"

	var wg sync.WaitGroup
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.DeletePath(context.Background(), path); err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	// No errors should occur
	for err := range errs {
		t.Errorf("unexpected error: %v", err)
	}

	// Methods should be called multiple times (once per goroutine)
	g.mu.Lock()
	if len(g.deletedPaths) != goroutines {
		t.Errorf("expected %d DeleteFile calls, got %d", goroutines, len(g.deletedPaths))
	}
	g.mu.Unlock()
}

func TestCleaner_DeletePath_RegistryBulkDeleteError(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	// Set bulk delete to fail
	reg.bulkDeleteError = errors.New("bulk delete failed")

	c := New(reg, g, bus)

	// Should not return error even if registry bulk delete fails
	err := c.DeletePath(context.Background(), "/test/path")
	if err != nil {
		t.Fatalf("expected no error with registry bulk delete failure, got: %v", err)
	}

	// Graph operations should still proceed
	g.mu.Lock()
	if len(g.deletedPaths) != 1 {
		t.Errorf("expected graph operations to continue despite registry error")
	}
	g.mu.Unlock()
}

func TestCleaner_DeletePath_PathPrefixNoFalsePositives(t *testing.T) {
	// This test documents that prefix matching uses trailing slash
	// to avoid false positives like /foo matching /foobar
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	c := New(reg, g, bus)

	// Delete /test/foo - this should NOT affect /test/foobar
	err := c.DeletePath(context.Background(), "/test/foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The prefix matching in graph methods appends "/" to the path
	// So DeleteFilesUnderPath("/test/foo") uses "STARTS WITH '/test/foo/'"
	// which correctly won't match "/test/foobar/file.txt"
	g.mu.Lock()
	if len(g.deletedFilesUnderPaths) != 1 || g.deletedFilesUnderPaths[0] != "/test/foo" {
		t.Errorf("expected DeleteFilesUnderPath to be called with /test/foo, got %v", g.deletedFilesUnderPaths)
	}
	g.mu.Unlock()

	// Note: The actual prefix matching behavior is tested in graph tests.
	// This test just verifies the path is passed through correctly.
}
