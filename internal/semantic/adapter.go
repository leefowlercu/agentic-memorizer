package semantic

import (
	"context"
	"log/slog"
	"time"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// Analyzer provides backward compatibility for existing code.
// This wraps a Provider implementation and will be removed in Phase 5.
// Deprecated: Use Provider interface directly.
type Analyzer struct {
	provider Provider
}

// NewAnalyzer creates a new analyzer (compatibility wrapper).
// This creates a Claude provider instance internally.
// Deprecated: Use provider registry directly in Phase 5.
func NewAnalyzer(client *Client, enableVision bool, maxFileSize int64) *Analyzer {
	// Create provider config from client parameters
	config := ProviderConfig{
		APIKey:       client.apiKey,
		Model:        client.model,
		MaxTokens:    client.maxTokens,
		Timeout:      int(client.timeout.Seconds()),
		EnableVision: enableVision,
		MaxFileSize:  maxFileSize,
	}

	// Get Claude provider factory from registry
	registry := GlobalRegistry()
	factory, err := registry.Get("claude")
	if err != nil {
		panic("claude provider not registered: " + err.Error())
	}

	// Create provider instance with noop logger (existing code doesn't pass logger)
	provider, err := factory(config, slog.Default())
	if err != nil {
		panic("failed to create claude provider: " + err.Error())
	}

	return &Analyzer{
		provider: provider,
	}
}

// Analyze wraps the provider's Analyze method
func (a *Analyzer) Analyze(metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	return a.provider.Analyze(context.Background(), metadata)
}

// Client provides backward compatibility for existing code.
// This is a simple struct that holds configuration for creating providers.
// Deprecated: Use ProviderConfig directly in Phase 5.
type Client struct {
	apiKey    string
	model     string
	maxTokens int
	timeout   time.Duration
}

// NewClient creates a new client (compatibility wrapper).
// Deprecated: Use provider registry directly in Phase 5.
func NewClient(apiKey, model string, maxTokens, timeoutSeconds int) *Client {
	return &Client{
		apiKey:    apiKey,
		model:     model,
		maxTokens: maxTokens,
		timeout:   time.Duration(timeoutSeconds) * time.Second,
	}
}
