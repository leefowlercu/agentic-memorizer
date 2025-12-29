package common

import "testing"

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "json code block",
			input:    "Here is the analysis:\n```json\n{\"summary\": \"test\"}\n```",
			expected: `{"summary": "test"}`,
		},
		{
			name:     "plain code block",
			input:    "Here is the analysis:\n```\n{\"summary\": \"test\"}\n```",
			expected: `{"summary": "test"}`,
		},
		{
			name:     "no code block",
			input:    `{"summary": "test"}`,
			expected: "",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "unclosed code block",
			input:    "```json\n{\"summary\": \"test\"}",
			expected: "",
		},
		{
			name:     "json block with whitespace",
			input:    "```json\n  {\"key\": \"value\"}  \n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "multiple code blocks returns first",
			input:    "```json\n{\"first\": true}\n```\n\n```json\n{\"second\": true}\n```",
			expected: `{"first": true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractJSON(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractJSON() = %q, want %q", result, tt.expected)
			}
		})
	}
}
