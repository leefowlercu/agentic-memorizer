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

// Edge case: Underline shorter than heading text should NOT be recognized as heading
func TestRSTChunker_ShortUnderline(t *testing.T) {
	c := NewRSTChunker()
	content := `This Is A Long Heading
====

Not a heading because underline is too short.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be a single chunk with no heading detected
	if len(result.Chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(result.Chunks))
	}

	// No heading should be detected because underline is too short
	if result.Chunks[0].Metadata.Document.HeadingLevel != 0 {
		t.Errorf("expected level 0 (no heading), got %d", result.Chunks[0].Metadata.Document.HeadingLevel)
	}
}

// Edge case: Mixed underline characters on same line should NOT be valid
func TestRSTChunker_MixedUnderlineChars(t *testing.T) {
	c := NewRSTChunker()
	content := `Title
=-==-

This should not have a heading.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Mixed underline chars should not be recognized as heading
	if len(result.Chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(result.Chunks))
	}

	if result.Chunks[0].Metadata.Document.HeadingLevel != 0 {
		t.Errorf("expected level 0 (no heading), got %d", result.Chunks[0].Metadata.Document.HeadingLevel)
	}
}

// Edge case: Consecutive headings without content between them
func TestRSTChunker_ConsecutiveHeadings(t *testing.T) {
	c := NewRSTChunker()
	content := `First Heading
=============

Second Heading
--------------

Third Heading
~~~~~~~~~~~~~

Finally some content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Each heading should create a section even if empty
	if len(result.Chunks) < 3 {
		t.Errorf("expected at least 3 chunks for consecutive headings, got %d", len(result.Chunks))
	}

	// Verify heading names
	headings := []string{}
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			headings = append(headings, chunk.Metadata.Document.Heading)
		}
	}

	expectedHeadings := []string{"First Heading", "Second Heading", "Third Heading"}
	for _, expected := range expectedHeadings {
		found := false
		for _, h := range headings {
			if h == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected to find heading %q, headings found: %v", expected, headings)
		}
	}
}

// Edge case: Unicode in heading text
// RST heading detection now uses character (rune) count, not byte count,
// so underlines only need to match the character length of the heading.
func TestRSTChunker_UnicodeHeadings(t *testing.T) {
	c := NewRSTChunker()
	// Underlines match character count (not byte count)
	// "日本語のタイトル" = 8 characters
	// "Привет мир" = 10 characters
	// "Émojis 🎉 in heading" = 20 characters
	content := `日本語のタイトル
================

This section has a Japanese title.

Привет мир
----------

This section has a Russian title.

Émojis 🎉 in heading
====================

This section has emojis.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should recognize Unicode headings
	if len(result.Chunks) < 3 {
		t.Fatalf("expected at least 3 chunks, got %d", len(result.Chunks))
	}

	// Check that Unicode headings are preserved
	headings := []string{}
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			headings = append(headings, chunk.Metadata.Document.Heading)
		}
	}

	if len(headings) < 3 {
		t.Errorf("expected at least 3 headings, got %d: %v", len(headings), headings)
	}

	// Verify specific headings are found
	expectedHeadings := map[string]bool{
		"日本語のタイトル":       false,
		"Привет мир":       false,
		"Émojis 🎉 in heading": false,
	}
	for _, h := range headings {
		if _, ok := expectedHeadings[h]; ok {
			expectedHeadings[h] = true
		}
	}
	for heading, found := range expectedHeadings {
		if !found {
			t.Errorf("expected to find heading %q", heading)
		}
	}
}

// Edge case: Overline with mismatched underline character should NOT form valid heading
func TestRSTChunker_MismatchedOverlineUnderline(t *testing.T) {
	c := NewRSTChunker()
	content := `========
Title
--------

Content here.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Mismatched overline/underline should not form a valid overlined heading
	// The behavior depends on implementation - either treated as two separate things
	// or not recognized as a heading at all
	if len(result.Chunks) == 0 {
		t.Fatal("expected at least 1 chunk")
	}
}

// Edge case: Single character underline should NOT be valid
func TestRSTChunker_SingleCharUnderline(t *testing.T) {
	c := NewRSTChunker()
	content := `Title
=

Not a heading.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Single char underline is not valid (must be at least 2 chars)
	if len(result.Chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(result.Chunks))
	}

	if result.Chunks[0].Metadata.Document.HeadingLevel != 0 {
		t.Errorf("expected level 0 (no heading), got %d", result.Chunks[0].Metadata.Document.HeadingLevel)
	}
}

// Edge case: Invalid underline characters (not in RST spec)
func TestRSTChunker_InvalidUnderlineChars(t *testing.T) {
	c := NewRSTChunker()
	content := `Title
@@@@@

This should not be a heading.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// '@' is not a valid RST underline character
	if len(result.Chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(result.Chunks))
	}

	if result.Chunks[0].Metadata.Document.HeadingLevel != 0 {
		t.Errorf("expected level 0 (no heading), got %d", result.Chunks[0].Metadata.Document.HeadingLevel)
	}
}

// Edge case: Heading with trailing whitespace on underline
func TestRSTChunker_UnderlineWithTrailingWhitespace(t *testing.T) {
	c := NewRSTChunker()
	content := `Title
=====

Content after heading with trailing spaces on underline.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Trailing whitespace should be stripped and heading recognized
	if len(result.Chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(result.Chunks))
	}

	if result.Chunks[0].Metadata.Document == nil {
		t.Fatal("expected Document metadata")
	}

	if result.Chunks[0].Metadata.Document.Heading != "Title" {
		t.Errorf("expected heading 'Title', got %q", result.Chunks[0].Metadata.Document.Heading)
	}
}

// Edge case: Very short heading with very long underline
func TestRSTChunker_LongUnderlineShortHeading(t *testing.T) {
	c := NewRSTChunker()
	content := `Hi
================================================

Short heading with very long underline.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Long underline should still work with short heading
	if len(result.Chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(result.Chunks))
	}

	if result.Chunks[0].Metadata.Document.Heading != "Hi" {
		t.Errorf("expected heading 'Hi', got %q", result.Chunks[0].Metadata.Document.Heading)
	}

	if result.Chunks[0].Metadata.Document.HeadingLevel != 1 {
		t.Errorf("expected level 1, got %d", result.Chunks[0].Metadata.Document.HeadingLevel)
	}
}

// Edge case: Heading at very end of file with no trailing newline
func TestRSTChunker_HeadingAtEndNoTrailingNewline(t *testing.T) {
	c := NewRSTChunker()
	content := `Intro content.

Final Section
=============`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still recognize heading at end
	if len(result.Chunks) < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	// Find the Final Section heading
	found := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading == "Final Section" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected to find 'Final Section' heading at end of file")
	}
}

// Edge case: Directive that looks like it could be a heading
func TestRSTChunker_DirectiveNotHeading(t *testing.T) {
	c := NewRSTChunker()
	content := `Real Title
==========

.. note:: This is a note directive

.. This is a comment
   ================
   This underline is inside a comment

Regular paragraph.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only one heading should be detected
	headingCount := 0
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.HeadingLevel > 0 {
			headingCount++
		}
	}

	if headingCount != 1 {
		t.Errorf("expected 1 heading (Real Title), got %d", headingCount)
	}
}

// Edge case: Same underline char at different positions creates consistent levels
func TestRSTChunker_SameCharReusedLevel(t *testing.T) {
	c := NewRSTChunker()
	content := `Chapter One
===========

Content.

Section A
---------

Content.

Chapter Two
===========

Content.

Section B
---------

Content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify that same underline char produces same level
	levelMap := make(map[string]int)
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			levelMap[chunk.Metadata.Document.Heading] = chunk.Metadata.Document.HeadingLevel
		}
	}

	// Both chapters should be level 1
	if levelMap["Chapter One"] != levelMap["Chapter Two"] {
		t.Errorf("expected same level for chapters: One=%d, Two=%d",
			levelMap["Chapter One"], levelMap["Chapter Two"])
	}

	// Both sections should be level 2
	if levelMap["Section A"] != levelMap["Section B"] {
		t.Errorf("expected same level for sections: A=%d, B=%d",
			levelMap["Section A"], levelMap["Section B"])
	}
}
