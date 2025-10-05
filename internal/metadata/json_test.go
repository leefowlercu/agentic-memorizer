package metadata

import (
	"os"
	"testing"
)

func TestJSONHandler_CanHandle(t *testing.T) {
	handler := &JSONHandler{}

	tests := []struct {
		name     string
		ext      string
		expected bool
	}{
		{"json", ".json", true},
		{"yaml", ".yaml", true},
		{"yml", ".yml", true},
		{"toml", ".toml", true},
		{"xml", ".xml", false}, // Not handled by JSONHandler
		{"txt", ".txt", false},
		{"md", ".md", false},
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

func TestJSONHandler_Extract(t *testing.T) {
	handler := &JSONHandler{}

	tests := []struct {
		name         string
		path         string
		expectError  bool
		expectedType string
		expectedCat  string
		expectedRead bool
	}{
		{
			name:         "valid json file",
			path:         "../../testdata/sample.json",
			expectError:  false,
			expectedType: "data",
			expectedCat:  "data",
			expectedRead: true,
		},
		{
			name:        "nonexistent file",
			path:        "../../testdata/nonexistent.json",
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
		})
	}
}
