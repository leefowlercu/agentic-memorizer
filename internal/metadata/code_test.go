package metadata

import (
	"os"
	"testing"
)

func TestCodeHandler_CanHandle(t *testing.T) {
	handler := &CodeHandler{}

	tests := []struct {
		name     string
		ext      string
		expected bool
	}{
		{"go", ".go", true},
		{"python", ".py", true},
		{"javascript", ".js", true},
		{"typescript", ".ts", true},
		{"java", ".java", true},
		{"c", ".c", true},
		{"cpp", ".cpp", true},
		{"rust", ".rs", true},
		{"ruby", ".rb", true},
		{"php", ".php", true},
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

func TestCodeHandler_DetectLanguage(t *testing.T) {
	handler := &CodeHandler{}

	tests := []struct {
		name     string
		ext      string
		expected string
	}{
		{"go", ".go", "Go"},
		{"python", ".py", "Python"},
		{"javascript", ".js", "JavaScript"},
		{"typescript", ".ts", "TypeScript"},
		{"java", ".java", "Java"},
		{"c", ".c", "C"},
		{"cpp", ".cpp", "C++"},
		{"rust", ".rs", "Rust"},
		{"ruby", ".rb", "Ruby"},
		{"php", ".php", "PHP"},
		{"unknown", ".xyz", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.detectLanguage(tt.ext)
			if result != tt.expected {
				t.Errorf("detectLanguage(%q) = %q, want %q", tt.ext, result, tt.expected)
			}
		})
	}
}

func TestCodeHandler_Extract(t *testing.T) {
	handler := &CodeHandler{}

	tests := []struct {
		name         string
		path         string
		expectError  bool
		expectedType string
		expectedCat  string
		expectedRead bool
		expectedLang string
		minLines     int
	}{
		{
			name:         "go source file",
			path:         "../../testdata/sample.go",
			expectError:  false,
			expectedType: "code",
			expectedCat:  "code",
			expectedRead: true,
			expectedLang: "Go",
			minLines:     40, // sample.go should have at least 40 lines
		},
		{
			name:         "python source file",
			path:         "../../testdata/sample.py",
			expectError:  false,
			expectedType: "code",
			expectedCat:  "code",
			expectedRead: true,
			expectedLang: "Python",
			minLines:     25, // sample.py should have at least 25 lines
		},
		{
			name:        "nonexistent file",
			path:        "../../testdata/nonexistent.go",
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

			if metadata.Language == nil {
				t.Error("Language should not be nil")
			} else if *metadata.Language != tt.expectedLang {
				t.Errorf("Language = %q, want %q", *metadata.Language, tt.expectedLang)
			}

			// WordCount is repurposed for line count
			if metadata.WordCount == nil {
				t.Error("WordCount (line count) should not be nil")
			} else if *metadata.WordCount < tt.minLines {
				t.Errorf("Line count = %d, want at least %d", *metadata.WordCount, tt.minLines)
			}
		})
	}
}
