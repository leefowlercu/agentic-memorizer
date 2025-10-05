package metadata

import (
	"os"
	"testing"
)

func TestMarkdownHandler_CanHandle(t *testing.T) {
	handler := &MarkdownHandler{}

	tests := []struct {
		name     string
		ext      string
		expected bool
	}{
		{"md", ".md", true},
		{"markdown", ".markdown", true},
		{"txt", ".txt", false},
		{"docx", ".docx", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.CanHandle(tt.ext)
			if result != tt.expected {
				t.Errorf("CanHandle(%q) = %v, want %v", tt.ext, result, tt.expected)
			}
		})
	}
}

func TestMarkdownHandler_Extract(t *testing.T) {
	handler := &MarkdownHandler{}

	tests := []struct {
		name            string
		path            string
		expectError     bool
		expectedType    string
		expectedCat     string
		expectedRead    bool
		minWordCount    int
		expectedSections []string
	}{
		{
			name:         "valid markdown file",
			path:         "../../testdata/sample.md",
			expectError:  false,
			expectedType: "markdown",
			expectedCat:  "documents",
			expectedRead: true,
			minWordCount: 50, // sample.md should have at least 50 words
			expectedSections: []string{
				"Sample Markdown Document",
				"Introduction",
				"Features",
				"Code Example",
				"Conclusion",
				"Subsection",
			},
		},
		{
			name:         "empty markdown file",
			path:         "../../testdata/empty.md",
			expectError:  false,
			expectedType: "markdown",
			expectedCat:  "documents",
			expectedRead: true,
			minWordCount: 0,
		},
		{
			name:        "nonexistent file",
			path:        "../../testdata/nonexistent.md",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := os.Stat(tt.path)
			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error getting file info, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Error getting file info: %v", err)
			}

			metadata, err := handler.Extract(tt.path, info)
			if err != nil {
				t.Fatalf("Extract failed: %v", err)
			}

			if metadata == nil {
				t.Fatal("Extract returned nil metadata")
			}

			if metadata.Type != tt.expectedType {
				t.Errorf("Type = %q, want %q", metadata.Type, tt.expectedType)
			}

			if metadata.Category != tt.expectedCat {
				t.Errorf("Category = %q, want %q", metadata.Category, tt.expectedCat)
			}

			if metadata.IsReadable != tt.expectedRead {
				t.Errorf("IsReadable = %v, want %v", metadata.IsReadable, tt.expectedRead)
			}

			if metadata.WordCount == nil {
				t.Error("WordCount should not be nil")
			} else if *metadata.WordCount < tt.minWordCount {
				t.Errorf("WordCount = %d, want at least %d", *metadata.WordCount, tt.minWordCount)
			}

			if tt.expectedSections != nil {
				if metadata.Sections == nil {
					t.Error("Sections should not be nil")
				} else if len(metadata.Sections) != len(tt.expectedSections) {
					t.Errorf("Expected %d sections, got %d", len(tt.expectedSections), len(metadata.Sections))
				} else {
					for i, section := range tt.expectedSections {
						if metadata.Sections[i] != section {
							t.Errorf("Section[%d] = %q, want %q", i, metadata.Sections[i], section)
						}
					}
				}
			}
		})
	}
}
