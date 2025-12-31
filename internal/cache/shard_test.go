package cache

import (
	"testing"
)

func TestShardPath(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		hash     string
		filename string
		expected string
	}{
		{
			name:     "sha256 prefixed hash",
			basePath: "/cache/summaries/claude",
			hash:     "sha256:41d63309faf26bf95fba3b553dba9fd793effe0116d9237a012499587f8ce94b",
			filename: "sha256:41d63309f-v1-1-2.json",
			expected: "/cache/summaries/claude/41/d6/sha256:41d63309f-v1-1-2.json",
		},
		{
			name:     "plain hash without prefix",
			basePath: "/cache/embeddings/openai",
			hash:     "abc123def456",
			filename: "abc123def456.emb",
			expected: "/cache/embeddings/openai/ab/c1/abc123def456.emb",
		},
		{
			name:     "short hash no sharding",
			basePath: "/cache/test",
			hash:     "ab",
			filename: "ab.json",
			expected: "/cache/test/ab.json",
		},
		{
			name:     "exactly 4 chars",
			basePath: "/cache/test",
			hash:     "abcd",
			filename: "abcd.json",
			expected: "/cache/test/ab/cd/abcd.json",
		},
		{
			name:     "empty base path",
			basePath: "",
			hash:     "sha256:fedcba9876543210",
			filename: "file.json",
			expected: "fe/dc/file.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShardPath(tt.basePath, tt.hash, tt.filename)
			if got != tt.expected {
				t.Errorf("ShardPath() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestExtractHashValue(t *testing.T) {
	tests := []struct {
		hash     string
		expected string
	}{
		{"sha256:abc123", "abc123"},
		{"abc123", "abc123"},
		{"sha256:", "sha256:"}, // Edge case: prefix only, no hash value - returns as-is
		{"sha25", "sha25"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.hash, func(t *testing.T) {
			got := extractHashValue(tt.hash)
			if got != tt.expected {
				t.Errorf("extractHashValue(%q) = %q, want %q", tt.hash, got, tt.expected)
			}
		})
	}
}
