package analysis

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/filetype"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
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

	expectedContentHash := filetype.HashBytes(content)
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

func TestMergeSemanticResults(t *testing.T) {
	t.Run("EmptyResults", func(t *testing.T) {
		result, err := mergeSemanticResults(context.Background(), nil, nil)
		if err != nil {
			t.Errorf("mergeSemanticResults failed: %v", err)
		}
		if result == nil {
			t.Error("Expected non-nil result")
		}
	})

	t.Run("SingleResult", func(t *testing.T) {
		input := &SemanticResult{
			Summary: "Test summary",
			Tags:    []string{"tag1", "tag2"},
		}

		result, err := mergeSemanticResults(context.Background(), nil, []*SemanticResult{input})
		if err != nil {
			t.Errorf("mergeSemanticResults failed: %v", err)
		}
		if result.Summary != "Test summary" {
			t.Errorf("Summary = %q, want %q", result.Summary, "Test summary")
		}
	})

	t.Run("DeduplicateTags", func(t *testing.T) {
		results := []*SemanticResult{
			{Tags: []string{"tag1", "tag2"}},
			{Tags: []string{"tag2", "tag3"}},
			{Tags: []string{"TAG1", "tag4"}}, // Case-insensitive dedup
		}

		merged, err := mergeSemanticResults(context.Background(), nil, results)
		if err != nil {
			t.Errorf("mergeSemanticResults failed: %v", err)
		}

		// Should have 4 unique tags (tag1, tag2, tag3, tag4)
		if len(merged.Tags) != 4 {
			t.Errorf("Expected 4 unique tags, got %d: %v", len(merged.Tags), merged.Tags)
		}
	})

	t.Run("DeduplicateEntities", func(t *testing.T) {
		results := []*SemanticResult{
			{Entities: []Entity{{Name: "Go", Type: "language"}}},
			{Entities: []Entity{{Name: "Go", Type: "language"}}}, // Duplicate
			{Entities: []Entity{{Name: "Python", Type: "language"}}},
		}

		merged, err := mergeSemanticResults(context.Background(), nil, results)
		if err != nil {
			t.Errorf("mergeSemanticResults failed: %v", err)
		}

		if len(merged.Entities) != 2 {
			t.Errorf("Expected 2 unique entities, got %d", len(merged.Entities))
		}
	})

	t.Run("MaxComplexity", func(t *testing.T) {
		results := []*SemanticResult{
			{Complexity: 3},
			{Complexity: 7},
			{Complexity: 5},
		}

		merged, err := mergeSemanticResults(context.Background(), nil, results)
		if err != nil {
			t.Errorf("mergeSemanticResults failed: %v", err)
		}

		if merged.Complexity != 7 {
			t.Errorf("Complexity = %d, want 7", merged.Complexity)
		}
	})

	t.Run("ConcatenateSummaries", func(t *testing.T) {
		results := []*SemanticResult{
			{Summary: "First part."},
			{Summary: "Second part."},
		}

		merged, err := mergeSemanticResults(context.Background(), nil, results)
		if err != nil {
			t.Errorf("mergeSemanticResults failed: %v", err)
		}

		if merged.Summary == "" {
			t.Error("Expected non-empty summary")
		}
	})
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
	summaries map[int]string // Map chunk index to summary
}

func (m *mockSemanticProvider) Name() string                 { return "mock-semantic" }
func (m *mockSemanticProvider) Type() providers.ProviderType { return providers.ProviderTypeSemantic }
func (m *mockSemanticProvider) Available() bool              { return m.available }
func (m *mockSemanticProvider) RateLimit() providers.RateLimitConfig {
	return providers.RateLimitConfig{}
}
func (m *mockSemanticProvider) SupportedMIMETypes() []string { return []string{"text/plain"} }
func (m *mockSemanticProvider) MaxContentSize() int64        { return 100000 }
func (m *mockSemanticProvider) SupportsVision() bool         { return false }
func (m *mockSemanticProvider) Analyze(ctx context.Context, req providers.SemanticRequest) (*providers.SemanticResult, error) {
	summary := "Default summary"
	// Check if this is for a specific chunk (content-based lookup would be ideal, but for testing we use a simple approach)
	for _, s := range m.summaries {
		summary = s
		break
	}
	return &providers.SemanticResult{
		Summary:    summary,
		Tags:       []string{"test-tag"},
		Topics:     []providers.Topic{{Name: "test-topic", Confidence: 0.9}},
		Entities:   []providers.Entity{{Name: "TestEntity", Type: "test"}},
		Complexity: 5,
	}, nil
}

// mockGraph is a mock implementation for testing graph persistence.
type mockGraph struct {
	chunks []*graph.ChunkNode
}

func (m *mockGraph) Name() string                                               { return "mock-graph" }
func (m *mockGraph) Errors() <-chan error                                       { return nil }
func (m *mockGraph) Start(ctx context.Context) error                            { return nil }
func (m *mockGraph) Stop(ctx context.Context) error                             { return nil }
func (m *mockGraph) IsConnected() bool                                          { return true }
func (m *mockGraph) UpsertFile(ctx context.Context, file *graph.FileNode) error { return nil }
func (m *mockGraph) DeleteFile(ctx context.Context, path string) error          { return nil }
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
func (m *mockGraph) SearchSimilarChunks(ctx context.Context, embedding []float32, k int) ([]graph.ChunkNode, error) {
	return nil, nil
}

func TestGenerateEmbeddingsPreservesMetadata(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	queue := NewQueue(bus)
	worker := NewWorker(0, queue)

	// Set up mock embeddings provider
	mockEmbed := &mockEmbeddingsProvider{
		available: true,
		embedding: []float32{0.1, 0.2, 0.3},
	}
	worker.SetEmbeddingsProvider(mockEmbed)

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

		_, results, err := worker.generateEmbeddings(context.Background(), chunks, nil)
		if err != nil {
			t.Fatalf("generateEmbeddings failed: %v", err)
		}

		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}

		result := results[0]
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

		_, results, err := worker.generateEmbeddings(context.Background(), chunks, nil)
		if err != nil {
			t.Fatalf("generateEmbeddings failed: %v", err)
		}

		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}

		result := results[0]
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

	t.Run("PreservesPerChunkSummaries", func(t *testing.T) {
		chunks := []chunkers.Chunk{
			{Index: 0, Content: "First chunk"},
			{Index: 1, Content: "Second chunk"},
		}
		summaries := []string{"Summary for first chunk", "Summary for second chunk"}

		_, results, err := worker.generateEmbeddings(context.Background(), chunks, summaries)
		if err != nil {
			t.Fatalf("generateEmbeddings failed: %v", err)
		}

		if len(results) != 2 {
			t.Fatalf("Expected 2 results, got %d", len(results))
		}

		if results[0].Summary != "Summary for first chunk" {
			t.Errorf("results[0].Summary = %q, want %q", results[0].Summary, "Summary for first chunk")
		}
		if results[1].Summary != "Summary for second chunk" {
			t.Errorf("results[1].Summary = %q, want %q", results[1].Summary, "Summary for second chunk")
		}
	})

	t.Run("HandlesNilSummaries", func(t *testing.T) {
		chunks := []chunkers.Chunk{
			{Index: 0, Content: "Test chunk"},
		}

		_, results, err := worker.generateEmbeddings(context.Background(), chunks, nil)
		if err != nil {
			t.Fatalf("generateEmbeddings failed: %v", err)
		}

		if results[0].Summary != "" {
			t.Errorf("Summary should be empty when no summaries provided, got %q", results[0].Summary)
		}
	})
}

func TestAnalyzeSemanticsReturnsPerChunkSummaries(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	queue := NewQueue(bus)
	worker := NewWorker(0, queue)

	// Set up mock semantic provider
	mockSemantic := &mockSemanticProvider{
		available: true,
		summaries: map[int]string{
			0: "Chunk 0 summary",
			1: "Chunk 1 summary",
		},
	}
	worker.SetSemanticProvider(mockSemantic)

	t.Run("ReturnsSummariesForSingleChunk", func(t *testing.T) {
		chunks := []chunkers.Chunk{
			{Index: 0, Content: "Single chunk content"},
		}

		result, summaries, err := worker.analyzeSemantics(context.Background(), chunks)
		if err != nil {
			t.Fatalf("analyzeSemantics failed: %v", err)
		}

		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		if len(summaries) != 1 {
			t.Fatalf("Expected 1 summary, got %d", len(summaries))
		}

		if summaries[0] == "" {
			t.Error("Expected non-empty summary for chunk 0")
		}
	})

	t.Run("ReturnsSummariesForMultipleChunks", func(t *testing.T) {
		chunks := []chunkers.Chunk{
			{Index: 0, Content: "First chunk"},
			{Index: 1, Content: "Second chunk"},
		}

		result, summaries, err := worker.analyzeSemantics(context.Background(), chunks)
		if err != nil {
			t.Fatalf("analyzeSemantics failed: %v", err)
		}

		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		if len(summaries) != 2 {
			t.Fatalf("Expected 2 summaries, got %d", len(summaries))
		}

		// Both should have summaries
		if summaries[0] == "" {
			t.Error("Expected non-empty summary for chunk 0")
		}
		if summaries[1] == "" {
			t.Error("Expected non-empty summary for chunk 1")
		}
	})

	t.Run("ReturnsEmptyForEmptyChunks", func(t *testing.T) {
		result, summaries, err := worker.analyzeSemantics(context.Background(), []chunkers.Chunk{})
		if err != nil {
			t.Fatalf("analyzeSemantics failed: %v", err)
		}

		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		if summaries != nil {
			t.Errorf("Expected nil summaries for empty chunks, got %v", summaries)
		}
	})
}

func TestPersistToGraphSetsAllChunkFields(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	queue := NewQueue(bus)
	worker := NewWorker(0, queue)

	// Set up mock graph
	mockG := &mockGraph{}
	worker.SetGraph(mockG)

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

		err := worker.persistToGraph(context.Background(), result)
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

		err := worker.persistToGraph(context.Background(), result)
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
