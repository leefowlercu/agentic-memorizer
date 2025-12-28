package prompts

import "strings"

// ExtractJSON extracts JSON content from markdown code blocks or plain text.
// Providers may wrap JSON responses in code blocks (```json ... ```).
// This function handles both wrapped and unwrapped JSON responses.
func ExtractJSON(text string) string {
	// Try to extract from ```json code block first
	if start := strings.Index(text, "```json"); start != -1 {
		start += 7
		if end := strings.Index(text[start:], "```"); end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	// Try to extract from generic ``` code block
	if start := strings.Index(text, "```"); start != -1 {
		start += 3
		if end := strings.Index(text[start:], "```"); end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	// No code block found, return empty string
	return ""
}
