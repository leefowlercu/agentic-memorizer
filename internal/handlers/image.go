package handlers

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"  // Register GIF format
	_ "image/jpeg" // Register JPEG format
	_ "image/png"  // Register PNG format
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "golang.org/x/image/webp" // Register WebP format
)

// DefaultMaxImageSize is the default maximum size for images (50MB).
const DefaultMaxImageSize = 50 * 1024 * 1024

// ImageHandler handles image files with optional vision API support.
type ImageHandler struct {
	maxSize   int64
	useVision bool
}

// ImageHandlerOption configures the ImageHandler.
type ImageHandlerOption func(*ImageHandler)

// WithMaxImageSize sets the maximum file size for image processing.
func WithMaxImageSize(size int64) ImageHandlerOption {
	return func(h *ImageHandler) {
		h.maxSize = size
	}
}

// WithVision enables vision API processing for images.
func WithVision(enabled bool) ImageHandlerOption {
	return func(h *ImageHandler) {
		h.useVision = enabled
	}
}

// NewImageHandler creates a new ImageHandler with the given options.
func NewImageHandler(opts ...ImageHandlerOption) *ImageHandler {
	h := &ImageHandler{
		maxSize:   DefaultMaxImageSize,
		useVision: true, // Default to using vision
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// Name returns the handler's unique identifier.
func (h *ImageHandler) Name() string {
	return "image"
}

// CanHandle returns true if this handler can process the given MIME type and extension.
func (h *ImageHandler) CanHandle(mimeType string, ext string) bool {
	// Check MIME type
	if IsImageMIME(mimeType) {
		return true
	}

	// Check common image extensions
	imageExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
		".svg":  true,
		".ico":  true,
		".bmp":  true,
		".tiff": true,
		".tif":  true,
		".heic": true,
		".heif": true,
		".avif": true,
	}

	return imageExts[strings.ToLower(ext)]
}

// Extract extracts content from the image file.
func (h *ImageHandler) Extract(ctx context.Context, path string, size int64) (*ExtractedContent, error) {
	// Check context
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Check file size
	if size > h.maxSize {
		return &ExtractedContent{
			Handler:      h.Name(),
			SkipAnalysis: true,
			Error:        fmt.Sprintf("image too large: %d bytes (max %d)", size, h.maxSize),
			Metadata:     h.extractMetadata(path, size, nil),
		}, nil
	}

	// Open file to get image dimensions
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open image; %w", err)
	}
	defer file.Close()

	// Decode image config (dimensions without loading full image)
	config, format, err := image.DecodeConfig(file)
	var dimensions *ImageDimensions
	if err == nil {
		dimensions = &ImageDimensions{
			Width:  config.Width,
			Height: config.Height,
		}
	}

	metadata := h.extractMetadata(path, size, dimensions)
	if format != "" {
		metadata.Extra = map[string]any{
			"format": format,
		}
	}

	result := &ExtractedContent{
		Handler:  h.Name(),
		Metadata: metadata,
	}

	// If vision is enabled, prepare vision content
	if h.useVision {
		// Read the image data for vision API
		imageData, err := os.ReadFile(path)
		if err != nil {
			result.Error = fmt.Sprintf("failed to read image for vision; %v", err)
			result.SkipAnalysis = true
			return result, nil
		}

		result.VisionContent = &VisionContent{
			ImageData: imageData,
			MIMEType:  metadata.MIMEType,
		}
		if dimensions != nil {
			result.VisionContent.Width = dimensions.Width
			result.VisionContent.Height = dimensions.Height
		}
	} else {
		// Without vision, we can only extract metadata
		result.SkipAnalysis = true
	}

	return result, nil
}

// MaxSize returns the maximum file size this handler will process.
func (h *ImageHandler) MaxSize() int64 {
	return h.maxSize
}

// RequiresVision returns true if vision API is enabled for this handler.
func (h *ImageHandler) RequiresVision() bool {
	return h.useVision
}

// SupportedExtensions returns the file extensions this handler supports.
func (h *ImageHandler) SupportedExtensions() []string {
	return []string{
		".jpg", ".jpeg", ".png", ".gif", ".webp",
		".svg", ".ico", ".bmp", ".tiff", ".tif",
		".heic", ".heif", ".avif",
	}
}

// extractMetadata extracts metadata from the image file.
func (h *ImageHandler) extractMetadata(path string, size int64, dimensions *ImageDimensions) *FileMetadata {
	ext := filepath.Ext(path)
	info, _ := os.Stat(path)
	var modTime time.Time
	if info != nil {
		modTime = info.ModTime()
	}

	return &FileMetadata{
		Path:            path,
		Size:            size,
		ModTime:         modTime,
		MIMEType:        detectMIMEType(path, ext),
		Extension:       ext,
		ImageDimensions: dimensions,
	}
}
