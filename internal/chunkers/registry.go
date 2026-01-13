package chunkers

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

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

// Chunk uses the best available chunker for the content.
func (r *Registry) Chunk(ctx context.Context, content []byte, opts ChunkOptions) (*ChunkResult, error) {
	chunker := r.Get(opts.MIMEType, opts.Language)
	if chunker == nil {
		return nil, fmt.Errorf("no chunker available for mime=%s lang=%s", opts.MIMEType, opts.Language)
	}

	chunks, err := chunker.Chunk(ctx, content, opts)
	if err != nil {
		return nil, fmt.Errorf("chunking failed; %w", err)
	}

	return &ChunkResult{
		Chunks:       chunks,
		TotalChunks:  len(chunks),
		ChunkerUsed:  chunker.Name(),
		OriginalSize: len(content),
	}, nil
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
	r.Register(NewASTChunker())        // Priority 100: Code with AST parsing
	r.Register(NewMarkdownChunker())   // Priority 50: Markdown documents
	r.Register(NewStructuredChunker()) // Priority 40: JSON/YAML/CSV
	r.Register(NewRecursiveChunker())  // Priority 10: Plain text

	// Set fallback
	r.SetFallback(NewFallbackChunker())

	return r
}
