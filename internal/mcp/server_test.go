package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
)

// mockRegistry is a test implementation of RegistryChecker.
type mockRegistry struct {
	rememberedPaths map[string]bool
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		rememberedPaths: map[string]bool{
			"/test": true,
		},
	}
}

func (m *mockRegistry) IsPathRemembered(ctx context.Context, filePath string) bool {
	for path := range m.rememberedPaths {
		if len(filePath) >= len(path) && filePath[:len(path)] == path {
			return true
		}
	}
	return false
}

// mockGraph is a test implementation of graph.Graph.
type mockGraph struct {
	snapshot            *graph.GraphSnapshot
	searchHits          []graph.ChunkSearchHit
	searchErr           error
	lastSearchEmbedding []float32
	lastSearchK         int
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

func (m *mockGraph) Start(ctx context.Context) error                            { return nil }
func (m *mockGraph) Stop(ctx context.Context) error                             { return nil }
func (m *mockGraph) Name() string                                               { return "mock-graph" }
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
	return nil
}
func (m *mockGraph) UpsertChunkEmbedding(ctx context.Context, chunkID string, emb *graph.ChunkEmbeddingNode) error {
	return nil
}
func (m *mockGraph) DeleteChunkEmbeddings(ctx context.Context, chunkID string, provider, model string) error {
	return nil
}
func (m *mockGraph) DeleteChunks(ctx context.Context, filePath string) error           { return nil }
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
func (m *mockGraph) IsConnected() bool    { return true }
func (m *mockGraph) Errors() <-chan error { return nil }
func (m *mockGraph) HasEmbedding(ctx context.Context, contentHash string, version int) (bool, error) {
	return false, nil
}
func (m *mockGraph) ExportSnapshot(ctx context.Context) (*graph.GraphSnapshot, error) {
	return m.snapshot, nil
}
func (m *mockGraph) GetFileWithRelations(ctx context.Context, path string) (*graph.FileWithRelations, error) {
	// Return sample data for test file path
	if path == "/test/file.go" {
		return &graph.FileWithRelations{
			File: graph.FileNode{
				Path:      "/test/file.go",
				Name:      "file.go",
				Extension: ".go",
				Language:  "go",
				Size:      1024,
				Summary:   "Test file",
			},
			Tags:   []string{"go", "test"},
			Topics: []graph.Topic{{Name: "Testing", Confidence: 0.9}},
			Entities: []graph.Entity{
				{Name: "Go", Type: "language"},
				{Name: "Test", Type: "concept"},
			},
			References: []graph.Reference{
				{Type: "package", Target: "testing"},
			},
			ChunkCount: 5,
		}, nil
	}
	return nil, nil
}
func (m *mockGraph) SearchSimilarChunks(ctx context.Context, embedding []float32, k int) ([]graph.ChunkSearchHit, error) {
	m.lastSearchEmbedding = embedding
	m.lastSearchK = k
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return m.searchHits, nil
}

func TestNewServer(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	cfg := DefaultConfig()

	s := NewServer(g, nil, reg, bus, cfg)

	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.mcpServer == nil {
		t.Error("mcpServer is nil")
	}
	if s.httpServer == nil {
		t.Error("httpServer is nil")
	}
	if s.exporter == nil {
		t.Error("exporter is nil")
	}
	if s.subs == nil {
		t.Error("subs is nil")
	}
	if s.registry == nil {
		t.Error("registry is nil")
	}
	if s.bus == nil {
		t.Error("bus is nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Name != "memorizer" {
		t.Errorf("Name = %q, want %q", cfg.Name, "memorizer")
	}
	// Version should come from version package, not be empty
	if cfg.Version == "" {
		t.Error("Version should not be empty")
	}
	if cfg.BasePath != "/mcp" {
		t.Errorf("BasePath = %q, want %q", cfg.BasePath, "/mcp")
	}
}

func TestServerStartStop(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, nil, reg, bus, DefaultConfig())

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
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, nil, reg, bus, DefaultConfig())

	handler := s.Handler()
	if handler == nil {
		t.Error("Handler returned nil")
	}
}

func TestResourceNotFoundError(t *testing.T) {
	err := &ResourceNotFoundError{URI: "memorizer://invalid"}
	expected := "resource not found: memorizer://invalid"

	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestPathNotRememberedError(t *testing.T) {
	err := &PathNotRememberedError{Path: "/not/remembered"}
	expected := "path not remembered: /not/remembered"

	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestFileNotFoundError(t *testing.T) {
	err := &FileNotFoundError{Path: "/some/file.go"}
	expected := "file not found: /some/file.go"

	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestServer_GracefulDegradation(t *testing.T) {
	// Test that server can be created and started even with nil components
	g := newMockGraph()
	reg := newMockRegistry()
	// nil event bus should not cause panic
	s := NewServer(g, nil, reg, nil, DefaultConfig())

	ctx := context.Background()

	// Start should succeed even without event bus
	err := s.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed with nil bus: %v", err)
	}

	// Server should be running
	if !s.running {
		t.Error("Server should be running after Start")
	}

	// Stop should succeed
	err = s.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestServer_GracefulDegradationNilRegistry(t *testing.T) {
	g := newMockGraph()
	bus := events.NewBus()
	// nil registry should not cause panic during creation
	s := NewServer(g, nil, nil, bus, DefaultConfig())

	if s == nil {
		t.Fatal("NewServer returned nil with nil registry")
	}

	// isPathRemembered should handle nil registry gracefully
	ctx := context.Background()
	if s.isPathRemembered(ctx, "/some/path") {
		t.Error("isPathRemembered should return false with nil registry")
	}
}

func TestResource_ReadIndex(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, nil, reg, bus, DefaultConfig())

	ctx := context.Background()

	tests := []struct {
		name     string
		uri      string
		mimeType string
	}{
		{"default index", ResourceURIIndex, "application/xml"},
		{"xml index", ResourceURIIndexXML, "application/xml"},
		{"json index", ResourceURIIndexJSON, "application/json"},
		{"toon index", ResourceURIIndexTOON, "text/plain"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := mcplib.ReadResourceRequest{
				Params: mcplib.ReadResourceParams{
					URI: tt.uri,
				},
			}

			contents, err := s.handleReadResource(ctx, request)
			if err != nil {
				t.Fatalf("handleReadResource failed: %v", err)
			}

			if len(contents) != 1 {
				t.Fatalf("expected 1 content, got %d", len(contents))
			}

			textContent, ok := contents[0].(mcplib.TextResourceContents)
			if !ok {
				t.Fatalf("expected TextResourceContents, got %T", contents[0])
			}

			if textContent.MIMEType != tt.mimeType {
				t.Errorf("MIMEType = %q, want %q", textContent.MIMEType, tt.mimeType)
			}

			if textContent.URI != tt.uri {
				t.Errorf("URI = %q, want %q", textContent.URI, tt.uri)
			}

			if textContent.Text == "" {
				t.Error("Text content should not be empty")
			}
		})
	}
}

func TestResource_ReadFile(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, nil, reg, bus, DefaultConfig())

	ctx := context.Background()

	// Test reading a remembered file
	uri := ResourceURIFilePrefix + "/test/file.go"
	request := mcplib.ReadResourceRequest{
		Params: mcplib.ReadResourceParams{
			URI: uri,
		},
	}

	contents, err := s.handleReadResource(ctx, request)
	if err != nil {
		t.Fatalf("handleReadResource failed: %v", err)
	}

	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}

	textContent, ok := contents[0].(mcplib.TextResourceContents)
	if !ok {
		t.Fatalf("expected TextResourceContents, got %T", contents[0])
	}

	if textContent.MIMEType != "application/json" {
		t.Errorf("MIMEType = %q, want %q", textContent.MIMEType, "application/json")
	}

	// Verify the response contains expected file data
	if textContent.Text == "" {
		t.Error("Text content should not be empty")
	}

	// Check that JSON contains expected fields
	if !strings.Contains(textContent.Text, "file.go") {
		t.Error("Response should contain file name")
	}
	if !strings.Contains(textContent.Text, "Testing") {
		t.Error("Response should contain topic")
	}
}

func TestResource_ReadFile_NotRemembered(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, nil, reg, bus, DefaultConfig())

	ctx := context.Background()

	// Test reading a file that is not under a remembered path
	uri := ResourceURIFilePrefix + "/not/remembered/file.go"
	request := mcplib.ReadResourceRequest{
		Params: mcplib.ReadResourceParams{
			URI: uri,
		},
	}

	_, err := s.handleReadResource(ctx, request)
	if err == nil {
		t.Fatal("expected error for non-remembered path")
	}

	// Check it's the correct error type
	if _, ok := err.(*PathNotRememberedError); !ok {
		t.Errorf("expected PathNotRememberedError, got %T: %v", err, err)
	}
}

func TestResource_ReadFile_NotFound(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, nil, reg, bus, DefaultConfig())

	ctx := context.Background()

	// Test reading a remembered file that doesn't exist in graph
	// /test is remembered but /test/nonexistent.go returns nil from mockGraph
	uri := ResourceURIFilePrefix + "/test/nonexistent.go"
	request := mcplib.ReadResourceRequest{
		Params: mcplib.ReadResourceParams{
			URI: uri,
		},
	}

	_, err := s.handleReadResource(ctx, request)
	if err == nil {
		t.Fatal("expected error for file not found in graph")
	}

	// Check it's the correct error type
	if _, ok := err.(*ResourceNotFoundError); !ok {
		t.Errorf("expected ResourceNotFoundError, got %T: %v", err, err)
	}
}

func TestResource_InvalidURI(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, nil, reg, bus, DefaultConfig())

	ctx := context.Background()

	tests := []struct {
		name string
		uri  string
	}{
		{"unknown scheme", "unknown://resource"},
		{"invalid memorizer resource", "memorizer://invalid"},
		{"empty uri", ""},
		{"random string", "not-a-uri"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := mcplib.ReadResourceRequest{
				Params: mcplib.ReadResourceParams{
					URI: tt.uri,
				},
			}

			_, err := s.handleReadResource(ctx, request)
			if err == nil {
				t.Errorf("expected error for URI %q", tt.uri)
			}
		})
	}
}

func TestResource_FileEmptyPath(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, nil, reg, bus, DefaultConfig())

	ctx := context.Background()

	// Test reading file resource with empty path (just the prefix)
	uri := ResourceURIFilePrefix
	request := mcplib.ReadResourceRequest{
		Params: mcplib.ReadResourceParams{
			URI: uri,
		},
	}

	_, err := s.handleReadResource(ctx, request)
	if err == nil {
		t.Fatal("expected error for empty file path")
	}

	if _, ok := err.(*ResourceNotFoundError); !ok {
		t.Errorf("expected ResourceNotFoundError, got %T: %v", err, err)
	}
}

func TestSubscription_Subscribe(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, nil, reg, bus, DefaultConfig())

	// Test subscribing to resources
	subscriber := &Subscriber{
		ID:        "test-subscriber",
		SessionID: "test-session",
	}

	// Initially no subscribers
	if s.subs.HasSubscribers(ResourceURIIndex) {
		t.Error("should have no subscribers initially")
	}

	// Subscribe
	s.subs.Subscribe(ResourceURIIndex, subscriber)

	// Now should have subscribers
	if !s.subs.HasSubscribers(ResourceURIIndex) {
		t.Error("should have subscribers after subscribe")
	}

	// Verify subscriber info
	subs := s.subs.GetSubscribers(ResourceURIIndex)
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscriber, got %d", len(subs))
	}

	if subs[0].ID != "test-subscriber" {
		t.Errorf("subscriber ID = %q, want %q", subs[0].ID, "test-subscriber")
	}

	// Unsubscribe
	s.subs.Unsubscribe(ResourceURIIndex, "test-subscriber")

	if s.subs.HasSubscribers(ResourceURIIndex) {
		t.Error("should have no subscribers after unsubscribe")
	}
}

func TestServer_isPathRemembered(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, nil, reg, bus, DefaultConfig())

	ctx := context.Background()

	tests := []struct {
		path       string
		remembered bool
	}{
		{"/test", true},
		{"/test/file.go", true},
		{"/test/subdir/file.go", true},
		{"/other", false},
		{"/not/remembered", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := s.isPathRemembered(ctx, tt.path)
			if result != tt.remembered {
				t.Errorf("isPathRemembered(%q) = %v, want %v", tt.path, result, tt.remembered)
			}
		})
	}
}

func TestNotifyResourceChanged(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, nil, reg, bus, DefaultConfig())

	// This should not panic even if no clients are connected
	s.NotifyResourceChanged(ResourceURIIndex)
	s.NotifyResourceChanged(ResourceURIFilePrefix + "/test/file.go")
}

func TestServer_MultipleStartStop(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, nil, reg, bus, DefaultConfig())

	ctx := context.Background()

	// Start and stop multiple times should not cause issues
	for i := 0; i < 3; i++ {
		err := s.Start(ctx)
		if err != nil {
			t.Fatalf("Start iteration %d failed: %v", i, err)
		}

		if !s.running {
			t.Errorf("Server should be running after Start (iteration %d)", i)
		}

		err = s.Stop(ctx)
		if err != nil {
			t.Fatalf("Stop iteration %d failed: %v", i, err)
		}

		if s.running {
			t.Errorf("Server should not be running after Stop (iteration %d)", i)
		}
	}
}
