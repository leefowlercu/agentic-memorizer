package gemini

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/leefowlercu/agentic-memorizer/internal/document"
	"github.com/leefowlercu/agentic-memorizer/internal/semantic"
	"github.com/leefowlercu/agentic-memorizer/internal/semantic/common"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
	"google.golang.org/api/option"
)

const Version = "1.0.0"

// GeminiProvider implements the Provider interface for Google Gemini API
type GeminiProvider struct {
	client       *genai.Client
	model        *genai.GenerativeModel
	modelName    string
	maxTokens    int
	enableVision bool
	maxFileSize  int64
	logger       *slog.Logger
}

// NewGeminiProvider creates a new Gemini provider instance
func NewGeminiProvider(config semantic.ProviderConfig, logger *slog.Logger) (semantic.Provider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required for Gemini provider")
	}

	if config.Model == "" {
		return nil, fmt.Errorf("model is required for Gemini provider")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(config.APIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client; %w", err)
	}

	model := client.GenerativeModel(config.Model)
	model.SetMaxOutputTokens(int32(config.MaxTokens))

	return &GeminiProvider{
		client:       client,
		model:        model,
		modelName:    config.Model,
		maxTokens:    config.MaxTokens,
		enableVision: config.EnableVision,
		maxFileSize:  config.MaxFileSize,
		logger:       logger,
	}, nil
}

// Name returns the provider identifier
func (p *GeminiProvider) Name() string {
	return "gemini"
}

// Model returns the model being used
func (p *GeminiProvider) Model() string {
	return p.modelName
}

// SupportsVision returns whether provider supports image analysis
func (p *GeminiProvider) SupportsVision() bool {
	// All Gemini 2.x and 3.x models support vision natively
	model := strings.ToLower(p.modelName)
	return strings.Contains(model, "gemini-2") ||
		strings.Contains(model, "gemini-3") ||
		strings.Contains(model, "gemini-pro-vision") ||
		strings.Contains(model, "gemini-1.5")
}

// SupportsDocuments returns whether provider supports PDF/document blocks
func (p *GeminiProvider) SupportsDocuments() bool {
	// Gemini supports native PDF analysis via multimodal content
	return true
}

// Analyze generates semantic understanding for a file
func (p *GeminiProvider) Analyze(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	if metadata.Size > p.maxFileSize {
		return nil, fmt.Errorf("file too large for analysis: %d bytes", metadata.Size)
	}

	if metadata.Category == "images" && p.enableVision && p.SupportsVision() {
		return p.analyzeImage(ctx, metadata)
	}

	if metadata.Type == "pdf" && p.SupportsDocuments() {
		return p.analyzeDocument(ctx, metadata)
	}

	if metadata.Type == "pptx" || metadata.Type == "docx" {
		// Use text extraction for Office formats
		return p.analyzeDocument(ctx, metadata)
	}

	return p.analyzeText(ctx, metadata)
}

func (p *GeminiProvider) analyzeText(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
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

	resp, err := p.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to analyze with Gemini; %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	response := extractTextFromParts(resp.Candidates[0].Content.Parts)
	return common.ParseAnalysisResponse(response)
}

func (p *GeminiProvider) analyzeImage(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	imageData, err := os.ReadFile(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read image; %w", err)
	}

	mimeType := common.GetMediaType(metadata.Type)
	prompt := common.BuildImagePrompt(metadata)

	imagePart := genai.ImageData(mimeType, imageData)
	textPart := genai.Text(prompt)

	resp, err := p.model.GenerateContent(ctx, imagePart, textPart)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze image with Gemini; %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	response := extractTextFromParts(resp.Candidates[0].Content.Parts)
	return common.ParseAnalysisResponse(response)
}

func (p *GeminiProvider) analyzeDocument(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	// For PDFs, Gemini supports native multimodal analysis
	if metadata.Type == "pdf" {
		return p.analyzePDF(ctx, metadata)
	}

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
		// Other document types - fall back to metadata-only
		return common.AnalyzeBinary(metadata), nil
	}

	resp, err := p.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to analyze with Gemini; %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	response := extractTextFromParts(resp.Candidates[0].Content.Parts)
	return common.ParseAnalysisResponse(response)
}

func (p *GeminiProvider) analyzePDF(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	pdfData, err := os.ReadFile(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF; %w", err)
	}

	prompt := common.BuildPdfPrompt(metadata)

	pdfPart := genai.Blob{
		MIMEType: "application/pdf",
		Data:     pdfData,
	}
	textPart := genai.Text(prompt)

	resp, err := p.model.GenerateContent(ctx, pdfPart, textPart)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze PDF with Gemini; %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	response := extractTextFromParts(resp.Candidates[0].Content.Parts)
	return common.ParseAnalysisResponse(response)
}

func extractTextFromParts(parts []genai.Part) string {
	var result strings.Builder
	for _, part := range parts {
		if text, ok := part.(genai.Text); ok {
			result.WriteString(string(text))
		}
	}
	return result.String()
}
