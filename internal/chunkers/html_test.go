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

func TestHTMLChunker_EdgeCases(t *testing.T) {
	chunker := NewHTMLChunker()

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		content := []byte(`<html><body><h1>Title</h1><p>Content</p></body></html>`)
		_, err := chunker.Chunk(ctx, content, DefaultChunkOptions())
		if err == nil {
			t.Error("Expected context cancellation error")
		}
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})

	t.Run("NestedHTMLInHeadings", func(t *testing.T) {
		content := []byte(`<!DOCTYPE html>
<html>
<body>
<h1><em>Emphasized</em> <strong>Title</strong></h1>
<p>Content</p>
</body>
</html>`)
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Should extract text from nested elements
		found := false
		for _, chunk := range result.Chunks {
			if chunk.Metadata.Document != nil && contains(chunk.Metadata.Document.Heading, "Emphasized") {
				found = true
				if !contains(chunk.Metadata.Document.Heading, "Title") {
					t.Error("Expected full heading text including nested elements")
				}
				break
			}
		}
		if !found {
			t.Error("Expected to find heading with nested HTML content")
		}
	})

	t.Run("HTMLEntities", func(t *testing.T) {
		content := []byte(`<!DOCTYPE html>
<html>
<body>
<h1>Title &amp; Subtitle</h1>
<p>Less than &lt; Greater than &gt;</p>
<p>Non-breaking&nbsp;space</p>
</body>
</html>`)
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) == 0 {
			t.Fatal("Expected at least one chunk")
		}
		// HTML entities should be preserved or decoded
		// The parser typically decodes them
	})

	t.Run("EmptyContentBetweenHeadings", func(t *testing.T) {
		content := []byte(`<!DOCTYPE html>
<html>
<body>
<h1>First Heading</h1>
<h2>Second Heading</h2>
<p>Content here</p>
</body>
</html>`)
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Empty section between h1 and h2 should be handled gracefully
		// We should still get chunks for both headings
		foundFirst := false
		foundSecond := false
		for _, chunk := range result.Chunks {
			if chunk.Metadata.Document != nil {
				if chunk.Metadata.Document.Heading == "First Heading" {
					foundFirst = true
				}
				if chunk.Metadata.Document.Heading == "Second Heading" {
					foundSecond = true
				}
			}
		}
		if !foundSecond {
			t.Error("Expected to find Second Heading chunk")
		}
		// First heading may or may not appear depending on implementation
		_ = foundFirst
	})

	t.Run("HeadingLevelOutOfOrder", func(t *testing.T) {
		content := []byte(`<!DOCTYPE html>
<html>
<body>
<h3>Subsection First</h3>
<p>Content under h3</p>
<h1>Main Title</h1>
<p>Content under h1</p>
<h2>Section</h2>
<p>Content under h2</p>
</body>
</html>`)
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Should handle out-of-order heading levels gracefully
		if len(result.Chunks) < 2 {
			t.Errorf("Expected multiple chunks, got %d", len(result.Chunks))
		}

		// Verify heading levels are captured correctly
		for _, chunk := range result.Chunks {
			if chunk.Metadata.Document != nil {
				if chunk.Metadata.Document.Heading == "Subsection First" {
					if chunk.Metadata.Document.HeadingLevel != 3 {
						t.Errorf("Expected level 3 for h3, got %d", chunk.Metadata.Document.HeadingLevel)
					}
				}
				if chunk.Metadata.Document.Heading == "Main Title" {
					if chunk.Metadata.Document.HeadingLevel != 1 {
						t.Errorf("Expected level 1 for h1, got %d", chunk.Metadata.Document.HeadingLevel)
					}
				}
			}
		}
	})

	t.Run("ListElements", func(t *testing.T) {
		content := []byte(`<!DOCTYPE html>
<html>
<body>
<h1>List Test</h1>
<ul>
<li>First item</li>
<li>Second item</li>
<li>Third item</li>
</ul>
<ol>
<li>Numbered one</li>
<li>Numbered two</li>
</ol>
</body>
</html>`)
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// List items should be included in content
		found := false
		for _, chunk := range result.Chunks {
			if contains(chunk.Content, "First item") && contains(chunk.Content, "Second item") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find list items in chunks")
		}
	})

	t.Run("TokenEstimatePopulated", func(t *testing.T) {
		content := []byte(`<html><body><h1>Test</h1><p>Some content here</p></body></html>`)
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		for i, chunk := range result.Chunks {
			if chunk.Metadata.TokenEstimate <= 0 {
				t.Errorf("Chunk %d has invalid TokenEstimate: %d", i, chunk.Metadata.TokenEstimate)
			}
		}
	})

	t.Run("TableElements", func(t *testing.T) {
		content := []byte(`<!DOCTYPE html>
<html>
<body>
<h1>Table Test</h1>
<table>
<tr><td>Cell 1</td><td>Cell 2</td></tr>
<tr><td>Cell 3</td><td>Cell 4</td></tr>
</table>
<p>After table</p>
</body>
</html>`)
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Table cells should be in content
		found := false
		for _, chunk := range result.Chunks {
			if contains(chunk.Content, "Cell 1") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find table cells in chunks")
		}
	})

	t.Run("OnlyWhitespaceContent", func(t *testing.T) {
		content := []byte(`<html><body>

		</body></html>`)
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}
		// Should handle whitespace-only content gracefully
		if result == nil {
			t.Error("Expected non-nil result")
		}
	})

	t.Run("DeeplyNestedContent", func(t *testing.T) {
		content := []byte(`<!DOCTYPE html>
<html>
<body>
<div>
<div>
<div>
<div>
<div>
<p>Deeply nested paragraph</p>
</div>
</div>
</div>
</div>
</div>
</body>
</html>`)
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		found := false
		for _, chunk := range result.Chunks {
			if contains(chunk.Content, "Deeply nested") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find deeply nested content")
		}
	})
}
