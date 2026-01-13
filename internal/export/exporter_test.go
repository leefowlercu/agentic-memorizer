package export

import (
	"testing"
)

func TestDefaultExportOptions(t *testing.T) {
	opts := DefaultExportOptions()

	if opts.Format != "xml" {
		t.Errorf("Format = %q, want %q", opts.Format, "xml")
	}
	if opts.Envelope != "none" {
		t.Errorf("Envelope = %q, want %q", opts.Envelope, "none")
	}
	if opts.IncludeContent {
		t.Error("IncludeContent should be false by default")
	}
	if opts.IncludeEmbeddings {
		t.Error("IncludeEmbeddings should be false by default")
	}
	if opts.MaxFiles != 0 {
		t.Errorf("MaxFiles = %d, want %d", opts.MaxFiles, 0)
	}
}

func TestMatchPath(t *testing.T) {
	tests := []struct {
		path     string
		pattern  string
		expected bool
	}{
		{"/test/file.go", "/test/file.go", true},
		{"/test/file.go", "/test/*", true},
		{"/test/file.go", "*.go", true},
		{"/test/file.go", "/other/*", false},
		{"/test/file.go", "*.py", false},
		{"/test/file.go", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		result := matchPath(tt.path, tt.pattern)
		if result != tt.expected {
			t.Errorf("matchPath(%q, %q) = %v, want %v", tt.path, tt.pattern, result, tt.expected)
		}
	}
}

func TestExportStats(t *testing.T) {
	stats := ExportStats{
		FileCount:         10,
		DirectoryCount:    5,
		ChunkCount:        100,
		TagCount:          8,
		TopicCount:        3,
		EntityCount:       12,
		RelationshipCount: 50,
		Format:            "xml",
		OutputSize:        5000,
	}

	if stats.FileCount != 10 {
		t.Errorf("FileCount = %d, want %d", stats.FileCount, 10)
	}
	if stats.Format != "xml" {
		t.Errorf("Format = %q, want %q", stats.Format, "xml")
	}
}

func TestExportOptions(t *testing.T) {
	opts := ExportOptions{
		Format:            "json",
		Envelope:          "claude-code",
		IncludeContent:    true,
		IncludeEmbeddings: false,
		MaxFiles:          100,
		FilterTags:        []string{"go", "test"},
		FilterTopics:      []string{"programming"},
		FilterPaths:       []string{"/src/*"},
	}

	if opts.Format != "json" {
		t.Errorf("Format = %q, want %q", opts.Format, "json")
	}
	if len(opts.FilterTags) != 2 {
		t.Errorf("FilterTags length = %d, want %d", len(opts.FilterTags), 2)
	}
	if opts.MaxFiles != 100 {
		t.Errorf("MaxFiles = %d, want %d", opts.MaxFiles, 100)
	}
}
