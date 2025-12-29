package claude

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"

	"github.com/leefowlercu/agentic-memorizer/internal/document"
	"github.com/leefowlercu/agentic-memorizer/internal/semantic"
	"github.com/leefowlercu/agentic-memorizer/internal/semantic/common"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

const Version = "1.0.0"

// ClaudeProvider implements the Provider interface for Claude API
type ClaudeProvider struct {
	client       *Client
	model        string
	enableVision bool
	maxFileSize  int64
	logger       *slog.Logger
}

// NewClaudeProvider creates a new Claude provider instance
func NewClaudeProvider(config semantic.ProviderConfig, logger *slog.Logger) (semantic.Provider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required for Claude provider")
	}

	if config.Model == "" {
		return nil, fmt.Errorf("model is required for Claude provider")
	}

	client := NewClient(config.APIKey, config.Model, config.MaxTokens, config.Timeout)

	return &ClaudeProvider{
		client:       client,
		model:        config.Model,
		enableVision: config.EnableVision,
		maxFileSize:  config.MaxFileSize,
		logger:       logger,
	}, nil
}

// Name returns the provider identifier
func (p *ClaudeProvider) Name() string {
	return "claude"
}

// Model returns the model being used
func (p *ClaudeProvider) Model() string {
	return p.model
}

// SupportsVision returns whether provider supports image analysis
func (p *ClaudeProvider) SupportsVision() bool {
	return true // Claude supports vision API
}

// SupportsDocuments returns whether provider supports PDF/document blocks
func (p *ClaudeProvider) SupportsDocuments() bool {
	return true // Claude supports document content blocks for PDFs
}

// Analyze generates semantic understanding for a file
// Content routing strategy:
// - Images -> analyzeImage (vision API with base64 encoding)
// - Office docs (pptx/docx) -> analyzeDocument (text extraction + analysis)
// - PDFs -> analyzeDocument (document content blocks API)
// - Everything else -> analyzeText (standard text analysis)
// Files exceeding maxFileSize are rejected before routing.
func (p *ClaudeProvider) Analyze(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	if metadata.Size > p.maxFileSize {
		return nil, fmt.Errorf("file too large for analysis: %d bytes", metadata.Size)
	}

	if metadata.Category == "images" && p.enableVision {
		return p.analyzeImage(ctx, metadata)
	}

	if metadata.Type == "pptx" || metadata.Type == "docx" {
		return p.analyzeDocument(ctx, metadata)
	}

	return p.analyzeText(ctx, metadata)
}

func (p *ClaudeProvider) analyzeText(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	content, err := os.ReadFile(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file; %w", err)
	}

	if !metadata.IsReadable {
		return common.AnalyzeBinary(metadata), nil
	}

	contentStr := string(content)
	if len(contentStr) > 100000 { // 100,000 byte limit for analysis
		contentStr = contentStr[:100000] + "\n\n[Content truncated...]"
	}

	prompt := common.BuildTextPrompt(metadata, contentStr)

	response, err := p.client.SendMessage(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze with Claude; %w", err)
	}

	return common.ParseAnalysisResponse(response)
}

func (p *ClaudeProvider) analyzeImage(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	imageData, err := os.ReadFile(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read image; %w", err)
	}

	imageBase64 := base64.StdEncoding.EncodeToString(imageData)
	mediaType := common.GetMediaType(metadata.Type)
	prompt := common.BuildImagePrompt(metadata)

	response, err := p.client.SendMessageWithImage(ctx, prompt, imageBase64, mediaType)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze image with Claude; %w", err)
	}

	return common.ParseAnalysisResponse(response)
}

func (p *ClaudeProvider) analyzeDocument(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	// For PPTX files, extract text and analyze as text
	// (Claude API only supports PDF for document content blocks)
	if metadata.Type == "pptx" {
		return p.analyzePptx(ctx, metadata)
	}

	// For DOCX files, extract text and analyze as text
	if metadata.Type == "docx" {
		return p.analyzeDocx(ctx, metadata)
	}

	// For PDF files, send as document
	if metadata.Type == "pdf" {
		documentData, err := os.ReadFile(metadata.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to read document; %w", err)
		}

		documentBase64 := base64.StdEncoding.EncodeToString(documentData)
		prompt := common.BuildPdfPrompt(metadata)

		response, err := p.client.SendMessageWithDocument(ctx, prompt, documentBase64, "application/pdf")
		if err != nil {
			return nil, fmt.Errorf("failed to analyze document with Claude; %w", err)
		}

		return common.ParseAnalysisResponse(response)
	}

	// For other document types, fall back to generic analysis
	return common.AnalyzeBinary(metadata), nil
}

func (p *ClaudeProvider) analyzePptx(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	extractedText, err := document.ExtractPptxText(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to extract PPTX text; %w", err)
	}

	if extractedText == "" {
		// No text extracted, fall back to metadata-only analysis
		return common.AnalyzeBinary(metadata), nil
	}

	// Truncate if too long
	if len(extractedText) > 50000 {
		extractedText = extractedText[:50000] + "\n\n[Content truncated...]"
	}

	prompt := common.BuildPptxPrompt(metadata, extractedText)

	response, err := p.client.SendMessage(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze with Claude; %w", err)
	}

	return common.ParseAnalysisResponse(response)
}

func (p *ClaudeProvider) analyzeDocx(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	extractedText, err := document.ExtractDocxText(metadata.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to extract DOCX text; %w", err)
	}

	if extractedText == "" {
		// No text extracted, fall back to metadata-only analysis
		return common.AnalyzeBinary(metadata), nil
	}

	// Truncate if too long
	if len(extractedText) > 50000 {
		extractedText = extractedText[:50000] + "\n\n[Content truncated...]"
	}

	prompt := common.BuildDocxPrompt(metadata, extractedText)

	response, err := p.client.SendMessage(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze with Claude; %w", err)
	}

	return common.ParseAnalysisResponse(response)
}
