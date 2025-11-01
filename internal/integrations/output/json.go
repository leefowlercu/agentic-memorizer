package output

import (
	"encoding/json"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// JSONProcessor formats the memory index as JSON
// Note: This is a human-readable/agent-readable JSON format, distinct from
// the internal storage format. It includes pretty-printing and can be filtered.
type JSONProcessor struct {
	options Options
}

// NewJSONProcessor creates a new JSON output processor
func NewJSONProcessor(opts ...Options) *JSONProcessor {
	options := DefaultOptions()
	if len(opts) > 0 {
		options = opts[0]
	}

	return &JSONProcessor{
		options: options,
	}
}

// GetFormat returns the format name
func (p *JSONProcessor) GetFormat() string {
	return "json"
}

// Format renders the index as pretty-printed JSON
func (p *JSONProcessor) Format(index *types.Index) (string, error) {
	// The index already has JSON tags, so we can use it directly
	// However, we may want to apply filtering based on options

	var output any = index

	// If ShowRecentDays is set, we could create a filtered view
	// For now, we'll just return the full index as pretty-printed JSON
	if p.options.ShowRecentDays > 0 {
		// Create a filtered view with recent entries highlighted
		filteredIndex := &struct {
			*types.Index
			RecentEntries []types.IndexEntry `json:"recent_entries,omitempty"`
		}{
			Index:         index,
			RecentEntries: getRecentEntries(index.Entries, p.options.ShowRecentDays),
		}
		output = filteredIndex
	}

	// Pretty-print with 2-space indentation
	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}
