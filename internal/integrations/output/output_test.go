package output

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// Helper function to create a simple test graph index
func createTestGraphIndex() *types.GraphIndex {
	now := time.Now()
	return &types.GraphIndex{
		Generated:  now,
		MemoryRoot: "/test/root",
		Stats: types.IndexStats{
			TotalFiles:    2,
			TotalSize:     2048,
			AnalyzedFiles: 1,
			CachedFiles:   1,
		},
		Files: []types.FileEntry{
			{
				Path:       "/test/root/document.md",
				Name:       "document.md",
				Hash:       "abc123",
				Size:       1024,
				SizeHuman:  "1.0 KB",
				Modified:   now,
				Type:       "markdown",
				Category:   "documents",
				IsReadable: true,
				Summary:    "Test document summary",
				Tags:       []string{"test", "example"},
				Topics:     []string{"testing", "documentation"},
				DocumentType: "guide",
			},
			{
				Path:       "/test/root/code.go",
				Name:       "code.go",
				Hash:       "def456",
				Size:       1024,
				SizeHuman:  "1.0 KB",
				Modified:   now,
				Type:       "go",
				Category:   "code",
				IsReadable: true,
			},
		},
	}
}

func TestXMLProcessor(t *testing.T) {
	processor := NewXMLProcessor()

	if processor.GetFormat() != "xml" {
		t.Errorf("Expected format 'xml', got '%s'", processor.GetFormat())
	}

	index := createTestGraphIndex()
	output, err := processor.FormatGraph(index)
	if err != nil {
		t.Fatalf("Failed to format index as XML: %v", err)
	}

	// Basic structure checks
	if !strings.Contains(output, "<memory_index>") {
		t.Error("Expected XML to contain <memory_index> tag")
	}

	if !strings.Contains(output, "</memory_index>") {
		t.Error("Expected XML to contain closing </memory_index> tag")
	}

	if !strings.Contains(output, "<metadata>") {
		t.Error("Expected XML to contain <metadata> section")
	}

	if !strings.Contains(output, "<categories>") {
		t.Error("Expected XML to contain <categories> section")
	}

	if !strings.Contains(output, "<usage_guide>") {
		t.Error("Expected XML to contain <usage_guide> section")
	}

	// Content checks
	if !strings.Contains(output, "document.md") {
		t.Error("Expected XML to contain file name")
	}

	if !strings.Contains(output, "Test document summary") {
		t.Error("Expected XML to contain semantic summary")
	}

	// Verify escaping
	testIndex := createTestGraphIndex()
	testIndex.Files[0].Path = "/test/root/<test>.md"
	escapedOutput, _ := processor.FormatGraph(testIndex)
	if !strings.Contains(escapedOutput, "&lt;test&gt;") {
		t.Error("Expected XML escaping for special characters")
	}
}

func TestMarkdownProcessor(t *testing.T) {
	processor := NewMarkdownProcessor()

	if processor.GetFormat() != "markdown" {
		t.Errorf("Expected format 'markdown', got '%s'", processor.GetFormat())
	}

	index := createTestGraphIndex()
	output, err := processor.FormatGraph(index)
	if err != nil {
		t.Fatalf("Failed to format index as Markdown: %v", err)
	}

	// Basic structure checks
	if !strings.Contains(output, "# Claude Code Agentic Memory Index") {
		t.Error("Expected Markdown to contain title")
	}

	if !strings.Contains(output, "## Usage Guide") {
		t.Error("Expected Markdown to contain usage guide section")
	}

	// Content checks
	if !strings.Contains(output, "document.md") {
		t.Error("Expected Markdown to contain file name")
	}

	if !strings.Contains(output, "Test document summary") {
		t.Error("Expected Markdown to contain semantic summary")
	}

	if !strings.Contains(output, "`test`") || !strings.Contains(output, "`example`") {
		t.Error("Expected Markdown to contain formatted tags")
	}
}

func TestJSONProcessor(t *testing.T) {
	processor := NewJSONProcessor()

	if processor.GetFormat() != "json" {
		t.Errorf("Expected format 'json', got '%s'", processor.GetFormat())
	}

	index := createTestGraphIndex()
	output, err := processor.FormatGraph(index)
	if err != nil {
		t.Fatalf("Failed to format index as JSON: %v", err)
	}

	// Verify it's valid JSON
	var parsed types.GraphIndex
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Verify structure
	if parsed.Stats.TotalFiles != 2 {
		t.Errorf("Expected 2 files, got %d", parsed.Stats.TotalFiles)
	}

	if len(parsed.Files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(parsed.Files))
	}

	// Verify content
	if parsed.MemoryRoot != "/test/root" {
		t.Errorf("Expected root '/test/root', got '%s'", parsed.MemoryRoot)
	}
}

func TestOutputOptions(t *testing.T) {
	// Test with ShowRecentDays option
	opts := Options{
		ShowRecentDays: 7,
	}

	xmlProcessor := NewXMLProcessor(opts)
	mdProcessor := NewMarkdownProcessor(opts)
	jsonProcessor := NewJSONProcessor(opts)

	index := createTestGraphIndex()

	// XML with recent days
	xmlOutput, _ := xmlProcessor.FormatGraph(index)
	if !strings.Contains(xmlOutput, "<recent_activity") {
		t.Error("Expected XML to contain recent_activity section when ShowRecentDays is set")
	}

	// Markdown with recent days
	mdOutput, _ := mdProcessor.FormatGraph(index)
	if !strings.Contains(mdOutput, "Recent Activity") {
		t.Error("Expected Markdown to contain recent activity section when ShowRecentDays is set")
	}

	// JSON with recent days
	jsonOutput, _ := jsonProcessor.FormatGraph(index)
	var parsed map[string]any
	json.Unmarshal([]byte(jsonOutput), &parsed)
	if _, ok := parsed["recent_files"]; !ok {
		t.Error("Expected JSON to contain recent_files when ShowRecentDays is set")
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, test := range tests {
		result := formatSize(test.bytes)
		if result != test.expected {
			t.Errorf("formatSize(%d) = %s, expected %s", test.bytes, result, test.expected)
		}
	}
}

func TestGroupFilesByCategory(t *testing.T) {
	files := []types.FileEntry{
		{Path: "/test/doc1.md", Name: "doc1.md", Category: "documents"},
		{Path: "/test/code.go", Name: "code.go", Category: "code"},
		{Path: "/test/doc2.md", Name: "doc2.md", Category: "documents"},
	}

	grouped := groupFilesByCategory(files)

	if len(grouped) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(grouped))
	}

	if len(grouped["documents"]) != 2 {
		t.Errorf("Expected 2 documents, got %d", len(grouped["documents"]))
	}

	if len(grouped["code"]) != 1 {
		t.Errorf("Expected 1 code file, got %d", len(grouped["code"]))
	}
}

func TestGetRecentFileEntries(t *testing.T) {
	now := time.Now()
	files := []types.FileEntry{
		{Path: "/test/file1.txt", Name: "file1.txt", Modified: now.AddDate(0, 0, -1)},  // 1 day ago
		{Path: "/test/file2.txt", Name: "file2.txt", Modified: now.AddDate(0, 0, -5)},  // 5 days ago
		{Path: "/test/file3.txt", Name: "file3.txt", Modified: now.AddDate(0, 0, -10)}, // 10 days ago
	}

	// Get files from last 7 days
	recent := getRecentFileEntries(files, 7)

	if len(recent) != 2 {
		t.Errorf("Expected 2 recent files (within 7 days), got %d", len(recent))
	}

	// Verify they're sorted by most recent first
	if !recent[0].Modified.After(recent[1].Modified) {
		t.Error("Expected recent files to be sorted by most recent first")
	}
}

// Helper function to create a test graph index with entities
func createTestGraphIndexWithEntities() *types.GraphIndex {
	now := time.Now()
	return &types.GraphIndex{
		Generated:  now,
		MemoryRoot: "/test/root",
		Stats: types.IndexStats{
			TotalFiles:    1,
			TotalSize:     1024,
			AnalyzedFiles: 1,
		},
		Files: []types.FileEntry{
			{
				Path:         "/test/root/terraform.md",
				Name:         "terraform.md",
				Hash:         "abc123",
				Size:         1024,
				SizeHuman:    "1.0 KB",
				Modified:     now,
				Type:         "markdown",
				Category:     "documents",
				IsReadable:   true,
				Summary:      "Terraform infrastructure guide",
				Tags:         []string{"terraform", "infrastructure"},
				Topics:       []string{"infrastructure-as-code"},
				DocumentType: "technical-guide",
				Entities: []types.EntityRef{
					{Name: "Terraform", Type: "technology"},
					{Name: "AWS", Type: "technology"},
					{Name: "HashiCorp", Type: "organization"},
				},
			},
		},
	}
}

func TestEntityRendering(t *testing.T) {
	index := createTestGraphIndexWithEntities()

	t.Run("XML includes entities", func(t *testing.T) {
		processor := NewXMLProcessor()
		output, err := processor.FormatGraph(index)
		if err != nil {
			t.Fatalf("Failed to format index as XML: %v", err)
		}

		// Check entities section exists
		if !strings.Contains(output, "<entities>") {
			t.Error("Expected XML to contain <entities> section")
		}

		// Check individual entities
		if !strings.Contains(output, `<entity name="Terraform" type="technology"/>`) {
			t.Error("Expected XML to contain Terraform entity")
		}
		if !strings.Contains(output, `<entity name="AWS" type="technology"/>`) {
			t.Error("Expected XML to contain AWS entity")
		}
		if !strings.Contains(output, `<entity name="HashiCorp" type="organization"/>`) {
			t.Error("Expected XML to contain HashiCorp entity")
		}
	})

	t.Run("Markdown includes entities", func(t *testing.T) {
		processor := NewMarkdownProcessor()
		output, err := processor.FormatGraph(index)
		if err != nil {
			t.Fatalf("Failed to format index as Markdown: %v", err)
		}

		// Check entities line exists
		if !strings.Contains(output, "**Entities**:") {
			t.Error("Expected Markdown to contain Entities section")
		}

		// Check entity content
		if !strings.Contains(output, "Terraform (technology)") {
			t.Error("Expected Markdown to contain Terraform entity")
		}
		if !strings.Contains(output, "AWS (technology)") {
			t.Error("Expected Markdown to contain AWS entity")
		}
		if !strings.Contains(output, "HashiCorp (organization)") {
			t.Error("Expected Markdown to contain HashiCorp entity")
		}
	})

	t.Run("JSON includes entities", func(t *testing.T) {
		processor := NewJSONProcessor()
		output, err := processor.FormatGraph(index)
		if err != nil {
			t.Fatalf("Failed to format index as JSON: %v", err)
		}

		// Parse and verify
		var parsed types.GraphIndex
		if err := json.Unmarshal([]byte(output), &parsed); err != nil {
			t.Fatalf("Failed to parse JSON output: %v", err)
		}

		// Check entities
		if len(parsed.Files) != 1 {
			t.Fatalf("Expected 1 file, got %d", len(parsed.Files))
		}

		file := parsed.Files[0]
		if len(file.Entities) != 3 {
			t.Errorf("Expected 3 entities, got %d", len(file.Entities))
		}

		// Verify entity content
		foundTerraform := false
		foundAWS := false
		foundHashiCorp := false
		for _, entity := range file.Entities {
			if entity.Name == "Terraform" && entity.Type == "technology" {
				foundTerraform = true
			}
			if entity.Name == "AWS" && entity.Type == "technology" {
				foundAWS = true
			}
			if entity.Name == "HashiCorp" && entity.Type == "organization" {
				foundHashiCorp = true
			}
		}

		if !foundTerraform {
			t.Error("Expected JSON to contain Terraform entity")
		}
		if !foundAWS {
			t.Error("Expected JSON to contain AWS entity")
		}
		if !foundHashiCorp {
			t.Error("Expected JSON to contain HashiCorp entity")
		}
	})
}

// Helper function to create a test graph index with graph stats
func createTestGraphIndexWithGraphStats() *types.GraphIndex {
	now := time.Now()
	return &types.GraphIndex{
		Generated:  now,
		MemoryRoot: "/test/root",
		Stats: types.IndexStats{
			TotalFiles:        3,
			TotalSize:         3072,
			AnalyzedFiles:     3,
			TotalTags:         15,
			TotalTopics:       8,
			TotalEntities:     12,
			TotalEdges:        45,
			FilesWithSummary:  3,
			FilesWithTags:     3,
			FilesWithTopics:   2,
			FilesWithEntities: 2,
			AvgTagsPerFile:    5.0,
			ByCategory: map[string]int{
				"documents": 2,
				"code":      1,
			},
		},
		Files: []types.FileEntry{
			{
				Path:         "/test/root/doc1.md",
				Name:         "doc1.md",
				Hash:         "abc123",
				Size:         1024,
				SizeHuman:    "1.0 KB",
				Modified:     now,
				Type:         "markdown",
				Category:     "documents",
				IsReadable:   true,
				Summary:      "Document summary",
				Tags:         []string{"tag1", "tag2"},
				Topics:       []string{"topic1"},
				DocumentType: "guide",
			},
		},
	}
}

func TestGraphStatsRendering(t *testing.T) {
	index := createTestGraphIndexWithGraphStats()

	t.Run("XML includes graph stats", func(t *testing.T) {
		processor := NewXMLProcessor()
		output, err := processor.FormatGraph(index)
		if err != nil {
			t.Fatalf("Failed to format index as XML: %v", err)
		}

		// Check graph stats section exists
		if !strings.Contains(output, "<graph_stats>") {
			t.Error("Expected XML to contain <graph_stats> section")
		}

		// Check individual stats
		if !strings.Contains(output, "<total_tags>15</total_tags>") {
			t.Error("Expected XML to contain total_tags")
		}
		if !strings.Contains(output, "<total_topics>8</total_topics>") {
			t.Error("Expected XML to contain total_topics")
		}
		if !strings.Contains(output, "<total_entities>12</total_entities>") {
			t.Error("Expected XML to contain total_entities")
		}
		if !strings.Contains(output, "<total_edges>45</total_edges>") {
			t.Error("Expected XML to contain total_edges")
		}

		// Check coverage section
		if !strings.Contains(output, "<coverage>") {
			t.Error("Expected XML to contain <coverage> section")
		}
		if !strings.Contains(output, "<files_with_summary>3</files_with_summary>") {
			t.Error("Expected XML to contain files_with_summary")
		}
		if !strings.Contains(output, "<avg_tags_per_file>5.0</avg_tags_per_file>") {
			t.Error("Expected XML to contain avg_tags_per_file")
		}
	})

	t.Run("Markdown includes graph stats", func(t *testing.T) {
		processor := NewMarkdownProcessor()
		output, err := processor.FormatGraph(index)
		if err != nil {
			t.Fatalf("Failed to format index as Markdown: %v", err)
		}

		// Check graph stats line
		if !strings.Contains(output, "Tags: 15") {
			t.Error("Expected Markdown to contain tag count")
		}
		if !strings.Contains(output, "Topics: 8") {
			t.Error("Expected Markdown to contain topic count")
		}
		if !strings.Contains(output, "Entities: 12") {
			t.Error("Expected Markdown to contain entity count")
		}
		if !strings.Contains(output, "Edges: 45") {
			t.Error("Expected Markdown to contain edge count")
		}
	})

	t.Run("JSON includes graph stats", func(t *testing.T) {
		processor := NewJSONProcessor()
		output, err := processor.FormatGraph(index)
		if err != nil {
			t.Fatalf("Failed to format index as JSON: %v", err)
		}

		// Parse and verify
		var parsed types.GraphIndex
		if err := json.Unmarshal([]byte(output), &parsed); err != nil {
			t.Fatalf("Failed to parse JSON output: %v", err)
		}

		// Check stats
		if parsed.Stats.TotalTags != 15 {
			t.Errorf("Expected TotalTags 15, got %d", parsed.Stats.TotalTags)
		}
		if parsed.Stats.TotalTopics != 8 {
			t.Errorf("Expected TotalTopics 8, got %d", parsed.Stats.TotalTopics)
		}
		if parsed.Stats.TotalEntities != 12 {
			t.Errorf("Expected TotalEntities 12, got %d", parsed.Stats.TotalEntities)
		}
		if parsed.Stats.TotalEdges != 45 {
			t.Errorf("Expected TotalEdges 45, got %d", parsed.Stats.TotalEdges)
		}
		if parsed.Stats.FilesWithSummary != 3 {
			t.Errorf("Expected FilesWithSummary 3, got %d", parsed.Stats.FilesWithSummary)
		}
		if parsed.Stats.AvgTagsPerFile != 5.0 {
			t.Errorf("Expected AvgTagsPerFile 5.0, got %f", parsed.Stats.AvgTagsPerFile)
		}
	})
}

func TestGraphStatsOmittedWhenEmpty(t *testing.T) {
	// Test that graph stats are omitted when all zeros
	index := createTestGraphIndex() // Uses index without graph stats

	t.Run("XML omits empty graph stats", func(t *testing.T) {
		processor := NewXMLProcessor()
		output, err := processor.FormatGraph(index)
		if err != nil {
			t.Fatalf("Failed to format index as XML: %v", err)
		}

		if strings.Contains(output, "<graph_stats>") {
			t.Error("Expected XML to NOT contain <graph_stats> section when stats are zero")
		}
		if strings.Contains(output, "<coverage>") {
			t.Error("Expected XML to NOT contain <coverage> section when stats are zero")
		}
	})

	t.Run("Markdown omits empty graph stats", func(t *testing.T) {
		processor := NewMarkdownProcessor()
		output, err := processor.FormatGraph(index)
		if err != nil {
			t.Fatalf("Failed to format index as Markdown: %v", err)
		}

		if strings.Contains(output, "Tags: 0") {
			t.Error("Expected Markdown to NOT contain graph stats line when stats are zero")
		}
	})
}

func TestEntityRenderingEmptyEntities(t *testing.T) {
	// Test that empty entities slice doesn't create empty sections
	index := createTestGraphIndex() // Uses index without entities

	t.Run("XML omits empty entities section", func(t *testing.T) {
		processor := NewXMLProcessor()
		output, err := processor.FormatGraph(index)
		if err != nil {
			t.Fatalf("Failed to format index as XML: %v", err)
		}

		if strings.Contains(output, "<entities>") {
			t.Error("Expected XML to NOT contain <entities> section when no entities present")
		}
	})

	t.Run("Markdown omits empty entities section", func(t *testing.T) {
		processor := NewMarkdownProcessor()
		output, err := processor.FormatGraph(index)
		if err != nil {
			t.Fatalf("Failed to format index as Markdown: %v", err)
		}

		if strings.Contains(output, "**Entities**:") {
			t.Error("Expected Markdown to NOT contain Entities section when no entities present")
		}
	})
}

func TestEmptyIndex(t *testing.T) {
	emptyIndex := &types.GraphIndex{
		Generated:  time.Now(),
		MemoryRoot: "/empty",
		Stats:      types.IndexStats{},
		Files:      []types.FileEntry{},
	}

	// Test all processors with empty index
	xmlProc := NewXMLProcessor()
	xmlOut, err := xmlProc.FormatGraph(emptyIndex)
	if err != nil {
		t.Errorf("XML processor failed on empty index: %v", err)
	}
	if !strings.Contains(xmlOut, "<memory_index>") {
		t.Error("Expected XML output for empty index")
	}

	mdProc := NewMarkdownProcessor()
	mdOut, err := mdProc.FormatGraph(emptyIndex)
	if err != nil {
		t.Errorf("Markdown processor failed on empty index: %v", err)
	}
	if !strings.Contains(mdOut, "# Claude Code Agentic Memory Index") {
		t.Error("Expected Markdown output for empty index")
	}

	jsonProc := NewJSONProcessor()
	jsonOut, err := jsonProc.FormatGraph(emptyIndex)
	if err != nil {
		t.Errorf("JSON processor failed on empty index: %v", err)
	}
	var parsed types.GraphIndex
	if err := json.Unmarshal([]byte(jsonOut), &parsed); err != nil {
		t.Error("Expected valid JSON for empty index")
	}
}
