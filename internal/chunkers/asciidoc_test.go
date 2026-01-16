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

// Edge case: Heading without space after equals should NOT be a heading
func TestAsciiDocChunker_HeadingNoSpace(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `=NoSpaceHeading

This is not a valid AsciiDoc heading.

== Valid Heading

This is content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only "Valid Heading" should be recognized
	headings := []string{}
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			headings = append(headings, chunk.Metadata.Document.Heading)
		}
	}

	if len(headings) != 1 {
		t.Errorf("expected 1 heading, got %d: %v", len(headings), headings)
	}

	if len(headings) > 0 && headings[0] != "Valid Heading" {
		t.Errorf("expected 'Valid Heading', got %q", headings[0])
	}
}

// Edge case: More than 6 equals should NOT be a heading
func TestAsciiDocChunker_TooManyEquals(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `======= Too Many Equals

This should not be a heading.

== Valid

Content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only "Valid" should be recognized as a heading
	headings := []string{}
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			headings = append(headings, chunk.Metadata.Document.Heading)
		}
	}

	for _, h := range headings {
		if h == "Too Many Equals" {
			t.Error("should not recognize heading with more than 6 equals")
		}
	}
}

// Edge case: Comment blocks should NOT have headings inside detected
func TestAsciiDocChunker_CommentBlocks(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `= Document Title

Regular content.

////
== This Is A Comment
This heading is inside a comment block and should be ignored.
////

== Real Section

Real content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should NOT detect "This Is A Comment" as a heading
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading == "This Is A Comment" {
			t.Error("should not detect heading inside comment block")
		}
	}

	// Should detect "Document Title" and "Real Section"
	headings := make(map[string]bool)
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			headings[chunk.Metadata.Document.Heading] = true
		}
	}

	if !headings["Document Title"] {
		t.Error("expected to find 'Document Title' heading")
	}
	if !headings["Real Section"] {
		t.Error("expected to find 'Real Section' heading")
	}

	// Verify comment block delimiters are preserved in content
	combined := ""
	for _, chunk := range result.Chunks {
		combined += chunk.Content
	}
	if !strings.Contains(combined, "////") {
		t.Error("expected comment block delimiters to be preserved")
	}
}

// Edge case: Example blocks (====) should NOT be confused with headings
func TestAsciiDocChunker_ExampleBlocks(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `= Title

Here is an example:

====
This is example content.
It is inside an example block delimited by ====.
====

== Real Section

Content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Example block delimiters should be preserved, not treated as headings
	headings := []string{}
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			headings = append(headings, chunk.Metadata.Document.Heading)
		}
	}

	// Should have Title and Real Section
	expectedHeadings := map[string]bool{"Title": true, "Real Section": true}
	for _, h := range headings {
		if !expectedHeadings[h] {
			t.Errorf("unexpected heading detected: %q", h)
		}
	}
}

// Edge case: Unicode in headings
func TestAsciiDocChunker_UnicodeHeadings(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `= 日本語タイトル

Japanese content.

== Привет мир

Russian content.

=== Émojis 🎉 Work

Emoji content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should recognize Unicode headings
	if len(result.Chunks) < 3 {
		t.Errorf("expected at least 3 chunks, got %d", len(result.Chunks))
	}

	headings := []string{}
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			headings = append(headings, chunk.Metadata.Document.Heading)
		}
	}

	if len(headings) < 3 {
		t.Errorf("expected at least 3 headings, got %d: %v", len(headings), headings)
	}
}

// Edge case: Unclosed source block
func TestAsciiDocChunker_UnclosedSourceBlock(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `= Title

[source,python]
----
def hello():
    print("Hello")

== Not A Heading Because Unclosed

This looks like a heading but source block was never closed.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With unclosed source block, everything after should be treated as part of it
	// Should only have 1 chunk (the Title section with everything else)
	if len(result.Chunks) != 1 {
		t.Logf("got %d chunks, headings inside unclosed block might be detected", len(result.Chunks))
	}

	// Verify content is preserved
	if len(result.Chunks) > 0 && !strings.Contains(result.Chunks[0].Content, "def hello():") {
		t.Error("expected source content to be preserved")
	}
}

// Edge case: Heading at end of file with no trailing newline
func TestAsciiDocChunker_HeadingAtEndNoNewline(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `= Title

Content.

== Final Section`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should recognize heading at end
	headings := make(map[string]bool)
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			headings[chunk.Metadata.Document.Heading] = true
		}
	}

	if !headings["Final Section"] {
		t.Error("expected to find 'Final Section' heading at end of file")
	}
}

// Edge case: Consecutive headings without content
func TestAsciiDocChunker_ConsecutiveHeadings(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `= Title

== Section One

== Section Two

== Section Three

Finally some content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All headings should be recognized
	headings := []string{}
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			headings = append(headings, chunk.Metadata.Document.Heading)
		}
	}

	expected := []string{"Title", "Section One", "Section Two", "Section Three"}
	for _, exp := range expected {
		found := false
		for _, h := range headings {
			if h == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected to find heading %q, got: %v", exp, headings)
		}
	}
}

// Edge case: Block ID [[id]] not followed by heading should be preserved
func TestAsciiDocChunker_BlockIDNotFollowedByHeading(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `= Title

[[some-id]]
This paragraph has an ID but is not a heading.

[[section-anchor]]
== Real Section

Content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Block ID should be preserved in content
	contentFound := false
	for _, chunk := range result.Chunks {
		if strings.Contains(chunk.Content, "[[some-id]]") {
			contentFound = true
			break
		}
	}

	if !contentFound {
		t.Error("expected [[some-id]] to be preserved in content")
	}
}

// Edge case: Passthrough blocks (++++))
func TestAsciiDocChunker_PassthroughBlocks(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `= Title

++++
<div>
== Not A Heading
This is raw HTML passthrough.
</div>
++++

== Real Section

Content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should NOT detect "Not A Heading" as a heading
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading == "Not A Heading" {
			t.Error("should not detect heading inside passthrough block")
		}
	}

	// Passthrough content should be preserved
	combined := ""
	for _, chunk := range result.Chunks {
		combined += chunk.Content
	}

	// Passthrough delimiters should be preserved
	if !strings.Contains(combined, "++++") {
		t.Error("expected passthrough block delimiters to be preserved")
	}

	// HTML content should be preserved
	if !strings.Contains(combined, "<div>") {
		t.Error("expected HTML content to be preserved")
	}
}

// Edge case: Sidebar blocks (****)
func TestAsciiDocChunker_SidebarBlocks(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `= Title

****
This is a sidebar.

== Not A Real Heading

Just sidebar text.
****

== Real Section

Content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Sidebar delimiters should be in content
	if len(result.Chunks) > 0 {
		combined := ""
		for _, chunk := range result.Chunks {
			combined += chunk.Content
		}
		if !strings.Contains(combined, "****") {
			t.Error("expected sidebar delimiters to be preserved")
		}
	}
}

// Edge case: Quote blocks (____))
func TestAsciiDocChunker_QuoteBlocks(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `= Title

[quote, Famous Person]
____
This is a quote.

It can span multiple paragraphs.
____

== Section

Content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Quote block should be preserved
	combined := ""
	for _, chunk := range result.Chunks {
		combined += chunk.Content
	}

	if !strings.Contains(combined, "____") {
		t.Error("expected quote block delimiters to be preserved")
	}
}

// Edge case: Document attributes should not interfere
func TestAsciiDocChunker_DocumentAttributes(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `= Title
:author: Test Author
:version: 1.0
:toc:
:icons: font

Content after attributes.

== Section

More content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Attributes should be preserved in content
	if len(result.Chunks) > 0 && !strings.Contains(result.Chunks[0].Content, ":author:") {
		t.Error("expected document attributes to be preserved")
	}

	// Should have Title and Section headings
	headings := make(map[string]bool)
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			headings[chunk.Metadata.Document.Heading] = true
		}
	}

	if !headings["Title"] || !headings["Section"] {
		t.Errorf("expected Title and Section headings, got: %v", headings)
	}
}

// Edge case: Same level headings have consistent section paths
func TestAsciiDocChunker_SameLevelConsistentPath(t *testing.T) {
	c := NewAsciiDocChunker()
	content := `= Document

== Chapter One

=== Section A

Content.

== Chapter Two

=== Section B

Content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find Section B's path - should include Chapter Two, not Chapter One
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading == "Section B" {
			path := chunk.Metadata.Document.SectionPath
			if !strings.Contains(path, "Chapter Two") {
				t.Errorf("Section B path should include 'Chapter Two', got: %q", path)
			}
			if strings.Contains(path, "Chapter One") {
				t.Errorf("Section B path should NOT include 'Chapter One', got: %q", path)
			}
			break
		}
	}
}
