package handlers

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// UnsupportedHandler is the fallback handler for unsupported file types.
// It extracts basic metadata only and skips semantic analysis.
type UnsupportedHandler struct{}

// NewUnsupportedHandler creates a new UnsupportedHandler.
func NewUnsupportedHandler() *UnsupportedHandler {
	return &UnsupportedHandler{}
}

// Name returns the handler's unique identifier.
func (h *UnsupportedHandler) Name() string {
	return "unsupported"
}

// CanHandle always returns true as this is the fallback handler.
func (h *UnsupportedHandler) CanHandle(mimeType string, ext string) bool {
	// This handler accepts everything as it's the fallback
	return true
}

// Extract extracts basic metadata from the file.
func (h *UnsupportedHandler) Extract(ctx context.Context, path string, size int64) (*ExtractedContent, error) {
	// Check context
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	metadata := h.extractMetadata(path, size)

	return &ExtractedContent{
		Handler:      h.Name(),
		Metadata:     metadata,
		SkipAnalysis: true,
	}, nil
}

// MaxSize returns 0 as there's no limit for metadata-only extraction.
func (h *UnsupportedHandler) MaxSize() int64 {
	return 0
}

// RequiresVision returns false as unsupported files don't use vision.
func (h *UnsupportedHandler) RequiresVision() bool {
	return false
}

// SupportedExtensions returns an empty slice as this handles everything.
func (h *UnsupportedHandler) SupportedExtensions() []string {
	return []string{}
}

// extractMetadata extracts basic file metadata.
func (h *UnsupportedHandler) extractMetadata(path string, size int64) *FileMetadata {
	ext := filepath.Ext(path)
	info, _ := os.Stat(path)
	var modTime time.Time
	if info != nil {
		modTime = info.ModTime()
	}

	mimeType := detectMIMEType(path, ext)

	return &FileMetadata{
		Path:      path,
		Size:      size,
		ModTime:   modTime,
		MIMEType:  mimeType,
		Extension: ext,
		Extra: map[string]any{
			"reason": "no handler available for this file type",
		},
	}
}
