package embeddings

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements the Provider interface using OpenAI's embedding API
type OpenAIProvider struct {
	client     *openai.Client
	model      string
	dimensions int
	logger     *slog.Logger
}

// OpenAIConfig contains configuration for the OpenAI provider
type OpenAIConfig struct {
	APIKey     string
	Model      string
	Dimensions int
}

// DefaultOpenAIConfig returns default configuration for OpenAI provider
func DefaultOpenAIConfig() OpenAIConfig {
	return OpenAIConfig{
		Model:      "text-embedding-3-small",
		Dimensions: 1536,
	}
}

// NewOpenAIProvider creates a new OpenAI embedding provider
func NewOpenAIProvider(config OpenAIConfig, logger *slog.Logger) (*OpenAIProvider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	if logger == nil {
		logger = slog.Default()
	}

	client := openai.NewClient(config.APIKey)

	return &OpenAIProvider{
		client:     client,
		model:      config.Model,
		dimensions: config.Dimensions,
		logger:     logger.With("component", "openai-embeddings"),
	}, nil
}

// Embed generates an embedding vector for a single text
func (p *OpenAIProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}

// EmbedBatch generates embedding vectors for multiple texts
func (p *OpenAIProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
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
	embeddings := make([][]float32, len(texts))
	for _, data := range resp.Data {
		if data.Index >= len(texts) {
			continue
		}
		embeddings[data.Index] = data.Embedding
	}

	p.logger.Debug("created embeddings",
		"count", len(texts),
		"model", p.model,
		"dimensions", p.dimensions,
		"duration_ms", time.Since(start).Milliseconds(),
		"tokens_used", resp.Usage.TotalTokens,
	)

	return embeddings, nil
}

// Dimensions returns the number of dimensions in the embedding vector
func (p *OpenAIProvider) Dimensions() int {
	return p.dimensions
}

// Model returns the model name being used
func (p *OpenAIProvider) Model() string {
	return p.model
}
