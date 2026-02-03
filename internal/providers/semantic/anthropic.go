package semantic

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/providers"
)

const (
	anthropicAPIURL     = "https://api.anthropic.com/v1/messages"
	anthropicFilesURL   = "https://api.anthropic.com/v1/files"
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
	return providers.RateLimitConfig{
		RequestsPerMinute: 10,
		TokensPerMinute:   100000,
		BurstSize:         2,
	}
}

// ModelName returns the configured model name.
func (p *AnthropicProvider) ModelName() string {
	return p.model
}

// Capabilities returns model-specific input limits and supported modalities.
func (p *AnthropicProvider) Capabilities() providers.SemanticCapabilities {
	caps := providers.SemanticCapabilities{
		MaxInputTokens:  200000,
		MaxRequestBytes: 32 * 1024 * 1024,
		MaxPDFPages:     100,
		MaxImages:       100,
		SupportsPDF:     true,
		SupportsImages:  true,
		Model:           p.model,
	}

	switch p.model {
	case "claude-sonnet-4-5-20250929", "claude-opus-4-5-20251101":
		caps.MaxInputTokens = 1000000
	case "claude-haiku-4-5-20251015":
		caps.MaxInputTokens = 200000
	}

	return caps
}

// Analyze performs semantic analysis on the given file-level input.
func (p *AnthropicProvider) Analyze(ctx context.Context, input providers.SemanticInput) (*providers.SemanticResult, error) {
	if !p.Available() {
		return nil, fmt.Errorf("anthropic provider not available; ANTHROPIC_API_KEY not set")
	}

	if err := p.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed; %w", err)
	}

	content, err := p.buildUserContent(ctx, input)
	if err != nil {
		return nil, err
	}

	requestBody := map[string]any{
		"model":      p.model,
		"max_tokens": 4096,
		"system":     buildSystemPrompt(),
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": content,
			},
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request; %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", anthropicAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request; %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

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

	var apiResp anthropicResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response; %w", err)
	}

	var textContent string
	for _, c := range apiResp.Content {
		if c.Type == "text" {
			textContent = c.Text
			break
		}
	}
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
	result.TokensUsed = apiResp.Usage.InputTokens + apiResp.Usage.OutputTokens
	result.Version = analysisVersion

	return result, nil
}

func (p *AnthropicProvider) buildUserContent(ctx context.Context, input providers.SemanticInput) ([]map[string]any, error) {
	switch input.Type {
	case providers.SemanticInputImage:
		if len(input.ImageBytes) == 0 {
			return nil, fmt.Errorf("image input missing bytes")
		}
		encoded := base64.StdEncoding.EncodeToString(input.ImageBytes)
		return []map[string]any{
			{
				"type": "image",
				"source": map[string]any{
					"type":       "base64",
					"media_type": input.MIMEType,
					"data":       encoded,
				},
			},
			{
				"type": "text",
				"text": fmt.Sprintf("Analyze this image from file: %s", input.Path),
			},
		}, nil
	case providers.SemanticInputPDF:
		if len(input.FileBytes) == 0 {
			return nil, fmt.Errorf("pdf input missing bytes")
		}
		fileID, err := p.uploadFile(ctx, input.FileBytes, input.Path, "application/pdf")
		if err != nil {
			return nil, err
		}
		return []map[string]any{
			{
				"type": "document",
				"source": map[string]any{
					"type":    "file",
					"file_id": fileID,
				},
			},
			{
				"type": "text",
				"text": fmt.Sprintf("Analyze this PDF from file: %s", input.Path),
			},
		}, nil
	default:
		context := buildTextContext(input)
		return []map[string]any{{"type": "text", "text": context}}, nil
	}
}

func (p *AnthropicProvider) uploadFile(ctx context.Context, data []byte, path string, mimeType string) (string, error) {
	filename := filepath.Base(path)
	if filename == "." || filename == string(filepath.Separator) || filename == "" {
		filename = "document.pdf"
	}

	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)
	fileWriter, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("failed to create form file; %w", err)
	}
	if _, err := fileWriter.Write(data); err != nil {
		return "", fmt.Errorf("failed to write file data; %w", err)
	}
	if err := writer.WriteField("purpose", "analysis"); err != nil {
		return "", fmt.Errorf("failed to set purpose; %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to finalize multipart; %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", anthropicFilesURL, buf)
	if err != nil {
		return "", fmt.Errorf("failed to create file request; %w", err)
	}

	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("file upload failed; %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read file response; %w", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("file upload error %d: %s", resp.StatusCode, string(body))
	}

	var fileResp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &fileResp); err != nil {
		return "", fmt.Errorf("failed to parse file upload response; %w", err)
	}
	if fileResp.ID == "" {
		return "", fmt.Errorf("file upload did not return file id")
	}

	return fileResp.ID, nil
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
