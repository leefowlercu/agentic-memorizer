package initialize

import (
	"encoding/json"
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
)

// InitializeResult represents the output of the initialize command (FR-008).
type InitializeResult struct {
	Status     string           `json:"status"`
	ConfigPath string           `json:"config_path"`
	Config     *MaskedConfig    `json:"config"`
	Validation ValidationResult `json:"validation"`
}

// MaskedConfig contains configuration with API keys masked for safe display.
type MaskedConfig struct {
	Graph      MaskedGraphConfig      `json:"graph"`
	Semantic   MaskedSemanticConfig   `json:"semantic"`
	Embeddings MaskedEmbeddingsConfig `json:"embeddings"`
	Daemon     MaskedDaemonConfig     `json:"daemon"`
}

// MaskedGraphConfig contains graph configuration for output.
type MaskedGraphConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// MaskedSemanticConfig contains semantic configuration with masked API key.
type MaskedSemanticConfig struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key"`
}

// MaskedEmbeddingsConfig contains embeddings configuration with masked API key.
type MaskedEmbeddingsConfig struct {
	Enabled    bool   `json:"enabled"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
	Dimensions int    `json:"dimensions,omitempty"`
	APIKey     string `json:"api_key,omitempty"`
}

// MaskedDaemonConfig contains daemon configuration for output.
type MaskedDaemonConfig struct {
	HTTPPort int `json:"http_port"`
}

// ValidationResult contains validation warnings and errors.
type ValidationResult struct {
	Warnings []string `json:"warnings"`
	Errors   []string `json:"errors"`
}

// buildInitializeResult creates an InitializeResult from resolved config.
func buildInitializeResult(resolved *UnattendedConfig) *InitializeResult {
	maskedConfig := &MaskedConfig{
		Graph: MaskedGraphConfig{
			Host: resolved.GraphHost,
			Port: resolved.GraphPort,
		},
		Semantic: MaskedSemanticConfig{
			Provider: resolved.SemanticProvider,
			Model:    resolved.SemanticModel,
			APIKey:   maskAPIKey(resolved.SemanticAPIKey),
		},
		Daemon: MaskedDaemonConfig{
			HTTPPort: resolved.HTTPPort,
		},
	}

	if resolved.EmbeddingsEnabled {
		maskedConfig.Embeddings = MaskedEmbeddingsConfig{
			Enabled:    true,
			Provider:   resolved.EmbeddingsProvider,
			Model:      resolved.EmbeddingsModel,
			Dimensions: resolved.EmbeddingsDimensions,
			APIKey:     maskAPIKey(resolved.EmbeddingsAPIKey),
		}
	} else {
		maskedConfig.Embeddings = MaskedEmbeddingsConfig{
			Enabled: false,
		}
	}

	return &InitializeResult{
		Status:     "success",
		ConfigPath: config.GetConfigPath(),
		Config:     maskedConfig,
		Validation: ValidationResult{
			Warnings: []string{},
			Errors:   []string{},
		},
	}
}

// formatTextOutput formats the initialization result as human-readable text.
func formatTextOutput(resolved *UnattendedConfig) string {
	var output string

	output += "Configuration saved successfully (unattended mode).\n"
	output += fmt.Sprintf("Config file: %s\n", config.GetConfigPath())
	output += "\n"

	output += "FalkorDB:\n"
	output += fmt.Sprintf("  Host: %s\n", resolved.GraphHost)
	output += fmt.Sprintf("  Port: %d\n", resolved.GraphPort)
	output += "\n"

	output += "Semantic Provider:\n"
	output += fmt.Sprintf("  Provider: %s\n", resolved.SemanticProvider)
	output += fmt.Sprintf("  Model: %s\n", resolved.SemanticModel)
	if resolved.SemanticAPIKey != "" {
		output += fmt.Sprintf("  API Key: ******** (from %s)\n", resolved.SemanticAPIKeySource)
	} else {
		output += "  API Key: (not set)\n"
	}
	output += "\n"

	output += "Embeddings:\n"
	if resolved.EmbeddingsEnabled {
		output += "  Enabled: true\n"
		output += fmt.Sprintf("  Provider: %s\n", resolved.EmbeddingsProvider)
		output += fmt.Sprintf("  Model: %s\n", resolved.EmbeddingsModel)
		output += fmt.Sprintf("  Dimensions: %d\n", resolved.EmbeddingsDimensions)
		if resolved.EmbeddingsAPIKey != "" {
			output += fmt.Sprintf("  API Key: ******** (from %s)\n", resolved.EmbeddingsAPIKeySource)
		} else {
			output += "  API Key: (not set)\n"
		}
	} else {
		output += "  Enabled: false\n"
	}
	output += "\n"

	output += "Daemon:\n"
	output += fmt.Sprintf("  HTTP Port: %d\n", resolved.HTTPPort)
	output += "\n"

	output += "To start the daemon, run:\n"
	output += "  memorizer daemon start\n"

	return output
}

// formatJSONOutput formats the initialization result as JSON.
func formatJSONOutput(resolved *UnattendedConfig) (string, error) {
	result := buildInitializeResult(resolved)

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON output; %w", err)
	}

	return string(jsonBytes), nil
}

// maskAPIKey masks an API key for safe display.
// Returns "********" if key is non-empty, empty string otherwise.
func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	return "********"
}
