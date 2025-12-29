package document

import (
	"testing"
)

func TestExtractTextFromTags(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		tagPrefix string
		want      string
	}{
		{
			name:      "single tag",
			data:      "<a:t>Hello World</a:t>",
			tagPrefix: "a:t",
			want:      "Hello World",
		},
		{
			name:      "multiple tags",
			data:      "<a:t>Hello</a:t> <a:t>World</a:t>",
			tagPrefix: "a:t",
			want:      "Hello World",
		},
		{
			name:      "tag with attributes",
			data:      `<a:t xml:space="preserve">Content</a:t>`,
			tagPrefix: "a:t",
			want:      "Content",
		},
		{
			name:      "nested content",
			data:      "<w:t>First</w:t><w:r><w:t>Second</w:t></w:r>",
			tagPrefix: "w:t",
			want:      "First Second",
		},
		{
			name:      "empty tags",
			data:      "<a:t></a:t>",
			tagPrefix: "a:t",
			want:      "",
		},
		{
			name:      "no matching tags",
			data:      "<other>Content</other>",
			tagPrefix: "a:t",
			want:      "",
		},
		{
			name:      "unclosed tag",
			data:      "<a:t>Incomplete",
			tagPrefix: "a:t",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTextFromTags([]byte(tt.data), tt.tagPrefix)
			if got != tt.want {
				t.Errorf("ExtractTextFromTags() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOpenOfficeFile_NonExistent(t *testing.T) {
	_, err := OpenOfficeFile("/nonexistent/file.pptx")
	if err == nil {
		t.Error("OpenOfficeFile() expected error for non-existent file")
	}
}
