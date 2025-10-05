package metadata

import (
	"os"
	"testing"
)

func TestImageHandler_CanHandle(t *testing.T) {
	handler := &ImageHandler{}

	tests := []struct {
		name     string
		ext      string
		expected bool
	}{
		{"png", ".png", true},
		{"jpg", ".jpg", true},
		{"jpeg", ".jpeg", true},
		{"gif", ".gif", true},
		{"webp", ".webp", true},
		{"svg", ".svg", false},  // SVG not handled by ImageHandler
		{"bmp", ".bmp", false},  // BMP not handled by ImageHandler
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

func TestImageHandler_Extract(t *testing.T) {
	handler := &ImageHandler{}

	tests := []struct {
		name          string
		path          string
		expectError   bool
		expectedType  string
		expectedCat   string
		expectedRead  bool
		expectedWidth int
		expectedHeight int
	}{
		{
			name:           "png image",
			path:           "../../testdata/sample.png",
			expectError:    false,
			expectedType:   "image",
			expectedCat:    "images",
			expectedRead:   true,
			expectedWidth:  100,
			expectedHeight: 100,
		},
		{
			name:           "jpeg image",
			path:           "../../testdata/sample.jpg",
			expectError:    false,
			expectedType:   "image",
			expectedCat:    "images",
			expectedRead:   true,
			expectedWidth:  200,
			expectedHeight: 150,
		},
		{
			name:        "nonexistent file",
			path:        "../../testdata/nonexistent.png",
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

			if metadata.Dimensions == nil {
				t.Fatal("Dimensions should not be nil")
			}

			if metadata.Dimensions.Width != tt.expectedWidth {
				t.Errorf("Width = %d, want %d", metadata.Dimensions.Width, tt.expectedWidth)
			}

			if metadata.Dimensions.Height != tt.expectedHeight {
				t.Errorf("Height = %d, want %d", metadata.Dimensions.Height, tt.expectedHeight)
			}
		})
	}
}

func TestImageHandler_ExtractCorruptImage(t *testing.T) {
	handler := &ImageHandler{}

	// Use a text file as a corrupt image
	info, err := os.Stat("../../testdata/sample.md")
	if err != nil {
		t.Skip("sample.md not found, skipping test")
	}

	// Try to extract from a non-image file
	_, err = handler.Extract("../../testdata/sample.md", info)
	if err == nil {
		t.Error("Expected error when extracting from non-image file, got nil")
	}
}
