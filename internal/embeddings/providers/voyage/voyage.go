package voyage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/embeddings"
)

const (
	// ProviderName is the identifier for the Voyage AI embedding provider
	ProviderName = "voyage"

	// DefaultModel is the default Voyage AI embedding model
	DefaultModel = "voyage-3"

	// DefaultDimensions is the default vector dimension for voyage-3
	DefaultDimensions = 1024

	// DefaultRateLimitRPM is the default rate limit for Voyage AI (300 RPM)
	DefaultRateLimitRPM = 300

	// MaxBatchSize is the maximum number of texts per batch request
	MaxBatchSize = 128

	// APIBaseURL is the Voyage AI API endpoint
	APIBaseURL = "https://api.voyageai.com/v1/embeddings"

	// DefaultTimeout is the default HTTP request timeout
	DefaultTimeout = 60 * time.Second
)

// embeddingRequest represents the API request body
type embeddingRequest struct {
	Input     []string `json:"input"`
	Model     string   `json:"model"`
	InputType string   `json:"input_type,omitempty"`
}

// embeddingResponse represents the API response body
type embeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// errorResponse represents an API error response
type errorResponse struct {
	Detail string `json:"detail"`
}

// Provider implements the embeddings.Provider interface using Voyage AI's embedding API
type Provider struct {
	apiKey     string
	model      string
	dimensions int
	client     *http.Client
	logger     *slog.Logger
}

// NewProvider creates a new Voyage AI embedding provider from the shared ProviderConfig.
// This is the factory function registered with the embeddings registry.
func NewProvider(config embeddings.ProviderConfig, logger *slog.Logger) (embeddings.Provider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("Voyage AI API key is required")
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

	return &Provider{
		apiKey:     config.APIKey,
		model:      model,
		dimensions: dimensions,
		client: &http.Client{
			Timeout: DefaultTimeout,
		},
		logger: logger.With("component", "voyage-embeddings"),
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

	reqBody := embeddingRequest{
		Input:     texts,
		Model:     p.model,
		InputType: "document",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request; %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, APIBaseURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request; %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		p.logger.Error("failed to call Voyage AI API",
			"error", err,
			"model", p.model,
			"count", len(texts),
		)
		return nil, fmt.Errorf("failed to call Voyage AI API; %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body; %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp errorResponse
		if jsonErr := json.Unmarshal(body, &errResp); jsonErr == nil && errResp.Detail != "" {
			return nil, fmt.Errorf("Voyage AI API error (%d); %s", resp.StatusCode, errResp.Detail)
		}
		return nil, fmt.Errorf("Voyage AI API error (%d); %s", resp.StatusCode, string(body))
	}

	var embResp embeddingResponse
	if err := json.Unmarshal(body, &embResp); err != nil {
		return nil, fmt.Errorf("failed to parse response; %w", err)
	}

	// Extract embeddings in order
	results := make([][]float32, len(texts))
	for _, data := range embResp.Data {
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
		"tokens_used", embResp.Usage.TotalTokens,
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
