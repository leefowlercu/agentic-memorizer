package chunkers

import (
	"context"
	"testing"
)

func TestHTMLChunker(t *testing.T) {
	chunker := NewHTMLChunker()

	t.Run("Name", func(t *testing.T) {
		if chunker.Name() != "html" {
			t.Errorf("Name() = %q, want %q", chunker.Name(), "html")
		}
	})

	t.Run("Priority", func(t *testing.T) {
		if chunker.Priority() != 75 {
			t.Errorf("Priority() = %d, want 75", chunker.Priority())
		}
	})

	t.Run("CanHandle", func(t *testing.T) {
		tests := []struct {
			mimeType string
			language string
			expected bool
		}{
			{"text/html", "", true},
			{"application/xhtml+xml", "", true},
			{"", "file.html", true},
			{"", "file.htm", true},
			{"text/plain", "", false},
			{"text/markdown", "", false},
		}

		for _, tt := range tests {
			result := chunker.CanHandle(tt.mimeType, tt.language)
			if result != tt.expected {
				t.Errorf("CanHandle(%q, %q) = %v, want %v", tt.mimeType, tt.language, result, tt.expected)
			}
		}
	})

	t.Run("EmptyContent", func(t *testing.T) {
		result, err := chunker.Chunk(context.Background(), []byte{}, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) != 0 {
			t.Errorf("Expected 0 chunks for empty content, got %d", len(result.Chunks))
		}
	})

	t.Run("HeadingExtraction", func(t *testing.T) {
		content := []byte(`<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
<h1>Main Title</h1>
<p>Introduction paragraph.</p>

<h2>Section One</h2>
<p>First section content.</p>

<h2>Section Two</h2>
<p>Second section content.</p>

<h3>Subsection</h3>
<p>Subsection content.</p>
</body>
</html>`)
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		if len(result.Chunks) < 3 {
			t.Errorf("Expected at least 3 chunks for headings, got %d", len(result.Chunks))
		}

		// Verify first chunk has h1 heading
		found := false
		for _, chunk := range result.Chunks {
			if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading == "Main Title" {
				found = true
				if chunk.Metadata.Document.HeadingLevel != 1 {
					t.Errorf("Expected heading level 1 for Main Title, got %d", chunk.Metadata.Document.HeadingLevel)
				}
				break
			}
		}
		if !found {
			t.Error("Expected to find chunk with heading 'Main Title'")
		}
	})

	t.Run("ScriptStyleRemoval", func(t *testing.T) {
		content := []byte(`<!DOCTYPE html>
<html>
<head>
<script>console.log('should be removed');</script>
<style>body { color: red; }</style>
</head>
<body>
<h1>Title</h1>
<p>Visible content.</p>
<script>alert('also removed');</script>
<noscript>No script fallback</noscript>
</body>
</html>`)
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Check that script/style content is not in output
		for _, chunk := range result.Chunks {
			if contains(chunk.Content, "console.log") {
				t.Error("Script content should be removed")
			}
			if contains(chunk.Content, "color: red") {
				t.Error("Style content should be removed")
			}
			if contains(chunk.Content, "alert") {
				t.Error("Inline script content should be removed")
			}
		}

		// Check that visible content is present
		foundVisible := false
		for _, chunk := range result.Chunks {
			if contains(chunk.Content, "Visible content") {
				foundVisible = true
				break
			}
		}
		if !foundVisible {
			t.Error("Expected to find visible content in chunks")
		}
	})

	t.Run("SectionPathBuilding", func(t *testing.T) {
		content := []byte(`<!DOCTYPE html>
<html>
<body>
<h1>Chapter 1</h1>
<p>Chapter intro.</p>
<h2>Section 1.1</h2>
<p>Section content.</p>
<h3>Subsection 1.1.1</h3>
<p>Subsection content.</p>
</body>
</html>`)
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Find the subsection chunk and verify section path
		for _, chunk := range result.Chunks {
			if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading == "Subsection 1.1.1" {
				expected := "Chapter 1 > Section 1.1 > Subsection 1.1.1"
				if chunk.Metadata.Document.SectionPath != expected {
					t.Errorf("SectionPath = %q, want %q", chunk.Metadata.Document.SectionPath, expected)
				}
				return
			}
		}
		t.Error("Expected to find subsection chunk with section path")
	})

	t.Run("MalformedHTML", func(t *testing.T) {
		content := []byte(`<html>
<body>
<h1>Unclosed heading
<p>Unclosed paragraph
<div>Some content</div>
</body>`)
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		// Should handle malformed HTML gracefully
		if err != nil {
			t.Errorf("Should handle malformed HTML, got error: %v", err)
		}
		if result == nil {
			t.Error("Expected result for malformed HTML")
		}
	})

	t.Run("ChunkType", func(t *testing.T) {
		content := []byte(`<html><body><h1>Test</h1><p>Content</p></body></html>`)
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}
		for _, chunk := range result.Chunks {
			if chunk.Metadata.Type != ChunkTypeMarkdown {
				t.Errorf("Expected ChunkTypeMarkdown, got %v", chunk.Metadata.Type)
			}
		}
	})

	t.Run("LargeSection", func(t *testing.T) {
		// Create HTML with a very large section
		var content []byte
		content = append(content, []byte("<html><body><h1>Title</h1>")...)
		for i := 0; i < 100; i++ {
			content = append(content, []byte("<p>This is a paragraph with some content that adds up to create a large section. ")...)
			content = append(content, []byte("More text here to make it even larger.</p>")...)
		}
		content = append(content, []byte("</body></html>")...)

		opts := ChunkOptions{MaxChunkSize: 500}
		result, err := chunker.Chunk(context.Background(), content, opts)
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Should have multiple chunks due to size limit
		if len(result.Chunks) < 2 {
			t.Errorf("Expected multiple chunks for large content, got %d", len(result.Chunks))
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
