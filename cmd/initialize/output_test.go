package initialize

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{"non-empty key", "sk-test-12345", "********"},
		{"empty key", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskAPIKey(tt.key)
			if got != tt.want {
				t.Errorf("maskAPIKey(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestBuildInitializeResult(t *testing.T) {
	resolved := &UnattendedConfig{
		GraphHost:              "test-host",
		GraphPort:              6380,
		SemanticEnabled:        true,
		SemanticProvider:       "openai",
		SemanticModel:          "gpt-4o",
		SemanticAPIKey:         "secret-key",
		SemanticAPIKeySource:   "flag",
		EmbeddingsEnabled:      true,
		EmbeddingsProvider:     "voyage",
		EmbeddingsModel:        "voyage-3-large",
		EmbeddingsDimensions:   1024,
		EmbeddingsAPIKey:       "another-secret",
		EmbeddingsAPIKeySource: "VOYAGE_API_KEY",
		HTTPPort:               8080,
	}

	result := buildInitializeResult(resolved)

	// Check top-level fields
	if result.Status != "success" {
		t.Errorf("Status = %q, want %q", result.Status, "success")
	}

	// Check graph config
	if result.Config.Graph.Host != "test-host" {
		t.Errorf("Graph.Host = %q, want %q", result.Config.Graph.Host, "test-host")
	}
	if result.Config.Graph.Port != 6380 {
		t.Errorf("Graph.Port = %d, want %d", result.Config.Graph.Port, 6380)
	}

	// Check semantic config
	if !result.Config.Semantic.Enabled {
		t.Error("Semantic.Enabled = false, want true")
	}
	if result.Config.Semantic.Provider != "openai" {
		t.Errorf("Semantic.Provider = %q, want %q", result.Config.Semantic.Provider, "openai")
	}
	if result.Config.Semantic.Model != "gpt-4o" {
		t.Errorf("Semantic.Model = %q, want %q", result.Config.Semantic.Model, "gpt-4o")
	}
	if result.Config.Semantic.APIKey != "********" {
		t.Errorf("Semantic.APIKey = %q, want %q (should be masked)", result.Config.Semantic.APIKey, "********")
	}

	// Check embeddings config
	if !result.Config.Embeddings.Enabled {
		t.Error("Embeddings.Enabled = false, want true")
	}
	if result.Config.Embeddings.Provider != "voyage" {
		t.Errorf("Embeddings.Provider = %q, want %q", result.Config.Embeddings.Provider, "voyage")
	}
	if result.Config.Embeddings.APIKey != "********" {
		t.Errorf("Embeddings.APIKey = %q, want %q (should be masked)", result.Config.Embeddings.APIKey, "********")
	}

	// Check daemon config
	if result.Config.Daemon.HTTPPort != 8080 {
		t.Errorf("Daemon.HTTPPort = %d, want %d", result.Config.Daemon.HTTPPort, 8080)
	}

	// Check validation
	if len(result.Validation.Warnings) != 0 {
		t.Errorf("Validation.Warnings = %v, want empty", result.Validation.Warnings)
	}
	if len(result.Validation.Errors) != 0 {
		t.Errorf("Validation.Errors = %v, want empty", result.Validation.Errors)
	}
}

func TestBuildInitializeResult_EmbeddingsDisabled(t *testing.T) {
	resolved := &UnattendedConfig{
		GraphHost:         "localhost",
		GraphPort:         6379,
		SemanticEnabled:   true,
		SemanticProvider:  "anthropic",
		SemanticModel:     "claude-sonnet-4-5-20250929",
		SemanticAPIKey:    "key",
		EmbeddingsEnabled: false,
		HTTPPort:          7600,
	}

	result := buildInitializeResult(resolved)

	if result.Config.Embeddings.Enabled {
		t.Error("Embeddings.Enabled = true, want false")
	}
	if result.Config.Embeddings.Provider != "" {
		t.Errorf("Embeddings.Provider = %q, want empty when disabled", result.Config.Embeddings.Provider)
	}
}

func TestFormatTextOutput(t *testing.T) {
	resolved := &UnattendedConfig{
		GraphHost:              "test-host",
		GraphPort:              6380,
		SemanticEnabled:        true,
		SemanticProvider:       "anthropic",
		SemanticModel:          "claude-sonnet-4-5-20250929",
		SemanticAPIKey:         "secret",
		SemanticAPIKeySource:   "ANTHROPIC_API_KEY",
		EmbeddingsEnabled:      true,
		EmbeddingsProvider:     "openai",
		EmbeddingsModel:        "text-embedding-3-large",
		EmbeddingsDimensions:   3072,
		EmbeddingsAPIKey:       "another-secret",
		EmbeddingsAPIKeySource: "flag",
		HTTPPort:               8080,
	}

	output := formatTextOutput(resolved)

	// Check required content
	checks := []string{
		"Configuration saved successfully",
		"FalkorDB:",
		"Host: test-host",
		"Port: 6380",
		"Semantic Analysis:",
		"Enabled: true",
		"Provider: anthropic",
		"Model: claude-sonnet-4-5-20250929",
		"API Key: ********",
		"ANTHROPIC_API_KEY",
		"Embeddings:",
		"Enabled: true",
		"Provider: openai",
		"Dimensions: 3072",
		"Daemon:",
		"HTTP Port: 8080",
		"memorizer daemon start",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("formatTextOutput() missing %q", check)
		}
	}

	// Ensure API keys are not exposed
	if strings.Contains(output, "secret") {
		t.Error("formatTextOutput() contains unmasked API key")
	}
}

func TestFormatTextOutput_EmbeddingsDisabled(t *testing.T) {
	resolved := &UnattendedConfig{
		GraphHost:            "localhost",
		GraphPort:            6379,
		SemanticEnabled:      true,
		SemanticProvider:     "anthropic",
		SemanticModel:        "claude-sonnet-4-5-20250929",
		SemanticAPIKey:       "key",
		SemanticAPIKeySource: "flag",
		EmbeddingsEnabled:    false,
		HTTPPort:             7600,
	}

	output := formatTextOutput(resolved)

	if !strings.Contains(output, "Enabled: false") {
		t.Error("formatTextOutput() should show 'Enabled: false' for disabled embeddings")
	}

	// Should not contain embeddings provider details when disabled
	if strings.Contains(output, "Provider: openai") {
		t.Error("formatTextOutput() should not show embeddings provider when disabled")
	}
}

func TestFormatJSONOutput(t *testing.T) {
	resolved := &UnattendedConfig{
		GraphHost:            "localhost",
		GraphPort:            6379,
		SemanticEnabled:      true,
		SemanticProvider:     "anthropic",
		SemanticModel:        "claude-sonnet-4-5-20250929",
		SemanticAPIKey:       "secret-key",
		EmbeddingsEnabled:    true,
		EmbeddingsProvider:   "openai",
		EmbeddingsModel:      "text-embedding-3-large",
		EmbeddingsDimensions: 3072,
		EmbeddingsAPIKey:     "another-secret",
		HTTPPort:             7600,
	}

	jsonOutput, err := formatJSONOutput(resolved)
	if err != nil {
		t.Fatalf("formatJSONOutput() error: %v", err)
	}

	// Verify it's valid JSON
	var result InitializeResult
	if err := json.Unmarshal([]byte(jsonOutput), &result); err != nil {
		t.Fatalf("formatJSONOutput() produced invalid JSON: %v", err)
	}

	// Check structure matches SPEC FR-008
	if result.Status != "success" {
		t.Errorf("JSON status = %q, want %q", result.Status, "success")
	}

	if result.Config == nil {
		t.Fatal("JSON config is nil")
	}

	// Verify API keys are masked
	if result.Config.Semantic.APIKey != "********" {
		t.Errorf("Semantic API key not masked: %q", result.Config.Semantic.APIKey)
	}
	if result.Config.Embeddings.APIKey != "********" {
		t.Errorf("Embeddings API key not masked: %q", result.Config.Embeddings.APIKey)
	}

	// Ensure raw secrets are not in output
	if strings.Contains(jsonOutput, "secret-key") || strings.Contains(jsonOutput, "another-secret") {
		t.Error("formatJSONOutput() contains unmasked API keys")
	}
}

func TestFormatJSONOutput_ValidStructure(t *testing.T) {
	resolved := &UnattendedConfig{
		GraphHost:         "localhost",
		GraphPort:         6379,
		SemanticEnabled:   true,
		SemanticProvider:  "anthropic",
		SemanticModel:     "claude-sonnet-4-5-20250929",
		SemanticAPIKey:    "key",
		EmbeddingsEnabled: false,
		HTTPPort:          7600,
	}

	jsonOutput, err := formatJSONOutput(resolved)
	if err != nil {
		t.Fatalf("formatJSONOutput() error: %v", err)
	}

	// Parse as generic map to check structure
	var result map[string]any
	if err := json.Unmarshal([]byte(jsonOutput), &result); err != nil {
		t.Fatalf("formatJSONOutput() produced invalid JSON: %v", err)
	}

	// Check required top-level fields per FR-008
	requiredFields := []string{"status", "config_path", "config", "validation"}
	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("JSON missing required field %q", field)
		}
	}

	// Check validation structure
	validation, ok := result["validation"].(map[string]any)
	if !ok {
		t.Fatal("validation is not an object")
	}
	if _, ok := validation["warnings"]; !ok {
		t.Error("validation missing 'warnings' field")
	}
	if _, ok := validation["errors"]; !ok {
		t.Error("validation missing 'errors' field")
	}
}
