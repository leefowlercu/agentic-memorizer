package semantic

import (
	"bytes"
	"context"
	"encoding/base64"
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

// ModelName returns the configured model name.
func (p *GoogleSemanticProvider) ModelName() string {
	return p.model
}

// Capabilities returns model-specific input limits and supported modalities.
func (p *GoogleSemanticProvider) Capabilities() providers.SemanticCapabilities {
	caps := providers.SemanticCapabilities{
		MaxInputTokens:  1000000,
		MaxRequestBytes: 50 * 1024 * 1024,
		MaxPDFPages:     1000,
		MaxImages:       100,
		SupportsPDF:     true,
		SupportsImages:  true,
		Model:           p.model,
	}

	switch p.model {
	case "gemini-2.5-flash":
		caps.MaxInputTokens = 200000
	}

	return caps
}

// Analyze performs semantic analysis on the given file-level input.
func (p *GoogleSemanticProvider) Analyze(ctx context.Context, input providers.SemanticInput) (*providers.SemanticResult, error) {
	if !p.Available() {
		return nil, fmt.Errorf("google provider not available; GOOGLE_API_KEY not set")
	}

	if err := p.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed; %w", err)
	}

	apiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", p.model, p.apiKey)

	parts, err := p.buildParts(input)
	if err != nil {
		return nil, err
	}

	requestBody := map[string]any{
		"contents": []map[string]any{
			{
				"role":  "user",
				"parts": parts,
			},
		},
		"systemInstruction": map[string]any{
			"parts": []map[string]any{{"text": buildSystemPrompt()}},
		},
		"generationConfig": map[string]any{
			"temperature":      0.1,
			"maxOutputTokens":  4096,
			"responseMimeType": "application/json",
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

	var apiResp googleResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response; %w", err)
	}

	textContent := apiResp.FirstText()
	if textContent == "" {
		return nil, fmt.Errorf("no text content in response")
	}

	result, err := parseAnalysisResponse(textContent)
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

func (p *GoogleSemanticProvider) buildParts(input providers.SemanticInput) ([]map[string]any, error) {
	switch input.Type {
	case providers.SemanticInputImage:
		if len(input.ImageBytes) == 0 {
			return nil, fmt.Errorf("image input missing bytes")
		}
		encoded := base64.StdEncoding.EncodeToString(input.ImageBytes)
		return []map[string]any{
			{
				"inlineData": map[string]any{
					"mimeType": input.MIMEType,
					"data":     encoded,
				},
			},
			{"text": fmt.Sprintf("Analyze this image from file: %s", input.Path)},
		}, nil
	case providers.SemanticInputPDF:
		if len(input.FileBytes) == 0 {
			return nil, fmt.Errorf("pdf input missing bytes")
		}
		encoded := base64.StdEncoding.EncodeToString(input.FileBytes)
		return []map[string]any{
			{
				"inlineData": map[string]any{
					"mimeType": "application/pdf",
					"data":     encoded,
				},
			},
			{"text": fmt.Sprintf("Analyze this PDF from file: %s", input.Path)},
		}, nil
	default:
		context := buildTextContext(input)
		return []map[string]any{{"text": context}}, nil
	}
}

// googleResponse represents the API response structure.
type googleResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Usage struct {
		TotalTokens int `json:"totalTokens"`
	} `json:"usageMetadata"`
}

func (r googleResponse) FirstText() string {
	if len(r.Candidates) == 0 {
		return ""
	}
	for _, part := range r.Candidates[0].Content.Parts {
		if part.Text != "" {
			return part.Text
		}
	}
	return ""
}
