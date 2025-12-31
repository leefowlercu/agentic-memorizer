package voyage

import "github.com/leefowlercu/agentic-memorizer/internal/embeddings"

func init() {
	// Register Voyage AI provider in the global registry
	embeddings.GlobalRegistry().Register(ProviderName, NewProvider)
}
