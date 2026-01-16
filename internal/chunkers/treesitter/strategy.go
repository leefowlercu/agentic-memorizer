package treesitter

import (
	sitter "github.com/smacker/go-tree-sitter"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
)

// LanguageStrategy defines how to parse and extract metadata for a specific language.
type LanguageStrategy interface {
	// Language returns the language identifier (e.g., "go", "python").
	Language() string

	// Extensions returns file extensions this strategy handles (e.g., ".go", ".py").
	Extensions() []string

	// MIMETypes returns MIME types this strategy handles.
	MIMETypes() []string

	// GetLanguage returns the tree-sitter Language for parsing.
	GetLanguage() *sitter.Language

	// NodeTypes returns the configuration for significant node types.
	NodeTypes() NodeTypeConfig

	// ExtractMetadata extracts code metadata from an AST node.
	ExtractMetadata(node *sitter.Node, source []byte) *chunkers.CodeMetadata

	// ShouldChunk determines if a node should be its own chunk.
	ShouldChunk(node *sitter.Node) bool
}

// NodeTypeConfig defines which AST node types are significant for chunking.
type NodeTypeConfig struct {
	// Functions are node types that represent functions.
	Functions []string

	// Classes are node types that represent classes/structs/interfaces.
	Classes []string

	// Methods are node types that represent methods.
	Methods []string

	// Declarations are other significant declarations to chunk.
	Declarations []string

	// TopLevel are node types that should always be chunked at the top level.
	TopLevel []string
}

// AllChunkableTypes returns all node types that should be chunked.
func (c NodeTypeConfig) AllChunkableTypes() []string {
	var all []string
	all = append(all, c.Functions...)
	all = append(all, c.Classes...)
	all = append(all, c.Methods...)
	all = append(all, c.Declarations...)
	all = append(all, c.TopLevel...)
	return all
}

// IsChunkable returns true if the node type should be chunked.
func (c NodeTypeConfig) IsChunkable(nodeType string) bool {
	for _, t := range c.AllChunkableTypes() {
		if t == nodeType {
			return true
		}
	}
	return false
}

// IsFunction returns true if the node type represents a function.
func (c NodeTypeConfig) IsFunction(nodeType string) bool {
	for _, t := range c.Functions {
		if t == nodeType {
			return true
		}
	}
	return false
}

// IsClass returns true if the node type represents a class/struct/interface.
func (c NodeTypeConfig) IsClass(nodeType string) bool {
	for _, t := range c.Classes {
		if t == nodeType {
			return true
		}
	}
	return false
}

// IsMethod returns true if the node type represents a method.
func (c NodeTypeConfig) IsMethod(nodeType string) bool {
	for _, t := range c.Methods {
		if t == nodeType {
			return true
		}
	}
	return false
}
