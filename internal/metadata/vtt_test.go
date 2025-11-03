package metadata

import (
	"os"
	"testing"
)

func TestVTTHandler_CanHandle(t *testing.T) {
	handler := &VTTHandler{}

	tests := []struct {
		name     string
		ext      string
		expected bool
	}{
		{"vtt", ".vtt", true},
		{"srt", ".srt", true},
		{"sub", ".sub", false}, // Not handled by VTTHandler based on code
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

func TestVTTHandler_Extract(t *testing.T) {
	handler := &VTTHandler{}

	tests := []struct {
		name             string
		path             string
		expectError      bool
		expectedType     string
		expectedCat      string
		expectedRead     bool
		expectedDuration string
	}{
		{
			name:             "valid vtt file",
			path:             "../../testdata/sample.vtt",
			expectError:      false,
			expectedType:     "transcript",
			expectedCat:      "transcripts",
			expectedRead:     true,
			expectedDuration: "00:00:23.750",
		},
		{
			name:        "nonexistent file",
			path:        "../../testdata/nonexistent.vtt",
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

			if tt.expectedDuration != "" {
				if metadata.Duration == nil {
					t.Error("Duration should not be nil")
				} else if *metadata.Duration != tt.expectedDuration {
					t.Errorf("Duration = %q, want %q", *metadata.Duration, tt.expectedDuration)
				}
			}
		})
	}
}

func TestVTTHandler_ExtractEmptyFile(t *testing.T) {
	handler := &VTTHandler{}

	info, err := os.Stat("../../testdata/empty.md")
	if err != nil {
		t.Skip("empty.md not found, skipping test")
	}

	metadata, err := handler.Extract("../../testdata/empty.md", info)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Empty file should have no duration
	if metadata.Duration != nil {
		t.Errorf("Expected nil Duration for empty file, got %v", *metadata.Duration)
	}
}
