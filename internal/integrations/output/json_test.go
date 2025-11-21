package output

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

func TestJSONProcessor_NoHTMLEscaping(t *testing.T) {
	// Create index with HTML/XML-like content
	modTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	index := &types.Index{
		Generated: modTime,
		Root:      "/test/path",
		Stats: types.IndexStats{
			TotalFiles:    2,
			TotalSize:     2048,
			CachedFiles:   2,
			AnalyzedFiles: 0,
		},
		Entries: []types.IndexEntry{
			{
				Metadata: types.FileMetadata{
					FileInfo: types.FileInfo{
						Path:     "/path/to/test<file>.xml",
						RelPath:  "test<file>.xml",
						Category: "documents",
						Size:     1024,
						Modified: modTime,
						Type:     "xml",
					},
				},
				Semantic: &types.SemanticAnalysis{
					Summary:      "This is a test <summary> with angle brackets & ampersands",
					Tags:         []string{"tag1", "tag<2>", "a&b"},
					KeyTopics:    []string{"topic<1>", "topic & notes"},
					DocumentType: "test<type>",
				},
			},
			{
				Metadata: types.FileMetadata{
					FileInfo: types.FileInfo{
						Path:     "/path/to/file2.html",
						RelPath:  "file2.html",
						Category: "documents",
						Size:     1024,
						Modified: modTime,
						Type:     "html",
					},
				},
				Semantic: &types.SemanticAnalysis{
					Summary:      "<html><body>Test</body></html>",
					Tags:         []string{"html", "web"},
					DocumentType: "webpage",
				},
			},
		},
	}

	processor := NewJSONProcessor()
	output, err := processor.Format(index)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	// Check that output doesn't contain escaped angle brackets or ampersands
	if strings.Contains(output, `\u003c`) {
		t.Error("Output contains escaped '<' (\\u003c), expected literal '<'")
	}
	if strings.Contains(output, `\u003e`) {
		t.Error("Output contains escaped '>' (\\u003e), expected literal '>'")
	}
	if strings.Contains(output, `\u0026`) {
		t.Error("Output contains escaped '&' (\\u0026), expected literal '&'")
	}

	// Verify output contains literal special characters
	if !strings.Contains(output, "test<file>.xml") {
		t.Error("Output should contain literal filename with angle brackets")
	}
	if !strings.Contains(output, "<summary>") {
		t.Error("Output should contain literal '<summary>' in text")
	}
	if !strings.Contains(output, "a&b") {
		t.Error("Output should contain literal '&' in tag")
	}
	if !strings.Contains(output, "<html><body>Test</body></html>") {
		t.Error("Output should contain literal HTML in summary")
	}
	if !strings.Contains(output, "topic<1>") {
		t.Error("Output should contain literal angle brackets in topics")
	}

	// Verify JSON is still valid
	var decoded types.Index
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}

	// Verify decoded data matches original
	if decoded.Stats.TotalFiles != index.Stats.TotalFiles {
		t.Errorf("TotalFiles = %d, want %d", decoded.Stats.TotalFiles, index.Stats.TotalFiles)
	}
	if len(decoded.Entries) != len(index.Entries) {
		t.Errorf("Entries count = %d, want %d", len(decoded.Entries), len(index.Entries))
	}
}

func TestMarshalIndentNoEscape_Basic(t *testing.T) {
	input := map[string]string{
		"xml":  "<root><child>value</child></root>",
		"html": "<div>Hello & Goodbye</div>",
		"text": "a < b > c & d",
	}

	result, err := marshalIndentNoEscape(input, "", "  ")
	if err != nil {
		t.Fatalf("marshalIndentNoEscape() error = %v", err)
	}

	resultStr := string(result)

	// Should contain literal characters
	wantLiterals := []string{
		`"xml": "<root><child>value</child></root>"`,
		`"html": "<div>Hello & Goodbye</div>"`,
		`"text": "a < b > c & d"`,
	}

	for _, want := range wantLiterals {
		if !strings.Contains(resultStr, want) {
			t.Errorf("Result should contain %q", want)
		}
	}

	// Should NOT contain escape sequences
	unwanted := []string{`\u003c`, `\u003e`, `\u0026`}
	for _, uw := range unwanted {
		if strings.Contains(resultStr, uw) {
			t.Errorf("Result should not contain %q", uw)
		}
	}

	// Verify valid JSON
	var decoded map[string]string
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Errorf("Result is not valid JSON: %v", err)
	}

	// Verify values decode correctly
	if decoded["xml"] != input["xml"] {
		t.Errorf("Decoded xml = %q, want %q", decoded["xml"], input["xml"])
	}
}

func TestMarshalIndentNoEscape_Indentation(t *testing.T) {
	input := map[string]any{
		"level1": map[string]any{
			"level2": map[string]string{
				"level3": "value<test>",
			},
		},
	}

	result, err := marshalIndentNoEscape(input, "", "  ")
	if err != nil {
		t.Fatalf("marshalIndentNoEscape() error = %v", err)
	}

	resultStr := string(result)

	// Check proper indentation exists
	if !strings.Contains(resultStr, "  \"level1\":") {
		t.Error("Should have 2-space indentation for level1")
	}
	if !strings.Contains(resultStr, "    \"level2\":") {
		t.Error("Should have 4-space indentation for level2")
	}
	if !strings.Contains(resultStr, "      \"level3\":") {
		t.Error("Should have 6-space indentation for level3")
	}

	// Check no trailing newline (matching json.MarshalIndent)
	if strings.HasSuffix(resultStr, "\n") {
		t.Error("Should not have trailing newline")
	}

	// Check that angle brackets aren't escaped even in nested values
	if strings.Contains(resultStr, `\u003c`) || strings.Contains(resultStr, `\u003e`) {
		t.Error("Nested values should not have escaped angle brackets")
	}
	if !strings.Contains(resultStr, "value<test>") {
		t.Error("Should contain literal angle brackets in nested value")
	}
}
