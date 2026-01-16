package languages

import (
	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/chunkers/code"
)

func init() {
	// Register the factory so DefaultRegistry can use TreeSitter chunker
	chunkers.RegisterTreeSitterFactory(func() chunkers.Chunker {
		return NewDefaultChunker()
	})
}

// NewDefaultChunker creates a TreeSitterChunker with all supported languages registered.
// This is the recommended way to create a production TreeSitterChunker.
func NewDefaultChunker() *code.TreeSitterChunker {
	c := code.NewTreeSitterChunker()

	// Register all language strategies
	c.RegisterStrategy(NewGoStrategy())
	c.RegisterStrategy(NewPythonStrategy())
	c.RegisterStrategy(NewJavaScriptStrategy())
	c.RegisterStrategy(NewTypeScriptStrategy())
	c.RegisterStrategy(NewJavaStrategy())
	c.RegisterStrategy(NewRustStrategy())
	c.RegisterStrategy(NewCStrategy())
	c.RegisterStrategy(NewCPPStrategy())

	return c
}
