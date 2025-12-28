package testutil

import (
	"testing"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// CreateMockFileIndex creates a test FileIndex with minimal data
func CreateMockFileIndex() *types.FileIndex {
	return &types.FileIndex{
		MemoryRoot: "/test/memory",
		Stats: types.IndexStats{
			TotalFiles:    1,
			TotalSize:     1024,
			CachedFiles:   1,
			AnalyzedFiles: 0,
		},
		Files: []types.FileEntry{
			{
				Path:     "/test/file.txt",
				Name:     "file.txt",
				Category: "documents",
				Size:     1024,
				Type:     "txt",
				Summary:  "Test file summary",
				Tags:     []string{"test"},
			},
		},
	}
}

// CreateMockFileIndexWithFiles creates a test FileIndex with specified number of files
func CreateMockFileIndexWithFiles(count int) *types.FileIndex {
	files := make([]types.FileEntry, count)
	for i := 0; i < count; i++ {
		files[i] = types.FileEntry{
			Path:     "/test/file" + string(rune('0'+i)) + ".txt",
			Name:     "file" + string(rune('0'+i)) + ".txt",
			Category: "documents",
			Size:     1024,
			Type:     "txt",
			Summary:  "Test file " + string(rune('0'+i)),
			Tags:     []string{"test"},
		}
	}

	return &types.FileIndex{
		MemoryRoot: "/test/memory",
		Stats: types.IndexStats{
			TotalFiles:    count,
			TotalSize:     int64(count * 1024),
			CachedFiles:   count,
			AnalyzedFiles: 0,
		},
		Files: files,
	}
}

// CreateMockFactsIndex creates a test FactsIndex with minimal data
func CreateMockFactsIndex() *types.FactsIndex {
	return &types.FactsIndex{
		Stats: types.FactStats{
			TotalFacts: 1,
			MaxFacts:   50,
		},
		Facts: []types.Fact{
			{
				ID:      "test-id-1",
				Content: "Test fact content",
			},
		},
	}
}

// CreateMockFactsIndexWithFacts creates a test FactsIndex with specified number of facts
func CreateMockFactsIndexWithFacts(count int) *types.FactsIndex {
	facts := make([]types.Fact, count)
	for i := 0; i < count; i++ {
		facts[i] = types.Fact{
			ID:      "test-id-" + string(rune('0'+i)),
			Content: "Test fact " + string(rune('0'+i)),
		}
	}

	return &types.FactsIndex{
		Stats: types.FactStats{
			TotalFacts: count,
			MaxFacts:   50,
		},
		Facts: facts,
	}
}

// AssertNoError fails the test if err is not nil
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// AssertError fails the test if err is nil
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error but got nil")
	}
}

// AssertTrue fails the test if condition is false
func AssertTrue(t *testing.T, condition bool, msg string) {
	t.Helper()
	if !condition {
		t.Fatalf("assertion failed: %s", msg)
	}
}

// AssertFalse fails the test if condition is true
func AssertFalse(t *testing.T, condition bool, msg string) {
	t.Helper()
	if condition {
		t.Fatalf("assertion failed: %s", msg)
	}
}

// AssertEqual fails the test if got != want
func AssertEqual[T comparable](t *testing.T, got, want T, name string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}
