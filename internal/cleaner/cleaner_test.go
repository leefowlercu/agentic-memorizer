package cleaner

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

// mockRegistry implements registry.Registry for testing.
type mockRegistry struct {
	mu               sync.Mutex
	fileStates       map[string]registry.FileState
	deletedPaths     []string
	deleteError      error
	listStatesError  error
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

// mockGraph implements graph.Graph for testing.
type mockGraph struct {
	mu           sync.Mutex
	deletedPaths []string
	deleteError  error
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
	return nil
}

func (m *mockGraph) UpsertChunk(ctx context.Context, chunk *graph.ChunkNode) error {
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

func TestCleaner_DeleteFile(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	// Add a file state
	reg.fileStates["/test/file.go"] = registry.FileState{Path: "/test/file.go"}

	c := New(reg, g, bus)

	err := c.DeleteFile(context.Background(), "/test/file.go")
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

func TestCleaner_DeleteFile_GraphNil(t *testing.T) {
	reg := newMockRegistry()
	bus := events.NewBus()
	defer bus.Close()

	reg.fileStates["/test/file.go"] = registry.FileState{Path: "/test/file.go"}

	// Create cleaner with nil graph
	c := New(reg, nil, bus)

	err := c.DeleteFile(context.Background(), "/test/file.go")
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

func TestCleaner_DeleteFile_GraphError(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	reg.fileStates["/test/file.go"] = registry.FileState{Path: "/test/file.go"}
	g.deleteError = errors.New("graph connection failed")

	c := New(reg, g, bus)

	// Should not return error even if graph fails
	err := c.DeleteFile(context.Background(), "/test/file.go")
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

	// Publish a FileDeleted event
	event := events.NewEvent(events.FileDeleted, events.FileEvent{
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
		t.Error("expected FileDeleted event to trigger deletion")
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
		Type:      events.FileDeleted,
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
	event := events.NewEvent(events.FileDeleted, events.FileEvent{
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
			if err := c.DeleteFile(context.Background(), path); err == nil {
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
	event := events.NewEvent(events.FileDeleted, events.FileEvent{
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
	event := events.NewEvent(events.FileDeleted, events.FileEvent{
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

func TestCleaner_DeleteFile_RegistryNotFoundError(t *testing.T) {
	reg := newMockRegistry()
	g := newMockGraph()
	bus := events.NewBus()
	defer bus.Close()

	// Don't add file to registry - DeleteFileState will return ErrPathNotFound
	c := New(reg, g, bus)

	// Should succeed even when file not in registry
	err := c.DeleteFile(context.Background(), "/test/nonexistent.go")
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
