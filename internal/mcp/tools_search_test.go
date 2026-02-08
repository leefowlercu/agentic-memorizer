package mcp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/providers"
)

type mockEmbeddingsProvider struct {
	available bool
	embedding []float32
	err       error
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
	if m.err != nil {
		return nil, m.err
	}
	return &providers.EmbeddingsResult{Embedding: m.embedding, Dimensions: len(m.embedding)}, nil
}
func (m *mockEmbeddingsProvider) EmbedBatch(ctx context.Context, texts []string) ([]providers.EmbeddingsBatchResult, error) {
	return nil, nil
}

func TestSearchMemoryToolRegistered(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	provider := &mockEmbeddingsProvider{
		available: true,
		embedding: []float32{0.1, 0.2},
	}
	s := NewServer(g, provider, reg, bus, DefaultConfig())

	tools := s.mcpServer.ListTools()
	if tools == nil {
		t.Fatal("expected tools to be registered")
	}
	if _, ok := tools[toolSearchMemory]; !ok {
		t.Fatalf("expected %q tool to be registered", toolSearchMemory)
	}
}

func TestSearchMemoryToolSuccessWithFiltersAndSnippets(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "service.go")
	content := "package main\n\nfunc SearchMemory() {}\n"
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	g := newMockGraph()
	g.searchHits = []graph.ChunkSearchHit{
		{
			Chunk: graph.ChunkNode{
				ID:          "chunk-go",
				FilePath:    filePath,
				Index:       0,
				ContentHash: "hash-go",
				StartOffset: 0,
				EndOffset:   len(content),
				ChunkType:   "code",
				Summary:     "Go handler for semantic search",
			},
			Score:    0.91,
			Provider: "openai-embeddings",
			Model:    "text-embedding-3-large",
		},
		{
			Chunk: graph.ChunkNode{
				ID:          "chunk-md",
				FilePath:    filepath.Join(tmpDir, "notes.md"),
				Index:       0,
				ContentHash: "hash-md",
				StartOffset: 0,
				EndOffset:   10,
				ChunkType:   "markdown",
				Summary:     "Should be filtered by extension",
			},
			Score: 0.95,
		},
	}

	reg := &mockRegistry{
		rememberedPaths: map[string]bool{
			tmpDir: true,
		},
	}
	provider := &mockEmbeddingsProvider{
		available: true,
		embedding: []float32{0.3, 0.4, 0.5},
	}
	s := NewServer(g, provider, reg, events.NewBus(), DefaultConfig())

	request := mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name: toolSearchMemory,
			Arguments: map[string]any{
				"query":              "find semantic search handler",
				"top_k":              5,
				"min_score":          0.8,
				"path_prefix":        tmpDir,
				"include_extensions": []string{".go"},
				"include_snippets":   true,
				"snippet_max_chars":  50,
			},
		},
	}

	result, err := s.handleSearchMemory(context.Background(), request)
	if err != nil {
		t.Fatalf("handleSearchMemory returned protocol error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected successful tool result, got error: %s", mcplib.GetTextFromContent(result.Content))
	}

	typed, ok := result.StructuredContent.(searchMemoryResult)
	if !ok {
		t.Fatalf("expected structured content type %T, got %T", searchMemoryResult{}, result.StructuredContent)
	}
	if typed.Returned != 1 {
		t.Fatalf("expected 1 hit, got %d", typed.Returned)
	}
	if len(typed.Hits) != 1 {
		t.Fatalf("expected one hit, got %d", len(typed.Hits))
	}
	if typed.Hits[0].Snippet == "" {
		t.Fatal("expected snippet to be populated")
	}
	if typed.Hits[0].FilePath != filePath {
		t.Fatalf("unexpected file path %q", typed.Hits[0].FilePath)
	}
	if g.lastSearchK != 100 {
		t.Fatalf("expected overfetch candidate_k=100, got %d", g.lastSearchK)
	}
	if len(g.lastSearchEmbedding) != 3 {
		t.Fatalf("expected query embedding length 3, got %d", len(g.lastSearchEmbedding))
	}
}

func TestSearchMemoryToolProviderUnavailable(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	s := NewServer(g, nil, reg, events.NewBus(), DefaultConfig())

	request := mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name: toolSearchMemory,
			Arguments: map[string]any{
				"query": "something",
			},
		},
	}

	result, err := s.handleSearchMemory(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error result")
	}
	if !strings.Contains(strings.ToLower(mcplib.GetTextFromContent(result.Content)), "embeddings provider") {
		t.Fatalf("unexpected error text: %q", mcplib.GetTextFromContent(result.Content))
	}
}

func TestSearchMemoryToolMinScoreValidation(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	provider := &mockEmbeddingsProvider{available: true, embedding: []float32{0.1}}
	s := NewServer(g, provider, reg, events.NewBus(), DefaultConfig())

	request := mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name: toolSearchMemory,
			Arguments: map[string]any{
				"query":     "invalid score",
				"min_score": 1.5,
			},
		},
	}

	result, err := s.handleSearchMemory(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error result")
	}
	if !strings.Contains(strings.ToLower(mcplib.GetTextFromContent(result.Content)), "min_score") {
		t.Fatalf("expected min_score error, got: %q", mcplib.GetTextFromContent(result.Content))
	}
}

func TestSearchMemoryToolGraphFailure(t *testing.T) {
	g := newMockGraph()
	g.searchErr = errors.New("db timeout")
	reg := newMockRegistry()
	provider := &mockEmbeddingsProvider{available: true, embedding: []float32{0.1}}
	s := NewServer(g, provider, reg, events.NewBus(), DefaultConfig())

	request := mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name: toolSearchMemory,
			Arguments: map[string]any{
				"query": "failure case",
			},
		},
	}

	result, err := s.handleSearchMemory(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error result")
	}
	if !strings.Contains(strings.ToLower(mcplib.GetTextFromContent(result.Content)), "semantic search failed") {
		t.Fatalf("unexpected error text: %q", mcplib.GetTextFromContent(result.Content))
	}
}
