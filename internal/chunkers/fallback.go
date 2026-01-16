package chunkers

import (
	"context"
)

const (
	fallbackChunkerName     = "fallback"
	fallbackChunkerPriority = 0 // Lowest priority
)

// FallbackChunker provides simple fixed-size chunking for any content type.
// Used when no specialized chunker is available.
type FallbackChunker struct{}

// NewFallbackChunker creates a new fallback chunker.
func NewFallbackChunker() *FallbackChunker {
	return &FallbackChunker{}
}

// Name returns the chunker's identifier.
func (c *FallbackChunker) Name() string {
	return fallbackChunkerName
}

// CanHandle returns true for any content type.
func (c *FallbackChunker) CanHandle(mimeType string, language string) bool {
	return true
}

// Priority returns the chunker's priority (lowest).
func (c *FallbackChunker) Priority() int {
	return fallbackChunkerPriority
}

// Chunk splits content into fixed-size chunks with overlap.
func (c *FallbackChunker) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	if len(content) == 0 {
		return &ChunkResult{
			Chunks:       []Chunk{},
			Warnings:     nil,
			TotalChunks:  0,
			ChunkerUsed:  fallbackChunkerName,
			OriginalSize: 0,
		}, nil
	}

	maxSize := opts.MaxChunkSize
	if maxSize <= 0 {
		maxSize = DefaultChunkOptions().MaxChunkSize
	}

	overlap := max(opts.Overlap, 0)
	if overlap >= maxSize {
		overlap = maxSize / 4
	}

	step := maxSize - overlap
	if step <= 0 {
		step = maxSize
	}

	var chunks []Chunk
	contentLen := len(content)

	for offset := 0; offset < contentLen; {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		end := min(offset+maxSize, contentLen)

		// Try to break at whitespace if possible
		if end < contentLen && end-offset > 100 {
			breakPoint := findBreakPoint(content, offset, end)
			if breakPoint > offset {
				end = breakPoint
			}
		}

		chunkContent := string(content[offset:end])
		chunk := Chunk{
			Index:       len(chunks),
			Content:     chunkContent,
			StartOffset: offset,
			EndOffset:   end,
			Metadata: ChunkMetadata{
				Type:          ChunkTypeUnknown,
				TokenEstimate: EstimateTokens(chunkContent),
			},
		}
		chunks = append(chunks, chunk)

		// Move to next position
		nextOffset := offset + step
		if nextOffset <= offset {
			nextOffset = offset + 1
		}
		if end >= contentLen {
			break
		}
		offset = nextOffset
	}

	return &ChunkResult{
		Chunks:       chunks,
		Warnings:     nil,
		TotalChunks:  len(chunks),
		ChunkerUsed:  fallbackChunkerName,
		OriginalSize: contentLen,
	}, nil
}

// findBreakPoint finds a good break point (whitespace) near the end of the range.
func findBreakPoint(content []byte, start, end int) int {
	// Search backwards from end for whitespace
	searchStart := max(end-100, start)

	for i := end - 1; i >= searchStart; i-- {
		if isWhitespace(content[i]) {
			return i + 1
		}
	}

	return end
}

// isWhitespace returns true if the byte is a whitespace character.
func isWhitespace(b byte) bool {
	return b == ' ' || b == '\n' || b == '\r' || b == '\t'
}
