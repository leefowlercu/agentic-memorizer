package metadata

import (
	"os"
	"strings"
	"testing"
)

func TestPptxHandler_CanHandle(t *testing.T) {
	handler := &PptxHandler{}

	tests := []struct {
		name     string
		ext      string
		expected bool
	}{
		{"pptx", ".pptx", true},
		{"ppt", ".ppt", false},  // Not handled by PptxHandler
		{"key", ".key", false},
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

func TestPptxHandler_Extract(t *testing.T) {
	handler := &PptxHandler{}

	tests := []struct {
		name             string
		path             string
		expectError      bool
		expectedType     string
		expectedCat      string
		expectedRead     bool
		expectedAuthor   string
		expectedSlides   int
	}{
		{
			name:           "valid pptx file",
			path:           "../../testdata/sample.pptx",
			expectError:    false,
			expectedType:   "pptx",
			expectedCat:    "presentations",
			expectedRead:   false,
			expectedAuthor: "Presentation Author",
			expectedSlides: 3,
		},
		{
			name:        "nonexistent file",
			path:        "../../testdata/nonexistent.pptx",
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

			if tt.expectedAuthor != "" {
				if metadata.Author == nil {
					t.Error("Author should not be nil")
				} else if *metadata.Author != tt.expectedAuthor {
					t.Errorf("Author = %q, want %q", *metadata.Author, tt.expectedAuthor)
				}
			}

			if tt.expectedSlides > 0 {
				if metadata.SlideCount == nil {
					t.Error("SlideCount should not be nil")
				} else if *metadata.SlideCount != tt.expectedSlides {
					t.Errorf("SlideCount = %d, want %d", *metadata.SlideCount, tt.expectedSlides)
				}
			}
		})
	}
}

func TestPptxHandler_ExtractText(t *testing.T) {
	handler := &PptxHandler{}

	tests := []struct {
		name        string
		path        string
		expectError bool
		contains    []string
	}{
		{
			name:        "valid pptx file",
			path:        "../../testdata/sample.pptx",
			expectError: false,
			contains:    []string{"Slide 1", "Slide 2", "Slide 3"},
		},
		{
			name:        "nonexistent file",
			path:        "../../testdata/nonexistent.pptx",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, err := handler.ExtractText(tt.path)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ExtractText failed: %v", err)
			}

			for _, substr := range tt.contains {
				if !strings.Contains(text, substr) {
					t.Errorf("Expected text to contain %q, but it doesn't. Text: %q", substr, text)
				}
			}
		})
	}
}

func TestPptxHandler_ExtractCoreProps(t *testing.T) {
	// This is implicitly tested in TestPptxHandler_Extract
	// but we can add specific test if needed
	handler := &PptxHandler{}

	info, err := os.Stat("../../testdata/sample.pptx")
	if err != nil {
		t.Skip("sample.pptx not found, skipping test")
	}

	metadata, err := handler.Extract("../../testdata/sample.pptx", info)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Verify author extraction worked
	if metadata.Author == nil {
		t.Error("Author extraction failed, Author is nil")
	}
}
