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

// GoogleSemanticProvider implements SemanticProvider using Google's Gemini API.
type GoogleSemanticProvider struct {
	apiKey          string
	model           string
	httpClient      *http.Client
	rateLimiter     *providers.RateLimiter
	rateLimitConfig *providers.RateLimitConfig
}

// GoogleSemanticOption configures the GoogleSemanticProvider.
type GoogleSemanticOption func(*GoogleSemanticProvider)

// WithGoogleModel sets the model to use.
func WithGoogleModel(model string) GoogleSemanticOption {
	return func(p *GoogleSemanticProvider) {
		p.model = model
	}
}

// WithGoogleRateLimit sets a custom rate limit configuration.
func WithGoogleRateLimit(requestsPerMinute int) GoogleSemanticOption {
	return func(p *GoogleSemanticProvider) {
		p.rateLimitConfig = &providers.RateLimitConfig{
			RequestsPerMinute: requestsPerMinute,
			TokensPerMinute:   100000,
			BurstSize:         max(1, requestsPerMinute/5),
		}
	}
}

// NewGoogleSemanticProvider creates a new Google semantic provider.
func NewGoogleSemanticProvider(opts ...GoogleSemanticOption) *GoogleSemanticProvider {
	p := &GoogleSemanticProvider{
		apiKey:     os.Getenv("GOOGLE_API_KEY"),
		model:      "gemini-3-flash-preview",
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}

	for _, opt := range opts {
		opt(p)
	}

	p.rateLimiter = providers.NewRateLimiter(p.RateLimit())

	return p
}

// Name returns the provider's unique identifier.
func (p *GoogleSemanticProvider) Name() string {
	return "google"
}

// Type returns the provider type.
func (p *GoogleSemanticProvider) Type() providers.ProviderType {
	return providers.ProviderTypeSemantic
}

// Available returns true if the provider is configured and ready.
func (p *GoogleSemanticProvider) Available() bool {
	return p.apiKey != ""
}

// RateLimit returns the rate limit configuration.
func (p *GoogleSemanticProvider) RateLimit() providers.RateLimitConfig {
	if p.rateLimitConfig != nil {
		return *p.rateLimitConfig
	}
	return providers.RateLimitConfig{
		RequestsPerMinute: 60,
		TokensPerMinute:   100000,
		BurstSize:         10,
	}
}

// SupportedMIMETypes returns the MIME types this provider can analyze.
func (p *GoogleSemanticProvider) SupportedMIMETypes() []string {
	return []string{
		"text/plain",
		"text/markdown",
		"text/x-go",
		"text/x-python",
		"application/json",
		"image/jpeg",
		"image/png",
	}
}

// MaxContentSize returns the maximum content size in bytes.
func (p *GoogleSemanticProvider) MaxContentSize() int64 {
	return 100 * 1024
}

// SupportsVision returns true if the provider supports vision/image analysis.
func (p *GoogleSemanticProvider) SupportsVision() bool {
	return true
}

// Analyze performs semantic analysis on the given content.
func (p *GoogleSemanticProvider) Analyze(ctx context.Context, req providers.SemanticRequest) (*providers.SemanticResult, error) {
	if !p.Available() {
		return nil, fmt.Errorf("google provider not available; GOOGLE_API_KEY not set")
	}

	// Wait for rate limit
	if err := p.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed; %w", err)
	}

	// Build request
	apiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", p.model, p.apiKey)

	// Build content parts
	var parts []map[string]any

	if req.ImageData != "" {
		parts = append(parts, map[string]any{
			"inline_data": map[string]string{
				"mime_type": req.MIMEType,
				"data":      req.ImageData,
			},
		})
		parts = append(parts, map[string]any{
			"text": fmt.Sprintf("Analyze this image from file: %s\n\n%s", req.Path, buildSystemPrompt()),
		})
	} else {
		context := fmt.Sprintf("File: %s\nMIME Type: %s\n", req.Path, req.MIMEType)
		if req.TotalChunks > 1 {
			context += fmt.Sprintf("Chunk %d of %d\n", req.ChunkIndex+1, req.TotalChunks)
		}
		context += "\nContent:\n" + req.Content

		parts = append(parts, map[string]any{
			"text": buildSystemPrompt() + "\n\n" + context,
		})
	}

	requestBody := map[string]any{
		"contents": []map[string]any{
			{
				"parts": parts,
			},
		},
		"generationConfig": map[string]any{
			"temperature":     0.1,
			"maxOutputTokens": 4096,
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
	var apiResp googleResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response; %w", err)
	}

	if len(apiResp.Candidates) == 0 || len(apiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response content returned")
	}

	textContent := apiResp.Candidates[0].Content.Parts[0].Text

	// Parse the structured response
	result, err := parseAnalysisResponse(textContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse analysis; %w", err)
	}

	result.ProviderName = p.Name()
	result.ModelName = p.model
	result.AnalyzedAt = time.Now()
	result.TokensUsed = apiResp.UsageMetadata.TotalTokenCount
	result.Version = analysisVersion

	return result, nil
}

// googleResponse represents the Google API response structure.
type googleResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}
