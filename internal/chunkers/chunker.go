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

// ChunkWarning represents a non-fatal parsing issue encountered during chunking.
type ChunkWarning struct {
	// Offset is the byte offset where the issue occurred in the original content.
	Offset int

	// Message is a human-readable description of the issue.
	Message string

	// Code is a machine-readable identifier for the warning type.
	Code string
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
		MaxChunkSize:      8000, // ~2000 tokens for most content
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
	// Returns ChunkResult containing chunks and any non-fatal warnings.
	// Errors are reserved for fatal failures (cannot parse at all, context canceled).
	Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error)

	// Priority returns the chunker's priority (higher = preferred).
	Priority() int
}

// ChunkResult contains the result of chunking an entire file.
type ChunkResult struct {
	// Chunks is the list of content chunks.
	Chunks []Chunk

	// Warnings contains non-fatal parsing issues encountered during chunking.
	// The caller decides how to handle/log these warnings.
	Warnings []ChunkWarning

	// TotalChunks is the total number of chunks.
	TotalChunks int

	// ChunkerUsed is the name of the chunker that produced these chunks.
	ChunkerUsed string

	// OriginalSize is the original content size in bytes.
	OriginalSize int
}
