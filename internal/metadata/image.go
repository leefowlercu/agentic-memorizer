package metadata

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"

	_ "golang.org/x/image/webp"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// ImageHandler extracts metadata from image files
type ImageHandler struct{}

// CanHandle returns true if this handler can process the file
func (h *ImageHandler) CanHandle(ext string) bool {
	return ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".webp"
}

// Extract extracts metadata from an image file
func (h *ImageHandler) Extract(path string, info os.FileInfo) (*types.FileMetadata, error) {
	metadata := &types.FileMetadata{
		FileInfo: types.FileInfo{
			Path:       path,
			Size:       info.Size(),
			Modified:   info.ModTime(),
			Type:       "image",
			Category:   "images",
			IsReadable: true, // Claude Code can read images
		},
	}

	// Open image to get dimensions
	file, err := os.Open(path)
	if err != nil {
		return metadata, err
	}
	defer file.Close()

	// Decode image config (doesn't load full image)
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return metadata, err
	}

	metadata.Dimensions = &types.ImageDim{
		Width:  config.Width,
		Height: config.Height,
	}

	return metadata, nil
}
