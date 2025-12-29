package metadata

import (
	"bufio"
	"os"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// MarkdownHandler extracts metadata from Markdown files
type MarkdownHandler struct{}

// SupportedExtensions returns the list of extensions this handler supports
func (h *MarkdownHandler) SupportedExtensions() []string {
	return []string{".md", ".markdown"}
}

// CanHandle returns true if this handler can process the file
func (h *MarkdownHandler) CanHandle(ext string) bool {
	return ext == ".md" || ext == ".markdown"
}

// Extract extracts metadata from a Markdown file
func (h *MarkdownHandler) Extract(path string, info os.FileInfo) (*types.FileMetadata, error) {
	metadata := &types.FileMetadata{
		FileInfo: types.FileInfo{
			Path:       path,
			Size:       info.Size(),
			Modified:   info.ModTime(),
			Type:       "markdown",
			Category:   "documents",
			IsReadable: true,
		},
	}

	// Read file
	file, err := os.Open(path)
	if err != nil {
		return metadata, err
	}
	defer file.Close()

	// Parse content
	scanner := bufio.NewScanner(file)
	wordCount := 0
	sections := []string{}

	for scanner.Scan() {
		line := scanner.Text()

		// Count words
		words := strings.Fields(line)
		wordCount += len(words)

		// Extract headings (sections)
		if strings.HasPrefix(line, "#") {
			heading := strings.TrimLeft(line, "# ")
			if heading != "" {
				sections = append(sections, heading)
			}
		}
	}

	metadata.WordCount = &wordCount
	if len(sections) > 0 {
		metadata.Sections = sections
	}

	return metadata, scanner.Err()
}
