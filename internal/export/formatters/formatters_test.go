package formatters

import (
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/graph"
)

func testSnapshot() *graph.GraphSnapshot {
	return &graph.GraphSnapshot{
		Version:    1,
		ExportedAt: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Files: []graph.FileNode{
			{
				Path:        "/test/file1.go",
				Name:        "file1.go",
				Extension:   ".go",
				MIMEType:    "text/x-go",
				Language:    "go",
				Size:        1024,
				ModTime:     time.Date(2024, 1, 10, 8, 0, 0, 0, time.UTC),
				ContentHash: "abc123",
				Summary:     "Test file one",
				Complexity:  5,
			},
			{
				Path:        "/test/file2.py",
				Name:        "file2.py",
				Extension:   ".py",
				MIMEType:    "text/x-python",
				Language:    "python",
				Size:        2048,
				ModTime:     time.Date(2024, 1, 11, 9, 0, 0, 0, time.UTC),
				ContentHash: "def456",
				Summary:     "Test file two",
				Complexity:  3,
			},
		},
		Directories: []graph.DirectoryNode{
			{
				Path:         "/test",
				Name:         "test",
				IsRemembered: true,
				FileCount:    2,
			},
		},
		Tags: []graph.TagNode{
			{Name: "golang", UsageCount: 1},
			{Name: "python", UsageCount: 1},
		},
		Topics: []graph.TopicNode{
			{Name: "Programming", Description: "Code files", UsageCount: 2},
		},
		Entities: []graph.EntityNode{
			{Name: "Go", Type: "language", UsageCount: 1},
			{Name: "Python", Type: "language", UsageCount: 1},
		},
		TotalChunks:        10,
		TotalRelationships: 15,
	}
}

func TestXMLFormatter(t *testing.T) {
	formatter := NewXMLFormatter()

	t.Run("Name", func(t *testing.T) {
		if formatter.Name() != "xml" {
			t.Errorf("Name() = %q, want %q", formatter.Name(), "xml")
		}
	})

	t.Run("ContentType", func(t *testing.T) {
		if formatter.ContentType() != "application/xml" {
			t.Errorf("ContentType() = %q, want %q", formatter.ContentType(), "application/xml")
		}
	})

	t.Run("FileExtension", func(t *testing.T) {
		if formatter.FileExtension() != ".xml" {
			t.Errorf("FileExtension() = %q, want %q", formatter.FileExtension(), ".xml")
		}
	})

	t.Run("Format", func(t *testing.T) {
		snapshot := testSnapshot()
		output, err := formatter.Format(snapshot)
		if err != nil {
			t.Fatalf("Format failed: %v", err)
		}

		// Verify it's valid XML
		var parsed xmlSnapshot
		if err := xml.Unmarshal(output, &parsed); err != nil {
			t.Fatalf("Output is not valid XML: %v", err)
		}

		// Check content
		if parsed.Version != 1 {
			t.Errorf("Version = %d, want %d", parsed.Version, 1)
		}
		if len(parsed.Files) != 2 {
			t.Errorf("Files count = %d, want %d", len(parsed.Files), 2)
		}
		if len(parsed.Directories) != 1 {
			t.Errorf("Directories count = %d, want %d", len(parsed.Directories), 1)
		}
		if parsed.Stats.TotalChunks != 10 {
			t.Errorf("TotalChunks = %d, want %d", parsed.Stats.TotalChunks, 10)
		}
	})

	t.Run("EmptySnapshot", func(t *testing.T) {
		snapshot := &graph.GraphSnapshot{
			Version:    1,
			ExportedAt: time.Now(),
		}
		output, err := formatter.Format(snapshot)
		if err != nil {
			t.Fatalf("Format failed: %v", err)
		}

		if !strings.Contains(string(output), "<?xml version=") {
			t.Error("Output should contain XML declaration")
		}
	})
}

func TestJSONFormatter(t *testing.T) {
	formatter := NewJSONFormatter()

	t.Run("Name", func(t *testing.T) {
		if formatter.Name() != "json" {
			t.Errorf("Name() = %q, want %q", formatter.Name(), "json")
		}
	})

	t.Run("ContentType", func(t *testing.T) {
		if formatter.ContentType() != "application/json" {
			t.Errorf("ContentType() = %q, want %q", formatter.ContentType(), "application/json")
		}
	})

	t.Run("FileExtension", func(t *testing.T) {
		if formatter.FileExtension() != ".json" {
			t.Errorf("FileExtension() = %q, want %q", formatter.FileExtension(), ".json")
		}
	})

	t.Run("Format", func(t *testing.T) {
		snapshot := testSnapshot()
		output, err := formatter.Format(snapshot)
		if err != nil {
			t.Fatalf("Format failed: %v", err)
		}

		// Verify it's valid JSON
		var parsed jsonSnapshot
		if err := json.Unmarshal(output, &parsed); err != nil {
			t.Fatalf("Output is not valid JSON: %v", err)
		}

		// Check content
		if parsed.Version != 1 {
			t.Errorf("Version = %d, want %d", parsed.Version, 1)
		}
		if len(parsed.Files) != 2 {
			t.Errorf("Files count = %d, want %d", len(parsed.Files), 2)
		}
		if parsed.TotalChunks != 10 {
			t.Errorf("TotalChunks = %d, want %d", parsed.TotalChunks, 10)
		}
	})

	t.Run("PrettyFormat", func(t *testing.T) {
		snapshot := testSnapshot()
		output, err := formatter.Format(snapshot)
		if err != nil {
			t.Fatalf("Format failed: %v", err)
		}

		// Pretty format should have newlines
		if !strings.Contains(string(output), "\n") {
			t.Error("Pretty format should contain newlines")
		}
	})

	t.Run("CompactFormat", func(t *testing.T) {
		compact := NewCompactJSONFormatter()
		snapshot := testSnapshot()
		output, err := compact.Format(snapshot)
		if err != nil {
			t.Fatalf("Format failed: %v", err)
		}

		// Compact format should not have indentation newlines
		lines := strings.Split(string(output), "\n")
		if len(lines) > 2 { // May have trailing newline
			t.Error("Compact format should be single line")
		}
	})
}

func TestTOONFormatter(t *testing.T) {
	formatter := NewTOONFormatter()

	t.Run("Name", func(t *testing.T) {
		if formatter.Name() != "toon" {
			t.Errorf("Name() = %q, want %q", formatter.Name(), "toon")
		}
	})

	t.Run("ContentType", func(t *testing.T) {
		if formatter.ContentType() != "text/plain" {
			t.Errorf("ContentType() = %q, want %q", formatter.ContentType(), "text/plain")
		}
	})

	t.Run("FileExtension", func(t *testing.T) {
		if formatter.FileExtension() != ".toon" {
			t.Errorf("FileExtension() = %q, want %q", formatter.FileExtension(), ".toon")
		}
	})

	t.Run("Format", func(t *testing.T) {
		snapshot := testSnapshot()
		output, err := formatter.Format(snapshot)
		if err != nil {
			t.Fatalf("Format failed: %v", err)
		}

		content := string(output)

		// Check header
		if !strings.HasPrefix(content, "@kg v=1") {
			t.Error("Output should start with @kg header")
		}

		// Check sections
		if !strings.Contains(content, "#f\n") {
			t.Error("Output should contain files section #f")
		}
		if !strings.Contains(content, "#d\n") {
			t.Error("Output should contain directories section #d")
		}
		if !strings.Contains(content, "#t\n") {
			t.Error("Output should contain tags section #t")
		}
		if !strings.Contains(content, "#e\n") {
			t.Error("Output should contain entities section #e")
		}

		// Check stats footer
		if !strings.Contains(content, "@stats f=2 d=1 c=10") {
			t.Error("Output should contain stats footer")
		}
	})

	t.Run("TokenEfficiency", func(t *testing.T) {
		snapshot := testSnapshot()

		toonOutput, _ := formatter.Format(snapshot)
		jsonOutput, _ := NewJSONFormatter().Format(snapshot)

		// TOON should be significantly smaller than JSON
		ratio := float64(len(toonOutput)) / float64(len(jsonOutput))
		if ratio > 0.7 {
			t.Errorf("TOON output (%d bytes) should be at least 30%% smaller than JSON (%d bytes), ratio: %.2f",
				len(toonOutput), len(jsonOutput), ratio)
		}
	})
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		max      int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello..."},
		{"hi", 2, "hi"},
		{"hello", 3, "hel"}, // max <= 3, no room for ellipsis
		{"", 5, ""},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.max)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, result, tt.expected)
		}
	}
}

func TestEscapeTOON(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"hello|world", "hello\\|world"},
		{"line1\nline2", "line1\\nline2"},
		{"test\r\n", "test\\n"},
	}

	for _, tt := range tests {
		result := escapeTOON(tt.input)
		if result != tt.expected {
			t.Errorf("escapeTOON(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestShortenMIME(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"text/plain", "txt"},
		{"text/x-go", "go"},
		{"text/x-python", "py"},
		{"application/json", "json"},
		{"text/markdown", "md"},
		{"unknown/type", "type"},
		{"", ""},
	}

	for _, tt := range tests {
		result := shortenMIME(tt.input)
		if result != tt.expected {
			t.Errorf("shortenMIME(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
