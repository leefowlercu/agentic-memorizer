package document

import (
	"path/filepath"
	"runtime"
	"testing"
)

func getTestdataPath(filename string) string {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	return filepath.Join(dir, "..", "..", "testdata", filename)
}

func TestExtractPptxText(t *testing.T) {
	path := getTestdataPath("sample.pptx")

	text, err := ExtractPptxText(path)
	if err != nil {
		t.Fatalf("ExtractPptxText() error = %v", err)
	}

	if text == "" {
		t.Error("ExtractPptxText() returned empty string, expected text content")
	}
}

func TestExtractPptxText_NonExistent(t *testing.T) {
	_, err := ExtractPptxText("/nonexistent/file.pptx")
	if err == nil {
		t.Error("ExtractPptxText() expected error for non-existent file")
	}
}

func TestExtractPptxMetadata(t *testing.T) {
	path := getTestdataPath("sample.pptx")

	metadata, err := ExtractPptxMetadata(path)
	if err != nil {
		t.Fatalf("ExtractPptxMetadata() error = %v", err)
	}

	if metadata.SlideCount <= 0 {
		t.Errorf("ExtractPptxMetadata() SlideCount = %d, expected > 0", metadata.SlideCount)
	}
}

func TestExtractPptxMetadata_NonExistent(t *testing.T) {
	_, err := ExtractPptxMetadata("/nonexistent/file.pptx")
	if err == nil {
		t.Error("ExtractPptxMetadata() expected error for non-existent file")
	}
}
