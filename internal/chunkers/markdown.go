package chunkers

import (
	"context"
	"regexp"
	"strings"
)

const (
	markdownChunkerName     = "markdown"
	markdownChunkerPriority = 50
)

// Matches markdown headings (# to ######)
var headingRegex = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

// MarkdownChunker splits markdown content by sections.
type MarkdownChunker struct{}

// NewMarkdownChunker creates a new markdown chunker.
func NewMarkdownChunker() *MarkdownChunker {
	return &MarkdownChunker{}
}

// Name returns the chunker's identifier.
func (c *MarkdownChunker) Name() string {
	return markdownChunkerName
}

// CanHandle returns true for markdown content.
func (c *MarkdownChunker) CanHandle(mimeType string, language string) bool {
	return mimeType == "text/markdown" ||
		mimeType == "text/x-markdown" ||
		strings.HasSuffix(strings.ToLower(language), ".md")
}

// Priority returns the chunker's priority.
func (c *MarkdownChunker) Priority() int {
	return markdownChunkerPriority
}

// Chunk splits markdown content by headings.
func (c *MarkdownChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) ([]Chunk, error) {
	if len(content) == 0 {
		return []Chunk{}, nil
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

		heading, level := c.extractHeading(section)

		// If section is too large, split it further
		if len(section) > maxSize {
			subChunks := c.splitLargeSection(ctx, section, heading, level, maxSize, offset)
			for _, sc := range subChunks {
				sc.Index = len(chunks)
				chunks = append(chunks, sc)
			}
		} else if strings.TrimSpace(section) != "" {
			chunks = append(chunks, Chunk{
				Index:       len(chunks),
				Content:     section,
				StartOffset: offset,
				EndOffset:   offset + len(section),
				Metadata: ChunkMetadata{
					Type:          ChunkTypeMarkdown,
					Heading:       heading,
					HeadingLevel:  level,
					TokenEstimate: EstimateTokens(section),
				},
			})
		}

		offset += len(section)
	}

	return chunks, nil
}

// splitBySections splits markdown by top-level headings.
func (c *MarkdownChunker) splitBySections(text string) []string {
	lines := strings.Split(text, "\n")
	var sections []string
	var current strings.Builder
	inCodeBlock := false

	for _, line := range lines {
		// Track code blocks to avoid splitting inside them
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCodeBlock = !inCodeBlock
		}

		// Check for heading outside code block
		if !inCodeBlock && headingRegex.MatchString(line) && current.Len() > 0 {
			sections = append(sections, current.String())
			current.Reset()
		}

		current.WriteString(line)
		current.WriteString("\n")
	}

	if current.Len() > 0 {
		sections = append(sections, current.String())
	}

	return sections
}

// extractHeading extracts the heading text and level from a section.
func (c *MarkdownChunker) extractHeading(section string) (string, int) {
	lines := strings.SplitN(section, "\n", 2)
	if len(lines) == 0 {
		return "", 0
	}

	matches := headingRegex.FindStringSubmatch(lines[0])
	if matches == nil {
		return "", 0
	}

	level := len(matches[1])
	heading := strings.TrimSpace(matches[2])
	return heading, level
}

// splitLargeSection splits a large section into smaller chunks.
func (c *MarkdownChunker) splitLargeSection(ctx context.Context, section, heading string, level, maxSize, baseOffset int) []Chunk {
	var chunks []Chunk

	// Try to split by paragraphs first
	paragraphs := strings.Split(section, "\n\n")
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
					Type:          ChunkTypeMarkdown,
					Heading:       heading,
					HeadingLevel:  level,
					TokenEstimate: EstimateTokens(content),
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
				Type:          ChunkTypeMarkdown,
				Heading:       heading,
				HeadingLevel:  level,
				TokenEstimate: EstimateTokens(content),
			},
		})
	}

	return chunks
}
