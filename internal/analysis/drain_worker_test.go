package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/storage"
)

// drainMockQueue implements storage.DurablePersistenceQueue for testing.
type drainMockQueue struct {
	mu             sync.Mutex
	items          map[int64]*storage.QueuedResult
	nextID         int64
	dequeueErr     error
	completeCalled []int64
	failCalled     []int64
	purgeCalled    int
}

func newDrainMockQueue() *drainMockQueue {
	return &drainMockQueue{
		items: make(map[int64]*storage.QueuedResult),
	}
}

func (q *drainMockQueue) Enqueue(ctx context.Context, filePath, contentHash string, resultJSON []byte) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.nextID++
	q.items[q.nextID] = &storage.QueuedResult{
		ID:          q.nextID,
		FilePath:    filePath,
		ContentHash: contentHash,
		ResultJSON:  resultJSON,
		Status:      storage.QueueStatusPending,
		EnqueuedAt:  time.Now(),
	}
	return nil
}

func (q *drainMockQueue) DequeueBatch(ctx context.Context, n int) ([]*storage.QueuedResult, error) {
	if q.dequeueErr != nil {
		return nil, q.dequeueErr
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	var result []*storage.QueuedResult
	for id, item := range q.items {
		if item.Status == storage.QueueStatusPending && len(result) < n {
			item.Status = storage.QueueStatusInflight
			now := time.Now()
			item.StartedAt = &now
			result = append(result, item)
			q.items[id] = item
		}
	}
	return result, nil
}

func (q *drainMockQueue) Complete(ctx context.Context, id int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if item, ok := q.items[id]; ok {
		item.Status = storage.QueueStatusCompleted
		now := time.Now()
		item.CompletedAt = &now
		q.completeCalled = append(q.completeCalled, id)
	}
	return nil
}

func (q *drainMockQueue) Fail(ctx context.Context, id int64, maxRetries int, errMsg string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if item, ok := q.items[id]; ok {
		item.RetryCount++
		item.LastError = errMsg
		if item.RetryCount >= maxRetries {
			item.Status = storage.QueueStatusFailed
		} else {
			item.Status = storage.QueueStatusPending
			item.StartedAt = nil
		}
		q.failCalled = append(q.failCalled, id)
	}
	return nil
}

func (q *drainMockQueue) Stats(ctx context.Context) (*storage.QueueStats, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	stats := &storage.QueueStats{}
	for _, item := range q.items {
		switch item.Status {
		case storage.QueueStatusPending:
			stats.Pending++
		case storage.QueueStatusInflight:
			stats.Inflight++
		case storage.QueueStatusCompleted:
			stats.Completed++
		case storage.QueueStatusFailed:
			stats.Failed++
		}
	}
	return stats, nil
}

func (q *drainMockQueue) Purge(ctx context.Context, completedOlderThan, failedOlderThan time.Duration) (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.purgeCalled++
	var purged int64
	for id, item := range q.items {
		if item.Status == storage.QueueStatusCompleted || item.Status == storage.QueueStatusFailed {
			delete(q.items, id)
			purged++
		}
	}
	return purged, nil
}

// slowDrainMockQueue wraps drainMockQueue with configurable delays.
type slowDrainMockQueue struct {
	*drainMockQueue
	dequeueDelay time.Duration
}

func newSlowDrainMockQueue(dequeueDelay time.Duration) *slowDrainMockQueue {
	return &slowDrainMockQueue{
		drainMockQueue: newDrainMockQueue(),
		dequeueDelay:   dequeueDelay,
	}
}

func (q *slowDrainMockQueue) DequeueBatch(ctx context.Context, n int) ([]*storage.QueuedResult, error) {
	time.Sleep(q.dequeueDelay)
	return q.drainMockQueue.DequeueBatch(ctx, n)
}

// drainMockGraph implements graph.Graph for testing.
type drainMockGraph struct {
	connected     atomic.Bool
	persistErr    error
	persistCalled atomic.Int32
}

func newDrainMockGraph(connected bool) *drainMockGraph {
	g := &drainMockGraph{}
	g.connected.Store(connected)
	return g
}

func (g *drainMockGraph) Name() string                    { return "drain_mock_graph" }
func (g *drainMockGraph) Start(ctx context.Context) error { return nil }
func (g *drainMockGraph) Stop(ctx context.Context) error  { return nil }
func (g *drainMockGraph) IsConnected() bool               { return g.connected.Load() }
func (g *drainMockGraph) Errors() <-chan error            { return nil }
func (g *drainMockGraph) setConnected(connected bool)     { g.connected.Store(connected) }

func (g *drainMockGraph) UpsertFile(ctx context.Context, file *graph.FileNode) error {
	g.persistCalled.Add(1)
	return g.persistErr
}
func (g *drainMockGraph) DeleteFile(ctx context.Context, path string) error { return nil }
func (g *drainMockGraph) GetFile(ctx context.Context, path string) (*graph.FileNode, error) {
	return nil, nil
}
func (g *drainMockGraph) UpsertDirectory(ctx context.Context, dir *graph.DirectoryNode) error {
	return nil
}
func (g *drainMockGraph) DeleteDirectory(ctx context.Context, path string) error { return nil }
func (g *drainMockGraph) DeleteFilesUnderPath(ctx context.Context, parentPath string) error {
	return nil
}
func (g *drainMockGraph) DeleteDirectoriesUnderPath(ctx context.Context, path string) error {
	return nil
}
func (g *drainMockGraph) UpsertChunkWithMetadata(ctx context.Context, chunk *graph.ChunkNode, meta *chunkers.ChunkMetadata) error {
	return nil
}
func (g *drainMockGraph) UpsertChunkEmbedding(ctx context.Context, chunkID string, emb *graph.ChunkEmbeddingNode) error {
	return nil
}
func (g *drainMockGraph) DeleteChunkEmbeddings(ctx context.Context, chunkID, provider, model string) error {
	return nil
}
func (g *drainMockGraph) DeleteChunks(ctx context.Context, filePath string) error { return nil }
func (g *drainMockGraph) SetFileTags(ctx context.Context, path string, tags []string) error {
	return nil
}
func (g *drainMockGraph) SetFileTopics(ctx context.Context, path string, topics []graph.Topic) error {
	return nil
}
func (g *drainMockGraph) SetFileEntities(ctx context.Context, path string, entities []graph.Entity) error {
	return nil
}
func (g *drainMockGraph) SetFileReferences(ctx context.Context, path string, refs []graph.Reference) error {
	return nil
}
func (g *drainMockGraph) Query(ctx context.Context, cypher string) (*graph.QueryResult, error) {
	return nil, nil
}
func (g *drainMockGraph) HasEmbedding(ctx context.Context, contentHash string, version int) (bool, error) {
	return false, nil
}
func (g *drainMockGraph) ExportSnapshot(ctx context.Context) (*graph.GraphSnapshot, error) {
	return nil, nil
}
func (g *drainMockGraph) GetFileWithRelations(ctx context.Context, path string) (*graph.FileWithRelations, error) {
	return nil, nil
}
func (g *drainMockGraph) SearchSimilarChunks(ctx context.Context, embedding []float32, k int) ([]graph.ChunkNode, error) {
	return nil, nil
}

// drainMockBus implements events.Bus for testing.
type drainMockBus struct {
	mu        sync.Mutex
	handlers  map[events.EventType][]events.EventHandler
	published []events.Event
}

func newDrainMockBus() *drainMockBus {
	return &drainMockBus{
		handlers: make(map[events.EventType][]events.EventHandler),
	}
}

func (b *drainMockBus) Publish(ctx context.Context, event events.Event) error {
	b.mu.Lock()
	b.published = append(b.published, event)
	handlers := b.handlers[event.Type]
	b.mu.Unlock()

	for _, h := range handlers {
		h(event)
	}
	return nil
}

func (b *drainMockBus) Subscribe(eventType events.EventType, handler events.EventHandler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[eventType] = append(b.handlers[eventType], handler)
	return func() {
		// Simplified unsubscribe - not needed for most tests
	}
}

func (b *drainMockBus) SubscribeAll(handler events.EventHandler) func() {
	return func() {}
}

func (b *drainMockBus) Close() error {
	return nil
}

// testResultJSON creates test analysis result JSON
func testDrainResultJSON(t *testing.T, filePath string) []byte {
	t.Helper()
	result := AnalysisResult{
		FilePath:    filePath,
		ContentHash: "testhash123",
		Summary:     "Test summary",
		AnalyzedAt:  time.Now(),
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal test result: %v", err)
	}
	return data
}

func TestDrainWorker_NewDrainWorker(t *testing.T) {
	queue := newDrainMockQueue()
	graphClient := newDrainMockGraph(true)
	bus := newDrainMockBus()

	worker := NewDrainWorker(queue, graphClient, bus)

	if worker.queue != queue {
		t.Error("expected queue to be set")
	}
	if worker.graph != graphClient {
		t.Error("expected graph to be set")
	}
	if worker.bus != bus {
		t.Error("expected bus to be set")
	}
	if worker.config.BatchSize != DefaultDrainConfig().BatchSize {
		t.Errorf("expected default batch size %d, got %d",
			DefaultDrainConfig().BatchSize, worker.config.BatchSize)
	}
}

func TestDrainWorker_WithOptions(t *testing.T) {
	queue := newDrainMockQueue()
	graphClient := newDrainMockGraph(true)
	bus := newDrainMockBus()

	customConfig := DrainConfig{
		BatchSize:          5,
		MaxRetries:         5,
		RetryBackoff:       2 * time.Second,
		CompletedRetention: 2 * time.Hour,
		FailedRetention:    24 * time.Hour,
	}

	worker := NewDrainWorker(queue, graphClient, bus,
		WithDrainConfig(customConfig),
	)

	if worker.config.BatchSize != 5 {
		t.Errorf("expected batch size 5, got %d", worker.config.BatchSize)
	}
	if worker.config.MaxRetries != 5 {
		t.Errorf("expected max retries 5, got %d", worker.config.MaxRetries)
	}
}

func TestDrainWorker_StartWithConnectedGraph(t *testing.T) {
	queue := newDrainMockQueue()
	graphClient := newDrainMockGraph(true) // Already connected
	bus := newDrainMockBus()

	// Add item to queue
	queue.Enqueue(context.Background(), "/test/file.go", "hash1", testDrainResultJSON(t, "/test/file.go"))

	worker := NewDrainWorker(queue, graphClient, bus)
	ctx := context.Background()

	err := worker.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Wait for drain to complete
	time.Sleep(100 * time.Millisecond)

	// Stop the worker
	stopCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	worker.Stop(stopCtx)

	// Verify item was processed
	if len(queue.completeCalled) != 1 {
		t.Errorf("expected 1 complete call, got %d", len(queue.completeCalled))
	}
}

func TestDrainWorker_StartWithDisconnectedGraph(t *testing.T) {
	queue := newDrainMockQueue()
	graphClient := newDrainMockGraph(false) // Not connected
	bus := newDrainMockBus()

	// Add item to queue
	queue.Enqueue(context.Background(), "/test/file.go", "hash1", testDrainResultJSON(t, "/test/file.go"))

	worker := NewDrainWorker(queue, graphClient, bus)
	ctx := context.Background()

	err := worker.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Wait briefly
	time.Sleep(50 * time.Millisecond)

	// Stop the worker
	stopCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	worker.Stop(stopCtx)

	// Verify item was NOT processed (graph not connected)
	if len(queue.completeCalled) != 0 {
		t.Errorf("expected 0 complete calls, got %d", len(queue.completeCalled))
	}
}

func TestDrainWorker_DrainOnGraphConnectedEvent(t *testing.T) {
	queue := newDrainMockQueue()
	graphClient := newDrainMockGraph(false) // Start disconnected
	bus := newDrainMockBus()

	// Add item to queue
	queue.Enqueue(context.Background(), "/test/file.go", "hash1", testDrainResultJSON(t, "/test/file.go"))

	worker := NewDrainWorker(queue, graphClient, bus)
	ctx := context.Background()

	err := worker.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Verify no processing yet
	time.Sleep(50 * time.Millisecond)
	if len(queue.completeCalled) != 0 {
		t.Errorf("expected 0 complete calls before connection, got %d", len(queue.completeCalled))
	}

	// Simulate graph connection
	graphClient.setConnected(true)
	bus.Publish(ctx, events.NewGraphConnected("localhost:6379"))

	// Wait for drain to complete
	time.Sleep(100 * time.Millisecond)

	// Stop the worker
	stopCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	worker.Stop(stopCtx)

	// Verify item was processed
	if len(queue.completeCalled) != 1 {
		t.Errorf("expected 1 complete call, got %d", len(queue.completeCalled))
	}
}

func TestDrainWorker_AtomicGuardPreventsConcurrentDrains(t *testing.T) {
	queue := newDrainMockQueue()
	graphClient := newDrainMockGraph(true)
	bus := newDrainMockBus()

	// Add multiple items
	for i := range 10 {
		filePath := "/test/file" + string(rune('A'+i)) + ".go"
		queue.Enqueue(context.Background(), filePath, "hash", testDrainResultJSON(t, filePath))
	}

	worker := NewDrainWorker(queue, graphClient, bus)
	ctx := context.Background()

	// Trigger multiple concurrent drains
	for range 5 {
		worker.TriggerDrain(ctx)
	}

	// Wait for drains
	time.Sleep(200 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	worker.Stop(stopCtx)

	// Verify items were processed exactly once
	if len(queue.completeCalled) != 10 {
		t.Errorf("expected 10 complete calls, got %d", len(queue.completeCalled))
	}
}

func TestDrainWorker_StopDuringDrain(t *testing.T) {
	queue := newSlowDrainMockQueue(5 * time.Millisecond) // Add delay to each dequeue
	graphClient := newDrainMockGraph(true)
	bus := newDrainMockBus()

	// Add many items
	for i := range 100 {
		filePath := "/test/file" + string(rune(i)) + ".go"
		queue.Enqueue(context.Background(), filePath, "hash", testDrainResultJSON(t, filePath))
	}

	worker := NewDrainWorker(queue, graphClient, bus,
		WithDrainConfig(DrainConfig{
			BatchSize:          1, // Process one at a time to extend drain
			MaxRetries:         3,
			CompletedRetention: time.Hour,
			FailedRetention:    time.Hour,
		}),
	)
	ctx := context.Background()

	err := worker.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Let drain start
	time.Sleep(20 * time.Millisecond)

	// Stop while drain is in progress
	stopCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	worker.Stop(stopCtx)

	// Verify worker stopped (should have processed some but not all)
	processed := len(queue.completeCalled)
	if processed == 0 {
		t.Error("expected some items to be processed before stop")
	}
	if processed == 100 {
		t.Error("expected drain to be interrupted before all items processed")
	}
}

func TestDrainWorker_PurgeAfterDrain(t *testing.T) {
	queue := newDrainMockQueue()
	graphClient := newDrainMockGraph(true)
	bus := newDrainMockBus()

	// Add item to queue
	queue.Enqueue(context.Background(), "/test/file.go", "hash1", testDrainResultJSON(t, "/test/file.go"))

	worker := NewDrainWorker(queue, graphClient, bus)
	ctx := context.Background()

	err := worker.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Wait for drain to complete
	time.Sleep(100 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	worker.Stop(stopCtx)

	// Verify purge was called
	if queue.purgeCalled == 0 {
		t.Error("expected purge to be called after drain")
	}
}

func TestDrainWorker_FailedItemsReturnToQueue(t *testing.T) {
	queue := newDrainMockQueue()
	graphClient := newDrainMockGraph(true)
	bus := newDrainMockBus()

	// Add invalid JSON to cause unmarshal failure
	queue.Enqueue(context.Background(), "/test/bad.go", "hash1", []byte("invalid json"))

	worker := NewDrainWorker(queue, graphClient, bus,
		WithDrainConfig(DrainConfig{
			BatchSize:          10,
			MaxRetries:         3,
			CompletedRetention: time.Hour,
			FailedRetention:    time.Hour,
		}),
	)
	ctx := context.Background()

	err := worker.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Wait for drain to complete
	time.Sleep(100 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	worker.Stop(stopCtx)

	// Verify fail was called (item should be returned for retry)
	if len(queue.failCalled) == 0 {
		t.Error("expected fail to be called for invalid item")
	}
}

func TestDrainWorker_IsDraining(t *testing.T) {
	queue := newSlowDrainMockQueue(10 * time.Millisecond)
	graphClient := newDrainMockGraph(true)
	bus := newDrainMockBus()

	// Add items
	for i := range 50 {
		filePath := "/test/file" + string(rune(i)) + ".go"
		queue.Enqueue(context.Background(), filePath, "hash", testDrainResultJSON(t, filePath))
	}

	worker := NewDrainWorker(queue, graphClient, bus,
		WithDrainConfig(DrainConfig{
			BatchSize: 1, // Process slowly
		}),
	)

	if worker.IsDraining() {
		t.Error("expected IsDraining=false before start")
	}

	ctx := context.Background()
	worker.Start(ctx)

	// Wait briefly and check draining status
	time.Sleep(20 * time.Millisecond)

	// IsDraining should be true during drain
	if !worker.IsDraining() {
		t.Error("expected IsDraining=true during drain")
	}

	stopCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	worker.Stop(stopCtx)

	// After stop, should not be draining
	if worker.IsDraining() {
		t.Error("expected IsDraining=false after stop")
	}
}

func TestDrainWorker_GraphDisconnectsDuringDrain(t *testing.T) {
	queue := newSlowDrainMockQueue(10 * time.Millisecond)
	graphClient := newDrainMockGraph(true)
	bus := newDrainMockBus()

	// Add many items
	for i := range 50 {
		filePath := "/test/file" + string(rune(i)) + ".go"
		queue.Enqueue(context.Background(), filePath, "hash", testDrainResultJSON(t, filePath))
	}

	worker := NewDrainWorker(queue, graphClient, bus,
		WithDrainConfig(DrainConfig{
			BatchSize: 1, // Process one at a time
		}),
	)
	ctx := context.Background()

	worker.Start(ctx)

	// Let a few items process
	time.Sleep(50 * time.Millisecond)

	// Disconnect graph mid-drain
	graphClient.setConnected(false)

	// Wait for drain to notice disconnection
	time.Sleep(50 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	worker.Stop(stopCtx)

	// Should have processed some but not all items
	processed := len(queue.completeCalled)
	if processed == 0 {
		t.Error("expected some items to be processed before disconnect")
	}
	if processed == 50 {
		t.Error("expected drain to stop when graph disconnected")
	}
}

func TestDrainWorker_EmptyQueue(t *testing.T) {
	queue := newDrainMockQueue()
	graphClient := newDrainMockGraph(true)
	bus := newDrainMockBus()

	worker := NewDrainWorker(queue, graphClient, bus)
	ctx := context.Background()

	err := worker.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Wait for drain to complete
	time.Sleep(50 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	worker.Stop(stopCtx)

	// No errors should occur with empty queue
	if len(queue.completeCalled) != 0 {
		t.Errorf("expected 0 complete calls for empty queue, got %d", len(queue.completeCalled))
	}

	// Purge should still be called
	if queue.purgeCalled == 0 {
		t.Error("expected purge to be called even for empty queue")
	}
}

func TestDrainWorker_DequeueError(t *testing.T) {
	queue := newDrainMockQueue()
	queue.dequeueErr = errors.New("database error")
	graphClient := newDrainMockGraph(true)
	bus := newDrainMockBus()

	queue.Enqueue(context.Background(), "/test/file.go", "hash", testDrainResultJSON(t, "/test/file.go"))

	worker := NewDrainWorker(queue, graphClient, bus)
	ctx := context.Background()

	worker.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	worker.Stop(stopCtx)

	// Item should not be completed due to dequeue error
	if len(queue.completeCalled) != 0 {
		t.Errorf("expected 0 complete calls, got %d", len(queue.completeCalled))
	}
}

func TestDefaultDrainConfig(t *testing.T) {
	cfg := DefaultDrainConfig()

	if cfg.BatchSize <= 0 {
		t.Errorf("expected positive batch size, got %d", cfg.BatchSize)
	}
	if cfg.MaxRetries <= 0 {
		t.Errorf("expected positive max retries, got %d", cfg.MaxRetries)
	}
	if cfg.RetryBackoff <= 0 {
		t.Errorf("expected positive retry backoff, got %v", cfg.RetryBackoff)
	}
	if cfg.CompletedRetention <= 0 {
		t.Errorf("expected positive completed retention, got %v", cfg.CompletedRetention)
	}
	if cfg.FailedRetention <= 0 {
		t.Errorf("expected positive failed retention, got %v", cfg.FailedRetention)
	}
}

func TestDrainWorker_Name(t *testing.T) {
	queue := newDrainMockQueue()
	graphClient := newDrainMockGraph(true)
	bus := newDrainMockBus()

	worker := NewDrainWorker(queue, graphClient, bus)

	if worker.Name() != "drain_worker" {
		t.Errorf("expected name 'drain_worker', got %q", worker.Name())
	}
}
