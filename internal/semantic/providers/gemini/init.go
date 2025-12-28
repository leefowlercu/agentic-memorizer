package gemini

import (
	"github.com/leefowlercu/agentic-memorizer/internal/semantic"
)

func init() {
	semantic.GlobalRegistry().Register("gemini", NewGeminiProvider)
}
