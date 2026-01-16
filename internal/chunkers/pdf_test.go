package chunkers

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPDFChunker(t *testing.T) {
	chunker := NewPDFChunker()

	t.Run("Name", func(t *testing.T) {
		if chunker.Name() != "pdf" {
			t.Errorf("Name() = %q, want %q", chunker.Name(), "pdf")
		}
	})

	t.Run("Priority", func(t *testing.T) {
		if chunker.Priority() != 73 {
			t.Errorf("Priority() = %d, want 73", chunker.Priority())
		}
	})

	t.Run("CanHandle", func(t *testing.T) {
		tests := []struct {
			mimeType string
			language string
			expected bool
		}{
			{"application/pdf", "", true},
			{"", "file.pdf", true},
			{"", "FILE.PDF", true},
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

	t.Run("InvalidPDF", func(t *testing.T) {
		content := []byte("this is not a PDF file")
		_, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err == nil {
			t.Error("Expected error for invalid PDF content")
		}
	})

	t.Run("MinimalPDF", func(t *testing.T) {
		content := createMinimalPDF()
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}
		// Minimal PDF might not have text, but should parse successfully
		if result == nil {
			t.Error("Expected non-nil result")
		}
		if result.ChunkerUsed != "pdf" {
			t.Errorf("ChunkerUsed = %q, want %q", result.ChunkerUsed, "pdf")
		}
	})

	t.Run("PDFWithText", func(t *testing.T) {
		content := createPDFWithText("Hello World")
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}
		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		// Check extraction quality is set
		for _, chunk := range result.Chunks {
			if chunk.Metadata.Document == nil {
				t.Error("Expected Document metadata")
				continue
			}
			if chunk.Metadata.Document.ExtractionQuality == "" {
				t.Error("Expected ExtractionQuality to be set")
			}
		}
	})

	t.Run("ChunkType", func(t *testing.T) {
		content := createPDFWithText("Test content")
		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Errorf("Chunk returned error: %v", err)
		}
		for _, chunk := range result.Chunks {
			if chunk.Metadata.Type != ChunkTypeProse {
				t.Errorf("Expected ChunkTypeProse, got %v", chunk.Metadata.Type)
			}
		}
	})

	t.Run("HeadingDetection", func(t *testing.T) {
		// Test the heading detection function directly
		tests := []struct {
			line     string
			expected bool
			level    int
		}{
			{"1. Introduction", true, 1},
			{"1.2 Background", true, 2},
			{"1.2.3 Details", true, 3},
			{"INTRODUCTION", true, 1},
			{"Chapter One", true, 1},
			{"Section 1", true, 1},
			{"Regular paragraph text", false, 0},
			{"", false, 0},
		}

		for _, tt := range tests {
			isHeading, level := chunker.detectHeading(tt.line, "medium")
			if isHeading != tt.expected {
				t.Errorf("detectHeading(%q) = %v, want %v", tt.line, isHeading, tt.expected)
			}
			if tt.expected && level != tt.level {
				t.Errorf("detectHeading(%q) level = %d, want %d", tt.line, level, tt.level)
			}
		}
	})

	t.Run("ContentStreamExtraction", func(t *testing.T) {
		// Test content stream text extraction
		stream := []byte(`BT /F1 12 Tf (Hello World) Tj ET`)
		text := chunker.extractTextFromContentStream(stream)
		if text != "Hello World" {
			t.Errorf("extractTextFromContentStream got %q, want %q", text, "Hello World")
		}
	})

	t.Run("ContentStreamEscapes", func(t *testing.T) {
		// Test escape sequence handling
		stream := []byte(`BT (Hello\nWorld) Tj ET`)
		text := chunker.extractTextFromContentStream(stream)
		if !contains(text, "Hello") || !contains(text, "World") {
			t.Errorf("extractTextFromContentStream with escapes got %q", text)
		}
	})

	t.Run("RealPDF", func(t *testing.T) {
		// Test with actual PDF file (Attention Is All You Need paper)
		pdfPath := filepath.Join("..", "..", "testdata", "documents", "sample.pdf")
		content, err := os.ReadFile(pdfPath)
		if err != nil {
			t.Skipf("Skipping real PDF test: %v", err)
		}

		result, err := chunker.Chunk(context.Background(), content, DefaultChunkOptions())
		if err != nil {
			t.Fatalf("Chunk returned error: %v", err)
		}

		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		if result.ChunkerUsed != "pdf" {
			t.Errorf("ChunkerUsed = %q, want %q", result.ChunkerUsed, "pdf")
		}

		if len(result.Chunks) == 0 {
			t.Error("Expected at least one chunk from PDF")
		}

		// Verify document metadata is populated
		for i, chunk := range result.Chunks {
			if chunk.Metadata.Document == nil {
				t.Errorf("Chunk %d missing Document metadata", i)
				continue
			}
			if chunk.Metadata.Document.PageNumber == 0 {
				t.Errorf("Chunk %d has PageNumber 0", i)
			}
			if chunk.Metadata.Document.ExtractionQuality == "" {
				t.Errorf("Chunk %d missing ExtractionQuality", i)
			}
		}

		// Log some info about what was extracted
		t.Logf("Extracted %d chunks from PDF", len(result.Chunks))
		for i, chunk := range result.Chunks {
			if i < 3 {
				preview := chunk.Content
				if len(preview) > 100 {
					preview = preview[:100] + "..."
				}
				t.Logf("Chunk %d (page %d): %s", i, chunk.Metadata.Document.PageNumber, preview)
			}
		}
	})
}

// createMinimalPDF creates a minimal valid PDF document.
func createMinimalPDF() []byte {
	// This is a minimal valid PDF that can be parsed
	return []byte(`%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>
endobj
xref
0 4
0000000000 65535 f
0000000009 00000 n
0000000058 00000 n
0000000115 00000 n
trailer
<< /Size 4 /Root 1 0 R >>
startxref
193
%%EOF`)
}

// createPDFWithText creates a PDF with simple text content.
func createPDFWithText(text string) []byte {
	// Create PDF with a content stream containing text
	content := `%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>
endobj
4 0 obj
<< /Length 44 >>
stream
BT /F1 12 Tf 100 700 Td (` + text + `) Tj ET
endstream
endobj
5 0 obj
<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>
endobj
xref
0 6
0000000000 65535 f
0000000009 00000 n
0000000058 00000 n
0000000115 00000 n
0000000234 00000 n
0000000328 00000 n
trailer
<< /Size 6 /Root 1 0 R >>
startxref
406
%%EOF`
	return []byte(content)
}
