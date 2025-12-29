package common

import "testing"

func TestGetMediaType(t *testing.T) {
	tests := []struct {
		name     string
		fileType string
		expected string
	}{
		// PNG variants
		{name: "png lowercase", fileType: "png", expected: "image/png"},
		{name: "png with dot", fileType: ".png", expected: "image/png"},
		{name: "PNG uppercase", fileType: "PNG", expected: "image/png"},

		// JPEG variants
		{name: "jpg lowercase", fileType: "jpg", expected: "image/jpeg"},
		{name: "jpeg lowercase", fileType: "jpeg", expected: "image/jpeg"},
		{name: "jpg with dot", fileType: ".jpg", expected: "image/jpeg"},
		{name: "jpeg with dot", fileType: ".jpeg", expected: "image/jpeg"},

		// GIF
		{name: "gif lowercase", fileType: "gif", expected: "image/gif"},
		{name: "gif with dot", fileType: ".gif", expected: "image/gif"},

		// WebP
		{name: "webp lowercase", fileType: "webp", expected: "image/webp"},
		{name: "webp with dot", fileType: ".webp", expected: "image/webp"},

		// HEIC/HEIF (Apple formats)
		{name: "heic lowercase", fileType: "heic", expected: "image/heic"},
		{name: "heic with dot", fileType: ".heic", expected: "image/heic"},
		{name: "heif lowercase", fileType: "heif", expected: "image/heif"},
		{name: "heif with dot", fileType: ".heif", expected: "image/heif"},

		// Unknown defaults to jpeg
		{name: "unknown format", fileType: "bmp", expected: "image/jpeg"},
		{name: "empty string", fileType: "", expected: "image/jpeg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetMediaType(tt.fileType)
			if result != tt.expected {
				t.Errorf("GetMediaType(%q) = %q, want %q", tt.fileType, result, tt.expected)
			}
		})
	}
}
