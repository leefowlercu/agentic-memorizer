package chunkers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAsciiDocChunker_Name(t *testing.T) {
	c := NewAsciiDocChunker()
	if c.Name() != "asciidoc" {
		t.Errorf("expected name 'asciidoc', got %q", c.Name())
	}
}

func TestAsciiDocChunker_Priority(t *testing.T) {
	c := NewAsciiDocChunker()
	if c.Priority() != 54 {
		t.Errorf("expected priority 54, got %d", c.Priority())
	}
}

func TestAsciiDocChunker_CanHandle(t *testing.T) {
	c := NewAsciiDocChunker()

	tests := []struct {
		mimeType string
		language string
		want     bool
	}{
		{"text/asciidoc", "", true},
		{"text/x-asciidoc", "", true},
		{"", "test.adoc", true},
		{"", "test.ADOC", true},
		{"", "test.asciidoc", true},
		{"", "test.asc", true},
		{"text/plain", "", false},
		{"text/markdown", "", false},
		{"", "test.md", false},
		{"", "test.rst", false},
	}

	for _, tt := range tests {
		got := c.CanHandle(tt.mimeType, tt.language)
		if got != tt.want {
			t.Errorf("CanHandle(%q, %q) = %v, want %v", tt.mimeType, tt.language, got, tt.want)
		}
	}
}

func TestAsciiDocChunker_EmptyContent(t *testing.T) {
	c := NewAsciiDocChunker()
	result, err := c.Chunk(context.Background(), []byte{}, DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Chunks) != 0 {
		t.Errorf("expected 0 chunks, got %d", len(result.Chunks))
	}
	if result.ChunkerUsed != "asciidoc" {
		t.Errorf("expected chunker 'asciidoc', got %q", result.ChunkerUsed)
	}
}

func TestAsciiDocChunker_SimpleHeadings(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `= Document Title

Introduction paragraph.

== First Section

First section content.

== Second Section

Second section content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Chunks) < 3 {
		t.Fatalf("expected at least 3 chunks, got %d", len(result.Chunks))
	}

	// Document title (=) is level 1
	if result.Chunks[0].Metadata.Document == nil {
		t.Fatal("expected Document metadata")
	}
	if result.Chunks[0].Metadata.Document.Heading != "Document Title" {
		t.Errorf("expected heading 'Document Title', got %q", result.Chunks[0].Metadata.Document.Heading)
	}
	if result.Chunks[0].Metadata.Document.HeadingLevel != 1 {
		t.Errorf("expected level 1, got %d", result.Chunks[0].Metadata.Document.HeadingLevel)
	}

	// First section (==) is level 2
	if result.Chunks[1].Metadata.Document.Heading != "First Section" {
		t.Errorf("expected heading 'First Section', got %q", result.Chunks[1].Metadata.Document.Heading)
	}
	if result.Chunks[1].Metadata.Document.HeadingLevel != 2 {
		t.Errorf("expected level 2, got %d", result.Chunks[1].Metadata.Document.HeadingLevel)
	}
}

func TestAsciiDocChunker_HeadingLevels(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `= Level 1

Level 1 content.

== Level 2

Level 2 content.

=== Level 3

Level 3 content.

==== Level 4

Level 4 content.

===== Level 5

Level 5 content.

====== Level 6

Level 6 content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedLevels := []int{1, 2, 3, 4, 5, 6}
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

func TestAsciiDocChunker_SectionPath(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `= Document

Doc content.

== Chapter

Chapter content.

=== Section

Section content.

==== Subsection

Subsection content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Chunks) < 4 {
		t.Fatalf("expected at least 4 chunks, got %d", len(result.Chunks))
	}

	// Check section paths
	tests := []struct {
		idx          int
		expectedPath string
	}{
		{0, "Document"},
		{1, "Document > Chapter"},
		{2, "Document > Chapter > Section"},
		{3, "Document > Chapter > Section > Subsection"},
	}

	for _, tt := range tests {
		got := result.Chunks[tt.idx].Metadata.Document.SectionPath
		if got != tt.expectedPath {
			t.Errorf("chunk %d: expected path %q, got %q", tt.idx, tt.expectedPath, got)
		}
	}
}

func TestAsciiDocChunker_SourceBlocks(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `= Title

Here is some code:

[source,python]
----
def hello():
    print("Hello")

# This looks like a heading but is not: == Not a heading
----

More text after the code.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be a single section (source block content not split)
	if len(result.Chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(result.Chunks))
	}

	// Source block should be preserved in content
	if !strings.Contains(result.Chunks[0].Content, "[source,python]") {
		t.Error("expected source block in content")
	}
	if !strings.Contains(result.Chunks[0].Content, "def hello():") {
		t.Error("expected code content in chunk")
	}
	// The fake heading inside source block should NOT create a new section
	if !strings.Contains(result.Chunks[0].Content, "== Not a heading") {
		t.Error("expected fake heading to be preserved in source block")
	}
}

func TestAsciiDocChunker_Admonitions(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `= Title

NOTE: This is a note.

TIP: This is a tip.

WARNING: This is a warning.

== Next Section

Content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(result.Chunks))
	}

	// Admonitions should be preserved in first section
	if !strings.Contains(result.Chunks[0].Content, "NOTE:") {
		t.Error("expected NOTE admonition in content")
	}
	if !strings.Contains(result.Chunks[0].Content, "WARNING:") {
		t.Error("expected WARNING admonition in content")
	}
}

func TestAsciiDocChunker_Tables(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `= Title

|===
|Col1 |Col2 |Col3

|A1 |A2 |A3
|B1 |B2 |B3
|===

Text after table.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Chunks) == 0 {
		t.Fatal("expected at least 1 chunk")
	}

	// Table should be preserved
	if !strings.Contains(result.Chunks[0].Content, "|===") {
		t.Error("expected table delimiters in content")
	}
	if !strings.Contains(result.Chunks[0].Content, "|Col1") {
		t.Error("expected table content")
	}
}

func TestAsciiDocChunker_LargeSectionSplit(t *testing.T) {
	c := NewAsciiDocChunker()

	// Create content with a section larger than max size
	var sb strings.Builder
	sb.WriteString("= Title\n\n")

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

	// All chunks should be under max size (with some flexibility)
	for i, chunk := range result.Chunks {
		if len(chunk.Content) > opts.MaxChunkSize*2 {
			t.Errorf("chunk %d exceeds max size: %d > %d", i, len(chunk.Content), opts.MaxChunkSize)
		}
	}
}

func TestAsciiDocChunker_ContextCancellation(t *testing.T) {
	c := NewAsciiDocChunker()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	content := `= Title

Content.
`
	_, err := c.Chunk(ctx, []byte(content), DefaultChunkOptions())

	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestAsciiDocChunker_SampleFile(t *testing.T) {
	// Read the sample AsciiDoc file
	content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "markup", "sample.adoc"))
	if err != nil {
		t.Skipf("sample.adoc not found: %v", err)
	}

	c := NewAsciiDocChunker()
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

	// Sample should have at least 3 different levels
	if len(levelsSeen) < 3 {
		t.Errorf("expected at least 3 heading levels, got %d: %v", len(levelsSeen), levelsSeen)
	}

	// Verify metadata type
	if result.Chunks[0].Metadata.Type != ChunkTypeProse {
		t.Errorf("expected type 'prose', got %q", result.Chunks[0].Metadata.Type)
	}
}

func TestAsciiDocChunker_Anchors(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `= Document

[[custom-anchor]]
== Custom Section

This section has a custom anchor.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(result.Chunks))
	}

	// Anchor should be preserved in content
	if !strings.Contains(result.Chunks[1].Content, "[[custom-anchor]]") {
		t.Error("expected anchor in content")
	}
}

func TestAsciiDocChunker_NoHeadings(t *testing.T) {
	c := NewAsciiDocChunker()
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

func TestAsciiDocChunker_LiteralBlocks(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `= Title

Here is a literal block:

....
This is literal text.
== This looks like a heading but is inside literal block
....

Regular text.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be a single section (literal block content not split)
	if len(result.Chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(result.Chunks))
	}

	// Literal block should be preserved
	if !strings.Contains(result.Chunks[0].Content, "....") {
		t.Error("expected literal block delimiters in content")
	}
}
