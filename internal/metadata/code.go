package metadata

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// CodeHandler extracts metadata from source code files
type CodeHandler struct{}

// CanHandle returns true if this handler can process the file
func (h *CodeHandler) CanHandle(ext string) bool {
	codeExts := map[string]bool{
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
	}
	return codeExts[ext]
}

// Extract extracts metadata from a source code file
func (h *CodeHandler) Extract(path string, info os.FileInfo) (*types.FileMetadata, error) {
	ext := strings.ToLower(filepath.Ext(path))
	language := h.detectLanguage(ext)

	metadata := &types.FileMetadata{
		FileInfo: types.FileInfo{
			Path:       path,
			Size:       info.Size(),
			Modified:   info.ModTime(),
			Type:       "code",
			Category:   "code",
			IsReadable: true,
		},
		Language: &language,
	}

	// Count lines
	file, err := os.Open(path)
	if err != nil {
		return metadata, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}

	// Store line count as word count (repurposing the field)
	metadata.WordCount = &lineCount

	return metadata, scanner.Err()
}

// detectLanguage detects programming language from extension
func (h *CodeHandler) detectLanguage(ext string) string {
	languages := map[string]string{
		".go":   "Go",
		".py":   "Python",
		".js":   "JavaScript",
		".ts":   "TypeScript",
		".java": "Java",
		".c":    "C",
		".cpp":  "C++",
		".rs":   "Rust",
		".rb":   "Ruby",
		".php":  "PHP",
	}

	if lang, ok := languages[ext]; ok {
		return lang
	}
	return "Unknown"
}
