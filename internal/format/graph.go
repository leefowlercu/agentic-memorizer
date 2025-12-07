package format

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// GraphContent wraps a GraphIndex for formatting through the format package.
// It implements the Buildable interface to integrate with the format system.
type GraphContent struct {
	Index *types.GraphIndex
}

// NewGraphContent creates a new GraphContent builder.
func NewGraphContent(index *types.GraphIndex) *GraphContent {
	return &GraphContent{
		Index: index,
	}
}

// Type returns the builder type.
func (g *GraphContent) Type() BuilderType {
	return BuilderTypeGraph
}

// Validate checks if the GraphContent is valid.
func (g *GraphContent) Validate() error {
	if g.Index == nil {
		return fmt.Errorf("GraphIndex cannot be nil")
	}
	return nil
}
