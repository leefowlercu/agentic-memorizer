package testutil

import (
	"testing"
)

func TestCreateMockFileIndex(t *testing.T) {
	index := CreateMockFileIndex()

	if index == nil {
		t.Fatal("CreateMockFileIndex() returned nil")
	}

	if index.MemoryRoot == "" {
		t.Error("MemoryRoot should not be empty")
	}

	if len(index.Files) != 1 {
		t.Errorf("Files count = %d, want 1", len(index.Files))
	}

	if index.Stats.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1", index.Stats.TotalFiles)
	}
}

func TestCreateMockFileIndexWithFiles(t *testing.T) {
	tests := []struct {
		count int
	}{
		{0},
		{1},
		{5},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			index := CreateMockFileIndexWithFiles(tt.count)

			if index == nil {
				t.Fatal("CreateMockFileIndexWithFiles() returned nil")
			}

			if len(index.Files) != tt.count {
				t.Errorf("Files count = %d, want %d", len(index.Files), tt.count)
			}

			if index.Stats.TotalFiles != tt.count {
				t.Errorf("TotalFiles = %d, want %d", index.Stats.TotalFiles, tt.count)
			}
		})
	}
}

func TestCreateMockFactsIndex(t *testing.T) {
	facts := CreateMockFactsIndex()

	if facts == nil {
		t.Fatal("CreateMockFactsIndex() returned nil")
	}

	if len(facts.Facts) != 1 {
		t.Errorf("Facts count = %d, want 1", len(facts.Facts))
	}

	if facts.Stats.TotalFacts != 1 {
		t.Errorf("TotalFacts = %d, want 1", facts.Stats.TotalFacts)
	}
}

func TestCreateMockFactsIndexWithFacts(t *testing.T) {
	tests := []struct {
		count int
	}{
		{0},
		{1},
		{5},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			facts := CreateMockFactsIndexWithFacts(tt.count)

			if facts == nil {
				t.Fatal("CreateMockFactsIndexWithFacts() returned nil")
			}

			if len(facts.Facts) != tt.count {
				t.Errorf("Facts count = %d, want %d", len(facts.Facts), tt.count)
			}

			if facts.Stats.TotalFacts != tt.count {
				t.Errorf("TotalFacts = %d, want %d", facts.Stats.TotalFacts, tt.count)
			}
		})
	}
}
