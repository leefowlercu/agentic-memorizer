package openai

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/embeddings"
	"github.com/sashabaranov/go-openai"
)

const (
	// ProviderName is the identifier for the OpenAI embedding provider
	ProviderName = "openai"

	// DefaultModel is the default OpenAI embedding model
	DefaultModel = "text-embedding-3-small"

	// DefaultDimensions is the default vector dimension for text-embedding-3-small
	DefaultDimensions = 1536

	// DefaultRateLimitRPM is the default rate limit for OpenAI embeddings API (3000 RPM)
	DefaultRateLimitRPM = 3000
)

// Provider implements the embeddings.Provider interface using OpenAI's embedding API
type Provider struct {
	client     *openai.Client
	model      string
	dimensions int
	logger     *slog.Logger
}

// NewProvider creates a new OpenAI embedding provider from the shared ProviderConfig.
// This is the factory function registered with the embeddings registry.
func NewProvider(config embeddings.ProviderConfig, logger *slog.Logger) (embeddings.Provider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	if logger == nil {
		logger = slog.Default()
	}

	model := config.Model
	if model == "" {
		model = DefaultModel
	}

	dimensions := config.Dimensions
	if dimensions == 0 {
		dimensions = DefaultDimensions
	}

	client := openai.NewClient(config.APIKey)

	return &Provider{
		client:     client,
		model:      model,
		dimensions: dimensions,
		logger:     logger.With("component", "openai-embeddings"),
	}, nil
}

// Embed generates an embedding vector for a single text
func (p *Provider) Embed(ctx context.Context, text string) ([]float32, error) {
	results, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return results[0], nil
}

// EmbedBatch generates embedding vectors for multiple texts
func (p *Provider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	start := time.Now()

	req := openai.EmbeddingRequest{
		Input:      texts,
		Model:      openai.EmbeddingModel(p.model),
		Dimensions: p.dimensions,
	}

	resp, err := p.client.CreateEmbeddings(ctx, req)
	if err != nil {
		p.logger.Error("failed to create embeddings",
			"error", err,
			"model", p.model,
			"count", len(texts),
		)
		return nil, fmt.Errorf("failed to create embeddings; %w", err)
	}

	// Extract embeddings in order
	results := make([][]float32, len(texts))
	for _, data := range resp.Data {
		if data.Index >= len(texts) {
			continue
		}
		results[data.Index] = data.Embedding
	}

	p.logger.Debug("created embeddings",
		"count", len(texts),
		"model", p.model,
		"dimensions", p.dimensions,
		"duration_ms", time.Since(start).Milliseconds(),
		"tokens_used", resp.Usage.TotalTokens,
	)

	return results, nil
}

// Dimensions returns the number of dimensions in the embedding vector
func (p *Provider) Dimensions() int {
	return p.dimensions
}

// Model returns the model name being used
func (p *Provider) Model() string {
	return p.model
}

// Name returns the provider identifier
func (p *Provider) Name() string {
	return ProviderName
}

// DefaultRateLimit returns the default rate limit in requests per minute
func (p *Provider) DefaultRateLimit() int {
	return DefaultRateLimitRPM
}
