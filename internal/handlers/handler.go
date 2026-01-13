// Package handlers provides file-specific content extraction for different file types.
package handlers

import (
	"context"
	"time"
)

// FileHandler defines the interface for file-specific content extraction.
type FileHandler interface {
	// Name returns the handler's unique identifier.
	Name() string

	// CanHandle returns true if this handler can process the given MIME type and extension.
	CanHandle(mimeType string, ext string) bool

	// Extract extracts content from the file at the given path.
	Extract(ctx context.Context, path string, size int64) (*ExtractedContent, error)

	// MaxSize returns the maximum file size this handler will process.
	// Files larger than this will be skipped or handled by a fallback.
	MaxSize() int64

	// RequiresVision returns true if this handler needs vision API for full analysis.
	RequiresVision() bool

	// SupportedExtensions returns the file extensions this handler supports.
	SupportedExtensions() []string
}

// ExtractedContent contains the content and metadata extracted from a file.
type ExtractedContent struct {
	// Handler is the name of the handler that extracted this content.
	Handler string

	// TextContent is the extracted text content for semantic analysis.
	// Empty for binary-only files like images.
	TextContent string

	// Metadata contains file-specific metadata.
	Metadata *FileMetadata

	// Chunks contains pre-chunked content if the handler performed chunking.
	// If nil, the analysis queue will chunk the TextContent.
	Chunks []ContentChunk

	// VisionContent contains image data for vision API processing.
	// Only populated if RequiresVision() returns true.
	VisionContent *VisionContent

	// SkipAnalysis indicates that semantic analysis should be skipped.
	// Used for binary files, archives, etc.
	SkipAnalysis bool

	// Error contains any non-fatal error that occurred during extraction.
	// The extraction may still have partial results.
	Error string
}

// FileMetadata contains metadata about the file.
type FileMetadata struct {
	// Path is the absolute path to the file.
	Path string

	// Size is the file size in bytes.
	Size int64

	// ModTime is the file modification time.
	ModTime time.Time

	// MIMEType is the detected MIME type.
	MIMEType string

	// Extension is the file extension (with leading dot).
	Extension string

	// Encoding is the detected text encoding (for text files).
	Encoding string

	// Language is the detected programming language (for code files).
	Language string

	// LineCount is the number of lines (for text files).
	LineCount int

	// Title is the document title (for documents with titles).
	Title string

	// Author is the document author (for documents with author metadata).
	Author string

	// CreatedAt is the document creation time (if available in metadata).
	CreatedAt *time.Time

	// PageCount is the number of pages (for documents).
	PageCount int

	// WordCount is the approximate word count.
	WordCount int

	// ImageDimensions contains width/height for images.
	ImageDimensions *ImageDimensions

	// Extra contains handler-specific metadata.
	Extra map[string]any
}

// ImageDimensions contains image size information.
type ImageDimensions struct {
	Width  int
	Height int
}

// ContentChunk represents a chunk of content for embeddings.
type ContentChunk struct {
	// Index is the chunk's position in the sequence.
	Index int

	// Content is the chunk text.
	Content string

	// StartLine is the starting line number (1-based) in the original file.
	StartLine int

	// EndLine is the ending line number (1-based) in the original file.
	EndLine int

	// Type describes the chunk type (e.g., "function", "class", "section").
	Type string

	// Name is an optional name for the chunk (e.g., function name).
	Name string
}

// VisionContent contains data for vision API processing.
type VisionContent struct {
	// ImageData is the raw image bytes.
	ImageData []byte

	// MIMEType is the image MIME type.
	MIMEType string

	// Width is the image width.
	Width int

	// Height is the image height.
	Height int
}

// HandlerCategory represents the category of file handler.
type HandlerCategory string

const (
	// CategoryText handles text-based files (code, prose, config).
	CategoryText HandlerCategory = "text"

	// CategoryImage handles image files.
	CategoryImage HandlerCategory = "image"

	// CategoryDocument handles rich documents (PDF, DOCX, etc.).
	CategoryDocument HandlerCategory = "document"

	// CategoryStructured handles structured data (JSON, CSV, YAML).
	CategoryStructured HandlerCategory = "structured"

	// CategoryArchive handles archive files (ZIP, TAR, etc.).
	CategoryArchive HandlerCategory = "archive"

	// CategoryUnsupported is the fallback for unsupported file types.
	CategoryUnsupported HandlerCategory = "unsupported"
)
