package gemini

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/leefowlercu/agentic-memorizer/internal/embeddings"
	"google.golang.org/api/option"
)

const (
	// ProviderName is the identifier for the Gemini embedding provider
	ProviderName = "gemini"

	// DefaultModel is the default Gemini embedding model
	DefaultModel = "text-embedding-004"

	// DefaultDimensions is the default vector dimension for text-embedding-004
	DefaultDimensions = 768

	// DefaultRateLimitRPM is the default rate limit for Gemini embeddings (1500 RPM)
	DefaultRateLimitRPM = 1500

	// MaxBatchSize is the maximum number of texts per batch request
	MaxBatchSize = 100
)

// Provider implements the embeddings.Provider interface using Google Gemini's embedding API
type Provider struct {
	client     *genai.Client
	model      *genai.EmbeddingModel
	modelName  string
	dimensions int
	logger     *slog.Logger
}

// NewProvider creates a new Gemini embedding provider from the shared ProviderConfig.
// This is the factory function registered with the embeddings registry.
func NewProvider(config embeddings.ProviderConfig, logger *slog.Logger) (embeddings.Provider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("Gemini API key is required")
	}

	if logger == nil {
		logger = slog.Default()
	}

	modelName := config.Model
	if modelName == "" {
		modelName = DefaultModel
	}

	dimensions := config.Dimensions
	if dimensions == 0 {
		dimensions = DefaultDimensions
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(config.APIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client; %w", err)
	}

	model := client.EmbeddingModel(modelName)

	return &Provider{
		client:     client,
		model:      model,
		modelName:  modelName,
		dimensions: dimensions,
		logger:     logger.With("component", "gemini-embeddings"),
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

	// Handle batching if texts exceed MaxBatchSize
	if len(texts) > MaxBatchSize {
		return p.embedBatched(ctx, texts)
	}

	start := time.Now()

	// Create batch embedding request
	batch := p.model.NewBatch()
	for _, text := range texts {
		batch.AddContent(genai.Text(text))
	}

	resp, err := p.model.BatchEmbedContents(ctx, batch)
	if err != nil {
		p.logger.Error("failed to create embeddings",
			"error", err,
			"model", p.modelName,
			"count", len(texts),
		)
		return nil, fmt.Errorf("failed to create embeddings; %w", err)
	}

	// Extract embeddings
	results := make([][]float32, len(texts))
	for i, emb := range resp.Embeddings {
		if i >= len(texts) {
			break
		}
		results[i] = emb.Values
	}

	p.logger.Debug("created embeddings",
		"count", len(texts),
		"model", p.modelName,
		"dimensions", p.dimensions,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return results, nil
}

// embedBatched handles embedding requests that exceed MaxBatchSize
func (p *Provider) embedBatched(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))

	for i := 0; i < len(texts); i += MaxBatchSize {
		end := i + MaxBatchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		batchResults, err := p.EmbedBatch(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("failed to embed batch %d; %w", i/MaxBatchSize, err)
		}

		for j, emb := range batchResults {
			results[i+j] = emb
		}
	}

	return results, nil
}

// Dimensions returns the number of dimensions in the embedding vector
func (p *Provider) Dimensions() int {
	return p.dimensions
}

// Model returns the model name being used
func (p *Provider) Model() string {
	return p.modelName
}

// Name returns the provider identifier
func (p *Provider) Name() string {
	return ProviderName
}

// DefaultRateLimit returns the default rate limit in requests per minute
func (p *Provider) DefaultRateLimit() int {
	return DefaultRateLimitRPM
}

// Close releases resources associated with the provider
func (p *Provider) Close() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}
