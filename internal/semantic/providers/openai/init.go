package openai

import (
	"github.com/leefowlercu/agentic-memorizer/internal/semantic"
)

func init() {
	semantic.GlobalRegistry().Register("openai", NewOpenAIProvider)
}
