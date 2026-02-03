package providers

import (
	"context"
	"time"
)

// ProviderType represents the type of provider.
type ProviderType string

const (
	ProviderTypeSemantic   ProviderType = "semantic"
	ProviderTypeEmbeddings ProviderType = "embeddings"
)

// Provider is the base interface for all providers.
type Provider interface {
	// Name returns the provider's unique identifier.
	Name() string

	// Type returns the provider type.
	Type() ProviderType

	// Available returns true if the provider is configured and ready.
	Available() bool

	// RateLimit returns the rate limit configuration for this provider.
	RateLimit() RateLimitConfig
}

// RateLimitConfig defines rate limiting parameters for a provider.
type RateLimitConfig struct {
	RequestsPerMinute int
	TokensPerMinute   int
	BurstSize         int
}

// SemanticInputType represents the type of semantic input.
type SemanticInputType string

const (
	SemanticInputText  SemanticInputType = "text"
	SemanticInputPDF   SemanticInputType = "pdf"
	SemanticInputImage SemanticInputType = "image"
)

// SemanticInput represents a file-level semantic analysis input.
type SemanticInput struct {
	// Path is the file path being analyzed.
	Path string

	// MIMEType is the MIME type of the content.
	MIMEType string

	// Type indicates the input type (text/pdf/image).
	Type SemanticInputType

	// Text contains the text content to analyze (for text inputs).
	Text string

	// FileBytes contains raw file bytes (for PDF inputs).
	FileBytes []byte

	// ImageBytes contains raw image bytes (for vision inputs).
	ImageBytes []byte

	// TokenEstimate is the estimated token count for text inputs.
	TokenEstimate int

	// Truncated indicates whether the input was truncated or condensed.
	Truncated bool

	// Meta contains additional context about the file.
	Meta map[string]any
}

// SemanticCapabilities describes model-specific input limits and supported modalities.
type SemanticCapabilities struct {
	MaxInputTokens  int
	MaxRequestBytes int64
	MaxPDFPages     int
	MaxImages       int
	SupportsPDF     bool
	SupportsImages  bool
	Model           string
}

// SemanticProvider analyzes content and extracts semantic information.
type SemanticProvider interface {
	Provider

	// Analyze performs semantic analysis on the given file-level input.
	Analyze(ctx context.Context, input SemanticInput) (*SemanticResult, error)

	// Capabilities returns model-specific input limits and supported modalities.
	Capabilities() SemanticCapabilities

	// ModelName returns the model identifier used by this provider.
	ModelName() string
}

// SemanticResult contains the results of semantic analysis.
type SemanticResult struct {
	// Summary is a concise description of the content.
	Summary string `json:"summary"`

	// Tags are categorical labels for the content.
	Tags []string `json:"tags"`

	// Topics are subject areas covered by the content.
	Topics []Topic `json:"topics"`

	// Entities are named entities found in the content.
	Entities []Entity `json:"entities"`

	// References are external references found in the content.
	References []Reference `json:"references"`

	// Language is the programming language (for code) or natural language.
	Language string `json:"language,omitempty"`

	// Complexity is a subjective complexity score (1-10).
	Complexity int `json:"complexity,omitempty"`

	// Keywords are important terms from the content.
	Keywords []string `json:"keywords,omitempty"`

	// ProviderName is the name of the provider that generated this result.
	ProviderName string `json:"provider_name"`

	// ModelName is the specific model used.
	ModelName string `json:"model_name"`

	// AnalyzedAt is when the analysis was performed.
	AnalyzedAt time.Time `json:"analyzed_at"`

	// TokensUsed is the number of tokens consumed.
	TokensUsed int `json:"tokens_used"`

	// Version is the analysis version for cache invalidation.
	Version int `json:"version"`
}

// Topic represents a subject area covered by the content.
type Topic struct {
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
}

// Entity represents a named entity found in the content.
type Entity struct {
	Name string `json:"name"`
	Type string `json:"type"` // person, organization, location, concept, etc.
}

// Reference represents an external reference found in the content.
type Reference struct {
	Type   string `json:"type"`   // url, file, package, etc.
	Target string `json:"target"` // the actual reference value
}

// EmbeddingsProvider generates vector embeddings from content.
type EmbeddingsProvider interface {
	Provider

	// Embed generates embeddings for the given content.
	Embed(ctx context.Context, req EmbeddingsRequest) (*EmbeddingsResult, error)

	// EmbedBatch generates embeddings for multiple texts in a single API call.
	// This is more efficient than calling Embed multiple times for multi-chunk files.
	EmbedBatch(ctx context.Context, texts []string) ([]EmbeddingsBatchResult, error)

	// ModelName returns the name of the embedding model.
	ModelName() string

	// Dimensions returns the dimensionality of the embedding vectors.
	Dimensions() int

	// MaxTokens returns the maximum number of tokens per request.
	MaxTokens() int
}

// EmbeddingsRequest represents a request for embeddings generation.
type EmbeddingsRequest struct {
	// Content is the text to embed.
	Content string

	// ChunkID identifies this chunk for caching.
	ChunkID string

	// ContentHash is the hash of the content for cache lookup.
	ContentHash string
}

// EmbeddingsResult contains the results of embeddings generation.
type EmbeddingsResult struct {
	// Embedding is the vector representation.
	Embedding []float32 `json:"embedding"`

	// ProviderName is the name of the provider.
	ProviderName string `json:"provider_name"`

	// ModelName is the specific model used.
	ModelName string `json:"model_name"`

	// Dimensions is the dimensionality of the embedding.
	Dimensions int `json:"dimensions"`

	// TokensUsed is the number of tokens consumed.
	TokensUsed int `json:"tokens_used"`

	// GeneratedAt is when the embedding was generated.
	GeneratedAt time.Time `json:"generated_at"`

	// Version is the embedding version for cache invalidation.
	Version int `json:"version"`
}

// EmbeddingsBatchResult contains the result for a single item in a batch.
type EmbeddingsBatchResult struct {
	// Index is the position in the original input array.
	Index int `json:"index"`

	// Embedding is the vector representation.
	Embedding []float32 `json:"embedding"`

	// TokensUsed is the number of tokens consumed for this item.
	TokensUsed int `json:"tokens_used"`
}
