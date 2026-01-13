package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/providers"
)

const (
	voyageAPIURL       = "https://api.voyageai.com/v1/embeddings"
	voyageDefaultModel = "voyage-code-3"
)

// VoyageEmbeddingsProvider implements EmbeddingsProvider using Voyage AI's API.
type VoyageEmbeddingsProvider struct {
	apiKey      string
	model       string
	httpClient  *http.Client
	rateLimiter *providers.RateLimiter
}

// VoyageEmbeddingsOption configures the VoyageEmbeddingsProvider.
type VoyageEmbeddingsOption func(*VoyageEmbeddingsProvider)

// WithVoyageModel sets the model to use.
func WithVoyageModel(model string) VoyageEmbeddingsOption {
	return func(p *VoyageEmbeddingsProvider) {
		p.model = model
	}
}

// NewVoyageEmbeddingsProvider creates a new Voyage embeddings provider.
func NewVoyageEmbeddingsProvider(opts ...VoyageEmbeddingsOption) *VoyageEmbeddingsProvider {
	p := &VoyageEmbeddingsProvider{
		apiKey:     os.Getenv("VOYAGE_API_KEY"),
		model:      voyageDefaultModel,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}

	for _, opt := range opts {
		opt(p)
	}

	p.rateLimiter = providers.NewRateLimiter(p.RateLimit())

	return p
}

// Name returns the provider's unique identifier.
func (p *VoyageEmbeddingsProvider) Name() string {
	return "voyage-embeddings"
}

// Type returns the provider type.
func (p *VoyageEmbeddingsProvider) Type() providers.ProviderType {
	return providers.ProviderTypeEmbeddings
}

// Available returns true if the provider is configured and ready.
func (p *VoyageEmbeddingsProvider) Available() bool {
	return p.apiKey != ""
}

// RateLimit returns the rate limit configuration.
func (p *VoyageEmbeddingsProvider) RateLimit() providers.RateLimitConfig {
	return providers.RateLimitConfig{
		RequestsPerMinute: 300,
		TokensPerMinute:   1000000,
		BurstSize:         30,
	}
}

// ModelName returns the name of the embedding model.
func (p *VoyageEmbeddingsProvider) ModelName() string {
	return p.model
}

// Dimensions returns the dimensionality of the embedding vectors.
func (p *VoyageEmbeddingsProvider) Dimensions() int {
	return 1024 // voyage-code-3 default dimensions
}

// MaxTokens returns the maximum number of tokens per request.
func (p *VoyageEmbeddingsProvider) MaxTokens() int {
	return 32000 // voyage-code-3 context length
}

// Embed generates embeddings for the given content.
func (p *VoyageEmbeddingsProvider) Embed(ctx context.Context, req providers.EmbeddingsRequest) (*providers.EmbeddingsResult, error) {
	if !p.Available() {
		return nil, fmt.Errorf("voyage embeddings provider not available; VOYAGE_API_KEY not set")
	}

	// Wait for rate limit
	if err := p.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed; %w", err)
	}

	// Build request body
	requestBody := map[string]any{
		"model": p.model,
		"input": req.Content,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request; %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", voyageAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request; %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	// Execute request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed; %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response; %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// Parse response (same structure as OpenAI)
	var apiResp voyageEmbeddingsResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response; %w", err)
	}

	if len(apiResp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	// Convert []float64 to []float32
	embedding := make([]float32, len(apiResp.Data[0].Embedding))
	for i, v := range apiResp.Data[0].Embedding {
		embedding[i] = float32(v)
	}

	return &providers.EmbeddingsResult{
		Embedding:    embedding,
		ProviderName: p.Name(),
		ModelName:    p.model,
		Dimensions:   len(embedding),
		TokensUsed:   apiResp.Usage.TotalTokens,
		GeneratedAt:  time.Now(),
		Version:      embeddingsVersion,
	}, nil
}

// voyageEmbeddingsResponse represents the Voyage embeddings API response.
type voyageEmbeddingsResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}
