package chunkers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLaTeXChunker_Name(t *testing.T) {
	c := NewLaTeXChunker()
	if c.Name() != "latex" {
		t.Errorf("expected name 'latex', got %q", c.Name())
	}
}

func TestLaTeXChunker_Priority(t *testing.T) {
	c := NewLaTeXChunker()
	if c.Priority() != 53 {
		t.Errorf("expected priority 53, got %d", c.Priority())
	}
}

func TestLaTeXChunker_CanHandle(t *testing.T) {
	c := NewLaTeXChunker()

	tests := []struct {
		mimeType string
		language string
		want     bool
	}{
		{"text/x-latex", "", true},
		{"text/x-tex", "", true},
		{"application/x-latex", "", true},
		{"application/x-tex", "", true},
		{"", "test.tex", true},
		{"", "test.TEX", true},
		{"", "test.latex", true},
		{"", "test.ltx", true},
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

func TestLaTeXChunker_EmptyContent(t *testing.T) {
	c := NewLaTeXChunker()
	result, err := c.Chunk(context.Background(), []byte{}, DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Chunks) != 0 {
		t.Errorf("expected 0 chunks, got %d", len(result.Chunks))
	}
	if result.ChunkerUsed != "latex" {
		t.Errorf("expected chunker 'latex', got %q", result.ChunkerUsed)
	}
}

func TestLaTeXChunker_SimpleSections(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\documentclass{article}

\section{Introduction}

This is the introduction.

\section{Methods}

This describes the methods.

\section{Results}

Here are the results.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have preamble + 3 sections = 4 chunks
	if len(result.Chunks) < 3 {
		t.Fatalf("expected at least 3 chunks, got %d", len(result.Chunks))
	}

	// Find Introduction section
	var introChunk *Chunk
	for i := range result.Chunks {
		if result.Chunks[i].Metadata.Document != nil &&
			result.Chunks[i].Metadata.Document.Heading == "Introduction" {
			introChunk = &result.Chunks[i]
			break
		}
	}

	if introChunk == nil {
		t.Fatal("expected to find Introduction section")
	}

	// Section level should be 3 (section = level 2 in LaTeX, +1 for 1-indexed = 3)
	if introChunk.Metadata.Document.HeadingLevel != 3 {
		t.Errorf("expected level 3, got %d", introChunk.Metadata.Document.HeadingLevel)
	}
}

func TestLaTeXChunker_SectioningHierarchy(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\chapter{First Chapter}

Chapter content.

\section{First Section}

Section content.

\subsection{First Subsection}

Subsection content.

\subsubsection{First Subsubsection}

Subsubsection content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify we have chunks for each section level
	levelsSeen := make(map[int]bool)
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.HeadingLevel > 0 {
			levelsSeen[chunk.Metadata.Document.HeadingLevel] = true
		}
	}

	// Should have levels 2, 3, 4, 5 (chapter, section, subsection, subsubsection)
	expectedLevels := []int{2, 3, 4, 5}
	for _, level := range expectedLevels {
		if !levelsSeen[level] {
			t.Errorf("expected to see level %d, levels seen: %v", level, levelsSeen)
		}
	}
}

func TestLaTeXChunker_SectionPath(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\section{Parent}

Parent content.

\subsection{Child}

Child content.

\subsubsection{Grandchild}

Grandchild content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the Grandchild section and check its path
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading == "Grandchild" {
			expected := "Parent > Child > Grandchild"
			if chunk.Metadata.Document.SectionPath != expected {
				t.Errorf("expected path %q, got %q", expected, chunk.Metadata.Document.SectionPath)
			}
			return
		}
	}
	t.Error("did not find Grandchild section")
}

func TestLaTeXChunker_EquationPreservation(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\section{Math}

Some text before.

\begin{equation}
E = mc^2
\end{equation}

Some text after.

\begin{align}
a &= b + c \\
d &= e + f
\end{align}

Final text.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Chunks) == 0 {
		t.Fatal("expected at least 1 chunk")
	}

	// Find the Math section
	var mathChunk *Chunk
	for i := range result.Chunks {
		if result.Chunks[i].Metadata.Document != nil &&
			result.Chunks[i].Metadata.Document.Heading == "Math" {
			mathChunk = &result.Chunks[i]
			break
		}
	}

	if mathChunk == nil {
		t.Fatal("expected to find Math section")
	}

	// Equations should be preserved
	if !strings.Contains(mathChunk.Content, "\\begin{equation}") {
		t.Error("expected equation environment to be preserved")
	}
	if !strings.Contains(mathChunk.Content, "E = mc^2") {
		t.Error("expected equation content to be preserved")
	}
	if !strings.Contains(mathChunk.Content, "\\begin{align}") {
		t.Error("expected align environment to be preserved")
	}
}

func TestLaTeXChunker_StarredSections(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\section*{Unnumbered Section}

This section is unnumbered.

\section{Numbered Section}

This section is numbered.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both starred and non-starred sections should be detected
	headings := make(map[string]bool)
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			headings[chunk.Metadata.Document.Heading] = true
		}
	}

	if !headings["Unnumbered Section"] {
		t.Error("expected to find 'Unnumbered Section'")
	}
	if !headings["Numbered Section"] {
		t.Error("expected to find 'Numbered Section'")
	}
}

func TestLaTeXChunker_ShortTitles(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\section[Short]{Very Long Section Title That Appears in TOC}

Section content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should extract the full title (in braces), not the short title
	found := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil &&
			chunk.Metadata.Document.Heading == "Very Long Section Title That Appears in TOC" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected to find section with full title")
	}
}

func TestLaTeXChunker_VerbatimEnvironment(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\section{Code}

Here is some code:

\begin{verbatim}
\section{This Is Not A Section}
It's just text in verbatim.
\end{verbatim}

\section{Real Section}

Back to normal.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 real sections: Code and Real Section
	headings := []string{}
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			headings = append(headings, chunk.Metadata.Document.Heading)
		}
	}

	if len(headings) != 2 {
		t.Errorf("expected 2 sections, got %d: %v", len(headings), headings)
	}

	// The fake section inside verbatim should NOT be detected
	for _, h := range headings {
		if h == "This Is Not A Section" {
			t.Error("should not detect section inside verbatim environment")
		}
	}
}

func TestLaTeXChunker_LstlistingEnvironment(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\section{Code}

\begin{lstlisting}
\section{Fake Section}
\end{lstlisting}

\section{Real}

Content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	headings := []string{}
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			headings = append(headings, chunk.Metadata.Document.Heading)
		}
	}

	// Should have Code and Real, not Fake Section
	if len(headings) != 2 {
		t.Errorf("expected 2 sections, got %d: %v", len(headings), headings)
	}

	for _, h := range headings {
		if h == "Fake Section" {
			t.Error("should not detect section inside lstlisting environment")
		}
	}
}

func TestLaTeXChunker_LargeSectionSplit(t *testing.T) {
	c := NewLaTeXChunker()

	// Create content with a section larger than max size
	var sb strings.Builder
	sb.WriteString("\\section{Large Section}\n\n")

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

	// All chunks should retain the section heading info
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document == nil || chunk.Metadata.Document.Heading != "Large Section" {
			t.Error("expected all chunks to retain section heading")
		}
	}
}

func TestLaTeXChunker_MathEnvironmentNotSplit(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\section{Math}

Text before.

\begin{equation}
a = 1

b = 2

c = 3
\end{equation}

Text after.
`
	// Use a small max size that would normally split the content
	opts := ChunkOptions{
		MaxChunkSize: 50,
	}

	result, err := c.Chunk(context.Background(), []byte(content), opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The equation environment should be kept together
	foundEquation := false
	for _, chunk := range result.Chunks {
		if strings.Contains(chunk.Content, "\\begin{equation}") {
			// This chunk should contain the entire equation
			if !strings.Contains(chunk.Content, "\\end{equation}") {
				t.Error("equation environment should not be split")
			}
			foundEquation = true
		}
	}

	if !foundEquation {
		t.Error("expected to find equation environment")
	}
}

func TestLaTeXChunker_ContextCancellation(t *testing.T) {
	c := NewLaTeXChunker()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	content := `\section{Test}

Content.
`
	_, err := c.Chunk(ctx, []byte(content), DefaultChunkOptions())

	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestLaTeXChunker_SampleFile(t *testing.T) {
	// Read the sample LaTeX file
	content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "markup", "sample.tex"))
	if err != nil {
		t.Skipf("sample.tex not found: %v", err)
	}

	c := NewLaTeXChunker()
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

	// Verify equations are preserved
	hasEquation := false
	for _, chunk := range result.Chunks {
		if strings.Contains(chunk.Content, "\\begin{equation}") {
			hasEquation = true
			break
		}
	}
	if !hasEquation {
		t.Error("expected to find equation environment in sample")
	}

	// Verify metadata type
	if result.Chunks[0].Metadata.Type != ChunkTypeProse {
		t.Errorf("expected type 'prose', got %q", result.Chunks[0].Metadata.Type)
	}
}

func TestLaTeXChunker_NoSections(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\documentclass{article}
\begin{document}

This is just plain content.

It has multiple paragraphs.

But no sections at all.

\end{document}
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

func TestLaTeXChunker_ParagraphAndSubparagraph(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\section{Main}

Main content.

\paragraph{First Paragraph}

Paragraph content.

\subparagraph{First Subparagraph}

Subparagraph content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	headings := make(map[string]int)
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			headings[chunk.Metadata.Document.Heading] = chunk.Metadata.Document.HeadingLevel
		}
	}

	// Check that paragraph and subparagraph are detected
	if _, ok := headings["First Paragraph"]; !ok {
		t.Error("expected to find 'First Paragraph'")
	}
	if _, ok := headings["First Subparagraph"]; !ok {
		t.Error("expected to find 'First Subparagraph'")
	}

	// Paragraph level should be 6, subparagraph should be 7
	if headings["First Paragraph"] != 6 {
		t.Errorf("expected paragraph level 6, got %d", headings["First Paragraph"])
	}
	if headings["First Subparagraph"] != 7 {
		t.Errorf("expected subparagraph level 7, got %d", headings["First Subparagraph"])
	}
}
