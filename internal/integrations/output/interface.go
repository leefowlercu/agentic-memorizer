package output

import "github.com/leefowlercu/agentic-memorizer/pkg/types"

// GraphOutputProcessor defines the interface for formatting graph-native index output.
// Each processor implements a specific output format (XML, Markdown, JSON)
// and is independent of integration-specific wrappers.
type GraphOutputProcessor interface {
	// FormatGraph renders the graph index in the specific output format
	// Returns the formatted string representation of the graph index
	FormatGraph(index *types.GraphIndex) (string, error)

	// GetFormat returns the output format this processor implements
	GetFormat() string
}

// Options contains formatting options that can be applied to any output processor
type Options struct {
	// ShowRecentDays limits recent activity section to this many days (0 = disabled)
	ShowRecentDays int
}

// DefaultOptions returns the default formatting options
func DefaultOptions() Options {
	return Options{
		ShowRecentDays: 0,
	}
}
