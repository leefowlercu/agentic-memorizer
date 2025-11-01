package output

import "github.com/leefowlercu/agentic-memorizer/pkg/types"

// OutputProcessor defines the interface for formatting memory index output.
// Each processor implements a specific output format (XML, Markdown, JSON)
// and is independent of integration-specific wrappers.
type OutputProcessor interface {
	// Format renders the index in the specific output format
	// Returns the formatted string representation of the index
	Format(index *types.Index) (string, error)

	// GetFormat returns the output format this processor implements
	GetFormat() string
}

// Options contains formatting options that can be applied to any output processor
type Options struct {
	// Verbose controls whether to include detailed information
	Verbose bool

	// ShowRecentDays limits recent activity section to this many days (0 = disabled)
	ShowRecentDays int
}

// DefaultOptions returns the default formatting options
func DefaultOptions() Options {
	return Options{
		Verbose:        false,
		ShowRecentDays: 0,
	}
}
