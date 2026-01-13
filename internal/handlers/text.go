package handlers

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// DefaultMaxTextSize is the default maximum size for text files (10MB).
const DefaultMaxTextSize = 10 * 1024 * 1024

// TextHandler handles text-based files including code, prose, and configuration files.
type TextHandler struct {
	maxSize int64
}

// TextHandlerOption configures the TextHandler.
type TextHandlerOption func(*TextHandler)

// WithMaxTextSize sets the maximum file size for text processing.
func WithMaxTextSize(size int64) TextHandlerOption {
	return func(h *TextHandler) {
		h.maxSize = size
	}
}

// NewTextHandler creates a new TextHandler with the given options.
func NewTextHandler(opts ...TextHandlerOption) *TextHandler {
	h := &TextHandler{
		maxSize: DefaultMaxTextSize,
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// Name returns the handler's unique identifier.
func (h *TextHandler) Name() string {
	return "text"
}

// CanHandle returns true if this handler can process the given MIME type and extension.
func (h *TextHandler) CanHandle(mimeType string, ext string) bool {
	// Check MIME type
	if IsTextMIME(mimeType) {
		return true
	}

	// Check common text extensions that might not have correct MIME types
	textExts := map[string]bool{
		".txt":        true,
		".md":         true,
		".markdown":   true,
		".rst":        true,
		".adoc":       true,
		".go":         true,
		".py":         true,
		".js":         true,
		".ts":         true,
		".tsx":        true,
		".jsx":        true,
		".rs":         true,
		".rb":         true,
		".java":       true,
		".kt":         true,
		".swift":      true,
		".c":          true,
		".cpp":        true,
		".h":          true,
		".hpp":        true,
		".cs":         true,
		".php":        true,
		".scala":      true,
		".clj":        true,
		".ex":         true,
		".exs":        true,
		".erl":        true,
		".hs":         true,
		".lua":        true,
		".pl":         true,
		".r":          true,
		".sql":        true,
		".sh":         true,
		".bash":       true,
		".zsh":        true,
		".fish":       true,
		".ps1":        true,
		".vim":        true,
		".zig":        true,
		".yaml":       true,
		".yml":        true,
		".toml":       true,
		".ini":        true,
		".cfg":        true,
		".conf":       true,
		".env":        true,
		".properties": true,
		".gitignore":  true,
		".dockerignore": true,
		".editorconfig": true,
		".prettierrc":   true,
		".eslintrc":     true,
		".babelrc":      true,
		"Makefile":      true,
		"Dockerfile":    true,
		"Vagrantfile":   true,
		"Gemfile":       true,
		"Rakefile":      true,
		"Procfile":      true,
	}

	// Check extension
	if textExts[ext] {
		return true
	}

	// Check filename (for files without extension)
	filename := filepath.Base(ext) // ext might be the full filename for extensionless files
	if textExts[filename] {
		return true
	}

	return false
}

// Extract extracts content from the text file.
func (h *TextHandler) Extract(ctx context.Context, path string, size int64) (*ExtractedContent, error) {
	// Check context
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Check file size
	if size > h.maxSize {
		return &ExtractedContent{
			Handler:      h.Name(),
			SkipAnalysis: true,
			Error:        fmt.Sprintf("file too large: %d bytes (max %d)", size, h.maxSize),
			Metadata:     h.extractMetadata(path, size),
		}, nil
	}

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file; %w", err)
	}

	// Check if content is valid UTF-8
	if !utf8.Valid(content) {
		return &ExtractedContent{
			Handler:      h.Name(),
			SkipAnalysis: true,
			Error:        "file contains invalid UTF-8 content",
			Metadata:     h.extractMetadata(path, size),
		}, nil
	}

	textContent := string(content)

	// Extract metadata
	metadata := h.extractMetadata(path, size)
	metadata.LineCount = countLines(textContent)
	metadata.WordCount = countWords(textContent)
	metadata.Language = detectLanguage(filepath.Ext(path))
	metadata.Encoding = "utf-8"

	return &ExtractedContent{
		Handler:     h.Name(),
		TextContent: textContent,
		Metadata:    metadata,
	}, nil
}

// MaxSize returns the maximum file size this handler will process.
func (h *TextHandler) MaxSize() int64 {
	return h.maxSize
}

// RequiresVision returns false as text files don't need vision API.
func (h *TextHandler) RequiresVision() bool {
	return false
}

// SupportedExtensions returns the file extensions this handler supports.
func (h *TextHandler) SupportedExtensions() []string {
	return []string{
		".txt", ".md", ".markdown", ".rst", ".adoc",
		".go", ".py", ".js", ".ts", ".tsx", ".jsx",
		".rs", ".rb", ".java", ".kt", ".swift",
		".c", ".cpp", ".h", ".hpp", ".cs", ".php",
		".scala", ".clj", ".ex", ".exs", ".erl", ".hs",
		".lua", ".pl", ".r", ".sql",
		".sh", ".bash", ".zsh", ".fish", ".ps1",
		".vim", ".zig",
		".yaml", ".yml", ".toml", ".ini", ".cfg", ".conf", ".env", ".properties",
	}
}

// extractMetadata extracts basic file metadata.
func (h *TextHandler) extractMetadata(path string, size int64) *FileMetadata {
	ext := filepath.Ext(path)
	info, _ := os.Stat(path)
	var modTime time.Time
	if info != nil {
		modTime = info.ModTime()
	}

	return &FileMetadata{
		Path:      path,
		Size:      size,
		ModTime:   modTime,
		MIMEType:  detectMIMEType(path, ext),
		Extension: ext,
	}
}

// countLines counts the number of lines in the text.
func countLines(text string) int {
	if text == "" {
		return 0
	}

	count := 1
	for _, c := range text {
		if c == '\n' {
			count++
		}
	}

	// Don't count trailing newline as extra line
	if strings.HasSuffix(text, "\n") {
		count--
	}

	return count
}

// countWords counts the approximate number of words in the text.
func countWords(text string) int {
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Split(bufio.ScanWords)

	count := 0
	for scanner.Scan() {
		count++
	}

	return count
}

// detectLanguage returns the programming language based on file extension.
func detectLanguage(ext string) string {
	langMap := map[string]string{
		".go":     "Go",
		".py":     "Python",
		".js":     "JavaScript",
		".ts":     "TypeScript",
		".tsx":    "TypeScript React",
		".jsx":    "JavaScript React",
		".rs":     "Rust",
		".rb":     "Ruby",
		".java":   "Java",
		".kt":     "Kotlin",
		".swift":  "Swift",
		".c":      "C",
		".cpp":    "C++",
		".h":      "C Header",
		".hpp":    "C++ Header",
		".cs":     "C#",
		".php":    "PHP",
		".scala":  "Scala",
		".clj":    "Clojure",
		".ex":     "Elixir",
		".exs":    "Elixir",
		".erl":    "Erlang",
		".hs":     "Haskell",
		".lua":    "Lua",
		".pl":     "Perl",
		".r":      "R",
		".sql":    "SQL",
		".sh":     "Shell",
		".bash":   "Bash",
		".zsh":    "Zsh",
		".fish":   "Fish",
		".ps1":    "PowerShell",
		".vim":    "Vim Script",
		".zig":    "Zig",
		".md":     "Markdown",
		".yaml":   "YAML",
		".yml":    "YAML",
		".toml":   "TOML",
		".json":   "JSON",
		".xml":    "XML",
		".html":   "HTML",
		".css":    "CSS",
		".scss":   "SCSS",
		".sass":   "Sass",
		".less":   "Less",
	}

	if lang, ok := langMap[strings.ToLower(ext)]; ok {
		return lang
	}

	return ""
}
