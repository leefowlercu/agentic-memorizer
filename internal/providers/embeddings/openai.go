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
	openaiEmbeddingsURL   = "https://api.openai.com/v1/embeddings"
	openaiDefaultEmbModel = "text-embedding-3-small"
	embeddingsVersion     = 1
)

// OpenAIEmbeddingsProvider implements EmbeddingsProvider using OpenAI's API.
type OpenAIEmbeddingsProvider struct {
	apiKey      string
	model       string
	dimensions  int
	httpClient  *http.Client
	rateLimiter *providers.RateLimiter
}

// OpenAIEmbeddingsOption configures the OpenAIEmbeddingsProvider.
type OpenAIEmbeddingsOption func(*OpenAIEmbeddingsProvider)

// WithEmbeddingsModel sets the model to use.
func WithEmbeddingsModel(model string) OpenAIEmbeddingsOption {
	return func(p *OpenAIEmbeddingsProvider) {
		p.model = model
	}
}

// WithEmbeddingsDimensions sets the embedding dimensions.
func WithEmbeddingsDimensions(dims int) OpenAIEmbeddingsOption {
	return func(p *OpenAIEmbeddingsProvider) {
		p.dimensions = dims
	}
}

// WithEmbeddingsHTTPClient sets the HTTP client to use.
func WithEmbeddingsHTTPClient(client *http.Client) OpenAIEmbeddingsOption {
	return func(p *OpenAIEmbeddingsProvider) {
		p.httpClient = client
	}
}

// NewOpenAIEmbeddingsProvider creates a new OpenAI embeddings provider.
func NewOpenAIEmbeddingsProvider(opts ...OpenAIEmbeddingsOption) *OpenAIEmbeddingsProvider {
	p := &OpenAIEmbeddingsProvider{
		apiKey:     os.Getenv("OPENAI_API_KEY"),
		model:      openaiDefaultEmbModel,
		dimensions: 1536, // text-embedding-3-small default
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}

	for _, opt := range opts {
		opt(p)
	}

	p.rateLimiter = providers.NewRateLimiter(p.RateLimit())

	return p
}

// Name returns the provider's unique identifier.
func (p *OpenAIEmbeddingsProvider) Name() string {
	return "openai-embeddings"
}

// Type returns the provider type.
func (p *OpenAIEmbeddingsProvider) Type() providers.ProviderType {
	return providers.ProviderTypeEmbeddings
}

// Available returns true if the provider is configured and ready.
func (p *OpenAIEmbeddingsProvider) Available() bool {
	return p.apiKey != ""
}

// RateLimit returns the rate limit configuration.
func (p *OpenAIEmbeddingsProvider) RateLimit() providers.RateLimitConfig {
	return providers.RateLimitConfig{
		RequestsPerMinute: 500,
		TokensPerMinute:   1000000,
		BurstSize:         50,
	}
}

// ModelName returns the name of the embedding model.
func (p *OpenAIEmbeddingsProvider) ModelName() string {
	return p.model
}

// Dimensions returns the dimensionality of the embedding vectors.
func (p *OpenAIEmbeddingsProvider) Dimensions() int {
	return p.dimensions
}

// MaxTokens returns the maximum number of tokens per request.
func (p *OpenAIEmbeddingsProvider) MaxTokens() int {
	return 8191 // text-embedding-3-small limit
}

// Embed generates embeddings for the given content.
func (p *OpenAIEmbeddingsProvider) Embed(ctx context.Context, req providers.EmbeddingsRequest) (*providers.EmbeddingsResult, error) {
	if !p.Available() {
		return nil, fmt.Errorf("openai embeddings provider not available; OPENAI_API_KEY not set")
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

	// Add dimensions if using text-embedding-3 models
	if p.model == "text-embedding-3-small" || p.model == "text-embedding-3-large" {
		requestBody["dimensions"] = p.dimensions
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request; %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", openaiEmbeddingsURL, bytes.NewReader(jsonBody))
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

	// Parse response
	var apiResp openaiEmbeddingsResponse
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

// openaiEmbeddingsResponse represents the OpenAI embeddings API response.
type openaiEmbeddingsResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}
