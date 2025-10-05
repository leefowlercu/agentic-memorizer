package metadata

import (
	"os"
	"testing"
)

func TestPDFHandler_CanHandle(t *testing.T) {
	handler := &PDFHandler{}

	tests := []struct {
		name     string
		ext      string
		expected bool
	}{
		{"pdf", ".pdf", true},
		{"PDF uppercase", ".PDF", false}, // Extension should be lowercase
		{"docx", ".docx", false},
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

func TestPDFHandler_Extract(t *testing.T) {
	handler := &PDFHandler{}

	tests := []struct {
		name         string
		path         string
		expectError  bool
		expectedType string
		expectedCat  string
		expectedRead bool
	}{
		{
			name:         "valid pdf file",
			path:         "../../testdata/sample.pdf",
			expectError:  false,
			expectedType: "pdf",
			expectedCat:  "documents",
			expectedRead: false,
		},
		{
			name:        "nonexistent file",
			path:        "../../testdata/nonexistent.pdf",
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

			if metadata.Path != tt.path {
				t.Errorf("Path = %q, want %q", metadata.Path, tt.path)
			}

			// PDF handler is a stub, so these should be nil
			if metadata.PageCount != nil {
				t.Errorf("PageCount should be nil for stub handler, got %v", *metadata.PageCount)
			}

			if metadata.WordCount != nil {
				t.Errorf("WordCount should be nil for stub handler, got %v", *metadata.WordCount)
			}
		})
	}
}
