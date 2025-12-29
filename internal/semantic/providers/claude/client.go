package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

const anthropicAPIURL = "https://api.anthropic.com/v1/messages"

type Client struct {
	apiKey     string
	model      string
	maxTokens  int
	timeout    time.Duration
	httpClient *http.Client
}

func NewClient(apiKey, model string, maxTokens, timeoutSeconds int) *Client {
	return &Client{
		apiKey:    apiKey,
		model:     model,
		maxTokens: maxTokens,
		timeout:   time.Duration(timeoutSeconds) * time.Second,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
	}
}

// doWithRetry performs an HTTP request with exponential backoff retry logic
func (c *Client) doWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	const maxRetries = 3
	const baseDelay = 1 * time.Second

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check context cancellation before each attempt
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled; %w", err)
		}

		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled during retry backoff; %w", ctx.Err())
			}
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Network error - retry
			lastErr = err
			continue
		}

		// Success cases
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}

		// Read body for error message
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Determine if we should retry based on status code
		shouldRetry := false
		switch resp.StatusCode {
		case http.StatusTooManyRequests, // 429 rate limit
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout:      // 504
			shouldRetry = true
		}

		if !shouldRetry || attempt == maxRetries {
			// Return error with status and body
			return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		}

		lastErr = fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

type Message struct {
	Role    string `json:"role"`
	Content []any  `json:"content"`
}

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ImageContent struct {
	Type   string      `json:"type"`
	Source ImageSource `json:"source"`
}

type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"` // base64 encoded
}

type DocumentContent struct {
	Type   string         `json:"type"`
	Source DocumentSource `json:"source"`
}

type DocumentSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"` // base64 encoded
}

type Request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []Message `json:"messages"`
}

type Response struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model string `json:"model"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (c *Client) SendMessage(ctx context.Context, prompt string) (string, error) {
	req := Request{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		Messages: []Message{
			{
				Role: "user",
				Content: []any{
					TextContent{
						Type: "text",
						Text: prompt,
					},
				},
			},
		},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request; %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", anthropicAPIURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request; %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	httpResp, err := c.doWithRetry(ctx, httpReq)
	if err != nil {
		return "", err
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response; %w", err)
	}

	var resp Response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response; %w", err)
	}

	if len(resp.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return resp.Content[0].Text, nil
}

func (c *Client) SendMessageWithDocument(ctx context.Context, prompt, documentBase64, mediaType string) (string, error) {
	req := Request{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		Messages: []Message{
			{
				Role: "user",
				Content: []any{
					DocumentContent{
						Type: "document",
						Source: DocumentSource{
							Type:      "base64",
							MediaType: mediaType,
							Data:      documentBase64,
						},
					},
					TextContent{
						Type: "text",
						Text: prompt,
					},
				},
			},
		},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request; %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", anthropicAPIURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request; %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	httpResp, err := c.doWithRetry(ctx, httpReq)
	if err != nil {
		return "", err
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response; %w", err)
	}

	var resp Response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response; %w", err)
	}

	if len(resp.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return resp.Content[0].Text, nil
}

func (c *Client) SendMessageWithImage(ctx context.Context, prompt, imageBase64, mediaType string) (string, error) {
	req := Request{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		Messages: []Message{
			{
				Role: "user",
				Content: []any{
					ImageContent{
						Type: "image",
						Source: ImageSource{
							Type:      "base64",
							MediaType: mediaType,
							Data:      imageBase64,
						},
					},
					TextContent{
						Type: "text",
						Text: prompt,
					},
				},
			},
		},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request; %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", anthropicAPIURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request; %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	httpResp, err := c.doWithRetry(ctx, httpReq)
	if err != nil {
		return "", err
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response; %w", err)
	}

	var resp Response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response; %w", err)
	}

	if len(resp.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return resp.Content[0].Text, nil
}
