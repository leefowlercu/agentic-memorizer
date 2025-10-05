package output

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

func createTestIndex() *types.Index {
	modTime := time.Date(2025, 10, 5, 12, 0, 0, 0, time.UTC)
	wordCount := 100
	language := "Go"
	summary := "Test summary"
	docType := "Test"

	return &types.Index{
		Generated: time.Date(2025, 10, 5, 16, 19, 3, 0, time.UTC),
		Root:      "/test/root",
		Entries: []types.IndexEntry{
			{
				Metadata: types.FileMetadata{
					FileInfo: types.FileInfo{
						Path:       "/test/root/test.md",
						Size:       1024,
						Modified:   modTime,
						Type:       "markdown",
						Category:   "documents",
						IsReadable: true,
					},
					WordCount: &wordCount,
				},
				Semantic: &types.SemanticAnalysis{
					Summary:      summary,
					Tags:         []string{"test", "sample"},
					KeyTopics:    []string{"Testing", "Samples"},
					DocumentType: docType,
				},
			},
			{
				Metadata: types.FileMetadata{
					FileInfo: types.FileInfo{
						Path:       "/test/root/code.go",
						Size:       2048,
						Modified:   modTime,
						Type:       "code",
						Category:   "code",
						IsReadable: true,
					},
					Language:  &language,
					WordCount: &wordCount,
				},
			},
		},
		Stats: types.IndexStats{
			TotalFiles:    2,
			TotalSize:     3072,
			AnalyzedFiles: 1,
			CachedFiles:   1,
		},
	}
}

func TestFormatMarkdown(t *testing.T) {
	formatter := NewFormatter(false, 7)
	index := createTestIndex()

	output := formatter.FormatMarkdown(index)

	if output == "" {
		t.Fatal("FormatMarkdown returned empty string")
	}

	// Check for key components
	expectedStrings := []string{
		"# Claude Code Agentic Memory Index",
		"Generated:",
		"Files: 2",
		"Total Size:",
		"/test/root",
		"test.md",
		"code.go",
		"Test summary",
		"Usage Guide",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Output missing expected string: %q", expected)
		}
	}
}

func TestFormatXML(t *testing.T) {
	formatter := NewFormatter(false, 7)
	index := createTestIndex()

	output := formatter.FormatXML(index)

	if output == "" {
		t.Fatal("FormatXML returned empty string")
	}

	// Check for XML structure
	expectedStrings := []string{
		"<?xml version=\"1.0\" encoding=\"UTF-8\"?>",
		"<memory_index>",
		"<metadata>",
		"<generated>2025-10-05T16:19:03Z</generated>",
		"<file_count>2</file_count>",
		"<total_size_bytes>3072</total_size_bytes>",
		"<root_path>/test/root</root_path>",
		"<cached_files>1</cached_files>",
		"<analyzed_files>1</analyzed_files>",
		"</metadata>",
		"<categories>",
		"<category name=\"documents\"",
		"<category name=\"code\"",
		"<file>",
		"<name>test.md</name>",
		"<name>code.go</name>",
		"<semantic>",
		"<summary>Test summary</summary>",
		"<topics>",
		"<topic>Testing</topic>",
		"<tags>",
		"<tag>test</tag>",
		"</memory_index>",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Output missing expected string: %q", expected)
		}
	}
}

func TestFormatXMLWithSpecialChars(t *testing.T) {
	formatter := NewFormatter(false, 0)

	wordCount := 50
	summary := "Test with <special> & \"quoted\" 'chars'"
	docType := "Test & Demo"

	index := &types.Index{
		Generated: time.Date(2025, 10, 5, 16, 19, 3, 0, time.UTC),
		Root:      "/test/root",
		Entries: []types.IndexEntry{
			{
				Metadata: types.FileMetadata{
					FileInfo: types.FileInfo{
						Path:       "/test/root/special<file>.md",
						Size:       1024,
						Modified:   time.Date(2025, 10, 5, 12, 0, 0, 0, time.UTC),
						Type:       "markdown",
						Category:   "documents",
						IsReadable: true,
					},
					WordCount: &wordCount,
				},
				Semantic: &types.SemanticAnalysis{
					Summary:      summary,
					DocumentType: docType,
				},
			},
		},
		Stats: types.IndexStats{
			TotalFiles: 1,
			TotalSize:  1024,
		},
	}

	output := formatter.FormatXML(index)

	// Check for properly escaped special characters
	expectedEscapes := []string{
		"special&lt;file&gt;.md",
		"Test with &lt;special&gt; &amp; &quot;quoted&quot; &apos;chars&apos;",
		"Test &amp; Demo",
	}

	for _, expected := range expectedEscapes {
		if !strings.Contains(output, expected) {
			t.Errorf("Output missing expected escaped string: %q", expected)
		}
	}

	// Ensure unescaped versions are NOT present
	unescapedStrings := []string{
		"special<file>.md",
		"Test with <special>",
		"Test & Demo</document_type>",
	}

	for _, unescaped := range unescapedStrings {
		if strings.Contains(output, unescaped) {
			t.Errorf("Output contains unescaped string: %q", unescaped)
		}
	}
}

func TestWrapJSON(t *testing.T) {
	formatter := NewFormatter(false, 7)
	index := createTestIndex()

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "markdown content",
			content: "# Test Markdown\n\nSome content here.",
		},
		{
			name:    "xml content",
			content: "<?xml version=\"1.0\"?>\n<test>content</test>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := formatter.WrapJSON(tt.content, index)
			if err != nil {
				t.Fatalf("WrapJSON failed: %v", err)
			}

			if output == "" {
				t.Fatal("WrapJSON returned empty string")
			}

			// Parse as JSON to validate structure
			var hookOutput HookOutput
			if err := json.Unmarshal([]byte(output), &hookOutput); err != nil {
				t.Fatalf("Failed to parse JSON output: %v", err)
			}

			// Verify JSON structure
			if !hookOutput.Continue {
				t.Error("Continue should be true")
			}

			if !hookOutput.SuppressOutput {
				t.Error("SuppressOutput should be true")
			}

			if hookOutput.SystemMessage == "" {
				t.Error("SystemMessage should not be empty")
			}

			if !strings.Contains(hookOutput.SystemMessage, "Memory index updated") {
				t.Errorf("SystemMessage unexpected: %s", hookOutput.SystemMessage)
			}

			if hookOutput.HookSpecificOutput == nil {
				t.Fatal("HookSpecificOutput should not be nil")
			}

			if hookOutput.HookSpecificOutput.HookEventName != "SessionStart" {
				t.Errorf("HookEventName = %q, want %q", hookOutput.HookSpecificOutput.HookEventName, "SessionStart")
			}

			if hookOutput.HookSpecificOutput.AdditionalContext != tt.content {
				t.Errorf("AdditionalContext = %q, want %q", hookOutput.HookSpecificOutput.AdditionalContext, tt.content)
			}
		})
	}
}

func TestXMLEscape(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "simple text",
			expected: "simple text",
		},
		{
			input:    "text with <tags>",
			expected: "text with &lt;tags&gt;",
		},
		{
			input:    "ampersand & test",
			expected: "ampersand &amp; test",
		},
		{
			input:    "quotes \"double\" and 'single'",
			expected: "quotes &quot;double&quot; and &apos;single&apos;",
		},
		{
			input:    "<>&\"'",
			expected: "&lt;&gt;&amp;&quot;&apos;",
		},
		{
			input:    "mixed <test> & \"quotes\" 'here'",
			expected: "mixed &lt;test&gt; &amp; &quot;quotes&quot; &apos;here&apos;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := xmlEscape(tt.input)
			if result != tt.expected {
				t.Errorf("xmlEscape(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatXMLEmptyIndex(t *testing.T) {
	formatter := NewFormatter(false, 0)

	index := &types.Index{
		Generated: time.Date(2025, 10, 5, 16, 19, 3, 0, time.UTC),
		Root:      "/test/root",
		Entries:   []types.IndexEntry{},
		Stats: types.IndexStats{
			TotalFiles: 0,
			TotalSize:  0,
		},
	}

	output := formatter.FormatXML(index)

	// Should still have valid XML structure
	if !strings.Contains(output, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>") {
		t.Error("Missing XML declaration")
	}

	if !strings.Contains(output, "<memory_index>") {
		t.Error("Missing root element")
	}

	if !strings.Contains(output, "</memory_index>") {
		t.Error("Missing closing root element")
	}

	if !strings.Contains(output, "<file_count>0</file_count>") {
		t.Error("Missing file count")
	}
}

func TestGenerateSystemMessage(t *testing.T) {
	formatter := NewFormatter(false, 7)
	index := createTestIndex()

	msg := formatter.generateSystemMessage(index)

	if msg == "" {
		t.Fatal("generateSystemMessage returned empty string")
	}

	expectedParts := []string{
		"Memory index updated",
		"2 files",
		"3.0 KB total",
		"1 cached",
		"1 analyzed",
	}

	for _, part := range expectedParts {
		if !strings.Contains(msg, part) {
			t.Errorf("System message missing expected part: %q\nFull message: %s", part, msg)
		}
	}
}
