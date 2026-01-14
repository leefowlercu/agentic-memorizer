package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/graph"
)

// mockGraph is a test implementation of graph.Graph.
type mockGraph struct {
	snapshot *graph.GraphSnapshot
}

func newMockGraph() *mockGraph {
	return &mockGraph{
		snapshot: &graph.GraphSnapshot{
			Version:    1,
			ExportedAt: time.Now(),
			Files: []graph.FileNode{
				{
					Path:      "/test/file.go",
					Name:      "file.go",
					Extension: ".go",
					Language:  "go",
					Size:      1024,
					Summary:   "Test file",
				},
			},
			Directories: []graph.DirectoryNode{
				{
					Path:         "/test",
					Name:         "test",
					IsRemembered: true,
					FileCount:    1,
				},
			},
			Tags:               []graph.TagNode{{Name: "go", UsageCount: 1}},
			Topics:             []graph.TopicNode{{Name: "Testing", UsageCount: 1}},
			Entities:           []graph.EntityNode{{Name: "Go", Type: "language", UsageCount: 1}},
			TotalChunks:        5,
			TotalRelationships: 10,
		},
	}
}

func (m *mockGraph) Start(ctx context.Context) error                                     { return nil }
func (m *mockGraph) Stop(ctx context.Context) error                                      { return nil }
func (m *mockGraph) Name() string                                                        { return "mock-graph" }
func (m *mockGraph) UpsertFile(ctx context.Context, file *graph.FileNode) error          { return nil }
func (m *mockGraph) DeleteFile(ctx context.Context, path string) error                   { return nil }
func (m *mockGraph) GetFile(ctx context.Context, path string) (*graph.FileNode, error)   { return nil, nil }
func (m *mockGraph) UpsertDirectory(ctx context.Context, dir *graph.DirectoryNode) error { return nil }
func (m *mockGraph) DeleteDirectory(ctx context.Context, path string) error                  { return nil }
func (m *mockGraph) DeleteFilesUnderPath(ctx context.Context, parentPath string) error       { return nil }
func (m *mockGraph) DeleteDirectoriesUnderPath(ctx context.Context, parentPath string) error { return nil }
func (m *mockGraph) UpsertChunk(ctx context.Context, chunk *graph.ChunkNode) error           { return nil }
func (m *mockGraph) DeleteChunks(ctx context.Context, filePath string) error             { return nil }
func (m *mockGraph) SetFileTags(ctx context.Context, path string, tags []string) error   { return nil }
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
func (m *mockGraph) IsConnected() bool { return true }
func (m *mockGraph) HasEmbedding(ctx context.Context, contentHash string, version int) (bool, error) {
	return false, nil
}
func (m *mockGraph) ExportSnapshot(ctx context.Context) (*graph.GraphSnapshot, error) {
	return m.snapshot, nil
}
func (m *mockGraph) GetFileWithRelations(ctx context.Context, path string) (*graph.FileWithRelations, error) {
	return nil, nil
}
func (m *mockGraph) SearchSimilarChunks(ctx context.Context, embedding []float32, k int) ([]graph.ChunkNode, error) {
	return nil, nil
}

func TestNewServer(t *testing.T) {
	g := newMockGraph()
	cfg := DefaultConfig()

	s := NewServer(g, cfg)

	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.mcpServer == nil {
		t.Error("mcpServer is nil")
	}
	if s.sseServer == nil {
		t.Error("sseServer is nil")
	}
	if s.exporter == nil {
		t.Error("exporter is nil")
	}
	if s.subs == nil {
		t.Error("subs is nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Name != "memorizer" {
		t.Errorf("Name = %q, want %q", cfg.Name, "memorizer")
	}
	if cfg.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", cfg.Version, "1.0.0")
	}
	if cfg.BasePath != "/mcp" {
		t.Errorf("BasePath = %q, want %q", cfg.BasePath, "/mcp")
	}
}

func TestServerStartStop(t *testing.T) {
	g := newMockGraph()
	s := NewServer(g, DefaultConfig())

	ctx := context.Background()

	// Start
	err := s.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !s.running {
		t.Error("Server should be running after Start")
	}

	// Stop
	err = s.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if s.running {
		t.Error("Server should not be running after Stop")
	}
}

func TestServerHandler(t *testing.T) {
	g := newMockGraph()
	s := NewServer(g, DefaultConfig())

	handler := s.Handler()
	if handler == nil {
		t.Error("Handler returned nil")
	}

	sseHandler := s.SSEHandler()
	if sseHandler == nil {
		t.Error("SSEHandler returned nil")
	}

	msgHandler := s.MessageHandler()
	if msgHandler == nil {
		t.Error("MessageHandler returned nil")
	}
}

func TestResourceNotFoundError(t *testing.T) {
	err := &ResourceNotFoundError{URI: "memorizer://invalid"}
	expected := "resource not found: memorizer://invalid"

	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}
