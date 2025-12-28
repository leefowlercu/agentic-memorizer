package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/semantic"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
	openai "github.com/sashabaranov/go-openai"
)

const Version = "1.0.0"

// OpenAIProvider implements the Provider interface for OpenAI API
type OpenAIProvider struct {
	client       *openai.Client
	model        string
	maxTokens    int
	enableVision bool
	maxFileSize  int64
	logger       *slog.Logger
}

// NewOpenAIProvider creates a new OpenAI provider instance
func NewOpenAIProvider(config semantic.ProviderConfig, logger *slog.Logger) (semantic.Provider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required for OpenAI provider")
	}

	if config.Model == "" {
		return nil, fmt.Errorf("model is required for OpenAI provider")
	}

	client := openai.NewClient(config.APIKey)

	return &OpenAIProvider{
		client:       client,
		model:        config.Model,
		maxTokens:    config.MaxTokens,
		enableVision: config.EnableVision,
		maxFileSize:  config.MaxFileSize,
		logger:       logger,
	}, nil
}

// Name returns the provider identifier
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// Model returns the model being used
func (p *OpenAIProvider) Model() string {
	return p.model
}

// SupportsVision returns whether provider supports image analysis
func (p *OpenAIProvider) SupportsVision() bool {
	// GPT-4o and GPT-5.x models support vision
	model := strings.ToLower(p.model)
	return strings.Contains(model, "gpt-4o") ||
		strings.Contains(model, "gpt-5") ||
		strings.Contains(model, "gpt-4-vision")
}

// SupportsDocuments returns whether provider supports PDF/document blocks
func (p *OpenAIProvider) SupportsDocuments() bool {
	// OpenAI doesn't have native PDF support like Claude
	// We extract text and analyze as text
	return false
}

// Analyze generates semantic understanding for a file
func (p *OpenAIProvider) Analyze(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	if metadata.Size > p.maxFileSize {
		return nil, fmt.Errorf("file too large for analysis: %d bytes", metadata.Size)
	}

	if metadata.Category == "images" && p.enableVision && p.SupportsVision() {
		return p.analyzeImage(ctx, metadata)
	}

	if metadata.Type == "pptx" || metadata.Type == "docx" || metadata.Type == "pdf" {
		return p.analyzeDocument(ctx, metadata)
	}

	return p.analyzeText(ctx, metadata)
}

func (p *OpenAIProvider) analyzeText(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	content, err := os.ReadFile(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file; %w", err)
	}

	if !metadata.IsReadable {
		return p.analyzeBinary(metadata)
	}

	contentStr := string(content)
	if len(contentStr) > 100000 {
		contentStr = contentStr[:100000] + "\n\n[Content truncated...]"
	}

	prompt := p.buildPrompt(metadata, contentStr)

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:     p.model,
		MaxTokens: p.maxTokens,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to analyze with OpenAI; %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	response := resp.Choices[0].Message.Content
	return p.parseResponse(response)
}

func (p *OpenAIProvider) analyzeImage(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	imageData, err := os.ReadFile(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read image; %w", err)
	}

	imageBase64 := base64.StdEncoding.EncodeToString(imageData)
	mediaType := getMediaType(metadata.Type)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mediaType, imageBase64)

	prompt := fmt.Sprintf(`Analyze this image and provide semantic understanding.

File: %s
Type: %s
Dimensions: %dx%d

Provide a JSON response with:
1. summary: 2-3 sentence description of what the image shows
2. tags: 3-5 semantic tags (lowercase, hyphenated)
3. key_topics: 3-5 main subjects or themes in the image
4. document_type: The type/purpose of this image (e.g., "diagram", "screenshot", "photo", "chart")
5. entities: Array of named entities visible or referenced in the image, each with:
   - name: The entity name (e.g., brand names, people, tools shown)
   - type: One of: technology, person, concept, organization, project
6. references: Array of topic references if applicable, each with:
   - topic: The referenced topic or concept
   - type: One of: requires, extends, related-to, implements
   - confidence: Float 0.0-1.0 indicating strength of reference

Be concise but informative. Respond with ONLY valid JSON.`,
		filepath.Base(metadata.Path),
		metadata.Type,
		metadata.Dimensions.Width,
		metadata.Dimensions.Height,
	)

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:     p.model,
		MaxTokens: p.maxTokens,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL:    dataURL,
							Detail: openai.ImageURLDetailAuto,
						},
					},
					{
						Type: openai.ChatMessagePartTypeText,
						Text: prompt,
					},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to analyze image with OpenAI; %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	response := resp.Choices[0].Message.Content
	return p.parseResponse(response)
}

func (p *OpenAIProvider) analyzeDocument(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	// OpenAI doesn't support native PDF/document analysis
	// For now, return metadata-only analysis for documents
	// TODO: Add text extraction for PPTX/DOCX/PDF similar to Claude provider
	return p.analyzeBinary(metadata)
}

func (p *OpenAIProvider) analyzeBinary(metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	summary := fmt.Sprintf("%s file", strings.ToUpper(strings.TrimPrefix(metadata.Type, ".")))

	if metadata.PageCount != nil {
		summary += fmt.Sprintf(" with %d pages", *metadata.PageCount)
	} else if metadata.SlideCount != nil {
		summary += fmt.Sprintf(" with %d slides", *metadata.SlideCount)
	}

	return &types.SemanticAnalysis{
		Summary:      summary,
		Tags:         []string{metadata.Category, metadata.Type},
		KeyTopics:    []string{},
		DocumentType: metadata.Category,
		Confidence:   0.5,
		Entities:     []types.Entity{},
		References:   []types.Reference{},
	}, nil
}

func (p *OpenAIProvider) buildPrompt(metadata *types.FileMetadata, content string) string {
	return fmt.Sprintf(`Analyze this file and provide semantic understanding.

File: %s
Type: %s
Category: %s
Size: %d bytes

Content:
%s

Provide a JSON response with:
1. summary: 2-3 sentence summary capturing the main purpose/content
2. tags: 3-5 semantic tags (lowercase, hyphenated)
3. key_topics: 3-5 main topics or themes
4. document_type: The purpose/genre of this document
5. entities: Array of named entities found in the content, each with:
   - name: The entity name (e.g., "Terraform", "AWS", "Docker")
   - type: One of: technology, person, concept, organization, project
6. references: Array of topic references/dependencies, each with:
   - topic: The referenced topic or concept
   - type: One of: requires, extends, related-to, implements
   - confidence: Float 0.0-1.0 indicating strength of reference

Be concise but informative. Respond with ONLY valid JSON.`,
		filepath.Base(metadata.Path),
		metadata.Type,
		metadata.Category,
		metadata.Size,
		content,
	)
}

func (p *OpenAIProvider) parseResponse(response string) (*types.SemanticAnalysis, error) {
	var analysis types.SemanticAnalysis
	if err := json.Unmarshal([]byte(response), &analysis); err != nil {
		if jsonStr := extractJSON(response); jsonStr != "" {
			if err := json.Unmarshal([]byte(jsonStr), &analysis); err == nil {
				return &analysis, nil
			}
		}
		return nil, fmt.Errorf("failed to parse analysis response; %w", err)
	}
	return &analysis, nil
}

func extractJSON(text string) string {
	if start := strings.Index(text, "```json"); start != -1 {
		start += 7
		if end := strings.Index(text[start:], "```"); end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	if start := strings.Index(text, "```"); start != -1 {
		start += 3
		if end := strings.Index(text[start:], "```"); end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	return ""
}

func getMediaType(fileType string) string {
	switch strings.ToLower(fileType) {
	case ".png", "png":
		return "image/png"
	case ".jpg", ".jpeg", "jpg", "jpeg":
		return "image/jpeg"
	case ".gif", "gif":
		return "image/gif"
	case ".webp", "webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}
