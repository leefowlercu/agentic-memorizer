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
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/providers"
)

const (
	openaiResponsesURL   = "https://api.openai.com/v1/responses"
	openaiFilesURL       = "https://api.openai.com/v1/files"
	openaiDefaultModel   = "gpt-5.2"
	openaiMaxOutputToken = 4096
)

// OpenAISemanticProvider implements SemanticProvider using OpenAI's API.
type OpenAISemanticProvider struct {
	apiKey          string
	model           string
	httpClient      *http.Client
	rateLimiter     *providers.RateLimiter
	rateLimitConfig *providers.RateLimitConfig
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

// WithOpenAIRateLimit sets a custom rate limit configuration.
func WithOpenAIRateLimit(requestsPerMinute int) OpenAISemanticOption {
	return func(p *OpenAISemanticProvider) {
		p.rateLimitConfig = &providers.RateLimitConfig{
			RequestsPerMinute: requestsPerMinute,
			TokensPerMinute:   150000,
			BurstSize:         max(1, requestsPerMinute/5),
		}
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
	if p.rateLimitConfig != nil {
		return *p.rateLimitConfig
	}
	return providers.RateLimitConfig{
		RequestsPerMinute: 60,
		TokensPerMinute:   150000,
		BurstSize:         10,
	}
}

// ModelName returns the configured model name.
func (p *OpenAISemanticProvider) ModelName() string {
	return p.model
}

// Capabilities returns model-specific input limits and supported modalities.
func (p *OpenAISemanticProvider) Capabilities() providers.SemanticCapabilities {
	caps := providers.SemanticCapabilities{
		MaxInputTokens:  128000,
		MaxRequestBytes: 50 * 1024 * 1024,
		MaxPDFPages:     1000,
		MaxImages:       500,
		SupportsPDF:     true,
		SupportsImages:  true,
		Model:           p.model,
	}

	switch p.model {
	case "gpt-5.2", "gpt-5.2-pro":
		caps.MaxInputTokens = 400000
	case "gpt-5-mini":
		caps.MaxInputTokens = 128000
	}

	return caps
}

// Analyze performs semantic analysis on the given file-level input.
func (p *OpenAISemanticProvider) Analyze(ctx context.Context, input providers.SemanticInput) (*providers.SemanticResult, error) {
	if !p.Available() {
		return nil, fmt.Errorf("openai provider not available; OPENAI_API_KEY not set")
	}

	if err := p.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed; %w", err)
	}

	systemPrompt := buildSystemPrompt()
	userContent, err := p.buildUserContent(ctx, input)
	if err != nil {
		return nil, err
	}

	requestBody := map[string]any{
		"model": p.model,
		"input": []map[string]any{
			{
				"role": "system",
				"content": []map[string]any{
					{"type": "input_text", "text": systemPrompt},
				},
			},
			{
				"role":    "user",
				"content": userContent,
			},
		},
		"max_output_tokens": openaiMaxOutputToken,
		"temperature":       0.1,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request; %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", openaiResponsesURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request; %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

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

	text, tokens, err := parseOpenAIResponse(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response; %w", err)
	}

	result, err := parseAnalysisResponse(text)
	if err != nil {
		return nil, fmt.Errorf("failed to parse analysis; %w", err)
	}

	result.ProviderName = p.Name()
	result.ModelName = p.model
	result.AnalyzedAt = time.Now()
	result.TokensUsed = tokens
	result.Version = analysisVersion

	return result, nil
}

func (p *OpenAISemanticProvider) buildUserContent(ctx context.Context, input providers.SemanticInput) ([]map[string]any, error) {
	switch input.Type {
	case providers.SemanticInputImage:
		if len(input.ImageBytes) == 0 {
			return nil, fmt.Errorf("image input missing bytes")
		}
		encoded := base64.StdEncoding.EncodeToString(input.ImageBytes)
		return []map[string]any{
			{
				"type":      "input_image",
				"image_url": fmt.Sprintf("data:%s;base64,%s", input.MIMEType, encoded),
			},
			{
				"type": "input_text",
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
				"type":    "input_file",
				"file_id": fileID,
			},
			{
				"type": "input_text",
				"text": fmt.Sprintf("Analyze this PDF from file: %s", input.Path),
			},
		}, nil
	default:
		context := buildTextContext(input)
		return []map[string]any{{"type": "input_text", "text": context}}, nil
	}
}

func buildTextContext(input providers.SemanticInput) string {
	context := fmt.Sprintf("File: %s\nMIME Type: %s\n", input.Path, input.MIMEType)
	if input.Truncated {
		context += "Content was truncated to fit model limits.\n"
	}
	context += "\nContent:\n" + input.Text
	return context
}

func (p *OpenAISemanticProvider) uploadFile(ctx context.Context, data []byte, path string, mimeType string) (string, error) {
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
	if err := writer.WriteField("purpose", "assistants"); err != nil {
		return "", fmt.Errorf("failed to set purpose; %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to finalize multipart; %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", openaiFilesURL, buf)
	if err != nil {
		return "", fmt.Errorf("failed to create file request; %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
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

func parseOpenAIResponse(body []byte) (string, int, error) {
	var resp struct {
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", 0, err
	}

	for _, item := range resp.Output {
		for _, content := range item.Content {
			if content.Text != "" {
				return content.Text, resp.Usage.TotalTokens, nil
			}
		}
	}

	return "", resp.Usage.TotalTokens, fmt.Errorf("no output text in response")
}
