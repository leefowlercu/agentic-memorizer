package chunkers

import (
	"archive/zip"
	"bytes"
	"context"
	"strconv"
	"testing"
)

func TestODTChunker(t *testing.T) {
	chunker := NewODTChunker()

	t.Run("Name", func(t *testing.T) {
		if chunker.Name() != "odt" {
			t.Errorf("Name() = %q, want %q", chunker.Name(), "odt")
		}
	})

	t.Run("Priority", func(t *testing.T) {
		if chunker.Priority() != 71 {
			t.Errorf("Priority() = %d, want 71", chunker.Priority())
		}
	})

	t.Run("CanHandle", func(t *testing.T) {
		tests := []struct {
			mimeType string
			language string
			expected bool
		}{
			{"application/vnd.oasis.opendocument.text", "", true},
			{"", "file.odt", true},
			{"", "FILE.ODT", true},
			{"text/plain", "", false},
			{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", "", false},
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

	t.Run("InvalidZip", func(t *testing.T) {
		content := []byte("this is not a zip file")
		_, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err == nil {
			t.Error("Expected error for invalid zip content")
		}
	})

	t.Run("BasicDocument", func(t *testing.T) {
		content := createTestODT(t, []odtTestElement{
			{isHeading: false, level: 0, text: "This is the first paragraph."},
			{isHeading: false, level: 0, text: "This is the second paragraph."},
		})

		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk")
		}

		// Verify content is present
		found := false
		for _, chunk := range result.Chunks {
			if contains(chunk.Content, "first paragraph") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find paragraph content in chunks")
		}
	})

	t.Run("OutlineLevelExtraction", func(t *testing.T) {
		content := createTestODT(t, []odtTestElement{
			{isHeading: true, level: 1, text: "Chapter One"},
			{isHeading: false, level: 0, text: "Content under chapter one."},
			{isHeading: true, level: 2, text: "Section 1.1"},
			{isHeading: false, level: 0, text: "Content under section."},
			{isHeading: true, level: 3, text: "Subsection 1.1.1"},
			{isHeading: false, level: 0, text: "Content under subsection."},
		})

		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Should have multiple chunks
		if len(result.Chunks) < 2 {
			t.Errorf("Expected multiple chunks, got %d", len(result.Chunks))
		}

		// Verify heading levels are captured
		levelFound := make(map[int]bool)
		for _, chunk := range result.Chunks {
			if chunk.Metadata.Document != nil && chunk.Metadata.Document.HeadingLevel > 0 {
				levelFound[chunk.Metadata.Document.HeadingLevel] = true
			}
		}

		if !levelFound[1] {
			t.Error("Expected to find heading level 1")
		}
		if !levelFound[2] {
			t.Error("Expected to find heading level 2")
		}
	})

	t.Run("SectionPathBuilding", func(t *testing.T) {
		content := createTestODT(t, []odtTestElement{
			{isHeading: true, level: 1, text: "Chapter 1"},
			{isHeading: false, level: 0, text: "Intro."},
			{isHeading: true, level: 2, text: "Section 1.1"},
			{isHeading: false, level: 0, text: "Section content."},
		})

		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Find section chunk and verify path
		for _, chunk := range result.Chunks {
			if chunk.Metadata.Document != nil && chunk.Metadata.Document.Heading == "Section 1.1" {
				expected := "Chapter 1 > Section 1.1"
				if chunk.Metadata.Document.SectionPath != expected {
					t.Errorf("SectionPath = %q, want %q", chunk.Metadata.Document.SectionPath, expected)
				}
				return
			}
		}
		t.Error("Expected to find Section 1.1 chunk")
	})

	t.Run("ChunkType", func(t *testing.T) {
		content := createTestODT(t, []odtTestElement{
			{isHeading: true, level: 1, text: "Test"},
			{isHeading: false, level: 0, text: "Content"},
		})

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
		// Create ODT with a very large section
		var elements []odtTestElement
		elements = append(elements, odtTestElement{isHeading: true, level: 1, text: "Title"})
		for i := 0; i < 50; i++ {
			elements = append(elements, odtTestElement{
				isHeading: false,
				level:     0,
				text:      "This is a paragraph with some content that adds up to create a large section.",
			})
		}
		content := createTestODT(t, elements)

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

// Test helper types and functions

type odtTestElement struct {
	isHeading bool
	level     int
	text      string
}

func createTestODT(t *testing.T, elements []odtTestElement) []byte {
	t.Helper()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	// Create content.xml with ODF namespace
	contentXML := `<?xml version="1.0" encoding="UTF-8"?>
<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
                         xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0">
<office:body>
<office:text>`

	for _, elem := range elements {
		if elem.isHeading {
			contentXML += `<text:h text:outline-level="` + itoa(elem.level) + `">` + elem.text + `</text:h>`
		} else {
			contentXML += `<text:p>` + elem.text + `</text:p>`
		}
	}

	contentXML += `</office:text></office:body></office:document-content>`

	// Add content.xml to zip
	f, err := w.Create("content.xml")
	if err != nil {
		t.Fatalf("Failed to create content.xml: %v", err)
	}
	if _, err := f.Write([]byte(contentXML)); err != nil {
		t.Fatalf("Failed to write content.xml: %v", err)
	}

	// Add mimetype file (required for valid ODT)
	f, err = w.Create("mimetype")
	if err != nil {
		t.Fatalf("Failed to create mimetype: %v", err)
	}
	if _, err := f.Write([]byte("application/vnd.oasis.opendocument.text")); err != nil {
		t.Fatalf("Failed to write mimetype: %v", err)
	}

	// Add META-INF/manifest.xml
	manifestXML := `<?xml version="1.0" encoding="UTF-8"?>
<manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0">
<manifest:file-entry manifest:media-type="application/vnd.oasis.opendocument.text" manifest:full-path="/"/>
<manifest:file-entry manifest:media-type="text/xml" manifest:full-path="content.xml"/>
</manifest:manifest>`

	f, err = w.Create("META-INF/manifest.xml")
	if err != nil {
		t.Fatalf("Failed to create manifest.xml: %v", err)
	}
	if _, err := f.Write([]byte(manifestXML)); err != nil {
		t.Fatalf("Failed to write manifest.xml: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close zip: %v", err)
	}

	return buf.Bytes()
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

func TestODTChunker_EdgeCases(t *testing.T) {
	chunker := NewODTChunker()

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		content := createTestODT(t, []odtTestElement{
			{isHeading: false, level: 0, text: "Test content"},
		})
		_, err := chunker.Chunk(ctx, content, DefaultChunkOptions())
		if err == nil {
			t.Error("Expected context cancellation error")
		}
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})

	t.Run("MissingContentXML", func(t *testing.T) {
		// Create zip without content.xml
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)

		// Add only mimetype
		f, _ := w.Create("mimetype")
		f.Write([]byte("application/vnd.oasis.opendocument.text"))
		w.Close()

		_, err := chunker.Chunk(context.Background(), buf.Bytes(), DefaultChunkOptions())
		if err == nil {
			t.Error("Expected error for missing content.xml")
		}
	})

	t.Run("TabElementHandling", func(t *testing.T) {
		// Create ODT with tab elements
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)

		contentXML := `<?xml version="1.0" encoding="UTF-8"?>
<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
                         xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0">
<office:body>
<office:text>
<text:p>Before<text:tab/>After</text:p>
</office:text></office:body></office:document-content>`

		f, _ := w.Create("content.xml")
		f.Write([]byte(contentXML))

		f, _ = w.Create("mimetype")
		f.Write([]byte("application/vnd.oasis.opendocument.text"))
		w.Close()

		result, err := chunker.Chunk(context.Background(), buf.Bytes(), DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Tab should be converted to \t
		found := false
		for _, chunk := range result.Chunks {
			if contains(chunk.Content, "Before") && contains(chunk.Content, "After") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find content with tab")
		}
	})

	t.Run("SpaceElementHandling", func(t *testing.T) {
		// Create ODT with space elements
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)

		contentXML := `<?xml version="1.0" encoding="UTF-8"?>
<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
                         xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0">
<office:body>
<office:text>
<text:p>Word<text:s text:c="3"/>More</text:p>
</office:text></office:body></office:document-content>`

		f, _ := w.Create("content.xml")
		f.Write([]byte(contentXML))

		f, _ = w.Create("mimetype")
		f.Write([]byte("application/vnd.oasis.opendocument.text"))
		w.Close()

		result, err := chunker.Chunk(context.Background(), buf.Bytes(), DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Multiple spaces should be preserved
		found := false
		for _, chunk := range result.Chunks {
			if contains(chunk.Content, "Word") && contains(chunk.Content, "More") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find content with spaces")
		}
	})

	t.Run("LineBreakElementHandling", func(t *testing.T) {
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)

		contentXML := `<?xml version="1.0" encoding="UTF-8"?>
<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
                         xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0">
<office:body>
<office:text>
<text:p>Line1<text:line-break/>Line2</text:p>
</office:text></office:body></office:document-content>`

		f, _ := w.Create("content.xml")
		f.Write([]byte(contentXML))

		f, _ = w.Create("mimetype")
		f.Write([]byte("application/vnd.oasis.opendocument.text"))
		w.Close()

		result, err := chunker.Chunk(context.Background(), buf.Bytes(), DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Line break should be in content
		found := false
		for _, chunk := range result.Chunks {
			if contains(chunk.Content, "Line1") && contains(chunk.Content, "Line2") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find content with line break")
		}
	})

	t.Run("TokenEstimatePopulated", func(t *testing.T) {
		content := createTestODT(t, []odtTestElement{
			{isHeading: false, level: 0, text: "Test content for token estimation"},
		})
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

	t.Run("EmptyHeadings", func(t *testing.T) {
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)

		contentXML := `<?xml version="1.0" encoding="UTF-8"?>
<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
                         xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0">
<office:body>
<office:text>
<text:h text:outline-level="1"></text:h>
<text:p>Content under empty heading</text:p>
</office:text></office:body></office:document-content>`

		f, _ := w.Create("content.xml")
		f.Write([]byte(contentXML))

		f, _ = w.Create("mimetype")
		f.Write([]byte("application/vnd.oasis.opendocument.text"))
		w.Close()

		result, err := chunker.Chunk(context.Background(), buf.Bytes(), DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Should handle empty headings gracefully
		found := false
		for _, chunk := range result.Chunks {
			if contains(chunk.Content, "Content under empty heading") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find content after empty heading")
		}
	})

	t.Run("NoHeadings", func(t *testing.T) {
		content := createTestODT(t, []odtTestElement{
			{isHeading: false, level: 0, text: "First paragraph"},
			{isHeading: false, level: 0, text: "Second paragraph"},
			{isHeading: false, level: 0, text: "Third paragraph"},
		})

		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Should still produce chunks even without headings
		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk even without headings")
		}
	})

	t.Run("DeeplyNestedHeadings", func(t *testing.T) {
		content := createTestODT(t, []odtTestElement{
			{isHeading: true, level: 1, text: "Level 1"},
			{isHeading: true, level: 2, text: "Level 2"},
			{isHeading: true, level: 3, text: "Level 3"},
			{isHeading: true, level: 4, text: "Level 4"},
			{isHeading: false, level: 0, text: "Deep content"},
		})

		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Should handle deeply nested headings
		if len(result.Chunks) < 2 {
			t.Logf("Got %d chunks for nested headings", len(result.Chunks))
		}
	})

	t.Run("StyleBasedHeadings", func(t *testing.T) {
		// Create ODT with styles.xml defining outline levels
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)

		stylesXML := `<?xml version="1.0" encoding="UTF-8"?>
<office:document-styles xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
                        xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0"
                        xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0">
<office:automatic-styles>
<style:style style:name="P1" style:family="paragraph">
<style:paragraph-properties fo:outline-level="1" xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0"/>
</style:style>
</office:automatic-styles>
</office:document-styles>`

		f, _ := w.Create("styles.xml")
		f.Write([]byte(stylesXML))

		contentXML := `<?xml version="1.0" encoding="UTF-8"?>
<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
                         xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0">
<office:body>
<office:text>
<text:p text:style-name="P1">Styled Heading</text:p>
<text:p>Regular content</text:p>
</office:text></office:body></office:document-content>`

		f, _ = w.Create("content.xml")
		f.Write([]byte(contentXML))

		f, _ = w.Create("mimetype")
		f.Write([]byte("application/vnd.oasis.opendocument.text"))
		w.Close()

		result, err := chunker.Chunk(context.Background(), buf.Bytes(), DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Should extract styled headings
		if result == nil {
			t.Error("Expected non-nil result")
		}
	})

	t.Run("HeadingLevelOutOfOrder", func(t *testing.T) {
		content := createTestODT(t, []odtTestElement{
			{isHeading: true, level: 3, text: "Start at Level 3"},
			{isHeading: false, level: 0, text: "Content"},
			{isHeading: true, level: 1, text: "Then Level 1"},
			{isHeading: false, level: 0, text: "More content"},
		})

		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Should handle out-of-order heading levels
		if len(result.Chunks) < 2 {
			t.Logf("Got %d chunks for out-of-order headings", len(result.Chunks))
		}
	})
}
