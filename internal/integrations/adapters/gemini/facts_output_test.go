package gemini

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	_ "github.com/leefowlercu/agentic-memorizer/internal/format/formatters" // Register formatters
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

func TestFormatBeforeAgentJSON(t *testing.T) {
	now := time.Now()
	facts := &types.FactsIndex{
		Generated: now,
		Facts: []types.Fact{
			{
				ID:        "abc12345-6789-0abc-def0-123456789abc",
				Content:   "The user prefers dark mode in all applications",
				CreatedAt: now.Add(-24 * time.Hour),
				UpdatedAt: now,
				Source:    "cli",
			},
			{
				ID:        "def12345-6789-0abc-def0-123456789def",
				Content:   "The user works on a MacBook Pro M2",
				CreatedAt: now.Add(-48 * time.Hour),
				Source:    "cli",
			},
		},
		Stats: types.FactStats{
			TotalFacts: 2,
			MaxFacts:   50,
		},
	}

	output, err := formatBeforeAgentJSON(facts, integrations.FormatXML)
	if err != nil {
		t.Fatalf("formatBeforeAgentJSON() error = %v", err)
	}

	// Verify JSON structure
	var result BeforeAgentOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}

	// Verify structure fields
	if result.HookSpecificOutput == nil {
		t.Fatal("HookSpecificOutput should not be nil")
	}
	if result.HookSpecificOutput.HookEventName != BeforeAgentEvent {
		t.Errorf("HookEventName = %q, want %q", result.HookSpecificOutput.HookEventName, BeforeAgentEvent)
	}

	// Verify content contains facts data
	if !strings.Contains(result.HookSpecificOutput.AdditionalContext, "dark mode") {
		t.Error("AdditionalContext should contain fact content")
	}
}

func TestFormatBeforeAgentJSON_EmptyFacts(t *testing.T) {
	facts := &types.FactsIndex{
		Generated: time.Now(),
		Facts:     []types.Fact{},
		Stats: types.FactStats{
			TotalFacts: 0,
			MaxFacts:   50,
		},
	}

	output, err := formatBeforeAgentJSON(facts, integrations.FormatXML)
	if err != nil {
		t.Fatalf("formatBeforeAgentJSON() error = %v", err)
	}

	var result BeforeAgentOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}

	// Verify structure exists
	if result.HookSpecificOutput == nil {
		t.Fatal("HookSpecificOutput should not be nil")
	}
	if result.HookSpecificOutput.HookEventName != BeforeAgentEvent {
		t.Errorf("HookEventName = %q, want %q", result.HookSpecificOutput.HookEventName, BeforeAgentEvent)
	}
}

func TestFormatBeforeAgentJSON_NoHTMLEscaping(t *testing.T) {
	facts := &types.FactsIndex{
		Generated: time.Now(),
		Facts: []types.Fact{
			{
				ID:        "abc12345-6789-0abc-def0-123456789abc",
				Content:   "User prefers <TypeScript> & <Go> languages",
				CreatedAt: time.Now(),
				Source:    "cli",
			},
		},
		Stats: types.FactStats{
			TotalFacts: 1,
			MaxFacts:   50,
		},
	}

	output, err := formatBeforeAgentJSON(facts, integrations.FormatXML)
	if err != nil {
		t.Fatalf("formatBeforeAgentJSON() error = %v", err)
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
	if !strings.Contains(output, "<facts_index>") {
		t.Error("Output should contain literal '<facts_index>' tag")
	}
}

func TestFormatBeforeAgentJSON_AllFormats(t *testing.T) {
	facts := &types.FactsIndex{
		Generated: time.Now(),
		Facts: []types.Fact{
			{
				ID:        "abc12345-6789-0abc-def0-123456789abc",
				Content:   "Test fact content",
				CreatedAt: time.Now(),
				Source:    "cli",
			},
		},
		Stats: types.FactStats{
			TotalFacts: 1,
			MaxFacts:   50,
		},
	}

	tests := []struct {
		name   string
		format integrations.OutputFormat
	}{
		{"XML", integrations.FormatXML},
		{"Markdown", integrations.FormatMarkdown},
		{"JSON", integrations.FormatJSON},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := formatBeforeAgentJSON(facts, tt.format)
			if err != nil {
				t.Fatalf("formatBeforeAgentJSON() error = %v", err)
			}

			var result BeforeAgentOutput
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Errorf("Output is not valid JSON for format %s: %v", tt.name, err)
			}

			if result.HookSpecificOutput.AdditionalContext == "" {
				t.Errorf("AdditionalContext should not be empty for format %s", tt.name)
			}
		})
	}
}
