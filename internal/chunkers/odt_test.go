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
