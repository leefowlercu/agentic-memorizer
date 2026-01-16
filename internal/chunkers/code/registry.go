package code

import (
	"strings"
	"sync"
)

// StrategyRegistry manages language strategies for the tree-sitter chunker.
type StrategyRegistry struct {
	mu           sync.RWMutex
	strategies   map[string]LanguageStrategy // keyed by language name
	extensionMap map[string]LanguageStrategy // keyed by extension (e.g., ".go")
	mimeTypeMap  map[string]LanguageStrategy // keyed by MIME type
}

// NewStrategyRegistry creates a new strategy registry.
func NewStrategyRegistry() *StrategyRegistry {
	return &StrategyRegistry{
		strategies:   make(map[string]LanguageStrategy),
		extensionMap: make(map[string]LanguageStrategy),
		mimeTypeMap:  make(map[string]LanguageStrategy),
	}
}

// Register adds a language strategy to the registry.
func (r *StrategyRegistry) Register(strategy LanguageStrategy) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Register by language name
	r.strategies[strategy.Language()] = strategy

	// Register by extensions
	for _, ext := range strategy.Extensions() {
		r.extensionMap[strings.ToLower(ext)] = strategy
	}

	// Register by MIME types
	for _, mime := range strategy.MIMETypes() {
		r.mimeTypeMap[strings.ToLower(mime)] = strategy
	}
}

// Get returns a strategy by language name.
func (r *StrategyRegistry) Get(language string) LanguageStrategy {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.strategies[strings.ToLower(language)]
}

// GetByExtension returns a strategy by file extension.
func (r *StrategyRegistry) GetByExtension(ext string) LanguageStrategy {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return r.extensionMap[ext]
}

// GetByMIMEType returns a strategy by MIME type.
func (r *StrategyRegistry) GetByMIMEType(mimeType string) LanguageStrategy {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.mimeTypeMap[strings.ToLower(mimeType)]
}

// Resolve finds the best strategy for given MIME type and language hint.
func (r *StrategyRegistry) Resolve(mimeType, language string) LanguageStrategy {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try MIME type first
	if mimeType != "" {
		if s := r.mimeTypeMap[strings.ToLower(mimeType)]; s != nil {
			return s
		}
	}

	// Try language name
	if language != "" {
		lang := strings.ToLower(language)

		// Direct language match
		if s := r.strategies[lang]; s != nil {
			return s
		}

		// Try as extension
		if s := r.extensionMap[lang]; s != nil {
			return s
		}
		if !strings.HasPrefix(lang, ".") {
			if s := r.extensionMap["."+lang]; s != nil {
				return s
			}
		}
	}

	return nil
}

// Languages returns all registered language names.
func (r *StrategyRegistry) Languages() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	languages := make([]string, 0, len(r.strategies))
	for lang := range r.strategies {
		languages = append(languages, lang)
	}
	return languages
}

// CanHandle returns true if the registry has a strategy for the given MIME type or language.
func (r *StrategyRegistry) CanHandle(mimeType, language string) bool {
	return r.Resolve(mimeType, language) != nil
}
