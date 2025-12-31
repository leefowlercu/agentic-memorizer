package gemini

import "github.com/leefowlercu/agentic-memorizer/internal/embeddings"

func init() {
	// Register Gemini provider in the global registry
	embeddings.GlobalRegistry().Register(ProviderName, NewProvider)
}
