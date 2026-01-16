package chunkers

import (
	"context"
	"strings"
)

const (
	rstChunkerName     = "rst"
	rstChunkerPriority = 55
)

// RST underline characters in common usage order.
// Level assignment is based on first appearance in document.
var rstUnderlineChars = "=-~^\"'+`#*:._"

// RSTChunker splits reStructuredText content by section boundaries.
type RSTChunker struct{}

// NewRSTChunker creates a new RST chunker.
func NewRSTChunker() *RSTChunker {
	return &RSTChunker{}
}

// Name returns the chunker's identifier.
func (c *RSTChunker) Name() string {
	return rstChunkerName
}

// CanHandle returns true for RST content.
func (c *RSTChunker) CanHandle(mimeType string, language string) bool {
	return mimeType == "text/x-rst" ||
		mimeType == "text/restructuredtext" ||
		strings.HasSuffix(strings.ToLower(language), ".rst") ||
		strings.HasSuffix(strings.ToLower(language), ".rest")
}

// Priority returns the chunker's priority.
func (c *RSTChunker) Priority() int {
	return rstChunkerPriority
}

// Chunk splits RST content by section headings.
func (c *RSTChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	if len(content) == 0 {
		return &ChunkResult{
			Chunks:       []Chunk{},
			Warnings:     nil,
			TotalChunks:  0,
			ChunkerUsed:  rstChunkerName,
			OriginalSize: 0,
		}, nil
	}

	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	text := string(content)
	lines := strings.Split(text, "\n")

	// First pass: detect heading levels by underline character appearance order
	levelMap := c.buildLevelMap(lines)

	// Second pass: identify sections
	sections := c.splitBySections(lines, levelMap)

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
		ChunkerUsed:  rstChunkerName,
		OriginalSize: len(content),
	}, nil
}

// rstSection represents a detected section in RST content.
type rstSection struct {
	heading     string
	level       int
	content     string
	sectionPath string
}

// buildLevelMap scans lines and assigns levels based on underline character first appearance.
func (c *RSTChunker) buildLevelMap(lines []string) map[byte]int {
	levelMap := make(map[byte]int)
	currentLevel := 0

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Check if this line is an underline
		if c.isUnderline(line) {
			underlineChar := line[0]

			// Check if previous line could be a heading
			if i > 0 && c.isHeadingText(lines[i-1]) {
				headingLen := len(strings.TrimRight(lines[i-1], " \t"))
				underlineLen := len(strings.TrimRight(line, " \t"))

				// Underline must be at least as long as heading
				if underlineLen >= headingLen {
					// Also check for overline (same char on line before heading)
					hasOverline := false
					if i >= 2 && c.isUnderline(lines[i-2]) && lines[i-2][0] == underlineChar {
						hasOverline = true
					}

					// Assign level if not seen before
					key := underlineChar
					if hasOverline {
						// Overline+underline gets different level key
						key = byte(underlineChar + 128) // Use high bit to distinguish
					}

					if _, seen := levelMap[key]; !seen {
						currentLevel++
						levelMap[key] = currentLevel
					}
				}
			}
		}
	}

	return levelMap
}

// splitBySections splits lines into sections based on heading detection.
func (c *RSTChunker) splitBySections(lines []string, levelMap map[byte]int) []rstSection {
	var sections []rstSection
	var currentLines []string
	var currentHeading string
	var currentLevel int
	var sectionStack []string // Track heading hierarchy for section path

	flushSection := func() {
		if len(currentLines) > 0 {
			content := strings.Join(currentLines, "\n")
			if content != "" || currentHeading != "" {
				// Build section path from stack
				var sectionPath string
				if len(sectionStack) > 0 {
					sectionPath = strings.Join(sectionStack, " > ")
				}

				sections = append(sections, rstSection{
					heading:     currentHeading,
					level:       currentLevel,
					content:     content,
					sectionPath: sectionPath,
				})
			}
			currentLines = nil
		}
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Check for heading pattern: text followed by underline
		if i+1 < len(lines) && c.isUnderline(lines[i+1]) && c.isHeadingText(line) {
			underlineChar := lines[i+1][0]
			headingLen := len(strings.TrimRight(line, " \t"))
			underlineLen := len(strings.TrimRight(lines[i+1], " \t"))

			if underlineLen >= headingLen {
				// Check for overline
				hasOverline := false
				if i > 0 && c.isUnderline(lines[i-1]) && lines[i-1][0] == underlineChar {
					hasOverline = true
					// Remove overline from current content if present
					if len(currentLines) > 0 {
						currentLines = currentLines[:len(currentLines)-1]
					}
				}

				// Determine level
				key := underlineChar
				if hasOverline {
					key = byte(underlineChar + 128)
				}
				level := levelMap[key]

				// Flush previous section
				flushSection()

				// Update section stack
				heading := strings.TrimSpace(line)
				if level > 0 {
					// Pop deeper or same-level headings
					for len(sectionStack) >= level {
						sectionStack = sectionStack[:len(sectionStack)-1]
					}
					sectionStack = append(sectionStack, heading)
				}

				// Start new section
				currentHeading = heading
				currentLevel = level

				// Include heading line in content
				if hasOverline && i > 0 {
					currentLines = append(currentLines, lines[i-1])
				}
				currentLines = append(currentLines, line)
				currentLines = append(currentLines, lines[i+1])
				i++ // Skip underline

				continue
			}
		}

		currentLines = append(currentLines, line)
	}

	flushSection()
	return sections
}

// isUnderline checks if a line is a valid RST underline.
func (c *RSTChunker) isUnderline(line string) bool {
	trimmed := strings.TrimRight(line, " \t")
	if len(trimmed) < 2 {
		return false
	}

	// Check if all characters are the same valid underline character
	firstChar := trimmed[0]
	if !strings.ContainsRune(rstUnderlineChars, rune(firstChar)) {
		return false
	}

	for _, ch := range trimmed {
		if byte(ch) != firstChar {
			return false
		}
	}

	return true
}

// isHeadingText checks if a line could be heading text (not empty, not underline).
func (c *RSTChunker) isHeadingText(line string) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) == 0 {
		return false
	}

	// Heading text shouldn't start with certain directive characters
	if strings.HasPrefix(trimmed, "..") {
		return false
	}

	return true
}

// splitLargeSection splits a large section into smaller chunks.
func (c *RSTChunker) splitLargeSection(ctx context.Context, section rstSection, maxSize, baseOffset int) []Chunk {
	var chunks []Chunk

	// Split by blank lines (paragraphs)
	paragraphs := strings.Split(section.content, "\n\n")
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
