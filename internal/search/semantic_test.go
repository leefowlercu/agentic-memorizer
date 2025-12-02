package search

import (
	"testing"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

func TestNewSearcher(t *testing.T) {
	index := &types.GraphIndex{}
	searcher := NewSearcher(index)

	if searcher == nil {
		t.Fatal("NewSearcher returned nil")
	}

	if searcher.index != index {
		t.Error("Searcher does not reference the provided index")
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	index := &types.GraphIndex{
		Files: []types.FileEntry{
			{
				Path: "/test/file.txt",
				Name: "file.txt",
			},
		},
	}

	searcher := NewSearcher(index)
	results := searcher.Search(SearchQuery{Query: ""})

	if len(results) != 0 {
		t.Errorf("Expected 0 results for empty query, got %d", len(results))
	}
}

func TestSearch_FilenameMatch(t *testing.T) {
	index := &types.GraphIndex{
		Files: []types.FileEntry{
			{
				Path: "/test/terraform-guide.md",
				Name: "terraform-guide.md",
			},
			{
				Path: "/test/unrelated.txt",
				Name: "unrelated.txt",
			},
		},
	}

	searcher := NewSearcher(index)
	results := searcher.Search(SearchQuery{Query: "terraform"})

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].MatchType != "filename" {
		t.Errorf("Match type = %s, want filename", results[0].MatchType)
	}

	if results[0].Score != 3.0 {
		t.Errorf("Score = %.1f, want 3.0", results[0].Score)
	}
}

func TestSearch_SummaryMatch(t *testing.T) {
	index := &types.GraphIndex{
		Files: []types.FileEntry{
			{
				Path:    "/test/doc.md",
				Name:    "doc.md",
				Summary: "A guide about HashiCorp Terraform",
			},
		},
	}

	searcher := NewSearcher(index)
	results := searcher.Search(SearchQuery{Query: "terraform"})

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].MatchType != "summary" {
		t.Errorf("Match type = %s, want summary", results[0].MatchType)
	}

	if results[0].Score != 2.0 {
		t.Errorf("Score = %.1f, want 2.0", results[0].Score)
	}
}

func TestSearch_TagMatch(t *testing.T) {
	index := &types.GraphIndex{
		Files: []types.FileEntry{
			{
				Path: "/test/doc.md",
				Name: "doc.md",
				Tags: []string{"terraform", "infrastructure", "automation"},
			},
		},
	}

	searcher := NewSearcher(index)
	results := searcher.Search(SearchQuery{Query: "terraform"})

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].MatchType != "tag" {
		t.Errorf("Match type = %s, want tag", results[0].MatchType)
	}

	if results[0].Score != 1.5 {
		t.Errorf("Score = %.1f, want 1.5", results[0].Score)
	}
}

func TestSearch_TopicMatch(t *testing.T) {
	index := &types.GraphIndex{
		Files: []types.FileEntry{
			{
				Path:   "/test/doc.md",
				Name:   "doc.md",
				Topics: []string{"Terraform automation", "Infrastructure as Code"},
			},
		},
	}

	searcher := NewSearcher(index)
	results := searcher.Search(SearchQuery{Query: "terraform"})

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].MatchType != "topic" {
		t.Errorf("Match type = %s, want topic", results[0].MatchType)
	}

	if results[0].Score != 1.0 {
		t.Errorf("Score = %.1f, want 1.0", results[0].Score)
	}
}

func TestSearch_DocumentTypeMatch(t *testing.T) {
	index := &types.GraphIndex{
		Files: []types.FileEntry{
			{
				Path:         "/test/doc.md",
				Name:         "doc.md",
				DocumentType: "terraform-configuration",
			},
		},
	}

	searcher := NewSearcher(index)
	results := searcher.Search(SearchQuery{Query: "terraform"})

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].MatchType != "document_type" {
		t.Errorf("Match type = %s, want document_type", results[0].MatchType)
	}

	if results[0].Score != 0.5 {
		t.Errorf("Score = %.1f, want 0.5", results[0].Score)
	}
}

func TestSearch_MultipleMatches(t *testing.T) {
	index := &types.GraphIndex{
		Files: []types.FileEntry{
			{
				Path:    "/test/terraform-guide.md",
				Name:    "terraform-guide.md",
				Summary: "A comprehensive guide to Terraform",
				Tags:    []string{"terraform", "iac"},
				Topics:  []string{"Terraform fundamentals"},
			},
		},
	}

	searcher := NewSearcher(index)
	results := searcher.Search(SearchQuery{Query: "terraform"})

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	// Should have matches in filename (3.0) + summary (2.0) + tag (1.5) + topic (1.0) = 7.5
	expectedScore := 7.5
	if results[0].Score != expectedScore {
		t.Errorf("Score = %.1f, want %.1f", results[0].Score, expectedScore)
	}

	// Primary match type should be filename (highest weight)
	if results[0].MatchType != "filename" {
		t.Errorf("Match type = %s, want filename", results[0].MatchType)
	}
}

func TestSearch_Scoring(t *testing.T) {
	index := &types.GraphIndex{
		Files: []types.FileEntry{
			{
				Path:    "/test/terraform-config.tf",
				Name:    "terraform-config.tf",
				Summary: "Terraform configuration",
				Tags:    []string{"terraform"},
			},
			{
				Path:    "/test/guide.md",
				Name:    "guide.md",
				Summary: "Mentions terraform in passing",
			},
			{
				Path:    "/test/unrelated.txt",
				Name:    "unrelated.txt",
				Summary: "Not relevant",
			},
		},
	}

	searcher := NewSearcher(index)
	results := searcher.Search(SearchQuery{Query: "terraform"})

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// Results should be sorted by score descending
	if results[0].Score < results[1].Score {
		t.Error("Results not sorted by score descending")
	}

	// First result should be terraform-config.tf (filename + summary + tag = 6.5)
	if !contains(results[0].File.Path, "terraform-config") {
		t.Errorf("First result = %s, expected terraform-config", results[0].File.Path)
	}
}

func TestSearch_CategoryFilter(t *testing.T) {
	index := &types.GraphIndex{
		Files: []types.FileEntry{
			{
				Path:     "/test/terraform-guide.md",
				Name:     "terraform-guide.md",
				Category: "documents",
				Summary:  "Terraform guide",
			},
			{
				Path:     "/test/main.tf",
				Name:     "main.tf",
				Category: "code",
				Summary:  "Terraform configuration",
			},
		},
	}

	searcher := NewSearcher(index)
	results := searcher.Search(SearchQuery{
		Query:      "terraform",
		Categories: []string{"documents"},
	})

	if len(results) != 1 {
		t.Fatalf("Expected 1 result with category filter, got %d", len(results))
	}

	if results[0].File.Category != "documents" {
		t.Errorf("Category = %s, want documents", results[0].File.Category)
	}
}

func TestSearch_MaxResults(t *testing.T) {
	index := &types.GraphIndex{
		Files: []types.FileEntry{},
	}

	// Add 10 entries matching "test"
	for i := 0; i < 10; i++ {
		index.Files = append(index.Files, types.FileEntry{
			Path: "/test/file-test.txt",
			Name: "file-test.txt",
		})
	}

	searcher := NewSearcher(index)
	results := searcher.Search(SearchQuery{
		Query:      "test",
		MaxResults: 5,
	})

	if len(results) != 5 {
		t.Errorf("Expected 5 results with MaxResults=5, got %d", len(results))
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	index := &types.GraphIndex{
		Files: []types.FileEntry{
			{
				Path:    "/test/Terraform-Guide.md",
				Name:    "Terraform-Guide.md",
				Summary: "A Guide About TERRAFORM",
				Tags:    []string{"TERRAFORM", "IaC"},
			},
		},
	}

	searcher := NewSearcher(index)

	// Test with lowercase query
	results := searcher.Search(SearchQuery{Query: "terraform"})
	if len(results) != 1 {
		t.Errorf("Lowercase query: expected 1 result, got %d", len(results))
	}

	// Test with uppercase query
	results = searcher.Search(SearchQuery{Query: "TERRAFORM"})
	if len(results) != 1 {
		t.Errorf("Uppercase query: expected 1 result, got %d", len(results))
	}

	// Test with mixed case query
	results = searcher.Search(SearchQuery{Query: "TerraForm"})
	if len(results) != 1 {
		t.Errorf("Mixed case query: expected 1 result, got %d", len(results))
	}
}

func TestSearch_NoSemanticAnalysis(t *testing.T) {
	index := &types.GraphIndex{
		Files: []types.FileEntry{
			{
				Path:    "/test/terraform.config",
				Name:    "terraform.config",
				Summary: "", // No semantic analysis
			},
		},
	}

	searcher := NewSearcher(index)
	results := searcher.Search(SearchQuery{Query: "terraform"})

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	// Should only match on filename
	if results[0].Score != 3.0 {
		t.Errorf("Score = %.1f, want 3.0 (filename only)", results[0].Score)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
