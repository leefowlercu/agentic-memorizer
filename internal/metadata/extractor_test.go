package metadata

import (
	"os"
	"testing"
)

func TestNewExtractor(t *testing.T) {
	e := NewExtractor()

	if e == nil {
		t.Fatal("NewExtractor returned nil")
	}

	if len(e.handlers) == 0 {
		t.Fatal("NewExtractor did not register any handlers")
	}

	// Should have 8 handlers registered
	expectedHandlers := 8
	if len(e.handlers) != expectedHandlers {
		t.Errorf("Expected %d handlers, got %d", expectedHandlers, len(e.handlers))
	}
}

func TestRegisterHandler(t *testing.T) {
	e := &Extractor{
		handlers: make(map[string]FileHandler),
	}

	handler := &MarkdownHandler{}
	e.RegisterHandler(handler)

	if len(e.handlers) != 1 {
		t.Errorf("Expected 1 handler, got %d", len(e.handlers))
	}
}

func TestCategorizeFile(t *testing.T) {
	tests := []struct {
		name     string
		ext      string
		expected string
	}{
		// Documents
		{"markdown", ".md", "documents"},
		{"text", ".txt", "documents"},
		{"word", ".docx", "documents"},
		{"pdf", ".pdf", "documents"},
		{"rtf", ".rtf", "documents"},

		// Presentations
		{"powerpoint", ".pptx", "presentations"},
		{"ppt", ".ppt", "presentations"},
		{"keynote", ".key", "presentations"},

		// Images
		{"png", ".png", "images"},
		{"jpg", ".jpg", "images"},
		{"jpeg", ".jpeg", "images"},
		{"gif", ".gif", "images"},
		{"svg", ".svg", "images"},
		{"webp", ".webp", "images"},

		// Transcripts
		{"vtt", ".vtt", "transcripts"},
		{"srt", ".srt", "transcripts"},
		{"sub", ".sub", "transcripts"},

		// Data
		{"json", ".json", "data"},
		{"yaml", ".yaml", "data"},
		{"yml", ".yml", "data"},
		{"toml", ".toml", "data"},
		{"xml", ".xml", "data"},

		// Code
		{"go", ".go", "code"},
		{"python", ".py", "code"},
		{"javascript", ".js", "code"},
		{"typescript", ".ts", "code"},
		{"java", ".java", "code"},
		{"c", ".c", "code"},
		{"cpp", ".cpp", "code"},
		{"rust", ".rs", "code"},
		{"ruby", ".rb", "code"},
		{"php", ".php", "code"},

		// Videos
		{"mp4", ".mp4", "videos"},
		{"mov", ".mov", "videos"},
		{"avi", ".avi", "videos"},

		// Audio
		{"mp3", ".mp3", "audio"},
		{"wav", ".wav", "audio"},
		{"ogg", ".ogg", "audio"},

		// Archives
		{"zip", ".zip", "archives"},
		{"tar", ".tar", "archives"},
		{"gz", ".gz", "archives"},

		// Other
		{"unknown", ".xyz", "other"},
		{"empty", "", "other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := categorizeFile(tt.ext)
			if result != tt.expected {
				t.Errorf("categorizeFile(%q) = %q, want %q", tt.ext, result, tt.expected)
			}
		})
	}
}

func TestIsReadable(t *testing.T) {
	tests := []struct {
		name     string
		ext      string
		expected bool
	}{
		// Readable text files
		{"markdown", ".md", true},
		{"text", ".txt", true},
		{"json", ".json", true},
		{"yaml", ".yaml", true},
		{"yml", ".yml", true},
		{"vtt", ".vtt", true},

		// Readable code files
		{"go", ".go", true},
		{"python", ".py", true},
		{"javascript", ".js", true},
		{"html", ".html", true},
		{"css", ".css", true},

		// Readable images
		{"png", ".png", true},
		{"jpg", ".jpg", true},
		{"jpeg", ".jpeg", true},

		// Not readable
		{"docx", ".docx", false},
		{"pptx", ".pptx", false},
		{"pdf", ".pdf", false},
		{"zip", ".zip", false},
		{"mp4", ".mp4", false},
		{"unknown", ".xyz", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isReadable(tt.ext)
			if result != tt.expected {
				t.Errorf("isReadable(%q) = %v, want %v", tt.ext, result, tt.expected)
			}
		})
	}
}

func TestExtractor_Extract(t *testing.T) {
	e := NewExtractor()

	tests := []struct {
		name        string
		path        string
		expectError bool
		checkType   string
		checkCat    string
	}{
		{
			name:        "markdown file",
			path:        "../../testdata/sample.md",
			expectError: false,
			checkType:   "markdown",
			checkCat:    "documents",
		},
		{
			name:        "json file",
			path:        "../../testdata/sample.json",
			expectError: false,
			checkType:   "data",
			checkCat:    "data",
		},
		{
			name:        "go code file",
			path:        "../../testdata/sample.go",
			expectError: false,
			checkType:   "code",
			checkCat:    "code",
		},
		{
			name:        "nonexistent file",
			path:        "../../testdata/nonexistent.txt",
			expectError: true,
			checkType:   "",
			checkCat:    "",
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

			metadata, err := e.Extract(tt.path, info)
			if err != nil {
				t.Fatalf("Extract failed: %v", err)
			}

			if metadata == nil {
				t.Fatal("Extract returned nil metadata")
			}

			if metadata.Type != tt.checkType {
				t.Errorf("Type = %q, want %q", metadata.Type, tt.checkType)
			}

			if metadata.Category != tt.checkCat {
				t.Errorf("Category = %q, want %q", metadata.Category, tt.checkCat)
			}
		})
	}
}

func TestExtractor_ExtractNoHandler(t *testing.T) {
	e := NewExtractor()

	// Create a temp file with unknown extension
	tmpfile, err := os.CreateTemp("", "test.xyz")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	info, err := os.Stat(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}

	metadata, err := e.Extract(tmpfile.Name(), info)
	if err != nil {
		t.Errorf("Extract with no handler should not return error, got: %v", err)
	}

	if metadata == nil {
		t.Fatal("Extract returned nil metadata")
	}

	// Should return base metadata
	if metadata.Category != "other" {
		t.Errorf("Expected category 'other', got %q", metadata.Category)
	}
}

func TestExtractor_ExtractWithHandlerError(t *testing.T) {
	e := NewExtractor()

	// Use malformed.docx which will cause extraction error
	info, err := os.Stat("../../testdata/malformed.docx")
	if err != nil {
		t.Skip("malformed.docx not found, skipping test")
	}

	metadata, err := e.Extract("../../testdata/malformed.docx", info)

	// Should not return error (graceful degradation)
	if err != nil {
		t.Errorf("Extract should gracefully handle errors, got: %v", err)
	}

	if metadata == nil {
		t.Fatal("Extract returned nil metadata")
	}

	// Should return base metadata even with handler error
	if metadata.Type != ".docx" {
		t.Errorf("Expected type '.docx', got %q", metadata.Type)
	}
}
