package format

import (
	"fmt"
	"sync"
)

// Formatter renders Buildable structures to specific output formats
type Formatter interface {
	// Format renders a single buildable structure
	Format(b Buildable) (string, error)

	// FormatMultiple renders multiple buildable structures
	FormatMultiple(builders []Buildable) (string, error)

	// Name returns the formatter name (e.g., "text", "json")
	Name() string

	// SupportsColors returns true if the formatter supports color output
	SupportsColors() bool
}

// Registry manages registered formatters
type Registry struct {
	mu         sync.RWMutex
	formatters map[string]Formatter
}

// NewRegistry creates a new formatter registry
func NewRegistry() *Registry {
	return &Registry{
		formatters: make(map[string]Formatter),
	}
}

// Register adds a formatter to the registry
func (r *Registry) Register(name string, formatter Formatter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.formatters[name] = formatter
}

// Get retrieves a formatter by name
func (r *Registry) Get(name string) (Formatter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	formatter, ok := r.formatters[name]
	if !ok {
		return nil, fmt.Errorf("formatter %q not found", name)
	}
	return formatter, nil
}

// List returns all registered formatter names
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.formatters))
	for name := range r.formatters {
		names = append(names, name)
	}
	return names
}

// defaultRegistry is the global formatter registry
var defaultRegistry = NewRegistry()

// RegisterFormatter adds a formatter to the default registry
func RegisterFormatter(name string, formatter Formatter) {
	defaultRegistry.Register(name, formatter)
}

// GetFormatter retrieves a formatter from the default registry
func GetFormatter(name string) (Formatter, error) {
	return defaultRegistry.Get(name)
}

// ListFormatters returns all registered formatter names
func ListFormatters() []string {
	return defaultRegistry.List()
}
