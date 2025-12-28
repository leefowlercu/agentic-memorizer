package claude

import (
	"github.com/leefowlercu/agentic-memorizer/internal/semantic"
)

func init() {
	// Register Claude provider in the global registry
	semantic.GlobalRegistry().Register("claude", NewClaudeProvider)
}
