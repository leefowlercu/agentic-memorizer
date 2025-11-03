package metadata

import (
	"os"
	"testing"
)

func TestDocxHandler_CanHandle(t *testing.T) {
	handler := &DocxHandler{}

	tests := []struct {
		name     string
		ext      string
		expected bool
	}{
		{"docx", ".docx", true},
		{"doc", ".doc", false}, // Not handled by DocxHandler
		{"txt", ".txt", false},
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

func TestDocxHandler_Extract(t *testing.T) {
	handler := &DocxHandler{}

	tests := []struct {
		name           string
		path           string
		expectError    bool
		expectedType   string
		expectedCat    string
		expectedRead   bool
		expectedAuthor string
		minWordCount   int
	}{
		{
			name:           "valid docx file",
			path:           "../../testdata/sample.docx",
			expectError:    false,
			expectedType:   "docx",
			expectedCat:    "documents",
			expectedRead:   false,
			expectedAuthor: "Test Author",
			minWordCount:   10, // sample.docx should have at least 10 words
		},
		{
			name:        "malformed docx file",
			path:        "../../testdata/malformed.docx",
			expectError: true,
		},
		{
			name:        "nonexistent file",
			path:        "../../testdata/nonexistent.docx",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := os.Stat(tt.path)
			if tt.expectError {
				if err != nil {
					// File doesn't exist, that's expected
					return
				}
				// File exists but should fail extraction
				metadata, err := handler.Extract(tt.path, info)
				if err == nil && metadata.Type == tt.expectedType {
					// Graceful degradation - should return base metadata even with error
					return
				}
				if err == nil {
					t.Error("Expected error extracting malformed file, got nil")
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

			if tt.expectedAuthor != "" {
				if metadata.Author == nil {
					t.Error("Author should not be nil")
				} else if *metadata.Author != tt.expectedAuthor {
					t.Errorf("Author = %q, want %q", *metadata.Author, tt.expectedAuthor)
				}
			}

			if tt.minWordCount > 0 {
				if metadata.WordCount == nil {
					t.Error("WordCount should not be nil")
				} else if *metadata.WordCount < tt.minWordCount {
					t.Errorf("WordCount = %d, want at least %d", *metadata.WordCount, tt.minWordCount)
				}
			}
		})
	}
}

func TestDocxHandler_ExtractCoreProps(t *testing.T) {
	// This is implicitly tested in TestDocxHandler_Extract
	// but we can add specific test if needed
	handler := &DocxHandler{}

	info, err := os.Stat("../../testdata/sample.docx")
	if err != nil {
		t.Skip("sample.docx not found, skipping test")
	}

	metadata, err := handler.Extract("../../testdata/sample.docx", info)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Verify author extraction worked
	if metadata.Author == nil {
		t.Error("Author extraction failed, Author is nil")
	}
}

func TestDocxHandler_ExtractWordCount(t *testing.T) {
	// This is implicitly tested in TestDocxHandler_Extract
	// but we can add specific test if needed
	handler := &DocxHandler{}

	info, err := os.Stat("../../testdata/sample.docx")
	if err != nil {
		t.Skip("sample.docx not found, skipping test")
	}

	metadata, err := handler.Extract("../../testdata/sample.docx", info)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Verify word count extraction worked
	if metadata.WordCount == nil {
		t.Error("WordCount extraction failed, WordCount is nil")
	} else if *metadata.WordCount == 0 {
		t.Error("WordCount should be greater than 0")
	}
}
