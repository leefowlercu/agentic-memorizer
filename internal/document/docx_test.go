package document

import (
	"testing"
)

func TestExtractDocxText(t *testing.T) {
	path := getTestdataPath("sample.docx")

	text, err := ExtractDocxText(path)
	if err != nil {
		t.Fatalf("ExtractDocxText() error = %v", err)
	}

	if text == "" {
		t.Error("ExtractDocxText() returned empty string, expected text content")
	}
}

func TestExtractDocxText_NonExistent(t *testing.T) {
	_, err := ExtractDocxText("/nonexistent/file.docx")
	if err == nil {
		t.Error("ExtractDocxText() expected error for non-existent file")
	}
}

func TestExtractDocxMetadata(t *testing.T) {
	path := getTestdataPath("sample.docx")

	metadata, err := ExtractDocxMetadata(path)
	if err != nil {
		t.Fatalf("ExtractDocxMetadata() error = %v", err)
	}

	if metadata.WordCount <= 0 {
		t.Errorf("ExtractDocxMetadata() WordCount = %d, expected > 0", metadata.WordCount)
	}
}

func TestExtractDocxMetadata_NonExistent(t *testing.T) {
	_, err := ExtractDocxMetadata("/nonexistent/file.docx")
	if err == nil {
		t.Error("ExtractDocxMetadata() expected error for non-existent file")
	}
}

func TestExtractDocxMetadata_Malformed(t *testing.T) {
	path := getTestdataPath("malformed.docx")

	// Malformed files may or may not produce an error depending on the malformation
	// but should not panic
	_, _ = ExtractDocxMetadata(path)
}

func TestExtractCreatorFromCoreProps(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "valid creator",
			data: `<?xml version="1.0" encoding="UTF-8"?><cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:creator>John Doe</dc:creator></cp:coreProperties>`,
			want: "John Doe",
		},
		{
			name: "empty creator",
			data: `<?xml version="1.0" encoding="UTF-8"?><cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties"><dc:creator></dc:creator></cp:coreProperties>`,
			want: "",
		},
		{
			name: "no creator element",
			data: `<?xml version="1.0" encoding="UTF-8"?><cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties"></cp:coreProperties>`,
			want: "",
		},
		{
			name: "invalid xml",
			data: `not valid xml`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCreatorFromCoreProps([]byte(tt.data))
			if got != tt.want {
				t.Errorf("extractCreatorFromCoreProps() = %q, want %q", got, tt.want)
			}
		})
	}
}
