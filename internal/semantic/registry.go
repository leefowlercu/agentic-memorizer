package semantic

import (
	"fmt"
	"log/slog"
	"sync"
)

// ProviderFactory creates a Provider instance from configuration.
// Each provider registers its factory function during initialization.
type ProviderFactory func(config ProviderConfig, logger *slog.Logger) (Provider, error)

// Registry manages all available semantic analysis providers.
// It provides thread-safe registration, lookup, and listing operations.
type Registry struct {
	providers map[string]ProviderFactory
	mu        sync.RWMutex
}

var (
	// globalRegistry is the singleton registry instance
	globalRegistry *Registry
	once           sync.Once
)

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]ProviderFactory),
	}
}

// GlobalRegistry returns the singleton registry instance
func GlobalRegistry() *Registry {
	once.Do(func() {
		globalRegistry = NewRegistry()
	})
	return globalRegistry
}

// Register adds a provider factory to the registry.
// Provider implementations call this in their init() functions.
// If a provider with the same name already exists, it will be replaced.
func (r *Registry) Register(name string, factory ProviderFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.providers[name] = factory
}

// Get retrieves a provider factory by name.
// Returns error if the provider is not found.
func (r *Registry) Get(name string) (ProviderFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not found in registry", name)
	}

	return factory, nil
}

// List returns the names of all registered providers.
// The returned slice is a copy and safe for concurrent use.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}

	return names
}
