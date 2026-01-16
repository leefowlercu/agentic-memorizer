package handlers

import (
	"mime"
	"path/filepath"
	"strings"
	"sync"
)

// Registry manages file handlers and selects the appropriate handler for files.
type Registry struct {
	mu       sync.RWMutex
	handlers []FileHandler
	fallback FileHandler
}

// NewRegistry creates a new handler registry with the given handlers.
// The last handler in the list is used as the fallback for unsupported files.
func NewRegistry(handlers ...FileHandler) *Registry {
	r := &Registry{
		handlers: make([]FileHandler, 0, len(handlers)),
	}

	for _, h := range handlers {
		r.Register(h)
	}

	return r
}

// Register adds a handler to the registry.
func (r *Registry) Register(h FileHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers = append(r.handlers, h)
}

// SetFallback sets the fallback handler for unsupported files.
func (r *Registry) SetFallback(h FileHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fallback = h
}

// GetHandler returns the appropriate handler for the given file.
// It tries to match by MIME type first, then by extension.
// Returns the fallback handler if no specific handler matches.
func (r *Registry) GetHandler(path string) FileHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ext := strings.ToLower(filepath.Ext(path))
	mimeType := detectMIMEType(path, ext)

	// Try to find a matching handler
	for _, h := range r.handlers {
		if h.CanHandle(mimeType, ext) {
			return h
		}
	}

	// Return fallback if set
	if r.fallback != nil {
		return r.fallback
	}

	// Return the last handler as implicit fallback
	if len(r.handlers) > 0 {
		return r.handlers[len(r.handlers)-1]
	}

	return nil
}

// GetHandlerByName returns a handler by its name.
func (r *Registry) GetHandlerByName(name string) FileHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, h := range r.handlers {
		if h.Name() == name {
			return h
		}
	}

	return nil
}

// ListHandlers returns all registered handlers.
func (r *Registry) ListHandlers() []FileHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]FileHandler, len(r.handlers))
	copy(result, r.handlers)
	return result
}

// detectMIMEType attempts to determine the MIME type from path and extension.
func detectMIMEType(_ string, ext string) string {
	// Try standard mime package first
	mimeType := mime.TypeByExtension(ext)
	if mimeType != "" {
		// Strip parameters like charset
		if idx := strings.Index(mimeType, ";"); idx != -1 {
			mimeType = strings.TrimSpace(mimeType[:idx])
		}
		return mimeType
	}

	// Fall back to custom mappings for common types
	return extensionToMIME(ext)
}

// extensionToMIME maps file extensions to MIME types for common programming files.
func extensionToMIME(ext string) string {
	mimeMap := map[string]string{
		// Programming languages
		".go":    "text/x-go",
		".py":    "text/x-python",
		".js":    "text/javascript",
		".ts":    "text/typescript",
		".tsx":   "text/typescript-jsx",
		".jsx":   "text/javascript-jsx",
		".rs":    "text/x-rust",
		".rb":    "text/x-ruby",
		".java":  "text/x-java",
		".kt":    "text/x-kotlin",
		".swift": "text/x-swift",
		".c":     "text/x-c",
		".cpp":   "text/x-c++",
		".h":     "text/x-c-header",
		".hpp":   "text/x-c++-header",
		".cs":    "text/x-csharp",
		".php":   "text/x-php",
		".scala": "text/x-scala",
		".clj":   "text/x-clojure",
		".ex":    "text/x-elixir",
		".exs":   "text/x-elixir",
		".erl":   "text/x-erlang",
		".hs":    "text/x-haskell",
		".lua":   "text/x-lua",
		".pl":    "text/x-perl",
		".r":     "text/x-r",
		".sql":   "text/x-sql",
		".sh":    "text/x-shellscript",
		".bash":  "text/x-shellscript",
		".zsh":   "text/x-shellscript",
		".fish":  "text/x-shellscript",
		".ps1":   "text/x-powershell",
		".vim":   "text/x-vim",
		".zig":   "text/x-zig",

		// Markup and config
		".md":         "text/markdown",
		".markdown":   "text/markdown",
		".rst":        "text/x-rst",
		".adoc":       "text/asciidoc",
		".tex":        "text/x-tex",
		".yaml":       "text/yaml",
		".yml":        "text/yaml",
		".toml":       "text/toml",
		".ini":        "text/ini",
		".cfg":        "text/ini",
		".conf":       "text/plain",
		".env":        "text/plain",
		".properties": "text/x-java-properties",

		// Data formats
		".json":   "application/json",
		".jsonl":  "application/x-ndjson",
		".ndjson": "application/x-ndjson",
		".csv":    "text/csv",
		".tsv":    "text/tab-separated-values",
		".xml":    "application/xml",

		// Documents
		".pdf":  "application/pdf",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".doc":  "application/msword",
		".xls":  "application/vnd.ms-excel",
		".ppt":  "application/vnd.ms-powerpoint",
		".odt":  "application/vnd.oasis.opendocument.text",
		".ods":  "application/vnd.oasis.opendocument.spreadsheet",
		".odp":  "application/vnd.oasis.opendocument.presentation",
		".rtf":  "application/rtf",

		// Images
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".svg":  "image/svg+xml",
		".ico":  "image/x-icon",
		".bmp":  "image/bmp",
		".tiff": "image/tiff",
		".tif":  "image/tiff",
		".heic": "image/heic",
		".heif": "image/heif",
		".avif": "image/avif",

		// Archives
		".zip": "application/zip",
		".tar": "application/x-tar",
		".gz":  "application/gzip",
		".tgz": "application/gzip",
		".bz2": "application/x-bzip2",
		".xz":  "application/x-xz",
		".7z":  "application/x-7z-compressed",
		".rar": "application/vnd.rar",
		".jar": "application/java-archive",
		".war": "application/java-archive",
		".ear": "application/java-archive",

		// Binary/executable
		".exe":   "application/x-executable",
		".dll":   "application/x-executable",
		".so":    "application/x-sharedlib",
		".dylib": "application/x-sharedlib",
		".a":     "application/x-archive",
		".o":     "application/x-object",
		".wasm":  "application/wasm",
	}

	if mimeType, ok := mimeMap[ext]; ok {
		return mimeType
	}

	return "application/octet-stream"
}

// IsTextMIME returns true if the MIME type represents text content.
func IsTextMIME(mimeType string) bool {
	if strings.HasPrefix(mimeType, "text/") {
		return true
	}

	textMIMEs := map[string]bool{
		"application/json":       true,
		"application/xml":        true,
		"application/javascript": true,
		"application/x-ndjson":   true,
		"application/yaml":       true,
	}

	return textMIMEs[mimeType]
}

// IsImageMIME returns true if the MIME type represents an image.
func IsImageMIME(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}

// IsArchiveMIME returns true if the MIME type represents an archive.
func IsArchiveMIME(mimeType string) bool {
	archiveMIMEs := map[string]bool{
		"application/zip":             true,
		"application/x-tar":           true,
		"application/gzip":            true,
		"application/x-bzip2":         true,
		"application/x-xz":            true,
		"application/x-7z-compressed": true,
		"application/vnd.rar":         true,
		"application/java-archive":    true,
	}

	return archiveMIMEs[mimeType]
}

// IsBinaryMIME returns true if the MIME type represents binary content.
func IsBinaryMIME(mimeType string) bool {
	if mimeType == "application/octet-stream" {
		return true
	}

	binaryMIMEs := map[string]bool{
		"application/x-executable": true,
		"application/x-sharedlib":  true,
		"application/x-archive":    true,
		"application/x-object":     true,
		"application/wasm":         true,
	}

	return binaryMIMEs[mimeType]
}
