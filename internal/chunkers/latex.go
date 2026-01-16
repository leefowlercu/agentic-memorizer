package chunkers

import (
	"context"
	"regexp"
	"strings"
)

const (
	latexChunkerName     = "latex"
	latexChunkerPriority = 53
)

// LaTeX sectioning commands in hierarchical order.
// Levels: part=0, chapter=1, section=2, subsection=3, subsubsection=4, paragraph=5, subparagraph=6
var latexSectionLevels = map[string]int{
	"part":          0,
	"chapter":       1,
	"section":       2,
	"subsection":    3,
	"subsubsection": 4,
	"paragraph":     5,
	"subparagraph":  6,
}

// Matches LaTeX sectioning commands (with optional star for unnumbered).
var latexSectionRegex = regexp.MustCompile(`\\(part|chapter|section|subsection|subsubsection|paragraph|subparagraph)\*?\s*(?:\[[^\]]*\])?\s*\{([^}]*)\}`)

// Matches begin/end environment pairs.
var latexBeginEnvRegex = regexp.MustCompile(`\\begin\{([^}]+)\}`)
var latexEndEnvRegex = regexp.MustCompile(`\\end\{([^}]+)\}`)

// LaTeXChunker splits LaTeX content by sectioning commands.
type LaTeXChunker struct{}

// NewLaTeXChunker creates a new LaTeX chunker.
func NewLaTeXChunker() *LaTeXChunker {
	return &LaTeXChunker{}
}

// Name returns the chunker's identifier.
func (c *LaTeXChunker) Name() string {
	return latexChunkerName
}

// CanHandle returns true for LaTeX content.
func (c *LaTeXChunker) CanHandle(mimeType string, language string) bool {
	lang := strings.ToLower(language)
	return mimeType == "text/x-latex" ||
		mimeType == "text/x-tex" ||
		mimeType == "application/x-latex" ||
		mimeType == "application/x-tex" ||
		strings.HasSuffix(lang, ".tex") ||
		strings.HasSuffix(lang, ".latex") ||
		strings.HasSuffix(lang, ".ltx")
}

// Priority returns the chunker's priority.
func (c *LaTeXChunker) Priority() int {
	return latexChunkerPriority
}

// Chunk splits LaTeX content by sectioning commands.
func (c *LaTeXChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	if len(content) == 0 {
		return &ChunkResult{
			Chunks:       []Chunk{},
			Warnings:     nil,
			TotalChunks:  0,
			ChunkerUsed:  latexChunkerName,
			OriginalSize: 0,
		}, nil
	}

	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	text := string(content)
	sections := c.splitBySections(text)

	var chunks []Chunk
	offset := 0

	for _, section := range sections {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// If section is too large, split it further
		if len(section.content) > maxSize {
			subChunks := c.splitLargeSection(ctx, section, maxSize, offset)
			for _, sc := range subChunks {
				sc.Index = len(chunks)
				chunks = append(chunks, sc)
			}
		} else if strings.TrimSpace(section.content) != "" {
			chunks = append(chunks, Chunk{
				Index:       len(chunks),
				Content:     section.content,
				StartOffset: offset,
				EndOffset:   offset + len(section.content),
				Metadata: ChunkMetadata{
					Type:          ChunkTypeProse,
					TokenEstimate: EstimateTokens(section.content),
					Document: &DocumentMetadata{
						Heading:      section.heading,
						HeadingLevel: section.level,
						SectionPath:  section.sectionPath,
					},
				},
			})
		}

		offset += len(section.content)
	}

	return &ChunkResult{
		Chunks:       chunks,
		Warnings:     nil,
		TotalChunks:  len(chunks),
		ChunkerUsed:  latexChunkerName,
		OriginalSize: len(content),
	}, nil
}

// latexSection represents a detected section in LaTeX content.
type latexSection struct {
	heading     string
	level       int
	content     string
	sectionPath string
}

// splitBySections splits LaTeX text into sections based on sectioning commands.
func (c *LaTeXChunker) splitBySections(text string) []latexSection {
	var sections []latexSection
	var currentContent strings.Builder
	var currentHeading string
	var currentLevel int
	var sectionStack []string // Track heading hierarchy for section path

	// Track environment nesting
	envStack := []string{}

	lines := strings.Split(text, "\n")

	flushSection := func() {
		content := currentContent.String()
		if content != "" || currentHeading != "" {
			// Build section path from stack
			var sectionPath string
			if len(sectionStack) > 0 {
				sectionPath = strings.Join(sectionStack, " > ")
			}

			sections = append(sections, latexSection{
				heading:     currentHeading,
				level:       currentLevel,
				content:     content,
				sectionPath: sectionPath,
			})
		}
		currentContent.Reset()
	}

	for _, line := range lines {
		// Track environment nesting
		beginMatches := latexBeginEnvRegex.FindAllStringSubmatch(line, -1)
		for _, m := range beginMatches {
			envStack = append(envStack, m[1])
		}

		endMatches := latexEndEnvRegex.FindAllStringSubmatch(line, -1)
		for _, m := range endMatches {
			envName := m[1]
			// Pop matching environment from stack
			for i := len(envStack) - 1; i >= 0; i-- {
				if envStack[i] == envName {
					envStack = append(envStack[:i], envStack[i+1:]...)
					break
				}
			}
		}

		// Only look for section commands outside of certain environments
		// (e.g., don't split on section commands in verbatim, lstlisting, etc.)
		inProtectedEnv := false
		protectedEnvs := map[string]bool{
			"verbatim": true, "lstlisting": true, "comment": true, "minted": true,
		}
		for _, env := range envStack {
			if protectedEnvs[env] {
				inProtectedEnv = true
				break
			}
		}

		if !inProtectedEnv {
			if match := latexSectionRegex.FindStringSubmatch(line); match != nil {
				sectionType := match[1]
				heading := match[2]
				level := latexSectionLevels[sectionType]

				// Flush previous section
				flushSection()

				// Update section stack
				// For level L (0-indexed), keep only items at indices 0 to L-1 (first L items)
				// e.g., for \chapter (level=1), we want to keep only \part items (index 0) if any
				// This correctly handles going back up the hierarchy
				if len(sectionStack) > level {
					sectionStack = sectionStack[:level]
				}
				sectionStack = append(sectionStack, heading)

				// Start new section
				currentHeading = heading
				currentLevel = level + 1 // Convert to 1-indexed for consistency
			}
		}

		currentContent.WriteString(line)
		currentContent.WriteString("\n")
	}

	flushSection()
	return sections
}

// splitLargeSection splits a large section into smaller chunks.
// It keeps equations and other math environments together.
func (c *LaTeXChunker) splitLargeSection(ctx context.Context, section latexSection, maxSize, baseOffset int) []Chunk {
	var chunks []Chunk

	// Split by blank lines, but keep math environments together
	paragraphs := c.splitPreservingMath(section.content)
	var current strings.Builder
	offset := baseOffset

	for _, para := range paragraphs {
		select {
		case <-ctx.Done():
			return chunks
		default:
		}

		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		// If adding this paragraph exceeds max, finalize current chunk
		// But don't split in the middle of a math environment
		if current.Len()+len(para)+2 > maxSize && current.Len() > 0 {
			content := current.String()
			chunks = append(chunks, Chunk{
				Content:     content,
				StartOffset: offset - len(content),
				EndOffset:   offset,
				Metadata: ChunkMetadata{
					Type:          ChunkTypeProse,
					TokenEstimate: EstimateTokens(content),
					Document: &DocumentMetadata{
						Heading:      section.heading,
						HeadingLevel: section.level,
						SectionPath:  section.sectionPath,
					},
				},
			})
			current.Reset()
		}

		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(para)
		offset += len(para) + 2
	}

	// Finalize last chunk
	if current.Len() > 0 {
		content := current.String()
		chunks = append(chunks, Chunk{
			Content:     content,
			StartOffset: offset - len(content),
			EndOffset:   offset,
			Metadata: ChunkMetadata{
				Type:          ChunkTypeProse,
				TokenEstimate: EstimateTokens(content),
				Document: &DocumentMetadata{
					Heading:      section.heading,
					HeadingLevel: section.level,
					SectionPath:  section.sectionPath,
				},
			},
		})
	}

	return chunks
}

// splitPreservingMath splits content by blank lines while keeping math environments intact.
func (c *LaTeXChunker) splitPreservingMath(content string) []string {
	var result []string
	var current strings.Builder
	inMathEnv := false

	mathEnvs := map[string]bool{
		"equation": true, "equation*": true,
		"align": true, "align*": true,
		"gather": true, "gather*": true,
		"multline": true, "multline*": true,
		"eqnarray": true, "eqnarray*": true,
		"displaymath": true,
		"math":        true,
	}

	lines := strings.Split(content, "\n")
	var envStack []string

	for _, line := range lines {
		// Track math environment nesting
		beginMatches := latexBeginEnvRegex.FindAllStringSubmatch(line, -1)
		for _, m := range beginMatches {
			envName := m[1]
			envStack = append(envStack, envName)
			if mathEnvs[envName] {
				inMathEnv = true
			}
		}

		endMatches := latexEndEnvRegex.FindAllStringSubmatch(line, -1)
		for _, m := range endMatches {
			envName := m[1]
			// Pop matching environment
			for i := len(envStack) - 1; i >= 0; i-- {
				if envStack[i] == envName {
					envStack = append(envStack[:i], envStack[i+1:]...)
					break
				}
			}
		}

		// Update math env status
		inMathEnv = false
		for _, env := range envStack {
			if mathEnvs[env] {
				inMathEnv = true
				break
			}
		}

		// Check for blank line (paragraph break)
		if strings.TrimSpace(line) == "" && !inMathEnv && current.Len() > 0 {
			result = append(result, current.String())
			current.Reset()
			continue
		}

		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}
