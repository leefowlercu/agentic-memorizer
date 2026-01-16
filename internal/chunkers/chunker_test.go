package chunkers

import (
	"context"
	"fmt"
	"sync"
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

func TestChunkWarningConstruction(t *testing.T) {
	t.Run("basic construction", func(t *testing.T) {
		warning := ChunkWarning{
			Offset:  100,
			Message: "Invalid syntax at position",
			Code:    "INVALID_SYNTAX",
		}

		if warning.Offset != 100 {
			t.Errorf("Offset = %d, want 100", warning.Offset)
		}
		if warning.Message != "Invalid syntax at position" {
			t.Errorf("Message = %q, want %q", warning.Message, "Invalid syntax at position")
		}
		if warning.Code != "INVALID_SYNTAX" {
			t.Errorf("Code = %q, want %q", warning.Code, "INVALID_SYNTAX")
		}
	})

	t.Run("zero offset is valid", func(t *testing.T) {
		warning := ChunkWarning{
			Offset:  0,
			Message: "Issue at start",
			Code:    "START_ISSUE",
		}

		if warning.Offset != 0 {
			t.Errorf("Offset = %d, want 0", warning.Offset)
		}
	})

	t.Run("empty fields are valid", func(t *testing.T) {
		warning := ChunkWarning{}

		if warning.Offset != 0 {
			t.Errorf("Offset = %d, want 0", warning.Offset)
		}
		if warning.Message != "" {
			t.Errorf("Message = %q, want empty", warning.Message)
		}
		if warning.Code != "" {
			t.Errorf("Code = %q, want empty", warning.Code)
		}
	})
}

func TestChunkResultConstruction(t *testing.T) {
	t.Run("basic construction", func(t *testing.T) {
		result := ChunkResult{
			Chunks: []Chunk{
				{Index: 0, Content: "Hello"},
				{Index: 1, Content: "World"},
			},
			Warnings:     []ChunkWarning{},
			TotalChunks:  2,
			ChunkerUsed:  "test-chunker",
			OriginalSize: 11,
		}

		if len(result.Chunks) != 2 {
			t.Errorf("len(Chunks) = %d, want 2", len(result.Chunks))
		}
		if result.TotalChunks != 2 {
			t.Errorf("TotalChunks = %d, want 2", result.TotalChunks)
		}
		if result.ChunkerUsed != "test-chunker" {
			t.Errorf("ChunkerUsed = %q, want %q", result.ChunkerUsed, "test-chunker")
		}
		if result.OriginalSize != 11 {
			t.Errorf("OriginalSize = %d, want 11", result.OriginalSize)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		result := ChunkResult{}

		if result.Chunks != nil {
			t.Error("Chunks should be nil for empty result")
		}
		if result.TotalChunks != 0 {
			t.Errorf("TotalChunks = %d, want 0", result.TotalChunks)
		}
	})

	t.Run("nil chunks vs empty chunks", func(t *testing.T) {
		nilResult := ChunkResult{Chunks: nil}
		emptyResult := ChunkResult{Chunks: []Chunk{}}

		if nilResult.Chunks != nil {
			t.Error("nilResult.Chunks should be nil")
		}
		if emptyResult.Chunks == nil {
			t.Error("emptyResult.Chunks should not be nil")
		}
		if len(emptyResult.Chunks) != 0 {
			t.Errorf("len(emptyResult.Chunks) = %d, want 0", len(emptyResult.Chunks))
		}
	})

	t.Run("with warnings", func(t *testing.T) {
		result := ChunkResult{
			Chunks: []Chunk{{Index: 0, Content: "Content"}},
			Warnings: []ChunkWarning{
				{Offset: 0, Message: "Warning 1", Code: "WARN1"},
				{Offset: 50, Message: "Warning 2", Code: "WARN2"},
			},
			TotalChunks:  1,
			ChunkerUsed:  "test",
			OriginalSize: 100,
		}

		if len(result.Warnings) != 2 {
			t.Errorf("len(Warnings) = %d, want 2", len(result.Warnings))
		}
		if result.Warnings[0].Code != "WARN1" {
			t.Errorf("Warnings[0].Code = %q, want %q", result.Warnings[0].Code, "WARN1")
		}
	})
}

func TestChunkConstruction(t *testing.T) {
	t.Run("basic construction", func(t *testing.T) {
		chunk := Chunk{
			Index:       0,
			Content:     "Test content",
			StartOffset: 0,
			EndOffset:   12,
			Metadata: ChunkMetadata{
				Type:          ChunkTypeCode,
				TokenEstimate: 5,
			},
		}

		if chunk.Index != 0 {
			t.Errorf("Index = %d, want 0", chunk.Index)
		}
		if chunk.Content != "Test content" {
			t.Errorf("Content = %q, want %q", chunk.Content, "Test content")
		}
		if chunk.StartOffset != 0 {
			t.Errorf("StartOffset = %d, want 0", chunk.StartOffset)
		}
		if chunk.EndOffset != 12 {
			t.Errorf("EndOffset = %d, want 12", chunk.EndOffset)
		}
	})

	t.Run("offset consistency", func(t *testing.T) {
		chunk := Chunk{
			Index:       5,
			Content:     "Content",
			StartOffset: 100,
			EndOffset:   107,
		}

		contentLen := len(chunk.Content)
		offsetLen := chunk.EndOffset - chunk.StartOffset

		if contentLen != offsetLen {
			t.Errorf("Content length (%d) != offset range (%d)", contentLen, offsetLen)
		}
	})

	t.Run("empty content", func(t *testing.T) {
		chunk := Chunk{
			Index:       0,
			Content:     "",
			StartOffset: 0,
			EndOffset:   0,
		}

		if chunk.Content != "" {
			t.Errorf("Content = %q, want empty", chunk.Content)
		}
		if chunk.StartOffset != chunk.EndOffset {
			t.Error("StartOffset should equal EndOffset for empty content")
		}
	})
}

func TestChunkOptionsEdgeCases(t *testing.T) {
	t.Run("zero MaxChunkSize", func(t *testing.T) {
		opts := ChunkOptions{
			MaxChunkSize: 0,
			MaxTokens:    100,
			Overlap:      10,
		}

		if opts.MaxChunkSize != 0 {
			t.Errorf("MaxChunkSize = %d, want 0", opts.MaxChunkSize)
		}
	})

	t.Run("zero overlap", func(t *testing.T) {
		opts := ChunkOptions{
			MaxChunkSize: 1000,
			MaxTokens:    100,
			Overlap:      0,
		}

		if opts.Overlap != 0 {
			t.Errorf("Overlap = %d, want 0", opts.Overlap)
		}
	})

	t.Run("all optional fields set", func(t *testing.T) {
		opts := ChunkOptions{
			MaxChunkSize:      4000,
			MaxTokens:         1000,
			Overlap:           100,
			Language:          "python",
			MIMEType:          "text/x-python",
			PreserveStructure: true,
		}

		if opts.Language != "python" {
			t.Errorf("Language = %q, want %q", opts.Language, "python")
		}
		if opts.MIMEType != "text/x-python" {
			t.Errorf("MIMEType = %q, want %q", opts.MIMEType, "text/x-python")
		}
		if !opts.PreserveStructure {
			t.Error("PreserveStructure should be true")
		}
	})

	t.Run("overlap larger than MaxChunkSize", func(t *testing.T) {
		// This is a technically invalid configuration but shouldn't panic
		opts := ChunkOptions{
			MaxChunkSize: 100,
			Overlap:      200,
		}

		if opts.Overlap <= opts.MaxChunkSize {
			t.Error("Test setup error: Overlap should be larger than MaxChunkSize")
		}
	})
}

func TestContextCancellation(t *testing.T) {
	chunker := NewFallbackChunker()

	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := chunker.Chunk(ctx, []byte("Hello, world!"), DefaultChunkOptions())
		if err == nil {
			// Some chunkers may not check context, which is acceptable
			// The test just ensures no panic occurs
		}
	})
}

func TestChunkTypeValues(t *testing.T) {
	t.Run("type constants are distinct", func(t *testing.T) {
		types := []ChunkType{
			ChunkTypeCode,
			ChunkTypeMarkdown,
			ChunkTypeProse,
			ChunkTypeStructured,
			ChunkTypeUnknown,
		}

		seen := make(map[ChunkType]bool)
		for _, ct := range types {
			if seen[ct] {
				t.Errorf("Duplicate ChunkType value: %q", ct)
			}
			seen[ct] = true
		}
	})

	t.Run("type constants are non-empty", func(t *testing.T) {
		types := []ChunkType{
			ChunkTypeCode,
			ChunkTypeMarkdown,
			ChunkTypeProse,
			ChunkTypeStructured,
			ChunkTypeUnknown,
		}

		for _, ct := range types {
			if ct == "" {
				t.Error("ChunkType constant should not be empty")
			}
		}
	})
}

// ============================================================================
// Phase 1 Edge Case Tests - Markdown Chunker
// ============================================================================

func TestMarkdownChunkerEdgeCases(t *testing.T) {
	chunker := NewMarkdownChunker()

	t.Run("headings inside code blocks not split", func(t *testing.T) {
		content := []byte("# Real Heading\n\nSome text.\n\n```markdown\n# This is not a heading\n## Neither is this\n```\n\nMore text after code block.")
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		// Should NOT split on headings inside code blocks
		// The code block content should be kept together with surrounding content
		if len(result.Chunks) > 2 {
			t.Errorf("Should not split on headings inside code blocks, got %d chunks", len(result.Chunks))
		}
	})

	t.Run("consecutive headings without content", func(t *testing.T) {
		content := []byte("# Heading 1\n## Heading 2\n### Heading 3\n\nActual content here.")
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		// Should handle consecutive headings gracefully
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
		// Verify no empty chunks
		for i, chunk := range result.Chunks {
			if len(chunk.Content) == 0 {
				t.Errorf("Chunk %d is empty", i)
			}
		}
	})

	t.Run("unicode in headings", func(t *testing.T) {
		content := []byte("# 你好世界\n\n中文内容\n\n## Émoji Test 🎉\n\nContent with émojis.")
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) == 0 {
			t.Fatal("Expected at least one chunk")
		}
		// Verify heading extraction works with unicode
		foundUnicode := false
		for _, chunk := range result.Chunks {
			if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading == "你好世界" {
				foundUnicode = true
				break
			}
		}
		if !foundUnicode {
			t.Error("Unicode heading not extracted correctly")
		}
	})

	t.Run("more than 6 hashes not a heading", func(t *testing.T) {
		content := []byte("####### This is not a heading\n\nRegular content.")
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) != 1 {
			t.Errorf("Expected 1 chunk (7 hashes is not a heading), got %d", len(result.Chunks))
		}
		// Heading should be empty since ####### is not valid
		if result.Chunks[0].Metadata.Document != nil && result.Chunks[0].Metadata.Document.HeadingLevel != 0 {
			t.Error("####### should not be recognized as a heading")
		}
	})

	t.Run("heading at end of file without newline", func(t *testing.T) {
		content := []byte("# Introduction\n\nSome content.\n\n## Final Heading")
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		// Should handle content that doesn't end with newline
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
	})

	t.Run("very large section splitting", func(t *testing.T) {
		// Create content with a single section larger than max chunk size
		largeContent := "# Large Section\n\n"
		for i := 0; i < 200; i++ {
			largeContent += fmt.Sprintf("This is paragraph %d with some content to make it longer.\n\n", i)
		}
		opts := ChunkOptions{
			MaxChunkSize: 1000,
			MaxTokens:    250,
		}
		result, err := chunker.Chunk(context.Background(), []byte(largeContent), opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		// Should split the large section
		if len(result.Chunks) < 2 {
			t.Errorf("Expected multiple chunks for large section, got %d", len(result.Chunks))
		}
		// All chunks should preserve the heading info
		for _, chunk := range result.Chunks {
			if chunk.Metadata.Document == nil {
				t.Error("Chunk missing document metadata")
			}
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		largeContent := make([]byte, 10000)
		for i := range largeContent {
			largeContent[i] = 'a'
		}

		_, err := chunker.Chunk(ctx, largeContent, ChunkOptions{MaxChunkSize: 100})
		if err == nil {
			// Context cancellation may or may not be checked depending on content
			// Just ensure no panic
		}
	})

	t.Run("empty heading text", func(t *testing.T) {
		content := []byte("# \n\nContent under empty heading.")
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		// Should handle empty heading gracefully
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
	})

	t.Run("whitespace-only content", func(t *testing.T) {
		content := []byte("   \n\n\t\t\n   ")
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		// Whitespace-only should result in no chunks (trimmed)
		if len(result.Chunks) != 0 {
			t.Errorf("Expected 0 chunks for whitespace-only content, got %d", len(result.Chunks))
		}
	})

	t.Run("heading with trailing spaces", func(t *testing.T) {
		content := []byte("# Heading With Spaces   \n\nContent here.")
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) == 0 {
			t.Fatal("Expected at least one chunk")
		}
		if result.Chunks[0].Metadata.Document == nil {
			t.Fatal("Missing document metadata")
		}
		// Heading should be trimmed
		if result.Chunks[0].Metadata.Document.Heading != "Heading With Spaces" {
			t.Errorf("Heading = %q, expected trailing spaces trimmed", result.Chunks[0].Metadata.Document.Heading)
		}
	})
}

// ============================================================================
// Phase 1 Edge Case Tests - Structured Chunker
// ============================================================================

func TestStructuredChunkerEdgeCases(t *testing.T) {
	chunker := NewStructuredChunker()

	t.Run("empty JSON array", func(t *testing.T) {
		content := []byte("[]")
		opts := ChunkOptions{
			MIMEType:     "application/json",
			MaxChunkSize: 1000,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		// Empty array should produce no chunks or one empty chunk
		if len(result.Chunks) > 1 {
			t.Errorf("Expected at most 1 chunk for empty array, got %d", len(result.Chunks))
		}
	})

	t.Run("empty JSON object", func(t *testing.T) {
		content := []byte("{}")
		opts := ChunkOptions{
			MIMEType:     "application/json",
			MaxChunkSize: 1000,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) != 1 {
			t.Errorf("Expected 1 chunk for empty object, got %d", len(result.Chunks))
		}
	})

	t.Run("deeply nested JSON", func(t *testing.T) {
		content := []byte(`{"a":{"b":{"c":{"d":{"e":"deep value"}}}}}`)
		opts := ChunkOptions{
			MIMEType:     "application/json",
			MaxChunkSize: 1000,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
	})

	t.Run("JSON with unicode and special characters", func(t *testing.T) {
		content := []byte(`{"message": "你好", "emoji": "🎉", "escaped": "line1\nline2\ttab"}`)
		opts := ChunkOptions{
			MIMEType:     "application/json",
			MaxChunkSize: 1000,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
	})

	t.Run("JSON primitive values", func(t *testing.T) {
		// Primitive JSON values that should produce 1 chunk
		// Note: `null` is special - it unmarshals to nil array in Go, producing 0 chunks
		primitives := []string{`"string"`, `42`, `true`}
		for _, p := range primitives {
			opts := ChunkOptions{
				MIMEType:     "application/json",
				MaxChunkSize: 1000,
			}
			result, err := chunker.Chunk(context.Background(), []byte(p), opts)
			if err != nil {
				t.Fatalf("Chunk returned error for %s: %v", p, err)
			}
			// Should fall back to single chunk
			if len(result.Chunks) != 1 {
				t.Errorf("Expected 1 chunk for primitive %s, got %d", p, len(result.Chunks))
			}
		}
	})

	t.Run("JSON null value", func(t *testing.T) {
		// `null` is a special case - json.Unmarshal([]byte("null"), &[]json.RawMessage{})
		// produces a nil slice, so the chunker returns 0 chunks (nothing to chunk)
		opts := ChunkOptions{
			MIMEType:     "application/json",
			MaxChunkSize: 1000,
		}
		result, err := chunker.Chunk(context.Background(), []byte("null"), opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		// null JSON produces 0 chunks (correct behavior)
		if len(result.Chunks) != 0 {
			t.Errorf("Expected 0 chunks for null JSON, got %d", len(result.Chunks))
		}
	})

	t.Run("very large single JSON key value", func(t *testing.T) {
		// Create a JSON object with one very large value
		largeValue := make([]byte, 5000)
		for i := range largeValue {
			largeValue[i] = 'a'
		}
		content := fmt.Sprintf(`{"largeKey": "%s"}`, string(largeValue))
		opts := ChunkOptions{
			MIMEType:     "application/json",
			MaxChunkSize: 1000,
		}
		result, err := chunker.Chunk(context.Background(), []byte(content), opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		// Should handle large value, may be in single chunk since can't split mid-value
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
	})

	t.Run("CSV with quoted fields containing commas", func(t *testing.T) {
		content := []byte(`name,address,phone
"Doe, John","123 Main St, Apt 4","555-1234"
"Smith, Jane","456 Oak Ave","555-5678"`)
		opts := ChunkOptions{
			MIMEType:     "text/csv",
			MaxChunkSize: 1000,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
	})

	t.Run("CSV with empty rows", func(t *testing.T) {
		content := []byte(`header1,header2
value1,value2

value3,value4

value5,value6`)
		opts := ChunkOptions{
			MIMEType:     "text/csv",
			MaxChunkSize: 1000,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		// Should skip empty rows gracefully
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
	})

	t.Run("CSV with only header", func(t *testing.T) {
		content := []byte(`header1,header2,header3`)
		opts := ChunkOptions{
			MIMEType:     "text/csv",
			MaxChunkSize: 1000,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		// Header only should result in no chunks (no data rows)
		if len(result.Chunks) != 0 {
			t.Errorf("Expected 0 chunks for header-only CSV, got %d", len(result.Chunks))
		}
	})

	t.Run("YAML content fallback to lines", func(t *testing.T) {
		content := []byte(`key1: value1
key2: value2
nested:
  subkey: subvalue
list:
  - item1
  - item2`)
		opts := ChunkOptions{
			MIMEType:     "text/yaml",
			MaxChunkSize: 1000,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
	})

	t.Run("context cancellation with JSON array", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Create large JSON array
		var items []string
		for i := 0; i < 100; i++ {
			items = append(items, fmt.Sprintf(`{"id": %d}`, i))
		}
		content := []byte("[" + fmt.Sprintf("%s", items[0]))
		for i := 1; i < len(items); i++ {
			content = append(content, ',')
			content = append(content, items[i]...)
		}
		content = append(content, ']')

		opts := ChunkOptions{
			MIMEType:     "application/json",
			MaxChunkSize: 100,
		}
		_, err := chunker.Chunk(ctx, content, opts)
		if err != nil && err != context.Canceled {
			// Context error is expected, other errors are not
			t.Logf("Got error (may be expected): %v", err)
		}
	})

	t.Run("JSON array with single large element", func(t *testing.T) {
		// Array with one element larger than maxSize
		largeValue := make([]byte, 500)
		for i := range largeValue {
			largeValue[i] = 'x'
		}
		content := fmt.Sprintf(`[{"data": "%s"}]`, string(largeValue))
		opts := ChunkOptions{
			MIMEType:     "application/json",
			MaxChunkSize: 100,
		}
		result, err := chunker.Chunk(context.Background(), []byte(content), opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		// Should still produce a chunk even if larger than maxSize
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
	})
}

// ============================================================================
// Phase 1 Edge Case Tests - Recursive Chunker
// ============================================================================

func TestRecursiveChunkerEdgeCases(t *testing.T) {
	chunker := NewRecursiveChunker()

	t.Run("content with no separators", func(t *testing.T) {
		// Long string with no separators - should fall back to character splitting
		content := make([]byte, 500)
		for i := range content {
			content[i] = 'a'
		}
		opts := ChunkOptions{
			MaxChunkSize: 100,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) < 5 {
			t.Errorf("Expected at least 5 chunks for 500 chars with maxSize 100, got %d", len(result.Chunks))
		}
	})

	t.Run("content with only spaces", func(t *testing.T) {
		content := []byte("word1 word2 word3 word4 word5 word6 word7 word8 word9 word10")
		opts := ChunkOptions{
			MaxChunkSize: 20,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
		// Each chunk should be within maxSize (approximately)
		for i, chunk := range result.Chunks {
			if len(chunk.Content) > opts.MaxChunkSize*2 { // Allow some tolerance for overlap
				t.Errorf("Chunk %d exceeds maxSize: %d > %d", i, len(chunk.Content), opts.MaxChunkSize)
			}
		}
	})

	t.Run("overlap handling", func(t *testing.T) {
		content := []byte("First paragraph content.\n\nSecond paragraph content.\n\nThird paragraph content.")
		opts := ChunkOptions{
			MaxChunkSize: 30,
			Overlap:      10,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		// With overlap, chunks should share some content
		if len(result.Chunks) >= 2 {
			// Check that overlapping content exists
			// (Implementation detail: chunks may share end/start content)
		}
	})

	t.Run("very small maxSize", func(t *testing.T) {
		content := []byte("Hello world this is a test.")
		opts := ChunkOptions{
			MaxChunkSize: 5,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		// Should still produce chunks
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
	})

	t.Run("unicode text splitting", func(t *testing.T) {
		content := []byte("你好世界 这是一个测试 中文文本分割测试")
		opts := ChunkOptions{
			MaxChunkSize: 20,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
	})

	t.Run("content ending with separator", func(t *testing.T) {
		content := []byte("Sentence one. Sentence two. ")
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
	})

	t.Run("mixed separator types", func(t *testing.T) {
		content := []byte("Para 1.\n\nPara 2.\n\n\nPara 3. Sentence 1. Sentence 2!\nLine 4")
		opts := ChunkOptions{
			MaxChunkSize: 50,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
	})

	t.Run("single character content", func(t *testing.T) {
		content := []byte("a")
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) != 1 {
			t.Errorf("Expected 1 chunk for single char, got %d", len(result.Chunks))
		}
	})

	t.Run("context cancellation during recursive split", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		largeContent := make([]byte, 5000)
		for i := range largeContent {
			if i%10 == 0 {
				largeContent[i] = ' '
			} else {
				largeContent[i] = 'a'
			}
		}

		opts := ChunkOptions{MaxChunkSize: 50}
		result, err := chunker.Chunk(ctx, largeContent, opts)
		// Should either return partial result or error
		if err == nil && result != nil {
			// Partial result is acceptable
		}
	})
}

// ============================================================================
// Phase 1 Edge Case Tests - Fallback Chunker
// ============================================================================

func TestFallbackChunkerEdgeCases(t *testing.T) {
	chunker := NewFallbackChunker()

	t.Run("overlap equals maxSize", func(t *testing.T) {
		content := []byte("Hello world this is a test of overlap handling.")
		opts := ChunkOptions{
			MaxChunkSize: 100,
			Overlap:      100,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		// Should handle gracefully by reducing overlap
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
	})

	t.Run("overlap larger than maxSize", func(t *testing.T) {
		content := []byte("Hello world this is a test of overlap handling.")
		opts := ChunkOptions{
			MaxChunkSize: 100,
			Overlap:      200, // Larger than maxSize
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		// Should handle gracefully
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
	})

	t.Run("very small maxSize", func(t *testing.T) {
		content := []byte("Hello world")
		opts := ChunkOptions{
			MaxChunkSize: 3,
			Overlap:      1,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
	})

	t.Run("content with no whitespace", func(t *testing.T) {
		content := make([]byte, 200)
		for i := range content {
			content[i] = 'a'
		}
		opts := ChunkOptions{
			MaxChunkSize: 50,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		// Should still chunk even without whitespace
		if len(result.Chunks) < 2 {
			t.Errorf("Expected multiple chunks, got %d", len(result.Chunks))
		}
	})

	t.Run("binary content", func(t *testing.T) {
		content := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0x00, 0x10, 0x20, 0x30}
		opts := ChunkOptions{
			MaxChunkSize: 5,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		// Should handle binary content without panic
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}
	})

	t.Run("single byte content", func(t *testing.T) {
		content := []byte{0x41} // 'A'
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) != 1 {
			t.Errorf("Expected 1 chunk, got %d", len(result.Chunks))
		}
		if result.Chunks[0].Content != "A" {
			t.Errorf("Content = %q, want %q", result.Chunks[0].Content, "A")
		}
	})

	t.Run("content exactly at maxSize", func(t *testing.T) {
		content := make([]byte, 100)
		for i := range content {
			content[i] = 'x'
		}
		opts := ChunkOptions{
			MaxChunkSize: 100,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) != 1 {
			t.Errorf("Expected 1 chunk for content at maxSize, got %d", len(result.Chunks))
		}
	})

	t.Run("content one byte over maxSize", func(t *testing.T) {
		content := make([]byte, 101)
		for i := range content {
			content[i] = 'x'
		}
		opts := ChunkOptions{
			MaxChunkSize: 100,
			Overlap:      0,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		// Should produce 2 chunks
		if len(result.Chunks) < 2 {
			t.Errorf("Expected at least 2 chunks, got %d", len(result.Chunks))
		}
	})

	t.Run("maxSize of 1", func(t *testing.T) {
		content := []byte("abc")
		opts := ChunkOptions{
			MaxChunkSize: 1,
			Overlap:      0,
		}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) != 3 {
			t.Errorf("Expected 3 chunks for 3-char content with maxSize 1, got %d", len(result.Chunks))
		}
	})

	t.Run("metadata type is unknown", func(t *testing.T) {
		content := []byte("test content")
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		for _, chunk := range result.Chunks {
			if chunk.Metadata.Type != ChunkTypeUnknown {
				t.Errorf("Expected ChunkTypeUnknown, got %v", chunk.Metadata.Type)
			}
		}
	})
}

// ============================================================================
// Phase 1 Edge Case Tests - Registry
// ============================================================================

func TestRegistryEdgeCases(t *testing.T) {
	t.Run("empty content through registry", func(t *testing.T) {
		registry := DefaultRegistry()
		content := []byte{}
		opts := ChunkOptions{
			MIMEType:     "text/plain",
			MaxChunkSize: 1000,
		}
		result, err := registry.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) != 0 {
			t.Errorf("Expected 0 chunks for empty content, got %d", len(result.Chunks))
		}
	})

	t.Run("multiple failing chunkers accumulate warnings", func(t *testing.T) {
		registry := NewRegistry()
		registry.Register(&failingChunker{priority: 100})
		registry.Register(&failingChunker{priority: 90})
		registry.Register(&failingChunker{priority: 80})
		registry.SetFallback(NewFallbackChunker())

		content := []byte("test content")
		opts := ChunkOptions{
			MIMEType:     "text/plain",
			MaxChunkSize: 1000,
		}
		result, err := registry.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Expected fallback to succeed, got error: %v", err)
		}
		// Should have 3 warnings from 3 failing chunkers
		if len(result.Warnings) < 3 {
			t.Errorf("Expected at least 3 warnings, got %d", len(result.Warnings))
		}
	})

	t.Run("context cancellation propagation", func(t *testing.T) {
		registry := DefaultRegistry()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		content := []byte("test content")
		opts := ChunkOptions{
			MIMEType:     "text/plain",
			MaxChunkSize: 1000,
		}
		_, err := registry.Chunk(ctx, content, opts)
		// Context cancellation may or may not cause error depending on chunker
		// Just verify no panic
		_ = err
	})

	t.Run("no chunkers match but fallback exists", func(t *testing.T) {
		registry := NewRegistry()
		// Register chunker that doesn't match any type
		registry.Register(&selectiveChunker{
			accepts:  "text/special",
			priority: 50,
		})
		registry.SetFallback(NewFallbackChunker())

		content := []byte("test content")
		opts := ChunkOptions{
			MIMEType:     "text/plain",
			MaxChunkSize: 1000,
		}
		result, err := registry.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Expected fallback to be used, got error: %v", err)
		}
		if result.ChunkerUsed != "fallback" {
			t.Errorf("Expected fallback chunker, got %q", result.ChunkerUsed)
		}
	})

	t.Run("registry with no chunkers and no fallback", func(t *testing.T) {
		registry := NewRegistry()

		content := []byte("test content")
		opts := ChunkOptions{
			MIMEType:     "text/plain",
			MaxChunkSize: 1000,
		}
		_, err := registry.Chunk(context.Background(), content, opts)
		if err == nil {
			t.Error("Expected error when no chunkers available")
		}
	})

	t.Run("chunker returns empty warnings", func(t *testing.T) {
		registry := NewRegistry()
		registry.Register(NewRecursiveChunker())

		content := []byte("test content")
		opts := ChunkOptions{
			MIMEType:     "text/plain",
			MaxChunkSize: 1000,
		}
		result, err := registry.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}
		// Warnings can be nil or empty slice, both are valid
		if result.Warnings != nil && len(result.Warnings) > 0 {
			t.Logf("Unexpected warnings: %v", result.Warnings)
		}
	})

	t.Run("concurrent registry access", func(t *testing.T) {
		registry := DefaultRegistry()
		content := []byte("test content for concurrent access")
		opts := ChunkOptions{
			MIMEType:     "text/plain",
			MaxChunkSize: 1000,
		}

		const goroutines = 10
		errors := make(chan error, goroutines)
		var wg sync.WaitGroup

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := registry.Chunk(context.Background(), content, opts)
				if err != nil {
					errors <- err
				}
			}()
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Errorf("Concurrent chunk failed: %v", err)
		}
	})
}

// selectiveChunker only handles specific MIME types.
type selectiveChunker struct {
	accepts  string
	priority int
}

func (s *selectiveChunker) Name() string { return "selective" }
func (s *selectiveChunker) CanHandle(mimeType string, language string) bool {
	return mimeType == s.accepts
}
func (s *selectiveChunker) Priority() int { return s.priority }
func (s *selectiveChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	return &ChunkResult{
		Chunks:       []Chunk{{Index: 0, Content: string(content)}},
		TotalChunks:  1,
		ChunkerUsed:  "selective",
		OriginalSize: len(content),
	}, nil
}
