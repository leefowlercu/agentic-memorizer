package common

import "strings"

// ExtractJSON extracts JSON from code block wrappers in LLM responses.
// It handles both ```json and plain ``` code blocks.
func ExtractJSON(text string) string {
	// Try ```json first
	if start := strings.Index(text, "```json"); start != -1 {
		start += 7
		if end := strings.Index(text[start:], "```"); end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	// Fall back to plain ```
	if start := strings.Index(text, "```"); start != -1 {
		start += 3
		if end := strings.Index(text[start:], "```"); end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	return ""
}
