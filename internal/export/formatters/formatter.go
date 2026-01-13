package formatters

import (
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
)

// Formatter formats a graph snapshot into a specific output format.
type Formatter interface {
	// Format converts the graph snapshot to the output format.
	Format(snapshot *graph.GraphSnapshot) ([]byte, error)

	// Name returns the formatter name.
	Name() string

	// ContentType returns the MIME content type.
	ContentType() string

	// FileExtension returns the typical file extension.
	FileExtension() string
}
