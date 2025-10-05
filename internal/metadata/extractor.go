package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// Extractor extracts metadata from files
type Extractor struct {
	handlers map[string]FileHandler
}

// FileHandler extracts metadata for a specific file type
type FileHandler interface {
	Extract(path string, info os.FileInfo) (*types.FileMetadata, error)
	CanHandle(ext string) bool
}

// NewExtractor creates a new metadata extractor
func NewExtractor() *Extractor {
	e := &Extractor{
		handlers: make(map[string]FileHandler),
	}

	// Register handlers
	e.RegisterHandler(&MarkdownHandler{})
	e.RegisterHandler(&DocxHandler{})
	e.RegisterHandler(&PptxHandler{})
	e.RegisterHandler(&PDFHandler{})
	e.RegisterHandler(&ImageHandler{})
	e.RegisterHandler(&VTTHandler{})
	e.RegisterHandler(&JSONHandler{})
	e.RegisterHandler(&CodeHandler{})

	return e
}

// RegisterHandler registers a file type handler
func (e *Extractor) RegisterHandler(handler FileHandler) {
	// Handler registers itself for extensions it can handle
	// This is a placeholder - handlers will implement CanHandle
	e.handlers[fmt.Sprintf("%T", handler)] = handler
}

// Extract extracts metadata from a file
func (e *Extractor) Extract(path string, info os.FileInfo) (*types.FileMetadata, error) {
	// Create base metadata
	metadata := &types.FileMetadata{
		FileInfo: types.FileInfo{
			Path:     path,
			Size:     info.Size(),
			Modified: info.ModTime(),
		},
	}

	// Determine file type and category
	ext := strings.ToLower(filepath.Ext(path))
	metadata.Type = ext
	metadata.Category = categorizeFile(ext)
	metadata.IsReadable = isReadable(ext)

	// Find appropriate handler
	for _, handler := range e.handlers {
		if handler.CanHandle(ext) {
			extracted, err := handler.Extract(path, info)
			if err != nil {
				// Log error but continue with base metadata
				fmt.Fprintf(os.Stderr, "Warning: failed to extract metadata from %s: %v\n", path, err)
				return metadata, nil
			}
			return extracted, nil
		}
	}

	// No specific handler, return base metadata
	return metadata, nil
}

// categorizeFile categorizes a file by extension
func categorizeFile(ext string) string {
	switch ext {
	case ".md", ".txt", ".doc", ".docx", ".pdf", ".rtf":
		return "documents"
	case ".pptx", ".ppt", ".key":
		return "presentations"
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".bmp", ".webp":
		return "images"
	case ".vtt", ".srt", ".sub":
		return "transcripts"
	case ".json", ".yaml", ".yml", ".toml", ".xml":
		return "data"
	case ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".rs", ".rb", ".php":
		return "code"
	case ".mp4", ".mov", ".avi", ".mkv", ".webm":
		return "videos"
	case ".mp3", ".wav", ".ogg", ".flac", ".m4a":
		return "audio"
	case ".zip", ".tar", ".gz", ".7z", ".rar":
		return "archives"
	default:
		return "other"
	}
}

// isReadable determines if Claude Code can read the file directly
func isReadable(ext string) bool {
	readableExts := map[string]bool{
		".md":   true,
		".txt":  true,
		".json": true,
		".yaml": true,
		".yml":  true,
		".toml": true,
		".xml":  true,
		".vtt":  true,
		".srt":  true,
		".go":   true,
		".py":   true,
		".js":   true,
		".ts":   true,
		".java": true,
		".c":    true,
		".cpp":  true,
		".rs":   true,
		".rb":   true,
		".php":  true,
		".html": true,
		".css":  true,
		".sh":   true,
		".bash": true,
		".png":  true, // Claude Code can read images
		".jpg":  true,
		".jpeg": true,
		".gif":  true,
		".webp": true,
	}

	return readableExts[ext]
}
