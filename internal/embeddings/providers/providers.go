// Package providers imports all embedding providers to trigger their registration.
// Import this package using a blank import to enable all providers:
//
//	import _ "github.com/leefowlercu/agentic-memorizer/internal/embeddings/providers"
package providers

import (
	// Import providers to trigger init() registration
	_ "github.com/leefowlercu/agentic-memorizer/internal/embeddings/providers/gemini"
	_ "github.com/leefowlercu/agentic-memorizer/internal/embeddings/providers/openai"
	_ "github.com/leefowlercu/agentic-memorizer/internal/embeddings/providers/voyage"
)
