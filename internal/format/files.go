package format

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// FilesContent wraps a FileIndex for formatting through the format package.
// It implements the Buildable interface to integrate with the format system.
type FilesContent struct {
	Index *types.FileIndex
}

// NewFilesContent creates a new FilesContent builder.
func NewFilesContent(index *types.FileIndex) *FilesContent {
	return &FilesContent{
		Index: index,
	}
}

// Type returns the builder type.
func (f *FilesContent) Type() BuilderType {
	return BuilderTypeFiles
}

// Validate checks if the FilesContent is valid.
func (f *FilesContent) Validate() error {
	if f.Index == nil {
		return fmt.Errorf("FileIndex cannot be nil")
	}
	return nil
}
