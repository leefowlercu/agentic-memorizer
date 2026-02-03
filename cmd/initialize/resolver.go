package initialize

import (
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

// UnattendedConfig holds the resolved configuration values for unattended mode.
type UnattendedConfig struct {
	// Graph configuration
	GraphHost string
	GraphPort int

	// Semantic provider configuration
	SemanticEnabled      bool
	SemanticProvider     string
	SemanticModel        string
	SemanticAPIKey       string
	SemanticAPIKeySource string // "flag", "MEMORIZER_SEMANTIC_API_KEY", or provider env var name

	// Embeddings configuration
	EmbeddingsEnabled      bool
	EmbeddingsProvider     string
	EmbeddingsModel        string
	EmbeddingsDimensions   int
	EmbeddingsAPIKey       string
	EmbeddingsAPIKeySource string // "flag", "MEMORIZER_EMBEDDINGS_API_KEY", or provider env var name

	// Daemon configuration
	HTTPPort int
}

// Default semantic models by provider (from SPEC.md FR-004).
var defaultSemanticModels = map[string]string{
	"anthropic": "claude-sonnet-4-5-20250929",
	"openai":    "gpt-4o",
	"google":    "gemini-2.0-flash",
}

// Default embeddings models by provider (from SPEC.md FR-004).
var defaultEmbeddingsModels = map[string]string{
	"openai": "text-embedding-3-large",
	"voyage": "voyage-3-large",
	"google": "text-embedding-004",
}

// Embeddings dimensions by model (from SPEC.md FR-005).
var embeddingsDimensions = map[string]int{
	"text-embedding-3-large": 3072,
	"text-embedding-3-small": 1536,
	"text-embedding-004":     768,
	"voyage-3-large":         1024,
	"voyage-3":               1024,
	"voyage-code-3":          1024,
}

// Provider environment variable names for API keys.
var semanticProviderEnvVars = map[string]string{
	"anthropic": "ANTHROPIC_API_KEY",
	"openai":    "OPENAI_API_KEY",
	"google":    "GOOGLE_API_KEY",
}

var embeddingsProviderEnvVars = map[string]string{
	"openai": "OPENAI_API_KEY",
	"voyage": "VOYAGE_API_KEY",
	"google": "GOOGLE_API_KEY",
}

// resolveUnattendedConfig resolves all configuration values for unattended mode.
// Values are resolved in priority order: flags > MEMORIZER_* env > provider env > defaults.
func resolveUnattendedConfig(cmd *cobra.Command) *UnattendedConfig {
	cfg := &UnattendedConfig{}

	// Resolve graph configuration
	cfg.GraphHost = resolveGraphHost(cmd)
	cfg.GraphPort = resolveGraphPort(cmd)

	// Resolve semantic configuration
	cfg.SemanticEnabled = resolveSemanticEnabled(cmd)
	if cfg.SemanticEnabled {
		cfg.SemanticProvider = resolveSemanticProvider(cmd)
		cfg.SemanticModel = resolveSemanticModel(cmd, cfg.SemanticProvider)
		cfg.SemanticAPIKey, cfg.SemanticAPIKeySource = resolveSemanticAPIKey(cmd, cfg.SemanticProvider)
	}

	// Resolve embeddings configuration
	cfg.EmbeddingsEnabled = resolveEmbeddingsEnabled(cmd)
	if cfg.EmbeddingsEnabled {
		cfg.EmbeddingsProvider = resolveEmbeddingsProvider(cmd)
		cfg.EmbeddingsModel = resolveEmbeddingsModel(cmd, cfg.EmbeddingsProvider)
		cfg.EmbeddingsDimensions = getEmbeddingsDimensions(cfg.EmbeddingsModel)
		cfg.EmbeddingsAPIKey, cfg.EmbeddingsAPIKeySource = resolveEmbeddingsAPIKey(cmd, cfg.EmbeddingsProvider)
	}

	// Resolve daemon configuration
	cfg.HTTPPort = resolveHTTPPort(cmd)

	return cfg
}

// resolveGraphHost resolves the FalkorDB host value.
func resolveGraphHost(cmd *cobra.Command) string {
	// Priority 1: CLI flag
	if cmd.Flags().Changed("graph-host") {
		return initializeGraphHost
	}

	// Priority 2: MEMORIZER_GRAPH_HOST env var
	if v := os.Getenv("MEMORIZER_GRAPH_HOST"); v != "" {
		return v
	}

	// Priority 3: Default
	return "localhost"
}

// resolveGraphPort resolves the FalkorDB port value.
func resolveGraphPort(cmd *cobra.Command) int {
	// Priority 1: CLI flag
	if cmd.Flags().Changed("graph-port") {
		return initializeGraphPort
	}

	// Priority 2: MEMORIZER_GRAPH_PORT env var
	if v := getEnvInt("MEMORIZER_GRAPH_PORT"); v != 0 {
		return v
	}

	// Priority 3: Default
	return 6379
}

// resolveSemanticProvider resolves the semantic analysis provider.
func resolveSemanticProvider(cmd *cobra.Command) string {
	// Priority 1: CLI flag
	if cmd.Flags().Changed("semantic-provider") {
		return initializeSemanticProvider
	}

	// Priority 2: MEMORIZER_SEMANTIC_PROVIDER env var
	if v := os.Getenv("MEMORIZER_SEMANTIC_PROVIDER"); v != "" {
		return v
	}

	// Priority 3: Default
	return "anthropic"
}

// resolveSemanticEnabled resolves whether semantic analysis is enabled.
func resolveSemanticEnabled(cmd *cobra.Command) bool {
	// If --no-semantic flag is set, semantic analysis is disabled
	if cmd.Flags().Changed("no-semantic") && initializeNoSemantic {
		return false
	}

	// Check MEMORIZER_SEMANTIC_ENABLED env var
	if v := os.Getenv("MEMORIZER_SEMANTIC_ENABLED"); v != "" {
		return getEnvBool("MEMORIZER_SEMANTIC_ENABLED")
	}

	// Default: enabled
	return true
}

// resolveSemanticModel resolves the semantic model based on provider.
func resolveSemanticModel(cmd *cobra.Command, provider string) string {
	// Priority 1: CLI flag
	if cmd.Flags().Changed("semantic-model") {
		return initializeSemanticModel
	}

	// Priority 2: MEMORIZER_SEMANTIC_MODEL env var
	if v := os.Getenv("MEMORIZER_SEMANTIC_MODEL"); v != "" {
		return v
	}

	// Priority 3: Provider default model
	return getDefaultSemanticModel(provider)
}

// resolveSemanticAPIKey resolves the semantic API key and its source.
func resolveSemanticAPIKey(cmd *cobra.Command, provider string) (string, string) {
	// Priority 1: CLI flag
	if cmd.Flags().Changed("semantic-api-key") {
		return initializeSemanticAPIKey, "flag"
	}

	// Priority 2: MEMORIZER_SEMANTIC_API_KEY env var (not in spec, but follows pattern)
	// Note: The spec doesn't mention MEMORIZER_SEMANTIC_API_KEY, only provider-native vars
	// Skipping this for strict spec compliance

	// Priority 3: Provider-native environment variable
	if envVar, ok := semanticProviderEnvVars[provider]; ok {
		if v := os.Getenv(envVar); v != "" {
			return v, envVar
		}
	}

	return "", ""
}

// resolveEmbeddingsEnabled resolves whether embeddings are enabled.
func resolveEmbeddingsEnabled(cmd *cobra.Command) bool {
	// If --no-embeddings flag is set, embeddings are disabled
	if cmd.Flags().Changed("no-embeddings") && initializeNoEmbeddings {
		return false
	}

	// Check MEMORIZER_EMBEDDINGS_ENABLED env var
	if v := os.Getenv("MEMORIZER_EMBEDDINGS_ENABLED"); v != "" {
		return getEnvBool("MEMORIZER_EMBEDDINGS_ENABLED")
	}

	// Default: enabled
	return true
}

// resolveEmbeddingsProvider resolves the embeddings provider.
func resolveEmbeddingsProvider(cmd *cobra.Command) string {
	// Priority 1: CLI flag
	if cmd.Flags().Changed("embeddings-provider") {
		return initializeEmbeddingsProvider
	}

	// Priority 2: MEMORIZER_EMBEDDINGS_PROVIDER env var
	if v := os.Getenv("MEMORIZER_EMBEDDINGS_PROVIDER"); v != "" {
		return v
	}

	// Priority 3: Default
	return "openai"
}

// resolveEmbeddingsModel resolves the embeddings model based on provider.
func resolveEmbeddingsModel(cmd *cobra.Command, provider string) string {
	// Priority 1: CLI flag
	if cmd.Flags().Changed("embeddings-model") {
		return initializeEmbeddingsModel
	}

	// Priority 2: MEMORIZER_EMBEDDINGS_MODEL env var
	if v := os.Getenv("MEMORIZER_EMBEDDINGS_MODEL"); v != "" {
		return v
	}

	// Priority 3: Provider default model
	return getDefaultEmbeddingsModel(provider)
}

// resolveEmbeddingsAPIKey resolves the embeddings API key and its source.
func resolveEmbeddingsAPIKey(cmd *cobra.Command, provider string) (string, string) {
	// Priority 1: CLI flag
	if cmd.Flags().Changed("embeddings-api-key") {
		return initializeEmbeddingsAPIKey, "flag"
	}

	// Priority 2: Provider-native environment variable
	if envVar, ok := embeddingsProviderEnvVars[provider]; ok {
		if v := os.Getenv(envVar); v != "" {
			return v, envVar
		}
	}

	return "", ""
}

// resolveHTTPPort resolves the daemon HTTP port.
func resolveHTTPPort(cmd *cobra.Command) int {
	// Priority 1: CLI flag
	if cmd.Flags().Changed("http-port") {
		return initializeHTTPPort
	}

	// Priority 2: MEMORIZER_DAEMON_HTTP_PORT env var
	if v := getEnvInt("MEMORIZER_DAEMON_HTTP_PORT"); v != 0 {
		return v
	}

	// Priority 3: Default
	return 7600
}

// getDefaultSemanticModel returns the default model for a semantic provider.
func getDefaultSemanticModel(provider string) string {
	if model, ok := defaultSemanticModels[provider]; ok {
		return model
	}
	return ""
}

// getDefaultEmbeddingsModel returns the default model for an embeddings provider.
func getDefaultEmbeddingsModel(provider string) string {
	if model, ok := defaultEmbeddingsModels[provider]; ok {
		return model
	}
	return ""
}

// getEmbeddingsDimensions returns the dimensions for an embeddings model.
func getEmbeddingsDimensions(model string) int {
	if dims, ok := embeddingsDimensions[model]; ok {
		return dims
	}
	// Default dimensions for unknown models
	return 1536
}

// getEnvInt reads an environment variable as an integer.
func getEnvInt(key string) int {
	v := os.Getenv(key)
	if v == "" {
		return 0
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return i
}

// getEnvBool reads an environment variable as a boolean.
func getEnvBool(key string) bool {
	v := os.Getenv(key)
	if v == "" {
		return false
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false
	}
	return b
}
