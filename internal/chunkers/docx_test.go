package chunkers

import (
	"archive/zip"
	"bytes"
	"context"
	"testing"
)

func TestDOCXChunker(t *testing.T) {
	chunker := NewDOCXChunker()

	t.Run("Name", func(t *testing.T) {
		if chunker.Name() != "docx" {
			t.Errorf("Name() = %q, want %q", chunker.Name(), "docx")
		}
	})

	t.Run("Priority", func(t *testing.T) {
		if chunker.Priority() != 72 {
			t.Errorf("Priority() = %d, want 72", chunker.Priority())
		}
	})

	t.Run("CanHandle", func(t *testing.T) {
		tests := []struct {
			mimeType string
			language string
			expected bool
		}{
			{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", "", true},
			{"", "file.docx", true},
			{"", "FILE.DOCX", true},
			{"text/plain", "", false},
			{"application/msword", "", false}, // .doc format
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
		content := createTestDOCX(t, []docxTestPara{
			{text: "Introduction", style: ""},
			{text: "This is the first paragraph.", style: ""},
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

	t.Run("HeadingStyleDetection", func(t *testing.T) {
		content := createTestDOCXWithStyles(t, []docxTestPara{
			{text: "Chapter One", style: "Heading1"},
			{text: "Some content under chapter one.", style: ""},
			{text: "Section 1.1", style: "Heading2"},
			{text: "Content under section.", style: ""},
		})

		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Should have chunks with heading metadata
		foundHeading := false
		for _, chunk := range result.Chunks {
			if chunk.Metadata.Document != nil && chunk.Metadata.Document.HeadingLevel > 0 {
				foundHeading = true
				break
			}
		}
		if !foundHeading {
			t.Error("Expected to find chunks with heading metadata")
		}
	})

	t.Run("TableCSVConversion", func(t *testing.T) {
		content := createTestDOCXWithTable(t)

		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}

		// Verify table was converted to CSV format
		found := false
		for _, chunk := range result.Chunks {
			// CSV format should have commas
			if contains(chunk.Content, ",") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected table to be converted to CSV format")
		}
	})

	t.Run("ChunkType", func(t *testing.T) {
		content := createTestDOCX(t, []docxTestPara{
			{text: "Test content", style: ""},
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
}

// Test helper types and functions

type docxTestPara struct {
	text  string
	style string
}

func createTestDOCX(t *testing.T, paragraphs []docxTestPara) []byte {
	t.Helper()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	// Create minimal document.xml
	docContent := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>`

	for _, p := range paragraphs {
		docContent += `<w:p>`
		if p.style != "" {
			docContent += `<w:pPr><w:pStyle w:val="` + p.style + `"/></w:pPr>`
		}
		docContent += `<w:r><w:t>` + p.text + `</w:t></w:r></w:p>`
	}

	docContent += `</w:body></w:document>`

	// Add document.xml to zip
	f, err := w.Create("word/document.xml")
	if err != nil {
		t.Fatalf("Failed to create document.xml: %v", err)
	}
	if _, err := f.Write([]byte(docContent)); err != nil {
		t.Fatalf("Failed to write document.xml: %v", err)
	}

	// Add [Content_Types].xml (required for valid DOCX)
	contentTypes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`

	f, err = w.Create("[Content_Types].xml")
	if err != nil {
		t.Fatalf("Failed to create [Content_Types].xml: %v", err)
	}
	if _, err := f.Write([]byte(contentTypes)); err != nil {
		t.Fatalf("Failed to write [Content_Types].xml: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close zip: %v", err)
	}

	return buf.Bytes()
}

func createTestDOCXWithStyles(t *testing.T, paragraphs []docxTestPara) []byte {
	t.Helper()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	// Create document.xml
	docContent := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>`

	for _, p := range paragraphs {
		docContent += `<w:p>`
		if p.style != "" {
			docContent += `<w:pPr><w:pStyle w:val="` + p.style + `"/></w:pPr>`
		}
		docContent += `<w:r><w:t>` + p.text + `</w:t></w:r></w:p>`
	}

	docContent += `</w:body></w:document>`

	f, err := w.Create("word/document.xml")
	if err != nil {
		t.Fatalf("Failed to create document.xml: %v", err)
	}
	if _, err := f.Write([]byte(docContent)); err != nil {
		t.Fatalf("Failed to write document.xml: %v", err)
	}

	// Create styles.xml with heading styles
	stylesContent := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:style w:type="paragraph" w:styleId="Heading1">
<w:name w:val="heading 1"/>
<w:pPr><w:outlineLvl w:val="0"/></w:pPr>
</w:style>
<w:style w:type="paragraph" w:styleId="Heading2">
<w:name w:val="heading 2"/>
<w:pPr><w:outlineLvl w:val="1"/></w:pPr>
</w:style>
<w:style w:type="paragraph" w:styleId="Heading3">
<w:name w:val="heading 3"/>
<w:pPr><w:outlineLvl w:val="2"/></w:pPr>
</w:style>
</w:styles>`

	f, err = w.Create("word/styles.xml")
	if err != nil {
		t.Fatalf("Failed to create styles.xml: %v", err)
	}
	if _, err := f.Write([]byte(stylesContent)); err != nil {
		t.Fatalf("Failed to write styles.xml: %v", err)
	}

	// Add [Content_Types].xml
	contentTypes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`

	f, err = w.Create("[Content_Types].xml")
	if err != nil {
		t.Fatalf("Failed to create [Content_Types].xml: %v", err)
	}
	if _, err := f.Write([]byte(contentTypes)); err != nil {
		t.Fatalf("Failed to write [Content_Types].xml: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close zip: %v", err)
	}

	return buf.Bytes()
}

func createTestDOCXWithTable(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	// Create document.xml with a table
	docContent := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>
<w:p><w:r><w:t>Table below:</w:t></w:r></w:p>
<w:tbl>
<w:tr>
<w:tc><w:p><w:r><w:t>Name</w:t></w:r></w:p></w:tc>
<w:tc><w:p><w:r><w:t>Value</w:t></w:r></w:p></w:tc>
</w:tr>
<w:tr>
<w:tc><w:p><w:r><w:t>Item1</w:t></w:r></w:p></w:tc>
<w:tc><w:p><w:r><w:t>100</w:t></w:r></w:p></w:tc>
</w:tr>
<w:tr>
<w:tc><w:p><w:r><w:t>Item2</w:t></w:r></w:p></w:tc>
<w:tc><w:p><w:r><w:t>200</w:t></w:r></w:p></w:tc>
</w:tr>
</w:tbl>
</w:body>
</w:document>`

	f, err := w.Create("word/document.xml")
	if err != nil {
		t.Fatalf("Failed to create document.xml: %v", err)
	}
	if _, err := f.Write([]byte(docContent)); err != nil {
		t.Fatalf("Failed to write document.xml: %v", err)
	}

	// Add [Content_Types].xml
	contentTypes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`

	f, err = w.Create("[Content_Types].xml")
	if err != nil {
		t.Fatalf("Failed to create [Content_Types].xml: %v", err)
	}
	if _, err := f.Write([]byte(contentTypes)); err != nil {
		t.Fatalf("Failed to write [Content_Types].xml: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close zip: %v", err)
	}

	return buf.Bytes()
}
