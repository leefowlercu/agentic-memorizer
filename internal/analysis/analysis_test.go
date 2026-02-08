package analysis

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/fsutil"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/ingest"
	"github.com/leefowlercu/agentic-memorizer/internal/providers"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

func TestQueueStats(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	queue := NewQueue(bus)

	stats := queue.Stats()
	if stats.State != QueueStateIdle {
		t.Errorf("State = %v, want QueueStateIdle", stats.State)
	}
	if stats.WorkerCount != 4 {
		t.Errorf("WorkerCount = %d, want 4", stats.WorkerCount)
	}
}

func TestQueueOptions(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	queue := NewQueue(bus,
		WithWorkerCount(8),
		WithBatchSize(20),
		WithMaxRetries(5),
		WithQueueCapacity(2000),
	)

	if queue.workerCount != 8 {
		t.Errorf("workerCount = %d, want 8", queue.workerCount)
	}
	if queue.batchSize != 20 {
		t.Errorf("batchSize = %d, want 20", queue.batchSize)
	}
	if queue.maxRetries != 5 {
		t.Errorf("maxRetries = %d, want 5", queue.maxRetries)
	}
	if queue.queueCapacity != 2000 {
		t.Errorf("queueCapacity = %d, want 2000", queue.queueCapacity)
	}
}

func TestQueueStartStop(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	queue := NewQueue(bus, WithWorkerCount(2))

	// Start
	err := queue.Start(context.Background())
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	stats := queue.Stats()
	if stats.State != QueueStateRunning {
		t.Errorf("State = %v, want QueueStateRunning", stats.State)
	}

	// Double start should fail
	err = queue.Start(context.Background())
	if err == nil {
		t.Error("Expected error on double start")
	}

	// Stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = queue.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	stats = queue.Stats()
	if stats.State != QueueStateStopped {
		t.Errorf("State = %v, want QueueStateStopped", stats.State)
	}
}

func TestQueueStopUnsubscribes(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	queue := NewQueue(bus, WithWorkerCount(1))

	if err := queue.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	t.Cleanup(func() {
		_ = queue.Stop(context.Background())
	})

	stats := bus.Stats()
	if stats.SubscriberCount != 2 {
		t.Fatalf("SubscriberCount = %d, want 2", stats.SubscriberCount)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := queue.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	stats = bus.Stats()
	if stats.SubscriberCount != 0 {
		t.Errorf("SubscriberCount = %d, want 0", stats.SubscriberCount)
	}
}

func TestQueueEnqueue(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	queue := NewQueue(bus, WithWorkerCount(1), WithQueueCapacity(10))

	// Enqueue before start should fail
	err := queue.Enqueue(WorkItem{FilePath: "/test/file.txt"})
	if err == nil {
		t.Error("Expected error when enqueueing to stopped queue")
	}

	// Start queue
	err = queue.Start(context.Background())
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer queue.Stop(context.Background())

	// Enqueue should succeed
	err = queue.Enqueue(WorkItem{FilePath: "/test/file.txt"})
	if err != nil {
		t.Errorf("Enqueue failed: %v", err)
	}

	// Give workers time to process
	time.Sleep(100 * time.Millisecond)

	stats := queue.Stats()
	if stats.PendingItems < 0 {
		t.Errorf("PendingItems should not be negative")
	}
}

func TestQueueRegistryUpdatesFileState(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	ctx := context.Background()
	reg, err := registry.Open(ctx, filepath.Join(t.TempDir(), "registry.db"))
	if err != nil {
		t.Fatalf("failed to open registry: %v", err)
	}
	defer reg.Close()

	queue := NewQueue(bus, WithWorkerCount(1))
	queue.SetRegistry(reg)

	if err := queue.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer queue.Stop(context.Background())

	mockSemantic := &mockSemanticProvider{available: true}
	mockEmbed := &mockEmbeddingsProvider{
		available: true,
		embedding: []float32{0.1, 0.2},
	}
	queue.SetProviders(mockSemantic, mockEmbed)

	dir := t.TempDir()
	filePath := filepath.Join(dir, "sample.txt")
	content := []byte("hello registry")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("failed to stat test file: %v", err)
	}

	done := make(chan struct{})
	var once sync.Once
	unsub := bus.Subscribe(events.AnalysisComplete, func(e events.Event) {
		ae, ok := e.Payload.(*events.AnalysisEvent)
		if !ok || ae.Path != filePath {
			return
		}
		once.Do(func() {
			close(done)
		})
	})
	defer unsub()

	failed := make(chan string, 1)
	unsubFailed := bus.Subscribe(events.AnalysisFailed, func(e events.Event) {
		ae, ok := e.Payload.(*events.AnalysisEvent)
		if !ok || ae.Path != filePath {
			return
		}
		failed <- ae.Error
	})
	defer unsubFailed()

	err = queue.Enqueue(WorkItem{
		FilePath:  filePath,
		FileSize:  info.Size(),
		ModTime:   info.ModTime(),
		EventType: WorkItemNew,
	})
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	select {
	case <-done:
	case errMsg := <-failed:
		t.Fatalf("analysis failed: %s", errMsg)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for analysis to complete")
	}

	state, err := reg.GetFileState(ctx, filePath)
	if err != nil {
		t.Fatalf("failed to read file state: %v", err)
	}

	expectedContentHash := fsutil.HashBytes(content)
	if state.ContentHash != expectedContentHash {
		t.Errorf("ContentHash = %q, want %q", state.ContentHash, expectedContentHash)
	}

	expectedMetadataHash := computeMetadataHash(filePath, info.Size(), info.ModTime())
	if state.MetadataHash != expectedMetadataHash {
		t.Errorf("MetadataHash = %q, want %q", state.MetadataHash, expectedMetadataHash)
	}

	if state.MetadataAnalyzedAt == nil {
		t.Error("expected MetadataAnalyzedAt to be set")
	}
	if state.SemanticAnalyzedAt == nil {
		t.Error("expected SemanticAnalyzedAt to be set")
	}
	if state.EmbeddingsAnalyzedAt == nil {
		t.Error("expected EmbeddingsAnalyzedAt to be set")
	}
	if state.SemanticError != nil {
		t.Errorf("expected SemanticError to be nil, got %q", *state.SemanticError)
	}
	if state.EmbeddingsError != nil {
		t.Errorf("expected EmbeddingsError to be nil, got %q", *state.EmbeddingsError)
	}
	if state.AnalysisVersion == "" {
		t.Error("expected AnalysisVersion to be set")
	}
}

func TestWorkerAnalyze_DegradationMetadataSkipsSemanticAndEmbeddings(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	ctx := context.Background()
	reg, err := registry.Open(ctx, filepath.Join(t.TempDir(), "registry.db"))
	if err != nil {
		t.Fatalf("failed to open registry: %v", err)
	}
	defer reg.Close()

	queue := NewQueue(bus)
	queue.queueCapacity = 1
	queue.workChan = make(chan WorkItem, 1)
	queue.workChan <- WorkItem{}

	worker := NewWorker(0, queue)
	worker.SetRegistry(reg)

	dir := t.TempDir()
	filePath := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(filePath, []byte("metadata only"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("failed to stat test file: %v", err)
	}

	_, err = worker.analyze(ctx, WorkItem{
		FilePath:  filePath,
		FileSize:  info.Size(),
		ModTime:   info.ModTime(),
		EventType: WorkItemNew,
	})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}

	state, err := reg.GetFileState(ctx, filePath)
	if err != nil {
		t.Fatalf("failed to get file state: %v", err)
	}
	if state.MetadataAnalyzedAt == nil {
		t.Fatal("expected metadata_analyzed_at to be set")
	}
	if state.SemanticAnalyzedAt != nil {
		t.Fatal("expected semantic_analyzed_at to be nil in degraded metadata mode")
	}
	if state.EmbeddingsAnalyzedAt != nil {
		t.Fatal("expected embeddings_analyzed_at to be nil in degraded metadata mode")
	}
}

func TestPublishAnalysisCompleteAnalysisType(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	queue := NewQueue(bus)
	queue.ctx = context.Background()

	ch := make(chan events.AnalysisType, 10)
	unsub := bus.Subscribe(events.AnalysisComplete, func(e events.Event) {
		ae, ok := e.Payload.(*events.AnalysisEvent)
		if !ok {
			return
		}
		ch <- ae.AnalysisType
	})
	defer unsub()

	tests := []struct {
		name   string
		result *AnalysisResult
		want   events.AnalysisType
	}{
		{
			name: "metadata_only",
			result: &AnalysisResult{
				FilePath:   "/test/metadata.txt",
				IngestMode: ingest.ModeMetadataOnly,
			},
			want: events.AnalysisMetadata,
		},
		{
			name: "skip",
			result: &AnalysisResult{
				FilePath:   "/test/skip.bin",
				IngestMode: ingest.ModeSkip,
			},
			want: events.AnalysisMetadata,
		},
		{
			name: "semantic",
			result: &AnalysisResult{
				FilePath:   "/test/semantic.md",
				IngestMode: ingest.ModeChunk,
			},
			want: events.AnalysisSemantic,
		},
		{
			name: "full",
			result: &AnalysisResult{
				FilePath:   "/test/full.go",
				IngestMode: ingest.ModeChunk,
				Embeddings: []float32{0.1, 0.2},
			},
			want: events.AnalysisFull,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue.publishAnalysisComplete(tt.result.FilePath, tt.result)
			select {
			case got := <-ch:
				if got != tt.want {
					t.Fatalf("analysis type = %q, want %q", got, tt.want)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("timeout waiting for analysis event")
			}
		})
	}
}

func TestDegradationMode(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	queue := NewQueue(bus)

	tests := []struct {
		capacity float64
		expected DegradationMode
	}{
		{0.0, DegradationFull},
		{0.5, DegradationFull},
		{0.79, DegradationFull},
		{0.80, DegradationNoEmbed},
		{0.90, DegradationNoEmbed},
		{0.95, DegradationMetadata},
		{1.0, DegradationMetadata},
	}

	for _, tt := range tests {
		result := queue.getDegradationMode(tt.capacity)
		if result != tt.expected {
			t.Errorf("getDegradationMode(%v) = %v, want %v", tt.capacity, result, tt.expected)
		}
	}
}

func TestSetWorkerCount(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	queue := NewQueue(bus, WithWorkerCount(2))

	// Before start
	queue.SetWorkerCount(4)
	if queue.workerCount != 4 {
		t.Errorf("workerCount = %d, want 4", queue.workerCount)
	}

	// Start queue
	err := queue.Start(context.Background())
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer queue.Stop(context.Background())

	// Increase workers
	queue.SetWorkerCount(6)
	time.Sleep(100 * time.Millisecond)

	if len(queue.workers) != 6 {
		t.Errorf("worker count = %d, want 6", len(queue.workers))
	}

	// Decrease workers
	queue.SetWorkerCount(3)
	time.Sleep(100 * time.Millisecond)

	if len(queue.workers) != 3 {
		t.Errorf("worker count = %d, want 3", len(queue.workers))
	}

	// Invalid count should be ignored
	queue.SetWorkerCount(0)
	if len(queue.workers) != 3 {
		t.Errorf("worker count = %d, want 3", len(queue.workers))
	}
}

func TestWorkerBackoff(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	queue := NewQueue(bus)
	queue.retryDelay = time.Second

	worker := NewWorker(0, queue)

	tests := []struct {
		retries  int
		expected time.Duration
	}{
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
	}

	for _, tt := range tests {
		result := worker.calculateBackoff(tt.retries)
		if result != tt.expected {
			t.Errorf("calculateBackoff(%d) = %v, want %v", tt.retries, result, tt.expected)
		}
	}
}

func TestComputeMetadataHash(t *testing.T) {
	now := time.Now()

	hash1 := computeMetadataHash("/path/to/file1", 100, now)
	hash2 := computeMetadataHash("/path/to/file2", 100, now)

	if hash1 == hash2 {
		t.Error("Different paths should produce different hashes")
	}
}

// mockEmbeddingsProvider is a mock implementation for testing.
type mockEmbeddingsProvider struct {
	available bool
	embedding []float32
}

func (m *mockEmbeddingsProvider) Name() string { return "mock-embeddings" }
func (m *mockEmbeddingsProvider) Type() providers.ProviderType {
	return providers.ProviderTypeEmbeddings
}
func (m *mockEmbeddingsProvider) Available() bool { return m.available }
func (m *mockEmbeddingsProvider) RateLimit() providers.RateLimitConfig {
	return providers.RateLimitConfig{}
}
func (m *mockEmbeddingsProvider) ModelName() string { return "mock-model" }
func (m *mockEmbeddingsProvider) Dimensions() int   { return len(m.embedding) }
func (m *mockEmbeddingsProvider) MaxTokens() int    { return 8192 }
func (m *mockEmbeddingsProvider) Embed(ctx context.Context, req providers.EmbeddingsRequest) (*providers.EmbeddingsResult, error) {
	return &providers.EmbeddingsResult{Embedding: m.embedding, Dimensions: len(m.embedding)}, nil
}
func (m *mockEmbeddingsProvider) EmbedBatch(ctx context.Context, texts []string) ([]providers.EmbeddingsBatchResult, error) {
	results := make([]providers.EmbeddingsBatchResult, len(texts))
	for i := range texts {
		results[i] = providers.EmbeddingsBatchResult{Index: i, Embedding: m.embedding}
	}
	return results, nil
}

// mockSemanticProvider is a mock implementation for testing.
type mockSemanticProvider struct {
	available bool
}

func (m *mockSemanticProvider) Name() string                 { return "mock-semantic" }
func (m *mockSemanticProvider) Type() providers.ProviderType { return providers.ProviderTypeSemantic }
func (m *mockSemanticProvider) Available() bool              { return m.available }
func (m *mockSemanticProvider) RateLimit() providers.RateLimitConfig {
	return providers.RateLimitConfig{}
}
func (m *mockSemanticProvider) ModelName() string { return "mock-model" }
func (m *mockSemanticProvider) Capabilities() providers.SemanticCapabilities {
	return providers.SemanticCapabilities{MaxInputTokens: 100000}
}
func (m *mockSemanticProvider) Analyze(ctx context.Context, input providers.SemanticInput) (*providers.SemanticResult, error) {
	return &providers.SemanticResult{
		Summary:    "Default summary",
		Tags:       []string{"test-tag"},
		Topics:     []providers.Topic{{Name: "test-topic", Confidence: 0.9}},
		Entities:   []providers.Entity{{Name: "TestEntity", Type: "test"}},
		Complexity: 5,
	}, nil
}

// mockGraph is a mock implementation for testing graph persistence.
type mockGraph struct {
	chunks        []*graph.ChunkNode
	deleteFileFor []string
}

func (m *mockGraph) Name() string                                               { return "mock-graph" }
func (m *mockGraph) Errors() <-chan error                                       { return nil }
func (m *mockGraph) Start(ctx context.Context) error                            { return nil }
func (m *mockGraph) Stop(ctx context.Context) error                             { return nil }
func (m *mockGraph) IsConnected() bool                                          { return true }
func (m *mockGraph) UpsertFile(ctx context.Context, file *graph.FileNode) error { return nil }
func (m *mockGraph) DeleteFile(ctx context.Context, path string) error {
	m.deleteFileFor = append(m.deleteFileFor, path)
	return nil
}
func (m *mockGraph) GetFile(ctx context.Context, path string) (*graph.FileNode, error) {
	return nil, nil
}
func (m *mockGraph) UpsertDirectory(ctx context.Context, dir *graph.DirectoryNode) error { return nil }
func (m *mockGraph) DeleteDirectory(ctx context.Context, path string) error              { return nil }
func (m *mockGraph) DeleteFilesUnderPath(ctx context.Context, parentPath string) error   { return nil }
func (m *mockGraph) DeleteDirectoriesUnderPath(ctx context.Context, parentPath string) error {
	return nil
}
func (m *mockGraph) UpsertChunkWithMetadata(ctx context.Context, chunk *graph.ChunkNode, meta *chunkers.ChunkMetadata) error {
	m.chunks = append(m.chunks, chunk)
	return nil
}
func (m *mockGraph) UpsertChunkEmbedding(ctx context.Context, chunkID string, emb *graph.ChunkEmbeddingNode) error {
	return nil
}
func (m *mockGraph) DeleteChunkEmbeddings(ctx context.Context, chunkID string, provider, model string) error {
	return nil
}
func (m *mockGraph) DeleteChunks(ctx context.Context, path string) error               { return nil }
func (m *mockGraph) SetFileTags(ctx context.Context, path string, tags []string) error { return nil }
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
func (m *mockGraph) SearchSimilarChunks(ctx context.Context, embedding []float32, k int) ([]graph.ChunkSearchHit, error) {
	return nil, nil
}

func TestBuildAnalyzedChunks(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		result := BuildAnalyzedChunks(nil)
		if result != nil {
			t.Errorf("Expected nil, got %v", result)
		}

		result = BuildAnalyzedChunks([]chunkers.Chunk{})
		if result != nil {
			t.Errorf("Expected nil for empty slice, got %v", result)
		}
	})

	t.Run("SingleChunk", func(t *testing.T) {
		chunks := []chunkers.Chunk{
			{
				Index:       0,
				Content:     "test content",
				StartOffset: 0,
				EndOffset:   12,
				Metadata: chunkers.ChunkMetadata{
					Type:          chunkers.ChunkTypeCode,
					TokenEstimate: 5,
				},
			},
		}

		result := BuildAnalyzedChunks(chunks)
		if len(result) != 1 {
			t.Fatalf("Expected 1 chunk, got %d", len(result))
		}

		if result[0].Index != 0 {
			t.Errorf("Index = %d, want 0", result[0].Index)
		}
		if result[0].Content != "test content" {
			t.Errorf("Content = %q, want %q", result[0].Content, "test content")
		}
		if result[0].ContentHash == "" {
			t.Error("ContentHash should not be empty")
		}
		if result[0].TokenCount != 5 {
			t.Errorf("TokenCount = %d, want 5", result[0].TokenCount)
		}
		if result[0].ChunkType != "code" {
			t.Errorf("ChunkType = %q, want %q", result[0].ChunkType, "code")
		}
	})

	t.Run("MultipleChunks", func(t *testing.T) {
		chunks := []chunkers.Chunk{
			{Index: 0, Content: "first", StartOffset: 0, EndOffset: 5},
			{Index: 1, Content: "second", StartOffset: 6, EndOffset: 12},
			{Index: 2, Content: "third", StartOffset: 13, EndOffset: 18},
		}

		result := BuildAnalyzedChunks(chunks)
		if len(result) != 3 {
			t.Fatalf("Expected 3 chunks, got %d", len(result))
		}

		for i, r := range result {
			if r.Index != i {
				t.Errorf("result[%d].Index = %d, want %d", i, r.Index, i)
			}
		}
	})

	t.Run("PreservesMetadata", func(t *testing.T) {
		chunks := []chunkers.Chunk{
			{
				Index:       0,
				Content:     "func TestFunc() {}",
				StartOffset: 0,
				EndOffset:   18,
				Metadata: chunkers.ChunkMetadata{
					Type:          chunkers.ChunkTypeCode,
					TokenEstimate: 10,
					Code: &chunkers.CodeMetadata{
						Language:     "go",
						FunctionName: "TestFunc",
						ClassName:    "TestClass",
					},
				},
			},
		}

		result := BuildAnalyzedChunks(chunks)
		if result[0].Metadata == nil {
			t.Fatal("Expected Metadata to be populated")
		}
		if result[0].Metadata.Code == nil {
			t.Fatal("Expected Metadata.Code to be populated")
		}
		if result[0].Metadata.Code.FunctionName != "TestFunc" {
			t.Errorf("FunctionName = %q, want %q", result[0].Metadata.Code.FunctionName, "TestFunc")
		}
		if result[0].Metadata.Code.ClassName != "TestClass" {
			t.Errorf("ClassName = %q, want %q", result[0].Metadata.Code.ClassName, "TestClass")
		}
	})
}

func TestGenerateEmbeddingsPreservesMetadata(t *testing.T) {
	// Set up mock embeddings provider
	mockEmbed := &mockEmbeddingsProvider{
		available: true,
		embedding: []float32{0.1, 0.2, 0.3},
	}
	stage := NewEmbeddingsStage(mockEmbed, nil, nil, nil)

	t.Run("PreservesCodeChunkMetadata", func(t *testing.T) {
		chunks := []chunkers.Chunk{
			{
				Index:       0,
				Content:     "func TestFunc() {}",
				StartOffset: 0,
				EndOffset:   18,
				Metadata: chunkers.ChunkMetadata{
					Type:          chunkers.ChunkTypeCode,
					TokenEstimate: 10,
					Code: &chunkers.CodeMetadata{
						Language:     "go",
						FunctionName: "TestFunc",
						ClassName:    "TestClass",
					},
				},
			},
		}

		// Build analyzed chunks first (new pattern)
		analyzedChunks := BuildAnalyzedChunks(chunks)

		_, err := stage.Generate(context.Background(), "/test/file.go", analyzedChunks)
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}

		if len(analyzedChunks) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(analyzedChunks))
		}

		result := analyzedChunks[0]
		if result.Metadata == nil || result.Metadata.Code == nil {
			t.Fatal("Expected Metadata.Code to be populated")
		}
		if result.Metadata.Code.FunctionName != "TestFunc" {
			t.Errorf("FunctionName = %q, want %q", result.Metadata.Code.FunctionName, "TestFunc")
		}
		if result.Metadata.Code.ClassName != "TestClass" {
			t.Errorf("ClassName = %q, want %q", result.Metadata.Code.ClassName, "TestClass")
		}
		if result.TokenCount != 10 {
			t.Errorf("TokenCount = %d, want 10", result.TokenCount)
		}
		if result.ChunkType != "code" {
			t.Errorf("ChunkType = %q, want %q", result.ChunkType, "code")
		}
		// Verify embedding was added
		if result.Embedding == nil {
			t.Error("Expected Embedding to be populated")
		}
	})

	t.Run("PreservesMarkdownChunkMetadata", func(t *testing.T) {
		chunks := []chunkers.Chunk{
			{
				Index:       0,
				Content:     "## Configuration\nSome content here.",
				StartOffset: 0,
				EndOffset:   35,
				Metadata: chunkers.ChunkMetadata{
					Type:          chunkers.ChunkTypeMarkdown,
					TokenEstimate: 8,
					Document: &chunkers.DocumentMetadata{
						Heading:      "Configuration",
						HeadingLevel: 2,
					},
				},
			},
		}

		// Build analyzed chunks first (new pattern)
		analyzedChunks := BuildAnalyzedChunks(chunks)

		_, err := stage.Generate(context.Background(), "/test/file.md", analyzedChunks)
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}

		if len(analyzedChunks) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(analyzedChunks))
		}

		result := analyzedChunks[0]
		if result.Metadata == nil || result.Metadata.Document == nil {
			t.Fatal("Expected Metadata.Document to be populated")
		}
		if result.Metadata.Document.Heading != "Configuration" {
			t.Errorf("Heading = %q, want %q", result.Metadata.Document.Heading, "Configuration")
		}
		if result.Metadata.Document.HeadingLevel != 2 {
			t.Errorf("HeadingLevel = %d, want 2", result.Metadata.Document.HeadingLevel)
		}
		if result.TokenCount != 8 {
			t.Errorf("TokenCount = %d, want 8", result.TokenCount)
		}
		if result.ChunkType != "markdown" {
			t.Errorf("ChunkType = %q, want %q", result.ChunkType, "markdown")
		}
	})

	t.Run("AddsEmbeddingsInPlace", func(t *testing.T) {
		chunks := []chunkers.Chunk{
			{Index: 0, Content: "Test chunk 1"},
			{Index: 1, Content: "Test chunk 2"},
		}

		analyzedChunks := BuildAnalyzedChunks(chunks)

		// Verify no embeddings before Generate
		for i, ac := range analyzedChunks {
			if ac.Embedding != nil {
				t.Errorf("analyzedChunks[%d].Embedding should be nil before Generate", i)
			}
		}

		_, err := stage.Generate(context.Background(), "/test/file.txt", analyzedChunks)
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}

		// Verify embeddings added after Generate
		for i, ac := range analyzedChunks {
			if ac.Embedding == nil {
				t.Errorf("analyzedChunks[%d].Embedding should be populated after Generate", i)
			}
		}
	})
}

func TestPersistToGraphSetsAllChunkFields(t *testing.T) {
	// Set up mock graph
	mockG := &mockGraph{}
	stage := NewPersistenceStage(mockG)

	t.Run("PersistsAllChunkMetadata", func(t *testing.T) {
		result := &AnalysisResult{
			FilePath:    "/test/file.go",
			ContentHash: "abc123",
			Chunks: []AnalyzedChunk{
				{
					Index:       0,
					Content:     "func TestFunc() {}",
					ContentHash: "chunk0hash",
					StartOffset: 0,
					EndOffset:   18,
					ChunkType:   "code",
					Metadata: &chunkers.ChunkMetadata{
						Type: chunkers.ChunkTypeCode,
						Code: &chunkers.CodeMetadata{
							FunctionName: "TestFunc",
							ClassName:    "TestClass",
						},
					},
					TokenCount: 10,
					Summary:    "This is a test function",
					Embedding:  []float32{0.1, 0.2, 0.3},
				},
			},
		}

		err := stage.Persist(context.Background(), result)
		if err != nil {
			t.Fatalf("persistToGraph failed: %v", err)
		}

		if len(mockG.chunks) != 1 {
			t.Fatalf("Expected 1 chunk, got %d", len(mockG.chunks))
		}

		chunk := mockG.chunks[0]
		// Note: FunctionName and ClassName are now stored in CodeMetaNode (separate upsert)
		if chunk.TokenCount != 10 {
			t.Errorf("TokenCount = %d, want 10", chunk.TokenCount)
		}
		if chunk.Summary != "This is a test function" {
			t.Errorf("Summary = %q, want %q", chunk.Summary, "This is a test function")
		}
	})

	t.Run("PersistsMarkdownMetadata", func(t *testing.T) {
		mockG.chunks = nil // Reset

		result := &AnalysisResult{
			FilePath:    "/test/file.md",
			ContentHash: "def456",
			Chunks: []AnalyzedChunk{
				{
					Index:       0,
					Content:     "## Configuration\nDetails here.",
					ContentHash: "chunk0hash",
					StartOffset: 0,
					EndOffset:   30,
					ChunkType:   "markdown",
					Metadata: &chunkers.ChunkMetadata{
						Type: chunkers.ChunkTypeMarkdown,
						Document: &chunkers.DocumentMetadata{
							Heading:      "Configuration",
							HeadingLevel: 2,
						},
					},
					TokenCount: 8,
					Summary:    "Configuration section",
					Embedding:  []float32{0.1, 0.2},
				},
			},
		}

		err := stage.Persist(context.Background(), result)
		if err != nil {
			t.Fatalf("persistToGraph failed: %v", err)
		}

		if len(mockG.chunks) != 1 {
			t.Fatalf("Expected 1 chunk, got %d", len(mockG.chunks))
		}

		chunk := mockG.chunks[0]
		// Note: Heading and HeadingLevel are now stored in DocumentMetaNode (separate upsert)
		if chunk.ChunkType != "markdown" {
			t.Errorf("ChunkType = %q, want %q", chunk.ChunkType, "markdown")
		}
	})

	t.Run("DeletesSkippedFiles", func(t *testing.T) {
		mockG.chunks = nil
		mockG.deleteFileFor = nil

		result := &AnalysisResult{
			FilePath:    "/test/skip.bin",
			ContentHash: "skiphash",
			IngestMode:  ingest.ModeSkip,
		}

		err := stage.Persist(context.Background(), result)
		if err != nil {
			t.Fatalf("persistToGraph failed: %v", err)
		}

		if len(mockG.deleteFileFor) != 1 {
			t.Fatalf("expected DeleteFile to be called once, got %d", len(mockG.deleteFileFor))
		}
		if mockG.deleteFileFor[0] != result.FilePath {
			t.Fatalf("DeleteFile path = %q, want %q", mockG.deleteFileFor[0], result.FilePath)
		}
	})
}

func TestWorkerAnalyze_DegradationNoEmbed_ChunksPopulated(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	ctx := context.Background()
	reg, err := registry.Open(ctx, filepath.Join(t.TempDir(), "registry.db"))
	if err != nil {
		t.Fatalf("failed to open registry: %v", err)
	}
	defer reg.Close()

	// Create queue with small capacity to easily trigger degradation mode.
	// DegradationNoEmbed is triggered when capacity >= 0.80 and < 0.95.
	// Using capacity 10, we need 8-9 items to hit this range.
	queue := NewQueue(bus)
	queue.queueCapacity = 10
	queue.workChan = make(chan WorkItem, 10)

	// Fill queue to 90% capacity (9 items) to trigger DegradationNoEmbed
	for i := 0; i < 9; i++ {
		queue.workChan <- WorkItem{FilePath: fmt.Sprintf("/fake/path%d", i)}
	}

	worker := NewWorker(0, queue)
	worker.SetRegistry(reg)

	// Set up mock providers
	mockEmbed := &mockEmbeddingsProvider{
		available: true,
		embedding: []float32{0.1, 0.2, 0.3},
	}
	worker.SetEmbeddingsProvider(mockEmbed)

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.go")
	if err := os.WriteFile(filePath, []byte("package main\nfunc main() {}"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("failed to stat test file: %v", err)
	}

	result, err := worker.analyze(ctx, WorkItem{
		FilePath:  filePath,
		FileSize:  info.Size(),
		ModTime:   info.ModTime(),
		EventType: WorkItemNew,
	})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}

	// Key assertion: chunks should be populated even in DegradationNoEmbed mode
	if result.Chunks == nil {
		t.Fatal("expected Chunks to be populated in DegradationNoEmbed mode")
	}
	if len(result.Chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	// Verify embeddings are NOT populated (because we're in DegradationNoEmbed mode)
	if result.Embeddings != nil {
		t.Error("expected Embeddings to be nil in DegradationNoEmbed mode")
	}

	// Verify chunk data is correct
	for i, chunk := range result.Chunks {
		if chunk.Content == "" {
			t.Errorf("chunk[%d].Content is empty", i)
		}
		if chunk.ContentHash == "" {
			t.Errorf("chunk[%d].ContentHash is empty", i)
		}
		// Embeddings should NOT be on chunks in degradation mode
		if chunk.Embedding != nil {
			t.Errorf("chunk[%d].Embedding should be nil in DegradationNoEmbed mode", i)
		}
	}
}

func TestWorkerAnalyze_EmbeddingsFailure_ChunksStillPopulated(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	ctx := context.Background()
	reg, err := registry.Open(ctx, filepath.Join(t.TempDir(), "registry.db"))
	if err != nil {
		t.Fatalf("failed to open registry: %v", err)
	}
	defer reg.Close()

	// Use larger capacity to stay in DegradationFull mode (capacity < 0.80)
	queue := NewQueue(bus)
	queue.queueCapacity = 100
	queue.workChan = make(chan WorkItem, 100)

	worker := NewWorker(0, queue)
	worker.SetRegistry(reg)

	// Set up a failing embeddings provider
	failingEmbed := &failingEmbeddingsProvider{
		available: true,
	}
	worker.SetEmbeddingsProvider(failingEmbed)

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.go")
	if err := os.WriteFile(filePath, []byte("package main\nfunc main() {}"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("failed to stat test file: %v", err)
	}

	result, err := worker.analyze(ctx, WorkItem{
		FilePath:  filePath,
		FileSize:  info.Size(),
		ModTime:   info.ModTime(),
		EventType: WorkItemNew,
	})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}

	// Key assertion: chunks should be populated even when embeddings fail
	if result.Chunks == nil {
		t.Fatal("expected Chunks to be populated even when embeddings fail")
	}
	if len(result.Chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	// Verify file-level embeddings are NOT populated (because provider failed)
	if result.Embeddings != nil {
		t.Error("expected Embeddings to be nil when embeddings generation fails")
	}

	// Verify chunk data is correct
	for i, chunk := range result.Chunks {
		if chunk.Content == "" {
			t.Errorf("chunk[%d].Content is empty", i)
		}
		if chunk.ContentHash == "" {
			t.Errorf("chunk[%d].ContentHash is empty", i)
		}
	}
}

func TestWorkerAnalyze_NoEmbeddingsProvider_ChunksStillPopulated(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	ctx := context.Background()
	reg, err := registry.Open(ctx, filepath.Join(t.TempDir(), "registry.db"))
	if err != nil {
		t.Fatalf("failed to open registry: %v", err)
	}
	defer reg.Close()

	// Use larger capacity to stay in DegradationFull mode (capacity < 0.80)
	queue := NewQueue(bus)
	queue.queueCapacity = 100
	queue.workChan = make(chan WorkItem, 100)

	worker := NewWorker(0, queue)
	worker.SetRegistry(reg)
	// Explicitly NOT setting embeddings provider

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.go")
	if err := os.WriteFile(filePath, []byte("package main\nfunc main() {}"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("failed to stat test file: %v", err)
	}

	result, err := worker.analyze(ctx, WorkItem{
		FilePath:  filePath,
		FileSize:  info.Size(),
		ModTime:   info.ModTime(),
		EventType: WorkItemNew,
	})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}

	// Key assertion: chunks should be populated even without embeddings provider
	if result.Chunks == nil {
		t.Fatal("expected Chunks to be populated even without embeddings provider")
	}
	if len(result.Chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	// Verify embeddings are NOT populated
	if result.Embeddings != nil {
		t.Error("expected Embeddings to be nil without embeddings provider")
	}

	// Verify chunk data is correct
	for i, chunk := range result.Chunks {
		if chunk.Content == "" {
			t.Errorf("chunk[%d].Content is empty", i)
		}
		if chunk.ContentHash == "" {
			t.Errorf("chunk[%d].ContentHash is empty", i)
		}
		// No embeddings on chunks without provider
		if chunk.Embedding != nil {
			t.Errorf("chunk[%d].Embedding should be nil without embeddings provider", i)
		}
	}
}

// failingEmbeddingsProvider always returns an error when embedding.
type failingEmbeddingsProvider struct {
	available bool
}

func (f *failingEmbeddingsProvider) Name() string { return "failing-embeddings" }
func (f *failingEmbeddingsProvider) Type() providers.ProviderType {
	return providers.ProviderTypeEmbeddings
}
func (f *failingEmbeddingsProvider) Available() bool { return f.available }
func (f *failingEmbeddingsProvider) RateLimit() providers.RateLimitConfig {
	return providers.RateLimitConfig{}
}
func (f *failingEmbeddingsProvider) ModelName() string { return "failing-model" }
func (f *failingEmbeddingsProvider) Dimensions() int   { return 0 }
func (f *failingEmbeddingsProvider) MaxTokens() int    { return 8192 }
func (f *failingEmbeddingsProvider) Embed(ctx context.Context, req providers.EmbeddingsRequest) (*providers.EmbeddingsResult, error) {
	return nil, fmt.Errorf("simulated embeddings failure")
}
func (f *failingEmbeddingsProvider) EmbedBatch(ctx context.Context, texts []string) ([]providers.EmbeddingsBatchResult, error) {
	return nil, fmt.Errorf("simulated batch embeddings failure")
}

func TestAnalyzedChunkContainsAllFields(t *testing.T) {
	// This test verifies the AnalyzedChunk struct has all expected fields
	ac := AnalyzedChunk{
		Index:       1,
		Content:     "test content",
		ContentHash: "hash123",
		StartOffset: 0,
		EndOffset:   12,
		ChunkType:   "code",
		Embedding:   []float32{0.1},
		Metadata: &chunkers.ChunkMetadata{
			Type: chunkers.ChunkTypeCode,
			Code: &chunkers.CodeMetadata{
				FunctionName: "myFunc",
				ClassName:    "MyClass",
			},
		},
		TokenCount: 5,
		Summary:    "test summary",
	}

	// Verify all fields are accessible and have expected values
	if ac.Metadata == nil || ac.Metadata.Code == nil {
		t.Fatal("Expected Metadata.Code to be populated")
	}
	if ac.Metadata.Code.FunctionName != "myFunc" {
		t.Errorf("FunctionName = %q, want %q", ac.Metadata.Code.FunctionName, "myFunc")
	}
	if ac.Metadata.Code.ClassName != "MyClass" {
		t.Errorf("ClassName = %q, want %q", ac.Metadata.Code.ClassName, "MyClass")
	}
	if ac.TokenCount != 5 {
		t.Errorf("TokenCount = %d, want 5", ac.TokenCount)
	}
	if ac.Summary != "test summary" {
		t.Errorf("Summary = %q, want %q", ac.Summary, "test summary")
	}
}
