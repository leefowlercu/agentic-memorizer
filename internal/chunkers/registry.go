package chunkers

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// TreeSitterFactory is a function type for creating TreeSitter chunkers.
// This is set by the treesitter/languages package at init time to avoid circular imports.
type TreeSitterFactory func() Chunker

var treeSitterFactory TreeSitterFactory

// RegisterTreeSitterFactory registers the factory function for creating TreeSitter chunkers.
// This should be called from the treesitter/languages package's init() function.
func RegisterTreeSitterFactory(factory TreeSitterFactory) {
	treeSitterFactory = factory
}

// Registry manages available chunkers and selects the best one for content.
type Registry struct {
	mu       sync.RWMutex
	chunkers []Chunker
	fallback Chunker
}

// NewRegistry creates a new chunker registry.
func NewRegistry() *Registry {
	return &Registry{
		chunkers: make([]Chunker, 0),
	}
}

// Register adds a chunker to the registry.
func (r *Registry) Register(c Chunker) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.chunkers = append(r.chunkers, c)

	// Keep sorted by priority (highest first)
	sort.Slice(r.chunkers, func(i, j int) bool {
		return r.chunkers[i].Priority() > r.chunkers[j].Priority()
	})
}

// SetFallback sets the fallback chunker used when no other chunker matches.
func (r *Registry) SetFallback(c Chunker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fallback = c
}

// Get returns the best chunker for the given content type.
func (r *Registry) Get(mimeType string, language string) Chunker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, c := range r.chunkers {
		if c.CanHandle(mimeType, language) {
			return c
		}
	}

	return r.fallback
}

// Chunk uses the best available chunker for the content with graceful degradation.
// If the primary chunker fails, it tries the next chunker in priority order.
// Warnings from failed attempts are aggregated into the final result.
func (r *Registry) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var aggregatedWarnings []ChunkWarning
	var lastErr error

	// Try each chunker that can handle this content type, in priority order
	for _, chunker := range r.chunkers {
		if !chunker.CanHandle(opts.MIMEType, opts.Language) {
			continue
		}

		result, err := chunker.Chunk(ctx, content, opts)
		if err != nil {
			// Record warning about failed chunker and try next
			aggregatedWarnings = append(aggregatedWarnings, ChunkWarning{
				Offset:  0,
				Message: fmt.Sprintf("chunker %q failed: %v", chunker.Name(), err),
				Code:    "CHUNKER_FAILED",
			})
			lastErr = err
			continue
		}

		// Success - merge warnings and return
		if len(aggregatedWarnings) > 0 {
			result.Warnings = append(aggregatedWarnings, result.Warnings...)
		}
		return result, nil
	}

	// All specialized chunkers failed or none matched - try fallback
	if r.fallback != nil {
		result, err := r.fallback.Chunk(ctx, content, opts)
		if err != nil {
			aggregatedWarnings = append(aggregatedWarnings, ChunkWarning{
				Offset:  0,
				Message: fmt.Sprintf("fallback chunker failed: %v", err),
				Code:    "FALLBACK_FAILED",
			})
			return nil, fmt.Errorf("all chunkers failed; last error: %w", err)
		}

		// Success with fallback - merge warnings
		if len(aggregatedWarnings) > 0 {
			result.Warnings = append(aggregatedWarnings, result.Warnings...)
		}
		return result, nil
	}

	// No chunker available
	if lastErr != nil {
		return nil, fmt.Errorf("all chunkers failed; last error: %w", lastErr)
	}
	return nil, fmt.Errorf("no chunker available for mime=%s lang=%s", opts.MIMEType, opts.Language)
}

// List returns all registered chunkers.
func (r *Registry) List() []Chunker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Chunker, len(r.chunkers))
	copy(result, r.chunkers)
	return result
}

// DefaultRegistry creates a registry with all standard chunkers registered.
func DefaultRegistry() *Registry {
	r := NewRegistry()

	// Register chunkers (order doesn't matter due to priority-based sorting)
	// TreeSitter chunker handles code files via tree-sitter AST parsing
	if treeSitterFactory != nil {
		r.Register(treeSitterFactory()) // Priority 100: Code with tree-sitter AST parsing
	}

	// Document chunkers
	r.Register(NewNotebookChunker()) // Priority 76: Jupyter notebooks
	r.Register(NewHTMLChunker())     // Priority 75: HTML documents
	r.Register(NewPDFChunker())      // Priority 73: PDF documents
	r.Register(NewDOCXChunker())     // Priority 72: DOCX documents
	r.Register(NewODTChunker())      // Priority 71: ODT documents

	// Text format chunkers
	r.Register(NewRSTChunker())      // Priority 55: reStructuredText
	r.Register(NewAsciiDocChunker()) // Priority 54: AsciiDoc
	r.Register(NewLaTeXChunker())    // Priority 53: LaTeX

	// Other chunkers
	r.Register(NewMarkdownChunker())   // Priority 50: Markdown documents

	// DevOps chunkers
	r.Register(NewDockerfileChunker()) // Priority 45: Dockerfiles
	r.Register(NewMakefileChunker())   // Priority 44: Makefiles
	r.Register(NewHCLChunker())        // Priority 43: HCL/Terraform
	r.Register(NewProtobufChunker())   // Priority 42: Protocol Buffers
	r.Register(NewGraphQLChunker())    // Priority 41: GraphQL schemas

	r.Register(NewStructuredChunker()) // Priority 40: JSON/YAML/CSV

	// Data format chunkers
	r.Register(NewSQLChunker())  // Priority 32: SQL files
	r.Register(NewTOMLChunker()) // Priority 31: TOML configuration
	r.Register(NewXMLChunker())  // Priority 25: XML documents
	r.Register(NewLogChunker())  // Priority 25: Log files

	r.Register(NewRecursiveChunker()) // Priority 10: Plain text

	// Set fallback
	r.SetFallback(NewFallbackChunker())

	return r
}
