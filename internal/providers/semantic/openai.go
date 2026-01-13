package semantic

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
	openaiAPIURL       = "https://api.openai.com/v1/chat/completions"
	openaiDefaultModel = "gpt-5.2"
)

// OpenAISemanticProvider implements SemanticProvider using OpenAI's API.
type OpenAISemanticProvider struct {
	apiKey      string
	model       string
	httpClient  *http.Client
	rateLimiter *providers.RateLimiter
}

// OpenAISemanticOption configures the OpenAISemanticProvider.
type OpenAISemanticOption func(*OpenAISemanticProvider)

// WithOpenAIModel sets the model to use.
func WithOpenAIModel(model string) OpenAISemanticOption {
	return func(p *OpenAISemanticProvider) {
		p.model = model
	}
}

// WithOpenAIHTTPClient sets the HTTP client to use.
func WithOpenAIHTTPClient(client *http.Client) OpenAISemanticOption {
	return func(p *OpenAISemanticProvider) {
		p.httpClient = client
	}
}

// NewOpenAISemanticProvider creates a new OpenAI semantic provider.
func NewOpenAISemanticProvider(opts ...OpenAISemanticOption) *OpenAISemanticProvider {
	p := &OpenAISemanticProvider{
		apiKey:     os.Getenv("OPENAI_API_KEY"),
		model:      openaiDefaultModel,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}

	for _, opt := range opts {
		opt(p)
	}

	p.rateLimiter = providers.NewRateLimiter(p.RateLimit())

	return p
}

// Name returns the provider's unique identifier.
func (p *OpenAISemanticProvider) Name() string {
	return "openai"
}

// Type returns the provider type.
func (p *OpenAISemanticProvider) Type() providers.ProviderType {
	return providers.ProviderTypeSemantic
}

// Available returns true if the provider is configured and ready.
func (p *OpenAISemanticProvider) Available() bool {
	return p.apiKey != ""
}

// RateLimit returns the rate limit configuration.
func (p *OpenAISemanticProvider) RateLimit() providers.RateLimitConfig {
	return providers.RateLimitConfig{
		RequestsPerMinute: 60,
		TokensPerMinute:   150000,
		BurstSize:         10,
	}
}

// SupportedMIMETypes returns the MIME types this provider can analyze.
func (p *OpenAISemanticProvider) SupportedMIMETypes() []string {
	return []string{
		"text/plain",
		"text/markdown",
		"text/x-go",
		"text/x-python",
		"text/x-java",
		"text/javascript",
		"application/json",
		"application/xml",
		"image/jpeg",
		"image/png",
		"image/gif",
		"image/webp",
	}
}

// MaxContentSize returns the maximum content size in bytes.
func (p *OpenAISemanticProvider) MaxContentSize() int64 {
	return 100 * 1024 // 100KB for text content
}

// SupportsVision returns true if the provider supports vision/image analysis.
func (p *OpenAISemanticProvider) SupportsVision() bool {
	return true
}

// Analyze performs semantic analysis on the given content.
func (p *OpenAISemanticProvider) Analyze(ctx context.Context, req providers.SemanticRequest) (*providers.SemanticResult, error) {
	if !p.Available() {
		return nil, fmt.Errorf("openai provider not available; OPENAI_API_KEY not set")
	}

	// Wait for rate limit
	if err := p.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed; %w", err)
	}

	// Build messages
	messages := []map[string]any{
		{
			"role":    "system",
			"content": buildSystemPrompt(),
		},
	}

	// Build user message
	userContent := buildOpenAIUserContent(req)
	messages = append(messages, map[string]any{
		"role":    "user",
		"content": userContent,
	})

	// Build request body
	requestBody := map[string]any{
		"model":                 p.model,
		"messages":              messages,
		"max_completion_tokens": 4096,
		"temperature":           0.1,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request; %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", openaiAPIURL, bytes.NewReader(jsonBody))
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
	var apiResp openaiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response; %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no response choices returned")
	}

	// Parse the structured response
	result, err := parseAnalysisResponse(apiResp.Choices[0].Message.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse analysis; %w", err)
	}

	result.ProviderName = p.Name()
	result.ModelName = p.model
	result.AnalyzedAt = time.Now()
	result.TokensUsed = apiResp.Usage.TotalTokens
	result.Version = analysisVersion

	return result, nil
}

// openaiResponse represents the OpenAI API response structure.
type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// buildOpenAIUserContent creates the user message content for OpenAI.
func buildOpenAIUserContent(req providers.SemanticRequest) any {
	if req.ImageData != "" {
		// Vision request
		return []map[string]any{
			{
				"type": "image_url",
				"image_url": map[string]string{
					"url": fmt.Sprintf("data:%s;base64,%s", req.MIMEType, req.ImageData),
				},
			},
			{
				"type": "text",
				"text": fmt.Sprintf("Analyze this image from file: %s", req.Path),
			},
		}
	}

	// Text request
	context := fmt.Sprintf("File: %s\nMIME Type: %s\n", req.Path, req.MIMEType)
	if req.TotalChunks > 1 {
		context += fmt.Sprintf("Chunk %d of %d\n", req.ChunkIndex+1, req.TotalChunks)
	}
	context += "\nContent:\n" + req.Content

	return context
}
