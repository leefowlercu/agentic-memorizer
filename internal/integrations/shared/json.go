package shared

import (
	"bytes"
	"encoding/json"
)

// MarshalIndentNoEscape marshals JSON with indentation but without HTML escaping.
// This prevents <, >, and & from being escaped to \u003c, \u003e, and \u0026.
// Unlike json.MarshalIndent which hard-codes escapeHTML: true, this function uses
// an Encoder with SetEscapeHTML(false) to produce cleaner output for CLI contexts.
func MarshalIndentNoEscape(v any, prefix, indent string) ([]byte, error) {
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
