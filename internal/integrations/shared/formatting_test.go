package shared

import (
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"zero", 0, "0 B"},
		{"bytes", 512, "512 B"},
		{"kilobytes", 1024, "1.0 KB"},
		{"megabytes", 1048576, "1.0 MB"},
		{"gigabytes", 1073741824, "1.0 GB"},
		{"fractional", 1536, "1.5 KB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSize(tt.bytes)
			if result != tt.expected {
				t.Errorf("FormatSize(%d) = %q, want %q", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestGroupByCategory(t *testing.T) {
	files := []types.FileEntry{
		{Path: "/a.txt", Category: "documents"},
		{Path: "/b.txt", Category: "documents"},
		{Path: "/c.go", Category: "code"},
	}

	result := GroupByCategory(files)

	if len(result["documents"]) != 2 {
		t.Errorf("Expected 2 documents, got %d", len(result["documents"]))
	}
	if len(result["code"]) != 1 {
		t.Errorf("Expected 1 code file, got %d", len(result["code"]))
	}
}

func TestJoin(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		sep      string
		expected string
	}{
		{"empty", []string{}, ", ", ""},
		{"single", []string{"a"}, ", ", "a"},
		{"multiple", []string{"a", "b", "c"}, ", ", "a, b, c"},
		{"newline sep", []string{"a", "b"}, "\n", "a\nb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Join(tt.parts, tt.sep)
			if result != tt.expected {
				t.Errorf("Join(%v, %q) = %q, want %q", tt.parts, tt.sep, result, tt.expected)
			}
		})
	}
}

func TestOutputFormatToString(t *testing.T) {
	tests := []struct {
		name        string
		format      integrations.OutputFormat
		expected    string
		expectError bool
	}{
		{
			name:        "xml format",
			format:      integrations.FormatXML,
			expected:    "xml",
			expectError: false,
		},
		{
			name:        "json format",
			format:      integrations.FormatJSON,
			expected:    "json",
			expectError: false,
		},
		{
			name:        "unsupported format",
			format:      integrations.OutputFormat("invalid"),
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := OutputFormatToString(tt.format)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("OutputFormatToString(%v) = %q, want %q", tt.format, result, tt.expected)
				}
			}
		})
	}
}
