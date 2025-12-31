package openai

import "github.com/leefowlercu/agentic-memorizer/internal/embeddings"

func init() {
	// Register OpenAI provider in the global registry
	embeddings.GlobalRegistry().Register(ProviderName, NewProvider)
}
