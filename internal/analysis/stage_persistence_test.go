package analysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/ingest"
	"github.com/leefowlercu/agentic-memorizer/internal/storage"
)

// mockGraphForPersistence implements graph.Graph for persistence stage testing.
type mockGraphForPersistence struct {
	connected    bool
	upsertErr    error
	deleteErr    error
	upsertCalled int
	deleteCalled int
}

func (m *mockGraphForPersistence) Name() string                    { return "mock-graph" }
func (m *mockGraphForPersistence) Errors() <-chan error            { return nil }
func (m *mockGraphForPersistence) Start(ctx context.Context) error { return nil }
func (m *mockGraphForPersistence) Stop(ctx context.Context) error  { return nil }
func (m *mockGraphForPersistence) IsConnected() bool               { return m.connected }
func (m *mockGraphForPersistence) UpsertFile(ctx context.Context, file *graph.FileNode) error {
	m.upsertCalled++
	return m.upsertErr
}
func (m *mockGraphForPersistence) DeleteFile(ctx context.Context, path string) error {
	m.deleteCalled++
	return m.deleteErr
}
func (m *mockGraphForPersistence) GetFile(ctx context.Context, path string) (*graph.FileNode, error) {
	return nil, nil
}
func (m *mockGraphForPersistence) UpsertDirectory(ctx context.Context, dir *graph.DirectoryNode) error {
	return nil
}
func (m *mockGraphForPersistence) DeleteDirectory(ctx context.Context, path string) error { return nil }
func (m *mockGraphForPersistence) DeleteFilesUnderPath(ctx context.Context, parentPath string) error {
	return nil
}
func (m *mockGraphForPersistence) DeleteDirectoriesUnderPath(ctx context.Context, parentPath string) error {
	return nil
}
func (m *mockGraphForPersistence) UpsertChunkWithMetadata(ctx context.Context, chunk *graph.ChunkNode, meta *chunkers.ChunkMetadata) error {
	return nil
}
func (m *mockGraphForPersistence) UpsertChunkEmbedding(ctx context.Context, chunkID string, emb *graph.ChunkEmbeddingNode) error {
	return nil
}
func (m *mockGraphForPersistence) DeleteChunkEmbeddings(ctx context.Context, chunkID string, provider, model string) error {
	return nil
}
func (m *mockGraphForPersistence) DeleteChunks(ctx context.Context, path string) error { return nil }
func (m *mockGraphForPersistence) SetFileTags(ctx context.Context, path string, tags []string) error {
	return nil
}
func (m *mockGraphForPersistence) SetFileTopics(ctx context.Context, path string, topics []graph.Topic) error {
	return nil
}
func (m *mockGraphForPersistence) SetFileEntities(ctx context.Context, path string, entities []graph.Entity) error {
	return nil
}
func (m *mockGraphForPersistence) SetFileReferences(ctx context.Context, path string, refs []graph.Reference) error {
	return nil
}
func (m *mockGraphForPersistence) Query(ctx context.Context, cypher string) (*graph.QueryResult, error) {
	return nil, nil
}
func (m *mockGraphForPersistence) HasEmbedding(ctx context.Context, contentHash string, version int) (bool, error) {
	return false, nil
}
func (m *mockGraphForPersistence) ExportSnapshot(ctx context.Context) (*graph.GraphSnapshot, error) {
	return nil, nil
}
func (m *mockGraphForPersistence) GetFileWithRelations(ctx context.Context, path string) (*graph.FileWithRelations, error) {
	return nil, nil
}
func (m *mockGraphForPersistence) SearchSimilarChunks(ctx context.Context, embedding []float32, k int) ([]graph.ChunkSearchHit, error) {
	return nil, nil
}

// mockPersistenceQueue implements storage.DurablePersistenceQueue for testing.
type mockPersistenceQueue struct {
	enqueued    []mockQueuedItem
	enqueueErr  error
	dequeueErr  error
	completeErr error
	failErr     error
}

type mockQueuedItem struct {
	filePath    string
	contentHash string
	resultJSON  []byte
}

func (m *mockPersistenceQueue) Enqueue(ctx context.Context, filePath, contentHash string, resultJSON []byte) error {
	if m.enqueueErr != nil {
		return m.enqueueErr
	}
	m.enqueued = append(m.enqueued, mockQueuedItem{
		filePath:    filePath,
		contentHash: contentHash,
		resultJSON:  resultJSON,
	})
	return nil
}

func (m *mockPersistenceQueue) DequeueBatch(ctx context.Context, n int) ([]*storage.QueuedResult, error) {
	return nil, m.dequeueErr
}

func (m *mockPersistenceQueue) Complete(ctx context.Context, id int64) error {
	return m.completeErr
}

func (m *mockPersistenceQueue) Fail(ctx context.Context, id int64, maxRetries int, errMsg string) error {
	return m.failErr
}

func (m *mockPersistenceQueue) Stats(ctx context.Context) (*storage.QueueStats, error) {
	return &storage.QueueStats{
		Pending: int64(len(m.enqueued)),
	}, nil
}

func (m *mockPersistenceQueue) Purge(ctx context.Context, completedOlderThan, failedOlderThan time.Duration) (int64, error) {
	return 0, nil
}

func TestPersistenceStage_Persist(t *testing.T) {
	tests := []struct {
		name            string
		graphConnected  bool
		graphUpsertErr  error
		queueConfigured bool
		queueEnqueueErr error
		wantQueued      bool
		wantErr         bool
		wantErrContains string
	}{
		{
			name:            "direct persist when graph connected",
			graphConnected:  true,
			queueConfigured: true,
			wantQueued:      false,
			wantErr:         false,
		},
		{
			name:            "queue when graph not connected",
			graphConnected:  false,
			queueConfigured: true,
			wantQueued:      true,
			wantErr:         false,
		},
		{
			name:            "queue when graph write fails",
			graphConnected:  true,
			graphUpsertErr:  errors.New("connection lost"),
			queueConfigured: true,
			wantQueued:      true,
			wantErr:         false,
		},
		{
			name:            "error when graph fails and no queue",
			graphConnected:  true,
			graphUpsertErr:  errors.New("connection lost"),
			queueConfigured: false,
			wantQueued:      false,
			wantErr:         true,
			wantErrContains: "failed to upsert file",
		},
		{
			name:            "no op when graph not connected and no queue",
			graphConnected:  false,
			queueConfigured: false,
			wantQueued:      false,
			wantErr:         false,
		},
		{
			name:            "error when queue fails",
			graphConnected:  false,
			queueConfigured: true,
			queueEnqueueErr: errors.New("queue full"),
			wantQueued:      false,
			wantErr:         true,
			wantErrContains: "failed to enqueue",
		},
		{
			name:            "combined error when persistence and queue both fail",
			graphConnected:  true,
			graphUpsertErr:  errors.New("graph error"),
			queueConfigured: true,
			queueEnqueueErr: errors.New("queue error"),
			wantQueued:      false,
			wantErr:         true,
			wantErrContains: "persistence failed and queuing failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGraph := &mockGraphForPersistence{
				connected: tt.graphConnected,
				upsertErr: tt.graphUpsertErr,
			}

			var mockQueue *mockPersistenceQueue
			var opts []PersistenceStageOption

			if tt.queueConfigured {
				mockQueue = &mockPersistenceQueue{
					enqueueErr: tt.queueEnqueueErr,
				}
				opts = append(opts, WithPersistenceQueue(mockQueue))
			}

			stage := NewPersistenceStage(mockGraph, opts...)

			result := &AnalysisResult{
				FilePath:    "/test/file.txt",
				ContentHash: "abc123",
				IngestMode:  ingest.ModeChunk,
			}

			err := stage.Persist(context.Background(), result)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil")
				} else if tt.wantErrContains != "" && !containsString(err.Error(), tt.wantErrContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.wantErrContains)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.queueConfigured {
				queued := len(mockQueue.enqueued) > 0
				if queued != tt.wantQueued {
					t.Errorf("queued = %v, want %v", queued, tt.wantQueued)
				}
			}
		})
	}
}

func TestPersistenceStage_NilGraphAndQueue(t *testing.T) {
	stage := NewPersistenceStage(nil)

	result := &AnalysisResult{
		FilePath:    "/test/file.txt",
		ContentHash: "abc123",
		IngestMode:  ingest.ModeChunk,
	}

	err := stage.Persist(context.Background(), result)
	if err != nil {
		t.Errorf("expected no error when graph and queue are nil, got: %v", err)
	}
}

func TestPersistenceStage_SkipModeDeletesFile(t *testing.T) {
	mockGraph := &mockGraphForPersistence{
		connected: true,
	}

	stage := NewPersistenceStage(mockGraph)

	result := &AnalysisResult{
		FilePath:   "/test/file.txt",
		IngestMode: ingest.ModeSkip,
	}

	err := stage.Persist(context.Background(), result)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if mockGraph.deleteCalled != 1 {
		t.Errorf("expected DeleteFile to be called once, got %d", mockGraph.deleteCalled)
	}
}

func TestPersistenceStage_SkipModeQueuesOnDeleteError(t *testing.T) {
	mockGraph := &mockGraphForPersistence{
		connected: true,
		deleteErr: errors.New("delete failed"),
	}

	mockQueue := &mockPersistenceQueue{}

	stage := NewPersistenceStage(mockGraph, WithPersistenceQueue(mockQueue))

	result := &AnalysisResult{
		FilePath:    "/test/file.txt",
		ContentHash: "abc123",
		IngestMode:  ingest.ModeSkip,
	}

	err := stage.Persist(context.Background(), result)
	if err != nil {
		t.Errorf("expected no error when queuing succeeds, got: %v", err)
	}

	if len(mockQueue.enqueued) != 1 {
		t.Errorf("expected result to be queued, got %d items", len(mockQueue.enqueued))
	}
}

func TestPersistenceStage_WithOptions(t *testing.T) {
	mockGraph := &mockGraphForPersistence{connected: true}
	mockQueue := &mockPersistenceQueue{}

	stage := NewPersistenceStage(
		mockGraph,
		WithPersistenceLogger(nil),
		WithPersistenceQueue(mockQueue),
	)

	if stage.queue == nil {
		t.Error("expected queue to be set via option")
	}
}

func TestPersistenceStage_QueuedResultContainsCorrectData(t *testing.T) {
	mockGraph := &mockGraphForPersistence{
		connected: false,
	}

	mockQueue := &mockPersistenceQueue{}

	stage := NewPersistenceStage(mockGraph, WithPersistenceQueue(mockQueue))

	result := &AnalysisResult{
		FilePath:    "/test/specific/path.go",
		ContentHash: "specific-hash-123",
		IngestMode:  ingest.ModeChunk,
		Summary:     "A test summary",
	}

	err := stage.Persist(context.Background(), result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mockQueue.enqueued) != 1 {
		t.Fatalf("expected 1 queued item, got %d", len(mockQueue.enqueued))
	}

	item := mockQueue.enqueued[0]
	if item.filePath != result.FilePath {
		t.Errorf("queued filePath = %q, want %q", item.filePath, result.FilePath)
	}
	if item.contentHash != result.ContentHash {
		t.Errorf("queued contentHash = %q, want %q", item.contentHash, result.ContentHash)
	}
	if len(item.resultJSON) == 0 {
		t.Error("expected resultJSON to be non-empty")
	}

	// Verify we can unmarshal the JSON
	var unmarshaled AnalysisResult
	if err := storage.UnmarshalAnalysisResult(item.resultJSON, &unmarshaled); err != nil {
		t.Errorf("failed to unmarshal queued result: %v", err)
	}
	if unmarshaled.FilePath != result.FilePath {
		t.Errorf("unmarshaled FilePath = %q, want %q", unmarshaled.FilePath, result.FilePath)
	}
	if unmarshaled.Summary != result.Summary {
		t.Errorf("unmarshaled Summary = %q, want %q", unmarshaled.Summary, result.Summary)
	}
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
