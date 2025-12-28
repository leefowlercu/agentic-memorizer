package gemini

import (
	"encoding/json"
	"strings"
	"testing"

	_ "github.com/leefowlercu/agentic-memorizer/internal/format/formatters" // Register formatters
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

func TestFormatGeminiHookJSON_NoHTMLEscaping(t *testing.T) {
	// Create a simple graph index with XML-like content in fields
	index := &types.FileIndex{
		MemoryRoot: "/test/path",
		Stats: types.IndexStats{
			TotalFiles:    1,
			TotalSize:     1024,
			CachedFiles:   1,
			AnalyzedFiles: 0,
		},
		Files: []types.FileEntry{
			{
				Path:     "/path/to/test<file>.txt",
				Name:     "test<file>.txt",
				Category: "documents",
				Size:     1024,
				Type:     "txt",
				Summary:  "This is a test <summary> with angle brackets",
				Tags:     []string{"tag1", "tag<2>"},
			},
		},
	}

	// Format with XML output
	output, err := formatGeminiHookJSON(index, integrations.FormatXML)
	if err != nil {
		t.Fatalf("formatGeminiHookJSON() error = %v", err)
	}

	// Check that output doesn't contain escaped angle brackets
	if strings.Contains(output, `\u003c`) {
		t.Error("Output contains escaped '<' (\\u003c), expected literal '<'")
	}
	if strings.Contains(output, `\u003e`) {
		t.Error("Output contains escaped '>' (\\u003e), expected literal '>'")
	}
	if strings.Contains(output, `\u0026`) {
		t.Error("Output contains escaped '&' (\\u0026), expected literal '&'")
	}

	// Verify output contains literal angle brackets from XML tags
	if !strings.Contains(output, "<memory_index>") {
		t.Error("Output should contain literal '<memory_index>' tag")
	}
	if !strings.Contains(output, "</memory_index>") {
		t.Error("Output should contain literal '</memory_index>' tag")
	}

	// Verify JSON is still valid
	var result GeminiHookOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}

	// Verify structure
	if result.HookSpecificOutput == nil {
		t.Fatal("HookSpecificOutput should not be nil")
	}
	if result.HookSpecificOutput.HookEventName != SessionStartEvent {
		t.Errorf("HookEventName = %q, want %q", result.HookSpecificOutput.HookEventName, SessionStartEvent)
	}
	if result.HookSpecificOutput.AdditionalContext == "" {
		t.Error("AdditionalContext should not be empty")
	}
}

func TestFormatGeminiHookJSON_Structure(t *testing.T) {
	// Create test index
	index := &types.FileIndex{
		MemoryRoot: "/test/path",
		Stats: types.IndexStats{
			TotalFiles:    2,
			TotalSize:     2048,
			CachedFiles:   1,
			AnalyzedFiles: 1,
		},
		Files: []types.FileEntry{
			{
				Path:     "/test/file1.txt",
				Name:     "file1.txt",
				Category: "documents",
				Size:     1024,
				Type:     "txt",
				Summary:  "Test file 1",
				Tags:     []string{"test", "document"},
			},
			{
				Path:     "/test/file2.md",
				Name:     "file2.md",
				Category: "documents",
				Size:     1024,
				Type:     "md",
				Summary:  "Test file 2",
				Tags:     []string{"test", "markdown"},
			},
		},
	}

	tests := []struct {
		name   string
		format integrations.OutputFormat
	}{
		{"XML format", integrations.FormatXML},
		{"Markdown format", integrations.FormatMarkdown},
		{"JSON format", integrations.FormatJSON},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := formatGeminiHookJSON(index, tt.format)
			if err != nil {
				t.Fatalf("formatGeminiHookJSON() error = %v", err)
			}

			// Verify JSON structure
			var result GeminiHookOutput
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Fatalf("Output is not valid JSON: %v", err)
			}

			// Verify hookSpecificOutput exists
			if result.HookSpecificOutput == nil {
				t.Fatal("HookSpecificOutput should not be nil")
			}

			// Verify hookEventName is correct
			if result.HookSpecificOutput.HookEventName != SessionStartEvent {
				t.Errorf("HookEventName = %q, want %q", result.HookSpecificOutput.HookEventName, SessionStartEvent)
			}

			// Verify additionalContext is not empty
			if result.HookSpecificOutput.AdditionalContext == "" {
				t.Error("AdditionalContext should not be empty")
			}

			// Verify content contains file information based on format
			content := result.HookSpecificOutput.AdditionalContext
			switch tt.format {
			case integrations.FormatXML:
				if !strings.Contains(content, "<file") {
					t.Error("XML output should contain <file> tags")
				}
			case integrations.FormatMarkdown:
				if !strings.Contains(content, "file1.txt") {
					t.Error("Markdown output should contain file names")
				}
			case integrations.FormatJSON:
				if !strings.Contains(content, `"files"`) {
					t.Error("JSON output should contain files array")
				}
			}
		})
	}
}

func TestMarshalIndentNoEscape(t *testing.T) {
	tests := []struct {
		name           string
		input          any
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "angle brackets not escaped",
			input: map[string]string{
				"html": "<div>Hello</div>",
				"xml":  "<root><child>test</child></root>",
			},
			wantContains: []string{
				`"html": "<div>Hello</div>"`,
				`"xml": "<root><child>test</child></root>"`,
			},
			wantNotContain: []string{
				`\u003c`,
				`\u003e`,
			},
		},
		{
			name: "ampersand not escaped",
			input: map[string]string{
				"text": "Tom & Jerry",
			},
			wantContains: []string{
				`"text": "Tom & Jerry"`,
			},
			wantNotContain: []string{
				`\u0026`,
			},
		},
		{
			name: "nested structure",
			input: map[string]any{
				"outer": map[string]string{
					"inner": "<test>value</test>",
				},
			},
			wantContains: []string{
				`"inner": "<test>value</test>"`,
			},
			wantNotContain: []string{
				`\u003c`,
				`\u003e`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := marshalIndentNoEscape(tt.input, "", "  ")
			if err != nil {
				t.Fatalf("marshalIndentNoEscape() error = %v", err)
			}

			resultStr := string(result)

			for _, want := range tt.wantContains {
				if !strings.Contains(resultStr, want) {
					t.Errorf("Result should contain %q, got:\n%s", want, resultStr)
				}
			}

			for _, unwanted := range tt.wantNotContain {
				if strings.Contains(resultStr, unwanted) {
					t.Errorf("Result should not contain %q, got:\n%s", unwanted, resultStr)
				}
			}

			// Verify it's still valid JSON
			var decoded any
			if err := json.Unmarshal(result, &decoded); err != nil {
				t.Errorf("Result is not valid JSON: %v", err)
			}
		})
	}
}

func TestMarshalIndentNoEscape_Formatting(t *testing.T) {
	input := map[string]any{
		"level1": map[string]string{
			"level2": "value",
		},
	}

	result, err := marshalIndentNoEscape(input, "", "  ")
	if err != nil {
		t.Fatalf("marshalIndentNoEscape() error = %v", err)
	}

	resultStr := string(result)

	// Check indentation
	if !strings.Contains(resultStr, "  \"level1\":") {
		t.Error("Should have proper 2-space indentation")
	}

	// Should not have trailing newline (matching MarshalIndent behavior)
	if strings.HasSuffix(resultStr, "\n") {
		t.Error("Should not have trailing newline")
	}

	// Verify multiple lines exist (formatting)
	if !strings.Contains(resultStr, "\n") {
		t.Error("Should have newlines for formatting")
	}
}
