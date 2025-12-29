package metadata

import (
	"os"

	"github.com/leefowlercu/agentic-memorizer/internal/document"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// PptxHandler extracts metadata from PPTX files
type PptxHandler struct{}

// SupportedExtensions returns the list of extensions this handler supports
func (h *PptxHandler) SupportedExtensions() []string {
	return []string{".pptx"}
}

// CanHandle returns true if this handler can process the file
func (h *PptxHandler) CanHandle(ext string) bool {
	return ext == ".pptx"
}

// Extract extracts metadata from a PPTX file
func (h *PptxHandler) Extract(path string, info os.FileInfo) (*types.FileMetadata, error) {
	metadata := &types.FileMetadata{
		FileInfo: types.FileInfo{
			Path:       path,
			Size:       info.Size(),
			Modified:   info.ModTime(),
			Type:       "pptx",
			Category:   "presentations",
			IsReadable: false,
		},
	}

	pptxMeta, err := document.ExtractPptxMetadata(path)
	if err != nil {
		return metadata, err
	}

	if pptxMeta.SlideCount > 0 {
		metadata.SlideCount = &pptxMeta.SlideCount
	}

	if pptxMeta.Author != "" {
		metadata.Author = &pptxMeta.Author
	}

	return metadata, nil
}

// ExtractText extracts text content from all slides in a PPTX file
func (h *PptxHandler) ExtractText(path string) (string, error) {
	return document.ExtractPptxText(path)
}
