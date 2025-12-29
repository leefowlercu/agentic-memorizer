package openai

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/document"
	"github.com/leefowlercu/agentic-memorizer/internal/semantic"
	"github.com/leefowlercu/agentic-memorizer/internal/semantic/common"
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
		return common.AnalyzeBinary(metadata), nil
	}

	contentStr := string(content)
	if len(contentStr) > 100000 {
		contentStr = contentStr[:100000] + "\n\n[Content truncated...]"
	}

	prompt := common.BuildTextPrompt(metadata, contentStr)

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
	return common.ParseAnalysisResponse(response)
}

func (p *OpenAIProvider) analyzeImage(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	imageData, err := os.ReadFile(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read image; %w", err)
	}

	imageBase64 := base64.StdEncoding.EncodeToString(imageData)
	mediaType := common.GetMediaType(metadata.Type)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mediaType, imageBase64)
	prompt := common.BuildImagePrompt(metadata)

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
	return common.ParseAnalysisResponse(response)
}

func (p *OpenAIProvider) analyzeDocument(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	var extractedText string
	var err error
	var prompt string

	switch metadata.Type {
	case "pptx":
		extractedText, err = document.ExtractPptxText(metadata.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to extract PPTX text; %w", err)
		}
		if extractedText == "" {
			return common.AnalyzeBinary(metadata), nil
		}
		if len(extractedText) > 50000 {
			extractedText = extractedText[:50000] + "\n\n[Content truncated...]"
		}
		prompt = common.BuildPptxPrompt(metadata, extractedText)

	case "docx":
		extractedText, err = document.ExtractDocxText(metadata.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to extract DOCX text; %w", err)
		}
		if extractedText == "" {
			return common.AnalyzeBinary(metadata), nil
		}
		if len(extractedText) > 50000 {
			extractedText = extractedText[:50000] + "\n\n[Content truncated...]"
		}
		prompt = common.BuildDocxPrompt(metadata, extractedText)

	default:
		// PDFs and other document types - fall back to metadata-only
		return common.AnalyzeBinary(metadata), nil
	}

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
	return common.ParseAnalysisResponse(response)
}
