package common

import (
	"testing"
)

func TestParseAnalysisResponse(t *testing.T) {
	tests := []struct {
		name        string
		response    string
		wantSummary string
		wantErr     bool
	}{
		{
			name:        "valid raw json",
			response:    `{"summary": "Test summary", "tags": ["tag1"], "key_topics": ["topic1"], "document_type": "test"}`,
			wantSummary: "Test summary",
			wantErr:     false,
		},
		{
			name:        "json in code block",
			response:    "```json\n{\"summary\": \"Code block summary\", \"tags\": [], \"key_topics\": [], \"document_type\": \"test\"}\n```",
			wantSummary: "Code block summary",
			wantErr:     false,
		},
		{
			name:        "json in plain code block",
			response:    "```\n{\"summary\": \"Plain block\", \"tags\": [], \"key_topics\": [], \"document_type\": \"test\"}\n```",
			wantSummary: "Plain block",
			wantErr:     false,
		},
		{
			name:        "invalid json",
			response:    "not valid json at all",
			wantSummary: "",
			wantErr:     true,
		},
		{
			name:        "empty response",
			response:    "",
			wantSummary: "",
			wantErr:     true,
		},
		{
			name:        "json with extra fields",
			response:    `{"summary": "Has extras", "tags": [], "key_topics": [], "document_type": "test", "extra_field": "ignored"}`,
			wantSummary: "Has extras",
			wantErr:     false,
		},
		{
			name:        "json with entities and references",
			response:    `{"summary": "Full analysis", "tags": ["a"], "key_topics": ["b"], "document_type": "test", "entities": [{"name": "Go", "type": "technology"}], "references": [{"topic": "testing", "type": "related-to", "confidence": 0.8}]}`,
			wantSummary: "Full analysis",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseAnalysisResponse(tt.response)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAnalysisResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result.Summary != tt.wantSummary {
				t.Errorf("ParseAnalysisResponse() summary = %q, want %q", result.Summary, tt.wantSummary)
			}
		})
	}
}
