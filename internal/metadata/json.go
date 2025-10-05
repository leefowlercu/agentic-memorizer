package metadata

import (
	"os"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// JSONHandler extracts metadata from JSON/YAML files
type JSONHandler struct{}

// CanHandle returns true if this handler can process the file
func (h *JSONHandler) CanHandle(ext string) bool {
	return ext == ".json" || ext == ".yaml" || ext == ".yml" || ext == ".toml"
}

// Extract extracts metadata from a JSON/YAML file
func (h *JSONHandler) Extract(path string, info os.FileInfo) (*types.FileMetadata, error) {
	metadata := &types.FileMetadata{
		FileInfo: types.FileInfo{
			Path:       path,
			Size:       info.Size(),
			Modified:   info.ModTime(),
			Type:       "data",
			Category:   "data",
			IsReadable: true,
		},
	}

	// For JSON/YAML, we mostly rely on semantic analysis
	// Basic metadata is sufficient here

	return metadata, nil
}
