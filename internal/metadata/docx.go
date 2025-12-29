package metadata

import (
	"os"

	"github.com/leefowlercu/agentic-memorizer/internal/document"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// DocxHandler extracts metadata from DOCX files
type DocxHandler struct{}

// SupportedExtensions returns the list of extensions this handler supports
func (h *DocxHandler) SupportedExtensions() []string {
	return []string{".docx"}
}

// CanHandle returns true if this handler can process the file
func (h *DocxHandler) CanHandle(ext string) bool {
	return ext == ".docx"
}

// Extract extracts metadata from a DOCX file
func (h *DocxHandler) Extract(path string, info os.FileInfo) (*types.FileMetadata, error) {
	metadata := &types.FileMetadata{
		FileInfo: types.FileInfo{
			Path:       path,
			Size:       info.Size(),
			Modified:   info.ModTime(),
			Type:       "docx",
			Category:   "documents",
			IsReadable: false,
		},
	}

	docxMeta, err := document.ExtractDocxMetadata(path)
	if err != nil {
		return metadata, err
	}

	if docxMeta.WordCount > 0 {
		metadata.WordCount = &docxMeta.WordCount
	}

	if docxMeta.Author != "" {
		metadata.Author = &docxMeta.Author
	}

	return metadata, nil
}
