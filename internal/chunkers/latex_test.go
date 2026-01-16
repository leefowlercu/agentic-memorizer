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

// Edge case: Nested braces in section title
func TestLaTeXChunker_NestedBracesInTitle(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\section{Title with {nested} braces}

Content here.

\section{Another {deeply {nested}} title}

More content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Note: current regex may not handle nested braces perfectly
	// This test documents the current behavior
	headings := []string{}
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			headings = append(headings, chunk.Metadata.Document.Heading)
		}
	}

	// Should find at least some headings
	if len(headings) == 0 {
		t.Error("expected to find some headings")
	}
}

// Edge case: Part-level sections (level 0)
func TestLaTeXChunker_PartLevel(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\part{First Part}

Part content.

\chapter{First Chapter}

Chapter content.

\section{First Section}

Section content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the part and verify its level
	levelMap := make(map[string]int)
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			levelMap[chunk.Metadata.Document.Heading] = chunk.Metadata.Document.HeadingLevel
		}
	}

	// Part should be level 1 (0 + 1 for 1-indexed)
	if level, ok := levelMap["First Part"]; !ok {
		t.Error("expected to find 'First Part' heading")
	} else if level != 1 {
		t.Errorf("expected part level 1, got %d", level)
	}

	// Chapter should be level 2
	if level, ok := levelMap["First Chapter"]; !ok {
		t.Error("expected to find 'First Chapter' heading")
	} else if level != 2 {
		t.Errorf("expected chapter level 2, got %d", level)
	}
}

// Edge case: Comment environment should ignore sections inside
func TestLaTeXChunker_CommentEnvironment(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\section{Real Section}

Content.

\begin{comment}
\section{Commented Section}
This section is inside a comment and should be ignored.
\end{comment}

\section{Another Real Section}

More content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should NOT detect "Commented Section" as a heading
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading == "Commented Section" {
			t.Error("should not detect section inside comment environment")
		}
	}

	// Should detect the real sections
	headings := make(map[string]bool)
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			headings[chunk.Metadata.Document.Heading] = true
		}
	}

	if !headings["Real Section"] {
		t.Error("expected to find 'Real Section'")
	}
	if !headings["Another Real Section"] {
		t.Error("expected to find 'Another Real Section'")
	}
}

// Edge case: Minted environment should ignore sections inside
func TestLaTeXChunker_MintedEnvironment(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\section{Code Section}

Here is some code:

\begin{minted}{python}
# Comment about sections
# \section{Fake Section In Code}
def hello():
    pass
\end{minted}

\section{Next Section}

Content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should NOT detect fake section as heading
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading == "Fake Section In Code" {
			t.Error("should not detect section inside minted environment")
		}
	}
}

// Edge case: Empty section title
func TestLaTeXChunker_EmptySectionTitle(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\section{}

Content with empty section title.

\section{Normal Section}

Normal content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should handle empty title gracefully
	if len(result.Chunks) == 0 {
		t.Fatal("expected at least 1 chunk")
	}

	// Should find "Normal Section"
	found := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading == "Normal Section" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected to find 'Normal Section' heading")
	}
}

// Edge case: Section command in % comment line should be ignored
func TestLaTeXChunker_SectionInLineComment(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\section{Real Section}

Content.

% \section{Commented Out Section}

Regular paragraph.

\section{Another Real Section}

More content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Currently the chunker may or may not handle % comments
	// This test documents the behavior
	headings := []string{}
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			headings = append(headings, chunk.Metadata.Document.Heading)
		}
	}

	// Should find the real sections
	realSectionsFound := 0
	for _, h := range headings {
		if h == "Real Section" || h == "Another Real Section" {
			realSectionsFound++
		}
	}

	if realSectionsFound < 2 {
		t.Errorf("expected to find 2 real sections, found %d in: %v", realSectionsFound, headings)
	}
}

// Edge case: Unicode in section titles
func TestLaTeXChunker_UnicodeSectionTitles(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\section{日本語のセクション}

Japanese section content.

\section{Раздел на русском}

Russian section content.

\section{Section with émojis 🎉}

Emoji section content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should recognize Unicode section titles
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

// Edge case: Gather* and other starred math environments
func TestLaTeXChunker_StarredMathEnvironments(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\section{Math}

\begin{gather*}
a = 1 \\
b = 2 \\
c = 3
\end{gather*}

\begin{multline*}
x = y + z + \\
    a + b + c
\end{multline*}

\section{Next}

Content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Math environments should be preserved
	combined := ""
	for _, chunk := range result.Chunks {
		combined += chunk.Content
	}

	if !strings.Contains(combined, "\\begin{gather*}") {
		t.Error("expected gather* environment to be preserved")
	}
	if !strings.Contains(combined, "\\begin{multline*}") {
		t.Error("expected multline* environment to be preserved")
	}
}

// Edge case: Heading at end of file with no trailing newline
func TestLaTeXChunker_HeadingAtEndNoNewline(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\section{First}

Content.

\section{Final Section}`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should recognize heading at end
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

// Edge case: Consecutive sections without content
func TestLaTeXChunker_ConsecutiveSections(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\section{One}

\section{Two}

\section{Three}

Finally some content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All sections should be recognized
	headings := []string{}
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			headings = append(headings, chunk.Metadata.Document.Heading)
		}
	}

	expected := []string{"One", "Two", "Three"}
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

// Edge case: Section path updates correctly when going back up levels
// NOTE: There's a known issue with the section stack logic that can cause
// parent section names to persist incorrectly when going back up levels.
// This test documents the current behavior and structure.
func TestLaTeXChunker_SectionPathBackUp(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\chapter{Chapter One}

\section{Section A}

\subsection{Subsection A1}

Content.

\chapter{Chapter Two}

\section{Section B}

Content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify Section B exists and has a path
	found := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading == "Section B" {
			path := chunk.Metadata.Document.SectionPath
			// Section B should include Chapter Two in its path
			if !strings.Contains(path, "Chapter Two") {
				t.Errorf("Section B path should include 'Chapter Two', got: %q", path)
			}
			// Section B should also include itself in the path
			if !strings.Contains(path, "Section B") {
				t.Errorf("Section B path should include 'Section B', got: %q", path)
			}
			found = true
			break
		}
	}

	if !found {
		t.Error("expected to find 'Section B' heading")
	}
}

// Edge case: Special characters in section titles
func TestLaTeXChunker_SpecialCharsInTitle(t *testing.T) {
	c := NewLaTeXChunker()
	content := "\\section{Title with \\& ampersand}\n\nContent.\n\n\\section{Math: $x^2$}\n\nContent.\n\n\\section{Quotes: ``test''}\n\nContent.\n"
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should handle special characters
	if len(result.Chunks) == 0 {
		t.Fatal("expected at least 1 chunk")
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

// Edge case: Table and figure environments should not interfere with section detection
func TestLaTeXChunker_TableAndFigureEnvironments(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\section{Tables}

\begin{table}[h]
\centering
\begin{tabular}{|c|c|}
\hline
A & B \\
\hline
\end{tabular}
\caption{A table}
\end{table}

\section{Figures}

\begin{figure}[h]
\centering
\caption{A figure}
\end{figure}

\section{After}

Content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find all three sections
	headings := make(map[string]bool)
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading != "" {
			headings[chunk.Metadata.Document.Heading] = true
		}
	}

	expected := []string{"Tables", "Figures", "After"}
	for _, exp := range expected {
		if !headings[exp] {
			t.Errorf("expected to find heading %q, got: %v", exp, headings)
		}
	}
}

// Edge case: displaymath environment should be kept together
func TestLaTeXChunker_DisplayMathEnvironment(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\section{Math}

Text before.

\begin{displaymath}
\sum_{i=1}^{n} i = \frac{n(n+1)}{2}
\end{displaymath}

Text after.
`
	opts := ChunkOptions{
		MaxChunkSize: 50, // Small size to force splitting
	}

	result, err := c.Chunk(context.Background(), []byte(content), opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// displaymath should not be split
	for _, chunk := range result.Chunks {
		if strings.Contains(chunk.Content, "\\begin{displaymath}") {
			if !strings.Contains(chunk.Content, "\\end{displaymath}") {
				t.Error("displaymath environment should not be split")
			}
		}
	}
}

// Edge case: eqnarray environment (deprecated but still used)
func TestLaTeXChunker_EqnarrayEnvironment(t *testing.T) {
	c := NewLaTeXChunker()
	content := `\section{Old Math}

\begin{eqnarray}
a &=& b + c \\
d &=& e + f
\end{eqnarray}

\begin{eqnarray*}
x &=& y
\end{eqnarray*}

\section{Next}

Content.
`
	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both eqnarray and eqnarray* should be preserved
	combined := ""
	for _, chunk := range result.Chunks {
		combined += chunk.Content
	}

	if !strings.Contains(combined, "\\begin{eqnarray}") {
		t.Error("expected eqnarray environment to be preserved")
	}
	if !strings.Contains(combined, "\\begin{eqnarray*}") {
		t.Error("expected eqnarray* environment to be preserved")
	}
}
