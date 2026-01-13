package walker

import (
	"path/filepath"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

// Filter determines whether files and directories should be processed.
type Filter struct {
	config *registry.PathConfig
}

// NewFilter creates a new Filter from a PathConfig.
func NewFilter(config *registry.PathConfig) *Filter {
	if config == nil {
		config = &registry.PathConfig{}
	}
	return &Filter{config: config}
}

// ShouldProcessFile returns true if the file should be processed.
func (f *Filter) ShouldProcessFile(path string) bool {
	name := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(path))

	// Check include overrides first (they take precedence)
	if f.isFileIncluded(name, ext) {
		return true
	}

	// Check skip rules
	if f.isFileSkipped(name, ext) {
		return false
	}

	// If include rules exist but file wasn't matched, skip it
	if f.hasIncludeRules() {
		return false
	}

	return true
}

// ShouldProcessDir returns true if the directory should be traversed.
func (f *Filter) ShouldProcessDir(path string) bool {
	name := filepath.Base(path)

	// Check include overrides first
	if f.isDirIncluded(name) {
		return true
	}

	// Check skip rules
	if f.isDirSkipped(name) {
		return false
	}

	return true
}

// isFileSkipped checks if a file matches skip rules.
func (f *Filter) isFileSkipped(name, ext string) bool {
	// Check hidden files
	if f.config.SkipHidden && strings.HasPrefix(name, ".") {
		return true
	}

	// Check skip extensions
	for _, skipExt := range f.config.SkipExtensions {
		if normalizeExt(skipExt) == ext {
			return true
		}
	}

	// Check skip files
	for _, skipFile := range f.config.SkipFiles {
		if matchPattern(skipFile, name) {
			return true
		}
	}

	return false
}

// isFileIncluded checks if a file matches include overrides.
func (f *Filter) isFileIncluded(name, ext string) bool {
	// Check include hidden
	if f.config.IncludeHidden && strings.HasPrefix(name, ".") {
		return true
	}

	// Check include extensions
	for _, includeExt := range f.config.IncludeExtensions {
		if normalizeExt(includeExt) == ext {
			return true
		}
	}

	// Check include files
	for _, includeFile := range f.config.IncludeFiles {
		if matchPattern(includeFile, name) {
			return true
		}
	}

	return false
}

// isDirSkipped checks if a directory matches skip rules.
func (f *Filter) isDirSkipped(name string) bool {
	// Check hidden directories
	if f.config.SkipHidden && strings.HasPrefix(name, ".") {
		return true
	}

	// Check skip directories
	for _, skipDir := range f.config.SkipDirectories {
		if matchPattern(skipDir, name) {
			return true
		}
	}

	return false
}

// isDirIncluded checks if a directory matches include overrides.
func (f *Filter) isDirIncluded(name string) bool {
	// Check include hidden
	if f.config.IncludeHidden && strings.HasPrefix(name, ".") {
		return true
	}

	// Check include directories
	for _, includeDir := range f.config.IncludeDirectories {
		if matchPattern(includeDir, name) {
			return true
		}
	}

	return false
}

// hasIncludeRules returns true if any include rules are defined.
func (f *Filter) hasIncludeRules() bool {
	return len(f.config.IncludeExtensions) > 0 ||
		len(f.config.IncludeFiles) > 0
}

// normalizeExt ensures extension has leading dot and is lowercase.
func normalizeExt(ext string) string {
	ext = strings.ToLower(ext)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return ext
}

// matchPattern matches a pattern against a name.
// Supports simple glob patterns with * wildcard.
func matchPattern(pattern, name string) bool {
	// Simple exact match
	if pattern == name {
		return true
	}

	// Check for glob pattern
	if strings.Contains(pattern, "*") {
		matched, err := filepath.Match(pattern, name)
		if err == nil && matched {
			return true
		}
	}

	return false
}
