package handlers

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistry_GetHandler(t *testing.T) {
	r := DefaultRegistry()

	tests := []struct {
		path        string
		wantHandler string
	}{
		// Text files
		{"/test/main.go", "text"},
		{"/test/script.py", "text"},
		{"/test/README.md", "text"},
		{"/test/config.yaml", "structured_data"},
		{"/test/data.json", "structured_data"},

		// Images
		{"/test/photo.jpg", "image"},
		{"/test/icon.png", "image"},
		{"/test/animation.gif", "image"},

		// Documents
		{"/test/document.pdf", "pdf"},
		{"/test/report.docx", "rich_document"},
		{"/test/spreadsheet.xlsx", "rich_document"},
		{"/test/presentation.pptx", "rich_document"},

		// Archives
		{"/test/archive.zip", "archive"},
		{"/test/backup.tar", "archive"},
		{"/test/bundle.tar.gz", "archive"},

		// Unsupported
		{"/test/binary.exe", "unsupported"},
		{"/test/library.dll", "unsupported"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			h := r.GetHandler(tt.path)
			if h == nil {
				t.Fatal("expected handler, got nil")
			}
			if h.Name() != tt.wantHandler {
				t.Errorf("expected handler %q, got %q", tt.wantHandler, h.Name())
			}
		})
	}
}

func TestTextHandler_Extract(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test Go file
	testFile := filepath.Join(tmpDir, "main.go")
	content := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	h := NewTextHandler()
	ctx := context.Background()

	result, err := h.Extract(ctx, testFile, int64(len(content)))
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if result.Handler != "text" {
		t.Errorf("expected handler 'text', got %q", result.Handler)
	}

	if result.TextContent != content {
		t.Error("extracted content doesn't match original")
	}

	if result.Metadata == nil {
		t.Fatal("expected metadata, got nil")
	}

	if result.Metadata.Language != "Go" {
		t.Errorf("expected language 'Go', got %q", result.Metadata.Language)
	}

	if result.Metadata.LineCount != 7 {
		t.Errorf("expected 7 lines, got %d", result.Metadata.LineCount)
	}
}

func TestTextHandler_FileTooLarge(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "small.txt")
	os.WriteFile(testFile, []byte("small"), 0644)

	h := NewTextHandler(WithMaxTextSize(1)) // 1 byte max
	ctx := context.Background()

	result, err := h.Extract(ctx, testFile, 1000) // Report larger size
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if !result.SkipAnalysis {
		t.Error("expected SkipAnalysis to be true for oversized file")
	}

	if result.Error == "" {
		t.Error("expected error message for oversized file")
	}
}

func TestTextHandler_CanHandle(t *testing.T) {
	h := NewTextHandler()

	tests := []struct {
		mimeType string
		ext      string
		want     bool
	}{
		{"text/plain", ".txt", true},
		{"text/x-go", ".go", true},
		{"text/markdown", ".md", true},
		{"application/json", ".json", true}, // TextHandler can handle, but StructuredDataHandler takes priority in registry
		{"image/png", ".png", false},
		{"", ".py", true},
		{"", ".rs", true},
		{"", ".unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := h.CanHandle(tt.mimeType, tt.ext)
			if got != tt.want {
				t.Errorf("CanHandle(%q, %q) = %v, want %v", tt.mimeType, tt.ext, got, tt.want)
			}
		})
	}
}

func TestImageHandler_Extract(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal valid PNG file
	testFile := filepath.Join(tmpDir, "test.png")
	// PNG header + minimal IHDR chunk (1x1 pixel)
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, // IHDR length
		0x49, 0x48, 0x44, 0x52, // IHDR
		0x00, 0x00, 0x00, 0x01, // width = 1
		0x00, 0x00, 0x00, 0x01, // height = 1
		0x08, 0x02, // bit depth, color type
		0x00, 0x00, 0x00, // compression, filter, interlace
		0x90, 0x77, 0x53, 0xDE, // CRC
		0x00, 0x00, 0x00, 0x0C, // IDAT length
		0x49, 0x44, 0x41, 0x54, // IDAT
		0x08, 0xD7, 0x63, 0xF8, 0x0F, 0x00, 0x00, 0x01, 0x01, 0x00, 0x18, 0xDD, 0x8D, 0xB4, // compressed data + CRC
		0x00, 0x00, 0x00, 0x00, // IEND length
		0x49, 0x45, 0x4E, 0x44, // IEND
		0xAE, 0x42, 0x60, 0x82, // CRC
	}
	os.WriteFile(testFile, pngData, 0644)

	h := NewImageHandler()
	ctx := context.Background()

	result, err := h.Extract(ctx, testFile, int64(len(pngData)))
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if result.Handler != "image" {
		t.Errorf("expected handler 'image', got %q", result.Handler)
	}

	if result.Metadata == nil {
		t.Fatal("expected metadata")
	}

	if result.Metadata.ImageDimensions == nil {
		t.Fatal("expected image dimensions")
	}

	if result.Metadata.ImageDimensions.Width != 1 || result.Metadata.ImageDimensions.Height != 1 {
		t.Errorf("expected 1x1 dimensions, got %dx%d",
			result.Metadata.ImageDimensions.Width,
			result.Metadata.ImageDimensions.Height)
	}

	// Should have vision content if vision is enabled
	if result.VisionContent == nil {
		t.Error("expected vision content")
	}
}

func TestImageHandler_WithoutVision(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.png")
	os.WriteFile(testFile, []byte{0x89, 0x50, 0x4E, 0x47}, 0644)

	h := NewImageHandler(WithVision(false))
	ctx := context.Background()

	result, err := h.Extract(ctx, testFile, 4)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if result.VisionContent != nil {
		t.Error("expected no vision content when vision is disabled")
	}

	if !result.SkipAnalysis {
		t.Error("expected SkipAnalysis when vision is disabled")
	}
}

func TestStructuredDataHandler_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "data.json")

	data := map[string]any{
		"name": "test",
		"count": 42,
		"items": []any{"a", "b", "c"},
	}
	jsonData, _ := json.Marshal(data)
	os.WriteFile(testFile, jsonData, 0644)

	h := NewStructuredDataHandler()
	ctx := context.Background()

	result, err := h.Extract(ctx, testFile, int64(len(jsonData)))
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if result.Handler != "structured_data" {
		t.Errorf("expected handler 'structured_data', got %q", result.Handler)
	}

	if !strings.Contains(result.TextContent, "JSON Document") {
		t.Error("expected JSON Document in content")
	}

	if result.Metadata.Extra == nil {
		t.Fatal("expected extra metadata")
	}

	if result.Metadata.Extra["format"] != "json" {
		t.Errorf("expected format 'json', got %v", result.Metadata.Extra["format"])
	}
}

func TestStructuredDataHandler_CSV(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "data.csv")

	csvData := "name,age,city\nAlice,30,NYC\nBob,25,LA\nCharlie,35,Chicago"
	os.WriteFile(testFile, []byte(csvData), 0644)

	h := NewStructuredDataHandler()
	ctx := context.Background()

	result, err := h.Extract(ctx, testFile, int64(len(csvData)))
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if result.Metadata.Extra["column_count"] != 3 {
		t.Errorf("expected 3 columns, got %v", result.Metadata.Extra["column_count"])
	}

	if result.Metadata.Extra["row_count"] != 3 {
		t.Errorf("expected 3 rows, got %v", result.Metadata.Extra["row_count"])
	}

	if !strings.Contains(result.TextContent, "name") {
		t.Error("expected column name in content")
	}
}

func TestArchiveHandler_ZIP(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.zip")

	// Create a test ZIP file
	zipFile, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("failed to create zip: %v", err)
	}

	w := zip.NewWriter(zipFile)
	files := []string{"file1.txt", "dir/file2.txt", "dir/subdir/file3.txt"}
	for _, name := range files {
		f, _ := w.Create(name)
		f.Write([]byte("content"))
	}
	w.Close()
	zipFile.Close()

	info, _ := os.Stat(testFile)

	h := NewArchiveHandler()
	ctx := context.Background()

	result, err := h.Extract(ctx, testFile, info.Size())
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if result.Handler != "archive" {
		t.Errorf("expected handler 'archive', got %q", result.Handler)
	}

	if !result.SkipAnalysis {
		t.Error("expected SkipAnalysis for archive")
	}

	if result.Metadata.Extra["file_count"] != 3 {
		t.Errorf("expected 3 files, got %v", result.Metadata.Extra["file_count"])
	}

	if !strings.Contains(result.TextContent, "file1.txt") {
		t.Error("expected file listing in content")
	}
}

func TestUnsupportedHandler_Extract(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "binary.exe")
	os.WriteFile(testFile, []byte{0x4D, 0x5A}, 0644) // MZ header

	h := NewUnsupportedHandler()
	ctx := context.Background()

	result, err := h.Extract(ctx, testFile, 2)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if result.Handler != "unsupported" {
		t.Errorf("expected handler 'unsupported', got %q", result.Handler)
	}

	if !result.SkipAnalysis {
		t.Error("expected SkipAnalysis for unsupported")
	}

	if result.Metadata == nil {
		t.Fatal("expected metadata")
	}
}

func TestMIMEDetection(t *testing.T) {
	tests := []struct {
		ext      string
		wantMIME string
	}{
		{".go", "text/x-go"},
		{".py", "text/x-python"},
		{".json", "application/json"},
		{".pdf", "application/pdf"},
		{".zip", "application/zip"},
		{".unknown", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := extensionToMIME(tt.ext)
			if got != tt.wantMIME {
				t.Errorf("extensionToMIME(%q) = %q, want %q", tt.ext, got, tt.wantMIME)
			}
		})
	}
}

func TestIsTextMIME(t *testing.T) {
	tests := []struct {
		mimeType string
		want     bool
	}{
		{"text/plain", true},
		{"text/html", true},
		{"text/x-go", true},
		{"application/json", true},
		{"application/xml", true},
		{"image/png", false},
		{"application/octet-stream", false},
	}

	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			got := IsTextMIME(tt.mimeType)
			if got != tt.want {
				t.Errorf("IsTextMIME(%q) = %v, want %v", tt.mimeType, got, tt.want)
			}
		})
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"", 0},
		{"one line", 1},
		{"line1\nline2", 2},
		{"line1\nline2\n", 2},
		{"line1\nline2\nline3", 3},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := countLines(tt.text)
			if got != tt.want {
				t.Errorf("countLines(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

func TestCountWords(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"", 0},
		{"word", 1},
		{"two words", 2},
		{"  spaced   out  ", 2},
		{"line1\nline2", 2},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := countWords(tt.text)
			if got != tt.want {
				t.Errorf("countWords(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		ext  string
		want string
	}{
		{".go", "Go"},
		{".py", "Python"},
		{".js", "JavaScript"},
		{".ts", "TypeScript"},
		{".unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := detectLanguage(tt.ext)
			if got != tt.want {
				t.Errorf("detectLanguage(%q) = %q, want %q", tt.ext, got, tt.want)
			}
		})
	}
}

func TestRegistry_GetHandlerByName(t *testing.T) {
	r := DefaultRegistry()

	h := r.GetHandlerByName("text")
	if h == nil {
		t.Fatal("expected text handler")
	}
	if h.Name() != "text" {
		t.Errorf("expected name 'text', got %q", h.Name())
	}

	h = r.GetHandlerByName("nonexistent")
	if h != nil {
		t.Error("expected nil for nonexistent handler")
	}
}

func TestRegistry_ListHandlers(t *testing.T) {
	r := DefaultRegistry()

	handlers := r.ListHandlers()
	if len(handlers) == 0 {
		t.Error("expected handlers in registry")
	}

	// Check that expected handlers are present
	names := make(map[string]bool)
	for _, h := range handlers {
		names[h.Name()] = true
	}

	expectedNames := []string{"text", "image", "pdf", "rich_document", "structured_data", "archive"}
	for _, name := range expectedNames {
		if !names[name] {
			t.Errorf("expected handler %q in registry", name)
		}
	}
}

func TestContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	h := NewTextHandler()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := h.Extract(ctx, testFile, 7)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}
