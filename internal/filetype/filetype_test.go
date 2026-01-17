package filetype

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashBytes(t *testing.T) {
	content1 := []byte("hello")
	content2 := []byte("world")

	hash1 := HashBytes(content1)
	hash2 := HashBytes(content2)

	if hash1 == hash2 {
		t.Error("different content should produce different hashes")
	}
	// SHA256 produces 32 bytes = 64 hex characters
	if len(hash1) != 64 {
		t.Errorf("hash length = %d, want 64 (SHA256)", len(hash1))
	}
}

func TestHashFileMatchesBytes(t *testing.T) {
	content := []byte("hash me")
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	hashFile, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile failed: %v", err)
	}

	hashBytes := HashBytes(content)
	if hashFile != hashBytes {
		t.Errorf("HashFile = %q, want %q", hashFile, hashBytes)
	}
}

func TestDetectMIME(t *testing.T) {
	tests := []struct {
		path     string
		content  []byte
		expected string
	}{
		{"/test/file.go", nil, "text/x-go"},
		{"/test/file.py", nil, "text/x-python"},
		{"/test/file.js", nil, "text/javascript"},
		{"/test/file.ts", nil, "text/typescript"},
		{"/test/file.md", nil, "text/markdown"},
		{"/test/file.json", nil, "application/json"},
		{"/test/file.yaml", nil, "text/yaml"},
		{"/test/file.unknown", nil, "application/octet-stream"},
		{"/test/file.unknown", []byte("{\"k\": \"v\"}"), "application/json"},
	}

	for _, tt := range tests {
		result := DetectMIME(tt.path, tt.content)
		if result != tt.expected {
			t.Errorf("DetectMIME(%q) = %q, want %q", tt.path, result, tt.expected)
		}
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/test/file.go", "go"},
		{"/test/file.py", "python"},
		{"/test/file.js", "javascript"},
		{"/test/file.ts", "typescript"},
		{"/test/file.rs", "rust"},
		{"/test/file.rb", "ruby"},
		{"/test/file.unknown", ""},
	}

	for _, tt := range tests {
		result := DetectLanguage(tt.path)
		if result != tt.expected {
			t.Errorf("DetectLanguage(%q) = %q, want %q", tt.path, result, tt.expected)
		}
	}
}
