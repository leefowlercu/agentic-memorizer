package config

import (
	"testing"
)

func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig()

	// Test top-level fields
	if cfg.LogLevel != DefaultLogLevel {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, DefaultLogLevel)
	}
	if cfg.LogFile != DefaultLogFile {
		t.Errorf("LogFile = %q, want %q", cfg.LogFile, DefaultLogFile)
	}

	// Test Daemon section
	if cfg.Daemon.HTTPPort != DefaultDaemonHTTPPort {
		t.Errorf("Daemon.HTTPPort = %d, want %d", cfg.Daemon.HTTPPort, DefaultDaemonHTTPPort)
	}
	if cfg.Daemon.HTTPBind != DefaultDaemonHTTPBind {
		t.Errorf("Daemon.HTTPBind = %q, want %q", cfg.Daemon.HTTPBind, DefaultDaemonHTTPBind)
	}
	if cfg.Daemon.ShutdownTimeout != DefaultDaemonShutdownTimeout {
		t.Errorf("Daemon.ShutdownTimeout = %d, want %d", cfg.Daemon.ShutdownTimeout, DefaultDaemonShutdownTimeout)
	}
	if cfg.Daemon.PIDFile != DefaultDaemonPIDFile {
		t.Errorf("Daemon.PIDFile = %q, want %q", cfg.Daemon.PIDFile, DefaultDaemonPIDFile)
	}
	if cfg.Daemon.RegistryPath != DefaultDaemonRegistryPath {
		t.Errorf("Daemon.RegistryPath = %q, want %q", cfg.Daemon.RegistryPath, DefaultDaemonRegistryPath)
	}
	if cfg.Daemon.Metrics.CollectionInterval != DefaultDaemonMetricsInterval {
		t.Errorf("Daemon.Metrics.CollectionInterval = %d, want %d", cfg.Daemon.Metrics.CollectionInterval, DefaultDaemonMetricsInterval)
	}

	// Test Graph section
	if cfg.Graph.Host != DefaultGraphHost {
		t.Errorf("Graph.Host = %q, want %q", cfg.Graph.Host, DefaultGraphHost)
	}
	if cfg.Graph.Port != DefaultGraphPort {
		t.Errorf("Graph.Port = %d, want %d", cfg.Graph.Port, DefaultGraphPort)
	}
	if cfg.Graph.Name != DefaultGraphName {
		t.Errorf("Graph.Name = %q, want %q", cfg.Graph.Name, DefaultGraphName)
	}
	if cfg.Graph.PasswordEnv != DefaultGraphPasswordEnv {
		t.Errorf("Graph.PasswordEnv = %q, want %q", cfg.Graph.PasswordEnv, DefaultGraphPasswordEnv)
	}
	if cfg.Graph.MaxRetries != DefaultGraphMaxRetries {
		t.Errorf("Graph.MaxRetries = %d, want %d", cfg.Graph.MaxRetries, DefaultGraphMaxRetries)
	}
	if cfg.Graph.RetryDelayMs != DefaultGraphRetryDelayMs {
		t.Errorf("Graph.RetryDelayMs = %d, want %d", cfg.Graph.RetryDelayMs, DefaultGraphRetryDelayMs)
	}
	if cfg.Graph.WriteQueueSize != DefaultGraphWriteQueueSize {
		t.Errorf("Graph.WriteQueueSize = %d, want %d", cfg.Graph.WriteQueueSize, DefaultGraphWriteQueueSize)
	}

	// Test Semantic section
	if cfg.Semantic.Provider != DefaultSemanticProvider {
		t.Errorf("Semantic.Provider = %q, want %q", cfg.Semantic.Provider, DefaultSemanticProvider)
	}
	if cfg.Semantic.Model != DefaultSemanticModel {
		t.Errorf("Semantic.Model = %q, want %q", cfg.Semantic.Model, DefaultSemanticModel)
	}
	if cfg.Semantic.RateLimit != DefaultSemanticRateLimit {
		t.Errorf("Semantic.RateLimit = %d, want %d", cfg.Semantic.RateLimit, DefaultSemanticRateLimit)
	}
	if cfg.Semantic.APIKey != nil {
		t.Errorf("Semantic.APIKey = %v, want nil", cfg.Semantic.APIKey)
	}
	if cfg.Semantic.APIKeyEnv != DefaultSemanticAPIKeyEnv {
		t.Errorf("Semantic.APIKeyEnv = %q, want %q", cfg.Semantic.APIKeyEnv, DefaultSemanticAPIKeyEnv)
	}

	// Test Embeddings section
	if cfg.Embeddings.Enabled != DefaultEmbeddingsEnabled {
		t.Errorf("Embeddings.Enabled = %v, want %v", cfg.Embeddings.Enabled, DefaultEmbeddingsEnabled)
	}
	if cfg.Embeddings.Provider != DefaultEmbeddingsProvider {
		t.Errorf("Embeddings.Provider = %q, want %q", cfg.Embeddings.Provider, DefaultEmbeddingsProvider)
	}
	if cfg.Embeddings.Model != DefaultEmbeddingsModel {
		t.Errorf("Embeddings.Model = %q, want %q", cfg.Embeddings.Model, DefaultEmbeddingsModel)
	}
	if cfg.Embeddings.Dimensions != DefaultEmbeddingsDimensions {
		t.Errorf("Embeddings.Dimensions = %d, want %d", cfg.Embeddings.Dimensions, DefaultEmbeddingsDimensions)
	}
	if cfg.Embeddings.APIKey != nil {
		t.Errorf("Embeddings.APIKey = %v, want nil", cfg.Embeddings.APIKey)
	}
	if cfg.Embeddings.APIKeyEnv != DefaultEmbeddingsAPIKeyEnv {
		t.Errorf("Embeddings.APIKeyEnv = %q, want %q", cfg.Embeddings.APIKeyEnv, DefaultEmbeddingsAPIKeyEnv)
	}

	// Test Defaults section
	if cfg.Defaults.Skip.Hidden != DefaultSkipHidden {
		t.Errorf("Defaults.Skip.Hidden = %v, want %v", cfg.Defaults.Skip.Hidden, DefaultSkipHidden)
	}
	if len(cfg.Defaults.Skip.Extensions) != len(DefaultSkipExtensions) {
		t.Errorf("Defaults.Skip.Extensions length = %d, want %d", len(cfg.Defaults.Skip.Extensions), len(DefaultSkipExtensions))
	}
	if len(cfg.Defaults.Skip.Directories) != len(DefaultSkipDirectories) {
		t.Errorf("Defaults.Skip.Directories length = %d, want %d", len(cfg.Defaults.Skip.Directories), len(DefaultSkipDirectories))
	}
	if len(cfg.Defaults.Skip.Files) != len(DefaultSkipFiles) {
		t.Errorf("Defaults.Skip.Files length = %d, want %d", len(cfg.Defaults.Skip.Files), len(DefaultSkipFiles))
	}
	if len(cfg.Defaults.Include.Extensions) != 0 {
		t.Errorf("Defaults.Include.Extensions length = %d, want 0", len(cfg.Defaults.Include.Extensions))
	}
	if len(cfg.Defaults.Include.Directories) != 0 {
		t.Errorf("Defaults.Include.Directories length = %d, want 0", len(cfg.Defaults.Include.Directories))
	}
	if len(cfg.Defaults.Include.Files) != 0 {
		t.Errorf("Defaults.Include.Files length = %d, want 0", len(cfg.Defaults.Include.Files))
	}
}

func TestSemanticConfigResolveAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		config   SemanticConfig
		envKey   string
		envValue string
		want     string
	}{
		{
			name: "returns config api_key when set",
			config: SemanticConfig{
				APIKey:    stringPtr("sk-config-key"),
				APIKeyEnv: "TEST_SEMANTIC_KEY",
			},
			envKey:   "TEST_SEMANTIC_KEY",
			envValue: "sk-env-key",
			want:     "sk-config-key",
		},
		{
			name: "returns env var when api_key is nil",
			config: SemanticConfig{
				APIKey:    nil,
				APIKeyEnv: "TEST_SEMANTIC_KEY",
			},
			envKey:   "TEST_SEMANTIC_KEY",
			envValue: "sk-env-key",
			want:     "sk-env-key",
		},
		{
			name: "returns env var when api_key is empty string",
			config: SemanticConfig{
				APIKey:    stringPtr(""),
				APIKeyEnv: "TEST_SEMANTIC_KEY",
			},
			envKey:   "TEST_SEMANTIC_KEY",
			envValue: "sk-env-key",
			want:     "sk-env-key",
		},
		{
			name: "returns empty when both are empty",
			config: SemanticConfig{
				APIKey:    nil,
				APIKeyEnv: "TEST_SEMANTIC_KEY_UNSET",
			},
			envKey:   "",
			envValue: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envKey != "" {
				t.Setenv(tt.envKey, tt.envValue)
			}

			got := tt.config.ResolveAPIKey()
			if got != tt.want {
				t.Errorf("ResolveAPIKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEmbeddingsConfigResolveAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		config   EmbeddingsConfig
		envKey   string
		envValue string
		want     string
	}{
		{
			name: "returns config api_key when set",
			config: EmbeddingsConfig{
				APIKey:    stringPtr("sk-config-key"),
				APIKeyEnv: "TEST_EMBEDDINGS_KEY",
			},
			envKey:   "TEST_EMBEDDINGS_KEY",
			envValue: "sk-env-key",
			want:     "sk-config-key",
		},
		{
			name: "returns env var when api_key is nil",
			config: EmbeddingsConfig{
				APIKey:    nil,
				APIKeyEnv: "TEST_EMBEDDINGS_KEY",
			},
			envKey:   "TEST_EMBEDDINGS_KEY",
			envValue: "sk-env-key",
			want:     "sk-env-key",
		},
		{
			name: "returns env var when api_key is empty string",
			config: EmbeddingsConfig{
				APIKey:    stringPtr(""),
				APIKeyEnv: "TEST_EMBEDDINGS_KEY",
			},
			envKey:   "TEST_EMBEDDINGS_KEY",
			envValue: "sk-env-key",
			want:     "sk-env-key",
		},
		{
			name: "returns empty when both are empty",
			config: EmbeddingsConfig{
				APIKey:    nil,
				APIKeyEnv: "TEST_EMBEDDINGS_KEY_UNSET",
			},
			envKey:   "",
			envValue: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envKey != "" {
				t.Setenv(tt.envKey, tt.envValue)
			}

			got := tt.config.ResolveAPIKey()
			if got != tt.want {
				t.Errorf("ResolveAPIKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

// stringPtr is a helper to create a pointer to a string.
func stringPtr(s string) *string {
	return &s
}
