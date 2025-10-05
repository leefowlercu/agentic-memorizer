package metadata

import (
	"os"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// PDFHandler extracts metadata from PDF files
type PDFHandler struct{}

// CanHandle returns true if this handler can process the file
func (h *PDFHandler) CanHandle(ext string) bool {
	return ext == ".pdf"
}

// Extract extracts metadata from a PDF file
func (h *PDFHandler) Extract(path string, info os.FileInfo) (*types.FileMetadata, error) {
	metadata := &types.FileMetadata{
		FileInfo: types.FileInfo{
			Path:       path,
			Size:       info.Size(),
			Modified:   info.ModTime(),
			Type:       "pdf",
			Category:   "documents",
			IsReadable: false,
		},
	}

	// Note: Full PDF parsing requires external libraries
	// For now, we'll rely on semantic analysis
	// Could add github.com/pdfcpu/pdfcpu or github.com/ledongthuc/pdf later

	return metadata, nil
}
