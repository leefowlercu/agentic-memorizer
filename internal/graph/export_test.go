package graph

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

func TestNewExporter(t *testing.T) {
	manager := NewManager(DefaultManagerConfig(), nil)

	tests := []struct {
		name   string
		logger *slog.Logger
	}{
		{
			name:   "with nil logger",
			logger: nil,
		},
		{
			name:   "with custom logger",
			logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter := NewExporter(manager, tt.logger)

			if exporter == nil {
				t.Fatal("expected non-nil exporter")
			}
			if exporter.manager == nil {
				t.Error("expected non-nil manager")
			}
			if exporter.logger == nil {
				t.Error("expected non-nil logger even when nil is passed")
			}
		})
	}
}

func TestExporter_ToGraphIndex_NotConnected(t *testing.T) {
	manager := NewManager(DefaultManagerConfig(), nil)
	exporter := NewExporter(manager, nil)
	ctx := context.Background()

	_, err := exporter.ToGraphIndex(ctx, "/test/memory")
	if err == nil {
		t.Error("expected error when exporting from unconnected manager")
	}
}

func TestExporter_ToSummary_NotConnected(t *testing.T) {
	manager := NewManager(DefaultManagerConfig(), nil)
	exporter := NewExporter(manager, nil)
	ctx := context.Background()

	_, err := exporter.ToSummary(ctx, "/test/memory", 7, 10)
	if err == nil {
		t.Error("expected error when exporting from unconnected manager")
	}
}

// Test helper functions

func TestCalculateTotalSize(t *testing.T) {
	tests := []struct {
		name     string
		entries  []types.IndexEntry
		expected int64
	}{
		{
			name:     "empty entries",
			entries:  []types.IndexEntry{},
			expected: 0,
		},
		{
			name: "single entry",
			entries: []types.IndexEntry{
				{
					Metadata: types.FileMetadata{
						FileInfo: types.FileInfo{Size: 1024},
					},
				},
			},
			expected: 1024,
		},
		{
			name: "multiple entries",
			entries: []types.IndexEntry{
				{
					Metadata: types.FileMetadata{
						FileInfo: types.FileInfo{Size: 1024},
					},
				},
				{
					Metadata: types.FileMetadata{
						FileInfo: types.FileInfo{Size: 2048},
					},
				},
				{
					Metadata: types.FileMetadata{
						FileInfo: types.FileInfo{Size: 512},
					},
				},
			},
			expected: 3584,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateTotalSize(tt.entries)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestCountAnalyzedFiles(t *testing.T) {
	tests := []struct {
		name     string
		entries  []types.IndexEntry
		expected int
	}{
		{
			name:     "empty entries",
			entries:  []types.IndexEntry{},
			expected: 0,
		},
		{
			name: "no analyzed files",
			entries: []types.IndexEntry{
				{Semantic: nil},
				{Semantic: nil},
			},
			expected: 0,
		},
		{
			name: "all analyzed",
			entries: []types.IndexEntry{
				{Semantic: &types.SemanticAnalysis{Summary: "test"}},
				{Semantic: &types.SemanticAnalysis{Summary: "test2"}},
			},
			expected: 2,
		},
		{
			name: "mixed",
			entries: []types.IndexEntry{
				{Semantic: &types.SemanticAnalysis{Summary: "test"}},
				{Semantic: nil},
				{Semantic: &types.SemanticAnalysis{Summary: "test2"}},
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countAnalyzedFiles(tt.entries)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestCountErrorFiles(t *testing.T) {
	errPtr := func(s string) *string { return &s }

	tests := []struct {
		name     string
		entries  []types.IndexEntry
		expected int
	}{
		{
			name:     "empty entries",
			entries:  []types.IndexEntry{},
			expected: 0,
		},
		{
			name: "no error files",
			entries: []types.IndexEntry{
				{Error: nil},
				{Error: nil},
			},
			expected: 0,
		},
		{
			name: "all errors",
			entries: []types.IndexEntry{
				{Error: errPtr("error1")},
				{Error: errPtr("error2")},
			},
			expected: 2,
		},
		{
			name: "mixed",
			entries: []types.IndexEntry{
				{Error: errPtr("error1")},
				{Error: nil},
				{Error: errPtr("error2")},
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countErrorFiles(tt.entries)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// Test struct definitions

func TestExportSummary_Struct(t *testing.T) {
	summary := &ExportSummary{
		Generated:  time.Now(),
		Root:       "/test/memory",
		TotalFiles: 100,
		Categories: map[string]int64{
			"documents": 50,
			"code":      50,
		},
		TopTags: []TagCount{
			{Name: "golang", Count: 20},
		},
		TopTopics: []TopicCount{
			{Name: "development", Count: 30},
		},
		TopEntities: []EntityCount{
			{Name: "FalkorDB", Type: "technology", Count: 15},
		},
		RecentFiles: []FileSummary{
			{Path: "/test/file.txt", Name: "file.txt", Category: "documents"},
		},
	}

	if summary.TotalFiles != 100 {
		t.Errorf("expected TotalFiles 100, got %d", summary.TotalFiles)
	}
	if summary.Categories["documents"] != 50 {
		t.Errorf("expected Categories['documents'] 50, got %d", summary.Categories["documents"])
	}
	if len(summary.TopTags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(summary.TopTags))
	}
	if summary.TopTags[0].Name != "golang" {
		t.Errorf("expected tag name 'golang', got %q", summary.TopTags[0].Name)
	}
}

func TestTagCount_Struct(t *testing.T) {
	tag := TagCount{
		Name:  "golang",
		Count: 25,
	}

	if tag.Name != "golang" {
		t.Errorf("expected Name 'golang', got %q", tag.Name)
	}
	if tag.Count != 25 {
		t.Errorf("expected Count 25, got %d", tag.Count)
	}
}

func TestTopicCount_Struct(t *testing.T) {
	topic := TopicCount{
		Name:  "development",
		Count: 30,
	}

	if topic.Name != "development" {
		t.Errorf("expected Name 'development', got %q", topic.Name)
	}
	if topic.Count != 30 {
		t.Errorf("expected Count 30, got %d", topic.Count)
	}
}

func TestEntityCount_Struct(t *testing.T) {
	entity := EntityCount{
		Name:  "Terraform",
		Type:  "technology",
		Count: 15,
	}

	if entity.Name != "Terraform" {
		t.Errorf("expected Name 'Terraform', got %q", entity.Name)
	}
	if entity.Type != "technology" {
		t.Errorf("expected Type 'technology', got %q", entity.Type)
	}
	if entity.Count != 15 {
		t.Errorf("expected Count 15, got %d", entity.Count)
	}
}

func TestFileSummary_Struct(t *testing.T) {
	now := time.Now()
	summary := FileSummary{
		Path:     "/test/file.txt",
		Name:     "file.txt",
		Category: "documents",
		Modified: now,
		Summary:  "Test file summary",
	}

	if summary.Path != "/test/file.txt" {
		t.Errorf("expected Path '/test/file.txt', got %q", summary.Path)
	}
	if summary.Name != "file.txt" {
		t.Errorf("expected Name 'file.txt', got %q", summary.Name)
	}
	if summary.Category != "documents" {
		t.Errorf("expected Category 'documents', got %q", summary.Category)
	}
	if summary.Summary != "Test file summary" {
		t.Errorf("expected Summary 'Test file summary', got %q", summary.Summary)
	}
}
