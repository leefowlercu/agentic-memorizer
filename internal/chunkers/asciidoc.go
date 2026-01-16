package chunkers

import (
	"context"
	"regexp"
	"strings"
)

const (
	asciidocChunkerName     = "asciidoc"
	asciidocChunkerPriority = 54
)

// Matches AsciiDoc headings with = prefix (1-6 equals signs).
var asciidocHeadingRegex = regexp.MustCompile(`^(={1,6})\s+(.+)$`)

// Matches AsciiDoc section anchors [[anchor-id]].
var asciidocAnchorRegex = regexp.MustCompile(`^\[\[([^\]]+)\]\]$`)

// AsciiDocChunker splits AsciiDoc content by section boundaries.
type AsciiDocChunker struct{}

// NewAsciiDocChunker creates a new AsciiDoc chunker.
func NewAsciiDocChunker() *AsciiDocChunker {
	return &AsciiDocChunker{}
}

// Name returns the chunker's identifier.
func (c *AsciiDocChunker) Name() string {
	return asciidocChunkerName
}

// CanHandle returns true for AsciiDoc content.
func (c *AsciiDocChunker) CanHandle(mimeType string, language string) bool {
	lang := strings.ToLower(language)
	return mimeType == "text/asciidoc" ||
		mimeType == "text/x-asciidoc" ||
		strings.HasSuffix(lang, ".adoc") ||
		strings.HasSuffix(lang, ".asciidoc") ||
		strings.HasSuffix(lang, ".asc")
}

// Priority returns the chunker's priority.
func (c *AsciiDocChunker) Priority() int {
	return asciidocChunkerPriority
}

// Chunk splits AsciiDoc content by section headings.
func (c *AsciiDocChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	if len(content) == 0 {
		return &ChunkResult{
			Chunks:       []Chunk{},
			Warnings:     nil,
			TotalChunks:  0,
			ChunkerUsed:  asciidocChunkerName,
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
		ChunkerUsed:  asciidocChunkerName,
		OriginalSize: len(content),
	}, nil
}

// asciidocSection represents a detected section in AsciiDoc content.
type asciidocSection struct {
	heading     string
	level       int
	content     string
	sectionPath string
	sectionID   string
}

// splitBySections splits AsciiDoc text into sections based on heading detection.
func (c *AsciiDocChunker) splitBySections(text string) []asciidocSection {
	lines := strings.Split(text, "\n")
	var sections []asciidocSection
	var currentLines []string
	var currentHeading string
	var currentLevel int
	var currentID string
	var sectionStack []string // Track heading hierarchy for section path
	var pendingAnchor string  // Anchor on previous line

	inSourceBlock := false

	flushSection := func() {
		if len(currentLines) > 0 {
			content := strings.Join(currentLines, "\n")
			if content != "" || currentHeading != "" {
				// Build section path from stack
				var sectionPath string
				if len(sectionStack) > 0 {
					sectionPath = strings.Join(sectionStack, " > ")
				}

				sections = append(sections, asciidocSection{
					heading:     currentHeading,
					level:       currentLevel,
					content:     content,
					sectionPath: sectionPath,
					sectionID:   currentID,
				})
			}
			currentLines = nil
		}
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Track source blocks to avoid false positives
		if strings.HasPrefix(trimmed, "----") || strings.HasPrefix(trimmed, "....") {
			inSourceBlock = !inSourceBlock
			currentLines = append(currentLines, line)
			continue
		}

		if inSourceBlock {
			currentLines = append(currentLines, line)
			continue
		}

		// Check for anchor
		if matches := asciidocAnchorRegex.FindStringSubmatch(trimmed); matches != nil {
			pendingAnchor = matches[1]
			currentLines = append(currentLines, line)
			continue
		}

		// Check for heading
		if matches := asciidocHeadingRegex.FindStringSubmatch(line); matches != nil {
			level := len(matches[1])
			heading := strings.TrimSpace(matches[2])

			// If there was a pending anchor, it belongs to this section, not the previous one
			// Remove it from currentLines before flushing
			var anchorLine string
			if pendingAnchor != "" && len(currentLines) > 0 {
				anchorLine = currentLines[len(currentLines)-1]
				currentLines = currentLines[:len(currentLines)-1]
			}

			// Flush previous section
			flushSection()

			// Add anchor line to the new section
			if anchorLine != "" {
				currentLines = append(currentLines, anchorLine)
			}

			// Update section stack
			// AsciiDoc: = is document title (0), == is level 1, etc.
			effectiveLevel := level
			if effectiveLevel > 0 {
				// Pop deeper or same-level headings
				for len(sectionStack) >= effectiveLevel {
					sectionStack = sectionStack[:len(sectionStack)-1]
				}
				sectionStack = append(sectionStack, heading)
			}

			// Start new section
			currentHeading = heading
			currentLevel = effectiveLevel
			currentID = pendingAnchor
			pendingAnchor = ""

			currentLines = append(currentLines, line)
			continue
		}

		pendingAnchor = "" // Clear anchor if not followed by heading
		currentLines = append(currentLines, line)
	}

	flushSection()
	return sections
}

// splitLargeSection splits a large section into smaller chunks.
func (c *AsciiDocChunker) splitLargeSection(ctx context.Context, section asciidocSection, maxSize, baseOffset int) []Chunk {
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
