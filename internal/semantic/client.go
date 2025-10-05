package semantic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const anthropicAPIURL = "https://api.anthropic.com/v1/messages"

// Client handles communication with Claude API
type Client struct {
	apiKey     string
	model      string
	maxTokens  int
	timeout    time.Duration
	httpClient *http.Client
}

// NewClient creates a new Claude API client
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

// Message represents a Claude API message
type Message struct {
	Role    string        `json:"role"`
	Content []interface{} `json:"content"`
}

// TextContent represents text content in a message
type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ImageContent represents image content in a message
type ImageContent struct {
	Type   string      `json:"type"`
	Source ImageSource `json:"source"`
}

// ImageSource represents an image source
type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"` // base64 encoded
}

// DocumentContent represents document content in a message
type DocumentContent struct {
	Type   string         `json:"type"`
	Source DocumentSource `json:"source"`
}

// DocumentSource represents a document source
type DocumentSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"` // base64 encoded
}

// Request represents a Claude API request
type Request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []Message `json:"messages"`
}

// Response represents a Claude API response
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

// SendMessage sends a message to Claude API
func (c *Client) SendMessage(prompt string) (string, error) {
	// Create request
	req := Request{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		Messages: []Message{
			{
				Role: "user",
				Content: []interface{}{
					TextContent{
						Type: "text",
						Text: prompt,
					},
				},
			},
		},
	}

	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request; %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", anthropicAPIURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request; %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// Send request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request; %w", err)
	}
	defer httpResp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response; %w", err)
	}

	// Check status code
	if httpResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", httpResp.StatusCode, string(respBody))
	}

	// Parse response
	var resp Response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response; %w", err)
	}

	// Extract text from response
	if len(resp.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return resp.Content[0].Text, nil
}

// SendMessageWithDocument sends a message with a document to Claude API
func (c *Client) SendMessageWithDocument(prompt, documentBase64, mediaType string) (string, error) {
	// Create request with document
	req := Request{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		Messages: []Message{
			{
				Role: "user",
				Content: []interface{}{
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

	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request; %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", anthropicAPIURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request; %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// Send request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request; %w", err)
	}
	defer httpResp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response; %w", err)
	}

	// Check status code
	if httpResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", httpResp.StatusCode, string(respBody))
	}

	// Parse response
	var resp Response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response; %w", err)
	}

	// Extract text from response
	if len(resp.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return resp.Content[0].Text, nil
}

// SendMessageWithImage sends a message with an image to Claude API
func (c *Client) SendMessageWithImage(prompt, imageBase64, mediaType string) (string, error) {
	// Create request with image
	req := Request{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		Messages: []Message{
			{
				Role: "user",
				Content: []interface{}{
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

	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request; %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", anthropicAPIURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request; %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// Send request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request; %w", err)
	}
	defer httpResp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response; %w", err)
	}

	// Check status code
	if httpResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", httpResp.StatusCode, string(respBody))
	}

	// Parse response
	var resp Response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response; %w", err)
	}

	// Extract text from response
	if len(resp.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return resp.Content[0].Text, nil
}
