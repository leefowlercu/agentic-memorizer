package format

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// FactsContent wraps a FactsIndex for formatting through the format package.
// It implements the Buildable interface to integrate with the format system.
type FactsContent struct {
	Index *types.FactsIndex
}

// NewFactsContent creates a new FactsContent builder.
func NewFactsContent(index *types.FactsIndex) *FactsContent {
	return &FactsContent{
		Index: index,
	}
}

// Type returns the builder type.
func (f *FactsContent) Type() BuilderType {
	return BuilderTypeFacts
}

// Validate checks if the FactsContent is valid.
func (f *FactsContent) Validate() error {
	if f.Index == nil {
		return fmt.Errorf("FactsIndex cannot be nil")
	}
	return nil
}
