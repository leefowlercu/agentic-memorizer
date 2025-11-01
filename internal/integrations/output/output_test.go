package output

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// Helper function to create a simple test index
func createTestIndex() *types.Index {
	now := time.Now()
	return &types.Index{
		Generated: now,
		Root:      "/test/root",
		Stats: types.IndexStats{
			TotalFiles:    2,
			TotalSize:     2048,
			AnalyzedFiles: 1,
			CachedFiles:   1,
		},
		Entries: []types.IndexEntry{
			{
				Metadata: types.FileMetadata{
					FileInfo: types.FileInfo{
						Path:       "/test/root/document.md",
						RelPath:    "document.md",
						Hash:       "abc123",
						Size:       1024,
						Modified:   now,
						Type:       "markdown",
						Category:   "documents",
						IsReadable: true,
					},
				},
				Semantic: &types.SemanticAnalysis{
					Summary:      "Test document summary",
					Tags:         []string{"test", "example"},
					KeyTopics:    []string{"testing", "documentation"},
					DocumentType: "guide",
					Confidence:   0.95,
				},
			},
			{
				Metadata: types.FileMetadata{
					FileInfo: types.FileInfo{
						Path:       "/test/root/code.go",
						RelPath:    "code.go",
						Hash:       "def456",
						Size:       1024,
						Modified:   now,
						Type:       "go",
						Category:   "code",
						IsReadable: true,
					},
				},
			},
		},
	}
}

func TestXMLProcessor(t *testing.T) {
	processor := NewXMLProcessor()

	if processor.GetFormat() != "xml" {
		t.Errorf("Expected format 'xml', got '%s'", processor.GetFormat())
	}

	index := createTestIndex()
	output, err := processor.Format(index)
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
	testIndex := createTestIndex()
	testIndex.Entries[0].Metadata.Path = "/test/root/<test>.md"
	escapedOutput, _ := processor.Format(testIndex)
	if !strings.Contains(escapedOutput, "&lt;test&gt;") {
		t.Error("Expected XML escaping for special characters")
	}
}

func TestMarkdownProcessor(t *testing.T) {
	processor := NewMarkdownProcessor()

	if processor.GetFormat() != "markdown" {
		t.Errorf("Expected format 'markdown', got '%s'", processor.GetFormat())
	}

	index := createTestIndex()
	output, err := processor.Format(index)
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

	index := createTestIndex()
	output, err := processor.Format(index)
	if err != nil {
		t.Fatalf("Failed to format index as JSON: %v", err)
	}

	// Verify it's valid JSON
	var parsed types.Index
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Verify structure
	if parsed.Stats.TotalFiles != 2 {
		t.Errorf("Expected 2 files, got %d", parsed.Stats.TotalFiles)
	}

	if len(parsed.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(parsed.Entries))
	}

	// Verify content
	if parsed.Root != "/test/root" {
		t.Errorf("Expected root '/test/root', got '%s'", parsed.Root)
	}
}

func TestOutputOptions(t *testing.T) {
	// Test with ShowRecentDays option
	opts := Options{
		Verbose:        true,
		ShowRecentDays: 7,
	}

	xmlProcessor := NewXMLProcessor(opts)
	mdProcessor := NewMarkdownProcessor(opts)
	jsonProcessor := NewJSONProcessor(opts)

	index := createTestIndex()

	// XML with recent days
	xmlOutput, _ := xmlProcessor.Format(index)
	if !strings.Contains(xmlOutput, "<recent_activity") {
		t.Error("Expected XML to contain recent_activity section when ShowRecentDays is set")
	}

	// Markdown with recent days
	mdOutput, _ := mdProcessor.Format(index)
	if !strings.Contains(mdOutput, "## 🕐 Recent Activity") {
		t.Error("Expected Markdown to contain recent activity section when ShowRecentDays is set")
	}

	// JSON with recent days
	jsonOutput, _ := jsonProcessor.Format(index)
	var parsed map[string]any
	json.Unmarshal([]byte(jsonOutput), &parsed)
	if _, ok := parsed["recent_entries"]; !ok {
		t.Error("Expected JSON to contain recent_entries when ShowRecentDays is set")
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

func TestGroupByCategory(t *testing.T) {
	entries := []types.IndexEntry{
		{Metadata: types.FileMetadata{FileInfo: types.FileInfo{Category: "documents"}}},
		{Metadata: types.FileMetadata{FileInfo: types.FileInfo{Category: "code"}}},
		{Metadata: types.FileMetadata{FileInfo: types.FileInfo{Category: "documents"}}},
	}

	grouped := groupByCategory(entries)

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

func TestGetRecentEntries(t *testing.T) {
	now := time.Now()
	entries := []types.IndexEntry{
		{Metadata: types.FileMetadata{FileInfo: types.FileInfo{Modified: now.AddDate(0, 0, -1)}}},  // 1 day ago
		{Metadata: types.FileMetadata{FileInfo: types.FileInfo{Modified: now.AddDate(0, 0, -5)}}},  // 5 days ago
		{Metadata: types.FileMetadata{FileInfo: types.FileInfo{Modified: now.AddDate(0, 0, -10)}}}, // 10 days ago
	}

	// Get entries from last 7 days
	recent := getRecentEntries(entries, 7)

	if len(recent) != 2 {
		t.Errorf("Expected 2 recent entries (within 7 days), got %d", len(recent))
	}

	// Verify they're sorted by most recent first
	if !recent[0].Metadata.Modified.After(recent[1].Metadata.Modified) {
		t.Error("Expected recent entries to be sorted by most recent first")
	}
}

func TestEmptyIndex(t *testing.T) {
	emptyIndex := &types.Index{
		Generated: time.Now(),
		Root:      "/empty",
		Stats:     types.IndexStats{},
		Entries:   []types.IndexEntry{},
	}

	// Test all processors with empty index
	xmlProc := NewXMLProcessor()
	xmlOut, err := xmlProc.Format(emptyIndex)
	if err != nil {
		t.Errorf("XML processor failed on empty index: %v", err)
	}
	if !strings.Contains(xmlOut, "<memory_index>") {
		t.Error("Expected XML output for empty index")
	}

	mdProc := NewMarkdownProcessor()
	mdOut, err := mdProc.Format(emptyIndex)
	if err != nil {
		t.Errorf("Markdown processor failed on empty index: %v", err)
	}
	if !strings.Contains(mdOut, "# Claude Code Agentic Memory Index") {
		t.Error("Expected Markdown output for empty index")
	}

	jsonProc := NewJSONProcessor()
	jsonOut, err := jsonProc.Format(emptyIndex)
	if err != nil {
		t.Errorf("JSON processor failed on empty index: %v", err)
	}
	var parsed types.Index
	if err := json.Unmarshal([]byte(jsonOut), &parsed); err != nil {
		t.Error("Expected valid JSON for empty index")
	}
}
