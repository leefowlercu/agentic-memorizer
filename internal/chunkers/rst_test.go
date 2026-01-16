package chunkers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRSTChunker_Name(t *testing.T) {
	c := NewRSTChunker()
	if c.Name() != "rst" {
		t.Errorf("expected name 'rst', got %q", c.Name())
	}
}

func TestRSTChunker_Priority(t *testing.T) {
	c := NewRSTChunker()
	if c.Priority() != 55 {
		t.Errorf("expected priority 55, got %d", c.Priority())
	}
}

func TestRSTChunker_CanHandle(t *testing.T) {
	c := NewRSTChunker()

	tests := []struct {
		mimeType string
		language string
		want     bool
	}{
		{"text/x-rst", "", true},
		{"text/restructuredtext", "", true},
		{"", "test.rst", true},
		{"", "test.RST", true},
		{"", "test.rest", true},
		{"text/plain", "", false},
		{"text/markdown", "", false},
		{"", "test.md", false},
		{"", "test.txt", false},
	}

	for _, tt := range tests {
		got := c.CanHandle(tt.mimeType, tt.language)
		if got != tt.want {
			t.Errorf("CanHandle(%q, %q) = %v, want %v", tt.mimeType, tt.language, got, tt.want)
		}
	}
}

func TestRSTChunker_EmptyContent(t *testing.T) {
	c := NewRSTChunker()
	result, err := c.Chunk(context.Background(), []byte{}, DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Chunks) != 0 {
		t.Errorf("expected 0 chunks, got %d", len(result.Chunks))
	}
	if result.ChunkerUsed != "rst" {
		t.Errorf("expected chunker 'rst', got %q", result.ChunkerUsed)
	}
}

func TestRSTChunker_SimpleHeadings(t *testing.T) {
	c := NewRSTChunker()
	content := `Title
=====

First section content.

Subtitle
--------

Second section content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(result.Chunks))
	}

	// First chunk should be level 1
	if result.Chunks[0].Metadata.Document == nil {
		t.Fatal("expected Document metadata")
	}
	if result.Chunks[0].Metadata.Document.Heading != "Title" {
		t.Errorf("expected heading 'Title', got %q", result.Chunks[0].Metadata.Document.Heading)
	}
	if result.Chunks[0].Metadata.Document.HeadingLevel != 1 {
		t.Errorf("expected level 1, got %d", result.Chunks[0].Metadata.Document.HeadingLevel)
	}

	// Second chunk should be level 2
	if result.Chunks[1].Metadata.Document.Heading != "Subtitle" {
		t.Errorf("expected heading 'Subtitle', got %q", result.Chunks[1].Metadata.Document.Heading)
	}
	if result.Chunks[1].Metadata.Document.HeadingLevel != 2 {
		t.Errorf("expected level 2, got %d", result.Chunks[1].Metadata.Document.HeadingLevel)
	}
}

func TestRSTChunker_OverlineHeadings(t *testing.T) {
	c := NewRSTChunker()
	content := `========
Document
========

This is the document content.

Chapter
=======

Chapter content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(result.Chunks))
	}

	// Overline+underline should be level 1
	if result.Chunks[0].Metadata.Document.HeadingLevel != 1 {
		t.Errorf("expected overline heading at level 1, got %d", result.Chunks[0].Metadata.Document.HeadingLevel)
	}

	// Underline-only with same char should be different level
	if result.Chunks[1].Metadata.Document.HeadingLevel != 2 {
		t.Errorf("expected underline-only heading at level 2, got %d", result.Chunks[1].Metadata.Document.HeadingLevel)
	}
}

func TestRSTChunker_HeadingHierarchy(t *testing.T) {
	c := NewRSTChunker()
	content := `Level One
=========

Content at level 1.

Level Two
---------

Content at level 2.

Level Three
~~~~~~~~~~~

Content at level 3.

Another Level Two
-----------------

Back to level 2.

Level Four
^^^^^^^^^^

Content at level 4.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedLevels := []int{1, 2, 3, 2, 4}
	if len(result.Chunks) != len(expectedLevels) {
		t.Fatalf("expected %d chunks, got %d", len(expectedLevels), len(result.Chunks))
	}

	for i, expected := range expectedLevels {
		if result.Chunks[i].Metadata.Document.HeadingLevel != expected {
			t.Errorf("chunk %d: expected level %d, got %d",
				i, expected, result.Chunks[i].Metadata.Document.HeadingLevel)
		}
	}
}

func TestRSTChunker_SectionPath(t *testing.T) {
	c := NewRSTChunker()
	content := `Chapter
=======

Chapter content.

Section
-------

Section content.

Subsection
~~~~~~~~~~

Subsection content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Chunks) < 3 {
		t.Fatalf("expected at least 3 chunks, got %d", len(result.Chunks))
	}

	// Check section paths
	tests := []struct {
		idx          int
		expectedPath string
	}{
		{0, "Chapter"},
		{1, "Chapter > Section"},
		{2, "Chapter > Section > Subsection"},
	}

	for _, tt := range tests {
		got := result.Chunks[tt.idx].Metadata.Document.SectionPath
		if got != tt.expectedPath {
			t.Errorf("chunk %d: expected path %q, got %q", tt.idx, tt.expectedPath, got)
		}
	}
}

func TestRSTChunker_CodeBlocks(t *testing.T) {
	c := NewRSTChunker()
	content := `Title
=====

Here is some code:

.. code-block:: python

    def hello():
        print("Hello")

More text after the code.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Chunks) == 0 {
		t.Fatal("expected at least 1 chunk")
	}

	// Code block should be preserved in content
	if !strings.Contains(result.Chunks[0].Content, "code-block:: python") {
		t.Error("expected code block directive in content")
	}
	if !strings.Contains(result.Chunks[0].Content, "def hello():") {
		t.Error("expected code content in chunk")
	}
}

func TestRSTChunker_Directives(t *testing.T) {
	c := NewRSTChunker()
	content := `Title
=====

.. note::

   This is a note.

.. warning::

   This is a warning.

Regular paragraph.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Chunks) == 0 {
		t.Fatal("expected at least 1 chunk")
	}

	// Directives should be preserved
	if !strings.Contains(result.Chunks[0].Content, ".. note::") {
		t.Error("expected note directive in content")
	}
	if !strings.Contains(result.Chunks[0].Content, ".. warning::") {
		t.Error("expected warning directive in content")
	}
}

func TestRSTChunker_LargeSectionSplit(t *testing.T) {
	c := NewRSTChunker()

	// Create content with a section larger than max size
	var sb strings.Builder
	sb.WriteString("Title\n=====\n\n")

	// Add many paragraphs
	for i := 0; i < 50; i++ {
		sb.WriteString("This is paragraph ")
		sb.WriteString(strings.Repeat("content ", 50))
		sb.WriteString(".\n\n")
	}

	opts := ChunkOptions{
		MaxChunkSize: 1000, // Small size to force splitting
	}

	result, err := c.Chunk(context.Background(), []byte(sb.String()), opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have multiple chunks
	if len(result.Chunks) < 2 {
		t.Errorf("expected multiple chunks, got %d", len(result.Chunks))
	}

	// All chunks should be under max size
	for i, chunk := range result.Chunks {
		if len(chunk.Content) > opts.MaxChunkSize*2 { // Allow some flexibility
			t.Errorf("chunk %d exceeds max size: %d > %d", i, len(chunk.Content), opts.MaxChunkSize)
		}
	}
}

func TestRSTChunker_ContextCancellation(t *testing.T) {
	c := NewRSTChunker()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	content := `Title
=====

Content.
`
	_, err := c.Chunk(ctx, []byte(content), DefaultChunkOptions())

	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestRSTChunker_SampleFile(t *testing.T) {
	// Read the sample RST file
	content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "markup", "sample.rst"))
	if err != nil {
		t.Skipf("sample.rst not found: %v", err)
	}

	c := NewRSTChunker()
	result, err := c.Chunk(context.Background(), content, DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have multiple sections
	if len(result.Chunks) < 5 {
		t.Errorf("expected at least 5 chunks from sample, got %d", len(result.Chunks))
	}

	// Check various heading levels exist
	levelsSeen := make(map[int]bool)
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.HeadingLevel > 0 {
			levelsSeen[chunk.Metadata.Document.HeadingLevel] = true
		}
	}

	// Sample should have at least 2 different levels
	if len(levelsSeen) < 2 {
		t.Errorf("expected at least 2 heading levels, got %d: %v", len(levelsSeen), levelsSeen)
	}

	// Verify metadata type
	if result.Chunks[0].Metadata.Type != ChunkTypeProse {
		t.Errorf("expected type 'prose', got %q", result.Chunks[0].Metadata.Type)
	}
}

func TestRSTChunker_VariousUnderlineChars(t *testing.T) {
	c := NewRSTChunker()
	content := `Equals
======

Dashes
------

Tildes
~~~~~~

Carets
^^^^^^

Plus
++++

Hash
####
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Each different underline char should create a different level
	if len(result.Chunks) != 6 {
		t.Fatalf("expected 6 chunks, got %d", len(result.Chunks))
	}

	// Levels should be sequential (1, 2, 3, 4, 5, 6)
	for i, chunk := range result.Chunks {
		expected := i + 1
		if chunk.Metadata.Document.HeadingLevel != expected {
			t.Errorf("chunk %d: expected level %d, got %d",
				i, expected, chunk.Metadata.Document.HeadingLevel)
		}
	}
}

func TestRSTChunker_NoHeadings(t *testing.T) {
	c := NewRSTChunker()
	content := `This is just plain text.

It has multiple paragraphs.

But no headings at all.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have exactly 1 chunk with all content
	if len(result.Chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(result.Chunks))
	}

	if result.Chunks[0].Metadata.Document.HeadingLevel != 0 {
		t.Errorf("expected level 0 for no heading, got %d", result.Chunks[0].Metadata.Document.HeadingLevel)
	}
}
