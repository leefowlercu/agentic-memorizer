package semantic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/providers"
)

const (
	anthropicAPIURL     = "https://api.anthropic.com/v1/messages"
	anthropicAPIVersion = "2023-06-01"
	defaultModel        = "claude-sonnet-4-5-20250929"
	analysisVersion     = 1
)

// AnthropicProvider implements SemanticProvider using Anthropic's Claude API.
type AnthropicProvider struct {
	apiKey          string
	model           string
	httpClient      *http.Client
	rateLimiter     *providers.RateLimiter
	rateLimitConfig *providers.RateLimitConfig
}

// AnthropicOption configures the AnthropicProvider.
type AnthropicOption func(*AnthropicProvider)

// WithModel sets the model to use.
func WithModel(model string) AnthropicOption {
	return func(p *AnthropicProvider) {
		p.model = model
	}
}

// WithHTTPClient sets the HTTP client to use.
func WithHTTPClient(client *http.Client) AnthropicOption {
	return func(p *AnthropicProvider) {
		p.httpClient = client
	}
}

// WithRateLimit sets a custom rate limit configuration.
func WithRateLimit(requestsPerMinute int) AnthropicOption {
	return func(p *AnthropicProvider) {
		p.rateLimitConfig = &providers.RateLimitConfig{
			RequestsPerMinute: requestsPerMinute,
			TokensPerMinute:   100000,
			BurstSize:         max(1, requestsPerMinute/5),
		}
	}
}

// NewAnthropicProvider creates a new Anthropic semantic provider.
func NewAnthropicProvider(opts ...AnthropicOption) *AnthropicProvider {
	p := &AnthropicProvider{
		apiKey:     os.Getenv("ANTHROPIC_API_KEY"),
		model:      defaultModel,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}

	for _, opt := range opts {
		opt(p)
	}

	p.rateLimiter = providers.NewRateLimiter(p.RateLimit())

	return p
}

// Name returns the provider's unique identifier.
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// Type returns the provider type.
func (p *AnthropicProvider) Type() providers.ProviderType {
	return providers.ProviderTypeSemantic
}

// Available returns true if the provider is configured and ready.
func (p *AnthropicProvider) Available() bool {
	return p.apiKey != ""
}

// RateLimit returns the rate limit configuration.
func (p *AnthropicProvider) RateLimit() providers.RateLimitConfig {
	if p.rateLimitConfig != nil {
		return *p.rateLimitConfig
	}
	// Default: conservative limit to avoid hitting output token limits
	// Anthropic limits output tokens per minute (often 8,000 for lower tiers).
	// With max_tokens=4096 per request, ~2 requests could exceed the limit.
	// Default to 10 req/min for safety.
	return providers.RateLimitConfig{
		RequestsPerMinute: 10,
		TokensPerMinute:   100000,
		BurstSize:         2,
	}
}

// SupportedMIMETypes returns the MIME types this provider can analyze.
func (p *AnthropicProvider) SupportedMIMETypes() []string {
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
func (p *AnthropicProvider) MaxContentSize() int64 {
	return 100 * 1024 // 100KB for text content
}

// SupportsVision returns true if the provider supports vision/image analysis.
func (p *AnthropicProvider) SupportsVision() bool {
	return true
}

// Analyze performs semantic analysis on the given content.
func (p *AnthropicProvider) Analyze(ctx context.Context, req providers.SemanticRequest) (*providers.SemanticResult, error) {
	if !p.Available() {
		return nil, fmt.Errorf("anthropic provider not available; ANTHROPIC_API_KEY not set")
	}

	// Wait for rate limit
	if err := p.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed; %w", err)
	}

	// Build the prompt
	systemPrompt := buildSystemPrompt()
	userContent := buildUserContent(req)

	// Build request body
	requestBody := map[string]any{
		"model":      p.model,
		"max_tokens": 4096,
		"system":     systemPrompt,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": userContent,
			},
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request; %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", anthropicAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request; %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

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
	var apiResp anthropicResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response; %w", err)
	}

	// Extract text content
	var textContent string
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			textContent = block.Text
			break
		}
	}

	// Parse the structured response
	result, err := parseAnalysisResponse(textContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse analysis; %w", err)
	}

	result.ProviderName = p.Name()
	result.ModelName = p.model
	result.AnalyzedAt = time.Now()
	result.TokensUsed = apiResp.Usage.InputTokens + apiResp.Usage.OutputTokens
	result.Version = analysisVersion

	return result, nil
}

// anthropicResponse represents the API response structure.
type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// buildSystemPrompt creates the system prompt for analysis.
func buildSystemPrompt() string {
	return `You are a semantic analysis assistant. Analyze the provided content and extract structured information.

Respond with a JSON object containing:
- summary: A 1-2 sentence description of the content
- tags: Array of categorical labels (e.g., "documentation", "implementation", "test", "config")
- topics: Array of objects with "name" and "confidence" (0.0-1.0) fields
- entities: Array of objects with "name" and "type" fields (types: person, organization, concept, technology, package)
- references: Array of objects with "type" (url, file, package) and "target" fields
- language: Programming language if code, or natural language
- complexity: Integer 1-10 indicating complexity
- keywords: Array of important terms

Respond ONLY with valid JSON, no markdown formatting or explanation.`
}

// buildUserContent creates the user message content.
func buildUserContent(req providers.SemanticRequest) any {
	if req.ImageData != "" {
		// Vision request
		return []map[string]any{
			{
				"type": "image",
				"source": map[string]string{
					"type":       "base64",
					"media_type": req.MIMEType,
					"data":       req.ImageData,
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

// parseAnalysisResponse parses the JSON response into a SemanticResult.
func parseAnalysisResponse(text string) (*providers.SemanticResult, error) {
	// Clean up potential markdown formatting
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var result providers.SemanticResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, err
	}

	return &result, nil
}
