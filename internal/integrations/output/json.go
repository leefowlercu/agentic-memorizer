package output

import (
	"bytes"
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

	// Pretty-print with 2-space indentation and no HTML escaping
	jsonBytes, err := marshalIndentNoEscape(output, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

// marshalIndentNoEscape marshals JSON with indentation but without HTML escaping.
// This prevents <, >, and & from being escaped to \u003c, \u003e, and \u0026.
// Unlike json.MarshalIndent which hard-codes escapeHTML: true, this function uses
// an Encoder with SetEscapeHTML(false) to produce cleaner output for CLI contexts.
func marshalIndentNoEscape(v any, prefix, indent string) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent(prefix, indent)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	// Encoder.Encode adds a trailing newline, trim it to match MarshalIndent behavior
	return bytes.TrimRight(buffer.Bytes(), "\n"), nil
}
