package daemon

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/providers"
	"github.com/leefowlercu/agentic-memorizer/internal/providers/embeddings"
	"github.com/leefowlercu/agentic-memorizer/internal/providers/semantic"
)

// createSemanticProvider creates a semantic provider based on configuration.
// Returns nil if the provider cannot be created (e.g., missing API key).
func createSemanticProvider(cfg *config.SemanticConfig) (providers.SemanticProvider, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	// Ensure API key is available in environment
	apiKey := cfg.ResolveAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("no API key available for semantic provider %q; set %s environment variable", cfg.Provider, cfg.APIKeyEnv)
	}

	// Set the environment variable if it's from config (not already in env)
	if os.Getenv(cfg.APIKeyEnv) == "" && cfg.APIKey != nil && *cfg.APIKey != "" {
		os.Setenv(cfg.APIKeyEnv, *cfg.APIKey)
	}

	switch cfg.Provider {
	case "anthropic":
		var opts []semantic.AnthropicOption
		if cfg.Model != "" {
			opts = append(opts, semantic.WithModel(cfg.Model))
		}
		if cfg.RateLimit > 0 {
			opts = append(opts, semantic.WithRateLimit(cfg.RateLimit))
		}
		return semantic.NewAnthropicProvider(opts...), nil

	case "openai":
		var opts []semantic.OpenAISemanticOption
		if cfg.Model != "" {
			opts = append(opts, semantic.WithOpenAIModel(cfg.Model))
		}
		if cfg.RateLimit > 0 {
			opts = append(opts, semantic.WithOpenAIRateLimit(cfg.RateLimit))
		}
		return semantic.NewOpenAISemanticProvider(opts...), nil

	case "google":
		var opts []semantic.GoogleSemanticOption
		if cfg.Model != "" {
			opts = append(opts, semantic.WithGoogleModel(cfg.Model))
		}
		if cfg.RateLimit > 0 {
			opts = append(opts, semantic.WithGoogleRateLimit(cfg.RateLimit))
		}
		return semantic.NewGoogleSemanticProvider(opts...), nil

	default:
		return nil, fmt.Errorf("unknown semantic provider: %s", cfg.Provider)
	}
}

// createEmbeddingsProvider creates an embeddings provider based on configuration.
// Returns nil if embeddings are disabled or the provider cannot be created.
func createEmbeddingsProvider(cfg *config.EmbeddingsConfig) (providers.EmbeddingsProvider, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	// Ensure API key is available in environment
	apiKey := cfg.ResolveAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("no API key available for embeddings provider %q; set %s environment variable", cfg.Provider, cfg.APIKeyEnv)
	}

	// Set the environment variable if it's from config (not already in env)
	if os.Getenv(cfg.APIKeyEnv) == "" && cfg.APIKey != nil && *cfg.APIKey != "" {
		os.Setenv(cfg.APIKeyEnv, *cfg.APIKey)
	}

	switch cfg.Provider {
	case "openai":
		var opts []embeddings.OpenAIEmbeddingsOption
		if cfg.Model != "" {
			opts = append(opts, embeddings.WithEmbeddingsModel(cfg.Model))
		}
		if cfg.Dimensions > 0 {
			opts = append(opts, embeddings.WithEmbeddingsDimensions(cfg.Dimensions))
		}
		return embeddings.NewOpenAIEmbeddingsProvider(opts...), nil

	case "google":
		var opts []embeddings.GoogleEmbeddingsOption
		if cfg.Model != "" {
			opts = append(opts, embeddings.WithGoogleEmbeddingsModel(cfg.Model))
		}
		return embeddings.NewGoogleEmbeddingsProvider(opts...), nil

	case "voyage":
		var opts []embeddings.VoyageEmbeddingsOption
		if cfg.Model != "" {
			opts = append(opts, embeddings.WithVoyageModel(cfg.Model))
		}
		return embeddings.NewVoyageEmbeddingsProvider(opts...), nil

	default:
		return nil, fmt.Errorf("unknown embeddings provider: %s", cfg.Provider)
	}
}

// logProviderStatus logs the availability status of providers.
func logProviderStatus(semanticProvider providers.SemanticProvider, embedProvider providers.EmbeddingsProvider) {
	if semanticProvider != nil {
		if semanticProvider.Available() {
			slog.Info("semantic provider initialized",
				"provider", semanticProvider.Name(),
			)
		} else {
			slog.Warn("semantic provider not available",
				"provider", semanticProvider.Name(),
			)
		}
	}

	if embedProvider != nil {
		if embedProvider.Available() {
			slog.Info("embeddings provider initialized",
				"provider", embedProvider.Name(),
				"model", embedProvider.ModelName(),
				"dimensions", embedProvider.Dimensions(),
			)
		} else {
			slog.Warn("embeddings provider not available",
				"provider", embedProvider.Name(),
			)
		}
	}
}
