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
	googleEmbeddingsDefaultModel = "gemini-embedding-001"
)

// GoogleEmbeddingsProvider implements EmbeddingsProvider using Google's API.
type GoogleEmbeddingsProvider struct {
	apiKey      string
	model       string
	httpClient  *http.Client
	rateLimiter *providers.RateLimiter
}

// GoogleEmbeddingsOption configures the GoogleEmbeddingsProvider.
type GoogleEmbeddingsOption func(*GoogleEmbeddingsProvider)

// WithGoogleEmbeddingsModel sets the model to use.
func WithGoogleEmbeddingsModel(model string) GoogleEmbeddingsOption {
	return func(p *GoogleEmbeddingsProvider) {
		p.model = model
	}
}

// NewGoogleEmbeddingsProvider creates a new Google embeddings provider.
func NewGoogleEmbeddingsProvider(opts ...GoogleEmbeddingsOption) *GoogleEmbeddingsProvider {
	p := &GoogleEmbeddingsProvider{
		apiKey:     os.Getenv("GOOGLE_API_KEY"),
		model:      googleEmbeddingsDefaultModel,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}

	for _, opt := range opts {
		opt(p)
	}

	p.rateLimiter = providers.NewRateLimiter(p.RateLimit())

	return p
}

// Name returns the provider's unique identifier.
func (p *GoogleEmbeddingsProvider) Name() string {
	return "google-embeddings"
}

// Type returns the provider type.
func (p *GoogleEmbeddingsProvider) Type() providers.ProviderType {
	return providers.ProviderTypeEmbeddings
}

// Available returns true if the provider is configured and ready.
func (p *GoogleEmbeddingsProvider) Available() bool {
	return p.apiKey != ""
}

// RateLimit returns the rate limit configuration.
func (p *GoogleEmbeddingsProvider) RateLimit() providers.RateLimitConfig {
	return providers.RateLimitConfig{
		RequestsPerMinute: 300,
		TokensPerMinute:   1000000,
		BurstSize:         30,
	}
}

// ModelName returns the name of the embedding model.
func (p *GoogleEmbeddingsProvider) ModelName() string {
	return p.model
}

// Dimensions returns the dimensionality of the embedding vectors.
func (p *GoogleEmbeddingsProvider) Dimensions() int {
	return 3072 // gemini-embedding-001 default dimensions (also supports 768, 1536)
}

// MaxTokens returns the maximum number of tokens per request.
func (p *GoogleEmbeddingsProvider) MaxTokens() int {
	return 2048
}

// Embed generates embeddings for the given content.
func (p *GoogleEmbeddingsProvider) Embed(ctx context.Context, req providers.EmbeddingsRequest) (*providers.EmbeddingsResult, error) {
	if !p.Available() {
		return nil, fmt.Errorf("google embeddings provider not available; GOOGLE_API_KEY not set")
	}

	// Wait for rate limit
	if err := p.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed; %w", err)
	}

	// Build request
	apiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:embedContent?key=%s", p.model, p.apiKey)

	requestBody := map[string]any{
		"model": fmt.Sprintf("models/%s", p.model),
		"content": map[string]any{
			"parts": []map[string]string{
				{"text": req.Content},
			},
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request; %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request; %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed; %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response; %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp googleEmbeddingsResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response; %w", err)
	}

	// Convert []float64 to []float32
	embedding := make([]float32, len(apiResp.Embedding.Values))
	for i, v := range apiResp.Embedding.Values {
		embedding[i] = float32(v)
	}

	return &providers.EmbeddingsResult{
		Embedding:    embedding,
		ProviderName: p.Name(),
		ModelName:    p.model,
		Dimensions:   len(embedding),
		TokensUsed:   0, // Google doesn't return token count
		GeneratedAt:  time.Now(),
		Version:      embeddingsVersion,
	}, nil
}

// googleEmbeddingsResponse represents the Google embeddings API response.
type googleEmbeddingsResponse struct {
	Embedding struct {
		Values []float64 `json:"values"`
	} `json:"embedding"`
}
