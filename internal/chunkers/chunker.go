package chunkers

import (
	"context"
)

// ChunkType represents the type of content being chunked.
type ChunkType string

const (
	ChunkTypeCode       ChunkType = "code"
	ChunkTypeMarkdown   ChunkType = "markdown"
	ChunkTypeProse      ChunkType = "prose"
	ChunkTypeStructured ChunkType = "structured"
	ChunkTypeUnknown    ChunkType = "unknown"
)

// Chunk represents a segment of content for analysis.
type Chunk struct {
	// Index is the zero-based position in the sequence.
	Index int

	// Content is the chunk text.
	Content string

	// StartOffset is the byte offset where this chunk begins in the original.
	StartOffset int

	// EndOffset is the byte offset where this chunk ends in the original.
	EndOffset int

	// Metadata contains chunk-specific information.
	Metadata ChunkMetadata
}

// ChunkMetadata contains additional information about a chunk.
type ChunkMetadata struct {
	// Type indicates the content type of this chunk.
	Type ChunkType

	// Language is the programming language (for code chunks).
	Language string

	// Heading is the section heading (for markdown chunks).
	Heading string

	// HeadingLevel is the heading depth (1-6 for markdown).
	HeadingLevel int

	// FunctionName is the function/method name (for AST chunks).
	FunctionName string

	// ClassName is the class/struct name (for AST chunks).
	ClassName string

	// RecordIndex is the record number (for structured data chunks).
	RecordIndex int

	// TokenEstimate is an estimated token count for this chunk.
	TokenEstimate int
}

// ChunkOptions configures chunking behavior.
type ChunkOptions struct {
	// MaxChunkSize is the maximum size in bytes for a chunk.
	MaxChunkSize int

	// MaxTokens is the target maximum tokens per chunk.
	MaxTokens int

	// Overlap is the number of bytes to overlap between chunks.
	Overlap int

	// Language is the programming language hint for code content.
	Language string

	// MIMEType is the content MIME type.
	MIMEType string

	// PreserveStructure attempts to keep logical units together.
	PreserveStructure bool
}

// DefaultChunkOptions returns sensible default chunking options.
func DefaultChunkOptions() ChunkOptions {
	return ChunkOptions{
		MaxChunkSize:      8000,  // ~2000 tokens for most content
		MaxTokens:         2000,
		Overlap:           200,
		PreserveStructure: true,
	}
}

// Chunker splits content into smaller pieces for analysis.
type Chunker interface {
	// Name returns the chunker's identifier.
	Name() string

	// CanHandle returns true if this chunker can process the given content type.
	CanHandle(mimeType string, language string) bool

	// Chunk splits content into chunks according to the options.
	Chunk(ctx context.Context, content []byte, opts ChunkOptions) ([]Chunk, error)

	// Priority returns the chunker's priority (higher = preferred).
	Priority() int
}

// ChunkResult contains the result of chunking an entire file.
type ChunkResult struct {
	// Chunks is the list of content chunks.
	Chunks []Chunk

	// TotalChunks is the total number of chunks.
	TotalChunks int

	// ChunkerUsed is the name of the chunker that produced these chunks.
	ChunkerUsed string

	// OriginalSize is the original content size in bytes.
	OriginalSize int
}

// EstimateTokens provides a rough token estimate for text.
// Uses a simple heuristic of ~4 characters per token for English text.
func EstimateTokens(text string) int {
	return (len(text) + 3) / 4
}
