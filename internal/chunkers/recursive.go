package chunkers

import (
	"context"
	"strings"
)

const (
	recursiveChunkerName     = "recursive"
	recursiveChunkerPriority = 10
)

// Default separators in order of preference (largest to smallest boundaries).
var defaultSeparators = []string{
	"\n\n\n",  // Triple newline (section break)
	"\n\n",    // Double newline (paragraph)
	"\n",      // Single newline
	". ",      // Sentence end
	"? ",      // Question end
	"! ",      // Exclamation end
	"; ",      // Semicolon
	", ",      // Comma
	" ",       // Space
	"",        // Character level (last resort)
}

// RecursiveChunker splits content by progressively smaller separators.
type RecursiveChunker struct {
	separators []string
}

// NewRecursiveChunker creates a new recursive boundary chunker.
func NewRecursiveChunker() *RecursiveChunker {
	return &RecursiveChunker{
		separators: defaultSeparators,
	}
}

// Name returns the chunker's identifier.
func (c *RecursiveChunker) Name() string {
	return recursiveChunkerName
}

// CanHandle returns true for plain text content.
func (c *RecursiveChunker) CanHandle(mimeType string, language string) bool {
	return mimeType == "text/plain" || mimeType == ""
}

// Priority returns the chunker's priority.
func (c *RecursiveChunker) Priority() int {
	return recursiveChunkerPriority
}

// Chunk splits content recursively using separators.
func (c *RecursiveChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) ([]Chunk, error) {
	if len(content) == 0 {
		return []Chunk{}, nil
	}

	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	text := string(content)
	segments := c.splitRecursive(ctx, text, c.separators, maxSize)

	// Merge small segments and create chunks
	chunks := c.mergeSegments(ctx, segments, maxSize, opts.Overlap)

	return chunks, nil
}

// splitRecursive splits text using the first applicable separator.
func (c *RecursiveChunker) splitRecursive(ctx context.Context, text string, separators []string, maxSize int) []string {
	if len(text) <= maxSize {
		return []string{text}
	}

	if len(separators) == 0 {
		// Last resort: split by characters
		return c.splitBySize(text, maxSize)
	}

	sep := separators[0]
	remainingSeps := separators[1:]

	if sep == "" {
		return c.splitBySize(text, maxSize)
	}

	parts := strings.Split(text, sep)
	if len(parts) == 1 {
		// Separator not found; try next
		return c.splitRecursive(ctx, text, remainingSeps, maxSize)
	}

	var result []string
	for _, part := range parts {
		select {
		case <-ctx.Done():
			return result
		default:
		}

		if part == "" {
			continue
		}

		// Add separator back (except for space)
		if sep != " " && sep != "" {
			part = part + sep
		}

		if len(part) <= maxSize {
			result = append(result, strings.TrimRight(part, sep))
		} else {
			// Recursively split with smaller separators
			subParts := c.splitRecursive(ctx, part, remainingSeps, maxSize)
			result = append(result, subParts...)
		}
	}

	return result
}

// splitBySize splits text into fixed-size chunks.
func (c *RecursiveChunker) splitBySize(text string, maxSize int) []string {
	var result []string
	for len(text) > 0 {
		end := min(maxSize, len(text))
		result = append(result, text[:end])
		text = text[end:]
	}
	return result
}

// mergeSegments combines small segments and builds final chunks.
func (c *RecursiveChunker) mergeSegments(ctx context.Context, segments []string, maxSize int, overlap int) []Chunk {
	if len(segments) == 0 {
		return []Chunk{}
	}

	var chunks []Chunk
	var current strings.Builder
	currentStart := 0
	totalOffset := 0

segmentLoop:
	for _, seg := range segments {
		if ctx.Err() != nil {
			break segmentLoop
		}

		segLen := len(seg)

		// If adding this segment exceeds max, finalize current chunk
		if current.Len()+segLen > maxSize && current.Len() > 0 {
			content := current.String()
			chunks = append(chunks, Chunk{
				Index:       len(chunks),
				Content:     content,
				StartOffset: currentStart,
				EndOffset:   totalOffset,
				Metadata: ChunkMetadata{
					Type:          ChunkTypeProse,
					TokenEstimate: EstimateTokens(content),
				},
			})

			// Handle overlap
			if overlap > 0 && len(content) > overlap {
				overlapText := content[len(content)-overlap:]
				current.Reset()
				current.WriteString(overlapText)
				currentStart = totalOffset - overlap
			} else {
				current.Reset()
				currentStart = totalOffset
			}
		}

		current.WriteString(seg)
		if current.Len() > 0 && !strings.HasSuffix(current.String(), " ") {
			current.WriteString(" ")
		}
		totalOffset += segLen + 1
	}

	// Finalize last chunk
	if current.Len() > 0 {
		content := strings.TrimSpace(current.String())
		if content != "" {
			chunks = append(chunks, Chunk{
				Index:       len(chunks),
				Content:     content,
				StartOffset: currentStart,
				EndOffset:   totalOffset,
				Metadata: ChunkMetadata{
					Type:          ChunkTypeProse,
					TokenEstimate: EstimateTokens(content),
				},
			})
		}
	}

	return chunks
}
