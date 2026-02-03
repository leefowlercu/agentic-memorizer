package analysis

import (
	"errors"
	"fmt"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/ingest"
	"github.com/leefowlercu/agentic-memorizer/internal/providers"
)

const defaultReservedOutputTokens = 4096

// BuildSemanticInput constructs a provider-ready semantic input from the file and chunk context.
func BuildSemanticInput(path string, fileResult *FileReadResult, chunkResult *chunkers.ChunkResult, provider providers.SemanticProvider) (providers.SemanticInput, error) {
	if fileResult == nil {
		return providers.SemanticInput{}, errors.New("file result is required")
	}

	caps := defaultSemanticCapabilities(provider)

	input := providers.SemanticInput{
		Path:     path,
		MIMEType: fileResult.MIMEType,
		Meta:     map[string]any{},
	}

	switch fileResult.Kind {
	case ingest.KindImage:
		if !caps.SupportsImages {
			return providers.SemanticInput{}, fmt.Errorf("provider does not support image inputs")
		}
		input.Type = providers.SemanticInputImage
		input.ImageBytes = fileResult.Content
		return input, nil
	case ingest.KindDocument:
		if strings.EqualFold(fileResult.MIMEType, "application/pdf") {
			return buildPDFSemanticInput(input, fileResult, chunkResult, caps)
		}
	}

	return buildTextSemanticInput(input, fileResult, chunkResult, caps)
}

func buildPDFSemanticInput(input providers.SemanticInput, fileResult *FileReadResult, chunkResult *chunkers.ChunkResult, caps providers.SemanticCapabilities) (providers.SemanticInput, error) {
	pageCount := extractPDFPageCount(chunkResult)
	input.Meta["page_count"] = pageCount

	withinBytes := caps.MaxRequestBytes <= 0 || int64(len(fileResult.Content)) <= caps.MaxRequestBytes
	withinPages := caps.MaxPDFPages <= 0 || pageCount == 0 || pageCount <= caps.MaxPDFPages

	if caps.SupportsPDF && withinBytes && withinPages {
		input.Type = providers.SemanticInputPDF
		input.FileBytes = fileResult.Content
		return input, nil
	}

	// Fall back to text extraction using chunked content.
	return buildTextSemanticInput(input, fileResult, chunkResult, caps)
}

func buildTextSemanticInput(input providers.SemanticInput, fileResult *FileReadResult, chunkResult *chunkers.ChunkResult, caps providers.SemanticCapabilities) (providers.SemanticInput, error) {
	input.Type = providers.SemanticInputText
	text := string(fileResult.Content)
	if chunkResult != nil && len(chunkResult.Chunks) > 0 {
		text = joinChunkText(chunkResult.Chunks)
	}

	input.TokenEstimate = chunkers.EstimateTokens(text)
	budget := caps.MaxInputTokens - defaultReservedOutputTokens
	if caps.MaxInputTokens <= 0 {
		budget = input.TokenEstimate
	}
	if budget <= 0 {
		budget = caps.MaxInputTokens
	}

	condensed, truncated := condenseTextToBudget(text, budget)
	input.Text = condensed
	input.Truncated = truncated
	input.Meta["token_estimate"] = input.TokenEstimate
	input.Meta["token_budget"] = budget

	return input, nil
}

func extractPDFPageCount(chunkResult *chunkers.ChunkResult) int {
	if chunkResult == nil {
		return 0
	}
	for _, chunk := range chunkResult.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.PageCount > 0 {
			return chunk.Metadata.Document.PageCount
		}
	}
	return 0
}

func joinChunkText(chunks []chunkers.Chunk) string {
	var b strings.Builder
	for i, chunk := range chunks {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(chunk.Content)
	}
	return b.String()
}

func condenseTextToBudget(text string, maxTokens int) (string, bool) {
	if maxTokens <= 0 {
		return text, false
	}

	maxChars := maxTokens * 4
	if len(text) <= maxChars {
		return text, false
	}

	headChars := int(float64(maxChars) * 0.7)
	if headChars < 0 {
		headChars = 0
	}
	tailChars := maxChars - headChars
	if tailChars < 0 {
		tailChars = 0
	}

	if tailChars > len(text) {
		tailChars = len(text)
	}

	truncated := text[:headChars] + "\n\n[...truncated...]\n\n" + text[len(text)-tailChars:]
	return truncated, true
}

func defaultSemanticCapabilities(provider providers.SemanticProvider) providers.SemanticCapabilities {
	if provider != nil {
		caps := provider.Capabilities()
		if caps.MaxInputTokens > 0 || caps.MaxRequestBytes > 0 {
			return caps
		}
	}

	return providers.SemanticCapabilities{
		MaxInputTokens:  100000,
		MaxRequestBytes: 32 * 1024 * 1024,
		MaxPDFPages:     100,
		MaxImages:       50,
		SupportsPDF:     false,
		SupportsImages:  false,
	}
}
