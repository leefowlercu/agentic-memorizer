package claude

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	_ "github.com/leefowlercu/agentic-memorizer/internal/format/formatters" // Register formatters
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

func TestFormatUserPromptSubmitJSON(t *testing.T) {
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

	output, err := formatUserPromptSubmitJSON(facts, integrations.FormatXML)
	if err != nil {
		t.Fatalf("formatUserPromptSubmitJSON() error = %v", err)
	}

	// Verify JSON structure
	var result UserPromptSubmitOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}

	// Verify structure fields
	if !result.Continue {
		t.Error("Continue should be true")
	}
	if !result.SuppressOutput {
		t.Error("SuppressOutput should be true")
	}
	if result.HookSpecific == nil {
		t.Fatal("HookSpecific should not be nil")
	}
	if result.HookSpecific.HookEventName != UserPromptSubmitEvent {
		t.Errorf("HookEventName = %q, want %q", result.HookSpecific.HookEventName, UserPromptSubmitEvent)
	}

	// Verify system message
	if !strings.Contains(result.SystemMessage, "User facts loaded: 2/50") {
		t.Errorf("SystemMessage should contain fact count, got: %s", result.SystemMessage)
	}

	// Verify content contains facts data
	if !strings.Contains(result.HookSpecific.AdditionalContext, "dark mode") {
		t.Error("AdditionalContext should contain fact content")
	}
}

func TestFormatUserPromptSubmitJSON_EmptyFacts(t *testing.T) {
	facts := &types.FactsIndex{
		Generated: time.Now(),
		Facts:     []types.Fact{},
		Stats: types.FactStats{
			TotalFacts: 0,
			MaxFacts:   50,
		},
	}

	output, err := formatUserPromptSubmitJSON(facts, integrations.FormatXML)
	if err != nil {
		t.Fatalf("formatUserPromptSubmitJSON() error = %v", err)
	}

	var result UserPromptSubmitOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}

	// Verify system message for empty facts
	if result.SystemMessage != "No user-defined facts stored" {
		t.Errorf("SystemMessage = %q, want 'No user-defined facts stored'", result.SystemMessage)
	}
}

func TestFormatUserPromptSubmitJSON_NoHTMLEscaping(t *testing.T) {
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

	output, err := formatUserPromptSubmitJSON(facts, integrations.FormatXML)
	if err != nil {
		t.Fatalf("formatUserPromptSubmitJSON() error = %v", err)
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

func TestFormatUserPromptSubmitJSON_AllFormats(t *testing.T) {
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
		{"JSON", integrations.FormatJSON},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := formatUserPromptSubmitJSON(facts, tt.format)
			if err != nil {
				t.Fatalf("formatUserPromptSubmitJSON() error = %v", err)
			}

			var result UserPromptSubmitOutput
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Errorf("Output is not valid JSON for format %s: %v", tt.name, err)
			}

			if result.HookSpecific.AdditionalContext == "" {
				t.Errorf("AdditionalContext should not be empty for format %s", tt.name)
			}
		})
	}
}

func TestGenerateFactsSystemMessage(t *testing.T) {
	tests := []struct {
		name       string
		facts      *types.FactsIndex
		wantPrefix string
	}{
		{
			name: "no facts",
			facts: &types.FactsIndex{
				Facts: []types.Fact{},
				Stats: types.FactStats{MaxFacts: 50},
			},
			wantPrefix: "No user-defined facts stored",
		},
		{
			name: "one fact",
			facts: &types.FactsIndex{
				Facts: []types.Fact{{ID: "test", Content: "test"}},
				Stats: types.FactStats{MaxFacts: 50},
			},
			wantPrefix: "User facts loaded: 1/50",
		},
		{
			name: "multiple facts",
			facts: &types.FactsIndex{
				Facts: []types.Fact{
					{ID: "1", Content: "fact 1"},
					{ID: "2", Content: "fact 2"},
					{ID: "3", Content: "fact 3"},
				},
				Stats: types.FactStats{MaxFacts: 50},
			},
			wantPrefix: "User facts loaded: 3/50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateFactsSystemMessage(tt.facts)
			if got != tt.wantPrefix {
				t.Errorf("generateFactsSystemMessage() = %q, want %q", got, tt.wantPrefix)
			}
		})
	}
}
