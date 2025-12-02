package embeddings

import "context"

// Provider defines the interface for embedding generation
type Provider interface {
	// Embed generates an embedding vector for a single text
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embedding vectors for multiple texts
	// This is more efficient than calling Embed multiple times
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimensions returns the number of dimensions in the embedding vector
	Dimensions() int

	// Model returns the model name being used
	Model() string
}

// EmbeddingResult contains the result of an embedding operation
type EmbeddingResult struct {
	Text      string    `json:"text"`
	Embedding []float32 `json:"embedding"`
	Model     string    `json:"model"`
}
