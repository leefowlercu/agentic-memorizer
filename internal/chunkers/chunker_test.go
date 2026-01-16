package chunkers

import (
	"context"
	"fmt"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{"empty string", "", 0},
		{"short text", "hello", 1},                                                                                                                      // tiktoken: 1 token
		{"medium text", "hello world, this is a test", 7},                                                                                               // tiktoken: 7 tokens
		{"long text", "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.", 22}, // tiktoken: 22 tokens
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EstimateTokens(tt.text)
			if result != tt.expected {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tt.text, result, tt.expected)
			}
		})
	}
}

func TestDefaultChunkOptions(t *testing.T) {
	opts := DefaultChunkOptions()

	if opts.MaxChunkSize != 8000 {
		t.Errorf("MaxChunkSize = %d, want 8000", opts.MaxChunkSize)
	}
	if opts.MaxTokens != 2000 {
		t.Errorf("MaxTokens = %d, want 2000", opts.MaxTokens)
	}
	if opts.Overlap != 200 {
		t.Errorf("Overlap = %d, want 200", opts.Overlap)
	}
	if !opts.PreserveStructure {
		t.Error("PreserveStructure should be true by default")
	}
}

func TestFallbackChunker(t *testing.T) {
	chunker := NewFallbackChunker()

	t.Run("Name", func(t *testing.T) {
		if chunker.Name() != "fallback" {
			t.Errorf("Name() = %q, want %q", chunker.Name(), "fallback")
		}
	})

	t.Run("Priority", func(t *testing.T) {
		if chunker.Priority() != 0 {
			t.Errorf("Priority() = %d, want 0", chunker.Priority())
		}
	})

	t.Run("CanHandle", func(t *testing.T) {
		if !chunker.CanHandle("any/type", "any") {
			t.Error("CanHandle should return true for any content")
		}
	})

	t.Run("EmptyContent", func(t *testing.T) {
		result, err := chunker.Chunk(context.Background(), []byte{}, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) != 0 {
			t.Errorf("Expected 0 chunks for empty content, got %d", len(result.Chunks))
		}
	})

	t.Run("SmallContent", func(t *testing.T) {
		content := []byte("Hello, world!")
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) != 1 {
			t.Errorf("Expected 1 chunk for small content, got %d", len(result.Chunks))
		}
		if result.Chunks[0].Content != "Hello, world!" {
			t.Errorf("Content = %q, want %q", result.Chunks[0].Content, "Hello, world!")
		}
	})

	t.Run("LargeContent", func(t *testing.T) {
		// Create content larger than max chunk size
		opts := ChunkOptions{MaxChunkSize: 100, Overlap: 10}
		content := make([]byte, 500)
		for i := range content {
			content[i] = 'a'
		}

		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) < 2 {
			t.Errorf("Expected multiple chunks, got %d", len(result.Chunks))
		}
	})
}

func TestRecursiveChunker(t *testing.T) {
	chunker := NewRecursiveChunker()

	t.Run("Name", func(t *testing.T) {
		if chunker.Name() != "recursive" {
			t.Errorf("Name() = %q, want %q", chunker.Name(), "recursive")
		}
	})

	t.Run("Priority", func(t *testing.T) {
		if chunker.Priority() != 10 {
			t.Errorf("Priority() = %d, want 10", chunker.Priority())
		}
	})

	t.Run("CanHandle", func(t *testing.T) {
		tests := []struct {
			mimeType string
			language string
			expected bool
		}{
			{"text/plain", "", true},
			{"", "", true},
			{"text/html", "", false},
			{"application/json", "", false},
		}

		for _, tt := range tests {
			result := chunker.CanHandle(tt.mimeType, tt.language)
			if result != tt.expected {
				t.Errorf("CanHandle(%q, %q) = %v, want %v", tt.mimeType, tt.language, result, tt.expected)
			}
		}
	})

	t.Run("ParagraphSplitting", func(t *testing.T) {
		content := []byte("First paragraph.\n\nSecond paragraph.\n\nThird paragraph.")
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
	})
}

func TestMarkdownChunker(t *testing.T) {
	chunker := NewMarkdownChunker()

	t.Run("Name", func(t *testing.T) {
		if chunker.Name() != "markdown" {
			t.Errorf("Name() = %q, want %q", chunker.Name(), "markdown")
		}
	})

	t.Run("Priority", func(t *testing.T) {
		if chunker.Priority() != 50 {
			t.Errorf("Priority() = %d, want 50", chunker.Priority())
		}
	})

	t.Run("CanHandle", func(t *testing.T) {
		tests := []struct {
			mimeType string
			language string
			expected bool
		}{
			{"text/markdown", "", true},
			{"text/x-markdown", "", true},
			{"", "file.md", true},
			{"text/plain", "", false},
		}

		for _, tt := range tests {
			result := chunker.CanHandle(tt.mimeType, tt.language)
			if result != tt.expected {
				t.Errorf("CanHandle(%q, %q) = %v, want %v", tt.mimeType, tt.language, result, tt.expected)
			}
		}
	})

	t.Run("HeadingSplitting", func(t *testing.T) {
		content := []byte(`# Heading 1

Content under heading 1.

## Heading 2

Content under heading 2.

## Heading 3

Content under heading 3.
`)
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) < 2 {
			t.Errorf("Expected multiple chunks for headings, got %d", len(result.Chunks))
		}

		// Check that metadata includes heading info
		for _, chunk := range result.Chunks {
			if chunk.Metadata.Type != ChunkTypeMarkdown {
				t.Errorf("Expected ChunkTypeMarkdown, got %v", chunk.Metadata.Type)
			}
		}
	})
}

func TestStructuredChunker(t *testing.T) {
	chunker := NewStructuredChunker()

	t.Run("Name", func(t *testing.T) {
		if chunker.Name() != "structured" {
			t.Errorf("Name() = %q, want %q", chunker.Name(), "structured")
		}
	})

	t.Run("Priority", func(t *testing.T) {
		if chunker.Priority() != 40 {
			t.Errorf("Priority() = %d, want 40", chunker.Priority())
		}
	})

	t.Run("CanHandle", func(t *testing.T) {
		tests := []struct {
			mimeType string
			expected bool
		}{
			{"application/json", true},
			{"text/json", true},
			{"text/csv", true},
			{"text/yaml", true},
			{"text/plain", false},
		}

		for _, tt := range tests {
			result := chunker.CanHandle(tt.mimeType, "")
			if result != tt.expected {
				t.Errorf("CanHandle(%q) = %v, want %v", tt.mimeType, result, tt.expected)
			}
		}
	})

	t.Run("JSONArray", func(t *testing.T) {
		content := []byte(`[{"id": 1}, {"id": 2}, {"id": 3}]`)
		opts := ChunkOptions{
			MIMEType:     "application/json",
			MaxChunkSize: 1000,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
	})
}

func TestRegistry(t *testing.T) {
	t.Run("NewRegistry", func(t *testing.T) {
		registry := NewRegistry()
		if registry == nil {
			t.Error("NewRegistry returned nil")
		}
		if len(registry.List()) != 0 {
			t.Errorf("New registry should be empty, got %d chunkers", len(registry.List()))
		}
	})

	t.Run("Register", func(t *testing.T) {
		registry := NewRegistry()
		registry.Register(NewFallbackChunker())
		registry.Register(NewMarkdownChunker())

		chunkers := registry.List()
		if len(chunkers) != 2 {
			t.Errorf("Expected 2 chunkers, got %d", len(chunkers))
		}

		// Verify priority ordering
		if chunkers[0].Priority() < chunkers[1].Priority() {
			t.Error("Chunkers should be sorted by priority (highest first)")
		}
	})

	t.Run("SetFallback", func(t *testing.T) {
		registry := NewRegistry()
		registry.SetFallback(NewFallbackChunker())

		chunker := registry.Get("unknown/type", "")
		if chunker == nil {
			t.Error("Expected fallback chunker")
		}
		if chunker.Name() != "fallback" {
			t.Errorf("Expected fallback chunker, got %q", chunker.Name())
		}
	})

	t.Run("Get", func(t *testing.T) {
		registry := NewRegistry()
		registry.Register(NewMarkdownChunker())
		registry.Register(NewRecursiveChunker())
		registry.SetFallback(NewFallbackChunker())

		tests := []struct {
			mimeType     string
			language     string
			expectedName string
		}{
			{"text/markdown", "", "markdown"},
			{"text/plain", "", "recursive"},
			{"unknown/type", "", "fallback"},
		}

		for _, tt := range tests {
			chunker := registry.Get(tt.mimeType, tt.language)
			if chunker == nil {
				t.Errorf("Get(%q, %q) returned nil", tt.mimeType, tt.language)
				continue
			}
			if chunker.Name() != tt.expectedName {
				t.Errorf("Get(%q, %q) = %q, want %q", tt.mimeType, tt.language, chunker.Name(), tt.expectedName)
			}
		}
	})

	t.Run("DefaultRegistry", func(t *testing.T) {
		registry := DefaultRegistry()
		if registry == nil {
			t.Error("DefaultRegistry returned nil")
		}

		chunkers := registry.List()
		if len(chunkers) < 4 {
			t.Errorf("DefaultRegistry should have at least 4 chunkers, got %d", len(chunkers))
		}

		// Verify fallback is set
		fallback := registry.Get("unknown/type", "")
		if fallback == nil {
			t.Error("DefaultRegistry should have a fallback")
		}
	})

	t.Run("ChunkThroughRegistry", func(t *testing.T) {
		registry := DefaultRegistry()
		content := []byte("Hello, world!")
		opts := ChunkOptions{
			MIMEType:     "text/plain",
			MaxChunkSize: 8000,
		}

		result, err := registry.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}
		if result == nil {
			t.Error("Chunk returned nil result")
		}
		if result.TotalChunks == 0 {
			t.Error("Expected at least one chunk")
		}
	})

	t.Run("GracefulDegradation", func(t *testing.T) {
		registry := NewRegistry()
		// Register a failing chunker with high priority
		registry.Register(&failingChunker{priority: 100})
		// Register a working chunker with lower priority
		registry.Register(NewRecursiveChunker())
		registry.SetFallback(NewFallbackChunker())

		content := []byte("Hello, world!")
		opts := ChunkOptions{
			MIMEType:     "text/plain",
			MaxChunkSize: 8000,
		}

		result, err := registry.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Errorf("Chunk should have degraded gracefully, got error: %v", err)
		}
		if result == nil {
			t.Fatal("Chunk returned nil result")
		}

		// Should have used recursive chunker after failing chunker failed
		if result.ChunkerUsed != "recursive" {
			t.Errorf("Expected recursive chunker, got %q", result.ChunkerUsed)
		}
	})

	t.Run("WarningAggregation", func(t *testing.T) {
		registry := NewRegistry()
		// Register a failing chunker
		registry.Register(&failingChunker{priority: 100})
		// Register a working chunker
		registry.Register(NewRecursiveChunker())

		content := []byte("Hello, world!")
		opts := ChunkOptions{
			MIMEType:     "text/plain",
			MaxChunkSize: 8000,
		}

		result, err := registry.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}

		// Should have warning about the failed chunker
		if len(result.Warnings) == 0 {
			t.Error("Expected warnings about failed chunker")
		}

		foundWarning := false
		for _, w := range result.Warnings {
			if w.Code == "CHUNKER_FAILED" {
				foundWarning = true
				break
			}
		}
		if !foundWarning {
			t.Error("Expected CHUNKER_FAILED warning")
		}
	})

	t.Run("AllChunkersFail", func(t *testing.T) {
		registry := NewRegistry()
		// Register only failing chunkers
		registry.Register(&failingChunker{priority: 100})
		registry.Register(&failingChunker{priority: 50})
		// No fallback set

		content := []byte("Hello, world!")
		opts := ChunkOptions{
			MIMEType:     "text/plain",
			MaxChunkSize: 8000,
		}

		_, err := registry.Chunk(context.Background(), content, opts)
		if err == nil {
			t.Error("Expected error when all chunkers fail")
		}
	})

	t.Run("FallbackUsedAfterAllFail", func(t *testing.T) {
		registry := NewRegistry()
		// Register only failing chunkers
		registry.Register(&failingChunker{priority: 100})
		// Set working fallback
		registry.SetFallback(NewFallbackChunker())

		content := []byte("Hello, world!")
		opts := ChunkOptions{
			MIMEType:     "text/plain",
			MaxChunkSize: 8000,
		}

		result, err := registry.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Errorf("Chunk should have used fallback, got error: %v", err)
		}
		if result == nil {
			t.Fatal("Chunk returned nil result")
		}

		// Should have used fallback chunker
		if result.ChunkerUsed != "fallback" {
			t.Errorf("Expected fallback chunker, got %q", result.ChunkerUsed)
		}

		// Should have warning about the failed chunker
		if len(result.Warnings) == 0 {
			t.Error("Expected warnings about failed chunker")
		}
	})
}

// failingChunker is a test chunker that always fails.
type failingChunker struct {
	priority int
}

func (f *failingChunker) Name() string { return "failing" }
func (f *failingChunker) CanHandle(mimeType string, language string) bool {
	return mimeType == "text/plain" || mimeType == ""
}
func (f *failingChunker) Priority() int { return f.priority }
func (f *failingChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	return nil, fmt.Errorf("intentional failure for testing")
}
