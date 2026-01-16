package languages

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/rust"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/chunkers/code"
)

// RustStrategy implements tree-sitter parsing for Rust code.
type RustStrategy struct{}

// NewRustStrategy creates a new Rust language strategy.
func NewRustStrategy() *RustStrategy {
	return &RustStrategy{}
}

// Language returns the language identifier.
func (s *RustStrategy) Language() string {
	return "rust"
}

// Extensions returns file extensions this strategy handles.
func (s *RustStrategy) Extensions() []string {
	return []string{".rs"}
}

// MIMETypes returns MIME types this strategy handles.
func (s *RustStrategy) MIMETypes() []string {
	return []string{
		"text/x-rust",
		"text/rust",
	}
}

// GetLanguage returns the tree-sitter Language for Rust.
func (s *RustStrategy) GetLanguage() *sitter.Language {
	return rust.GetLanguage()
}

// NodeTypes returns Rust-specific node type configuration.
func (s *RustStrategy) NodeTypes() code.NodeTypeConfig {
	return code.NodeTypeConfig{
		Functions: []string{
			"function_item",
		},
		Methods: []string{},
		Classes: []string{
			"struct_item",
			"enum_item",
			"trait_item",
			"impl_item",
			"type_item",
		},
		Declarations: []string{
			"const_item",
			"static_item",
			"mod_item",
		},
		TopLevel: []string{
			"macro_definition",
		},
	}
}

// ShouldChunk determines if a node should be its own chunk.
func (s *RustStrategy) ShouldChunk(node *sitter.Node) bool {
	nodeType := node.Type()
	switch nodeType {
	case "function_item":
		// Chunk top-level functions and impl methods
		parent := node.Parent()
		if parent == nil {
			return true
		}
		switch parent.Type() {
		case "source_file", "declaration_list":
			return true
		}
		return false
	case "struct_item", "enum_item", "trait_item", "impl_item", "type_item":
		return true
	case "const_item", "static_item", "mod_item", "macro_definition":
		// Only chunk top-level declarations
		parent := node.Parent()
		return parent != nil && parent.Type() == "source_file"
	}
	return false
}

// ExtractMetadata extracts Rust-specific metadata from an AST node.
func (s *RustStrategy) ExtractMetadata(node *sitter.Node, source []byte) *chunkers.CodeMetadata {
	meta := &chunkers.CodeMetadata{
		Language:  "rust",
		LineStart: int(node.StartPoint().Row) + 1,
		LineEnd:   int(node.EndPoint().Row) + 1,
	}

	switch node.Type() {
	case "function_item":
		s.extractFunctionMetadata(node, source, meta)
	case "struct_item":
		s.extractStructMetadata(node, source, meta)
	case "enum_item":
		s.extractEnumMetadata(node, source, meta)
	case "trait_item":
		s.extractTraitMetadata(node, source, meta)
	case "impl_item":
		s.extractImplMetadata(node, source, meta)
	case "type_item":
		s.extractTypeMetadata(node, source, meta)
	case "const_item", "static_item":
		s.extractConstMetadata(node, source, meta)
	case "mod_item":
		s.extractModMetadata(node, source, meta)
	}

	return meta
}

// extractFunctionMetadata extracts metadata from a function item.
func (s *RustStrategy) extractFunctionMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Check if in impl block
	parent := node.Parent()
	if parent != nil && parent.Type() == "declaration_list" {
		grandparent := parent.Parent()
		if grandparent != nil && grandparent.Type() == "impl_item" {
			// Get the impl type name
			for i := 0; i < int(grandparent.ChildCount()); i++ {
				child := grandparent.Child(i)
				if child.Type() == "type_identifier" || child.Type() == "generic_type" {
					meta.ClassName = string(source[child.StartByte():child.EndByte()])
					break
				}
			}
		}
	}

	// Check visibility
	visibility := s.findChild(node, "visibility_modifier")
	if visibility != nil {
		visText := string(source[visibility.StartByte():visibility.EndByte()])
		if strings.HasPrefix(visText, "pub") {
			meta.IsExported = true
			meta.Visibility = "public"
			if strings.Contains(visText, "crate") {
				meta.Visibility = "crate"
			} else if strings.Contains(visText, "super") {
				meta.Visibility = "super"
			}
		}
	} else {
		meta.Visibility = "private"
	}

	// Find function name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			meta.FunctionName = string(source[child.StartByte():child.EndByte()])
			break
		}
	}

	// Check for async
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if string(source[child.StartByte():child.EndByte()]) == "async" {
			meta.IsAsync = true
			break
		}
	}

	// Extract parameters
	params := s.findChild(node, "parameters")
	if params != nil {
		meta.Parameters = s.extractParameters(params, source)
	}

	// Extract return type
	returnType := s.findChild(node, "return_type")
	if returnType != nil {
		// Get the type inside return_type
		for i := 0; i < int(returnType.ChildCount()); i++ {
			child := returnType.Child(i)
			if child.Type() != "->" {
				meta.ReturnType = string(source[child.StartByte():child.EndByte()])
				break
			}
		}
	}

	// Build signature
	meta.Signature = s.buildFunctionSignature(meta)

	// Extract doc comments
	meta.Docstring = s.extractDocComment(node, source)

	// Extract attributes as decorators
	meta.Decorators = s.extractAttributes(node, source)
}

// extractStructMetadata extracts metadata from a struct item.
func (s *RustStrategy) extractStructMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Check visibility
	visibility := s.findChild(node, "visibility_modifier")
	if visibility != nil {
		visText := string(source[visibility.StartByte():visibility.EndByte()])
		if strings.HasPrefix(visText, "pub") {
			meta.IsExported = true
			meta.Visibility = "public"
		}
	} else {
		meta.Visibility = "private"
	}

	// Find struct name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_identifier" {
			meta.ClassName = string(source[child.StartByte():child.EndByte()])
			break
		}
	}

	// Extract doc comments
	meta.Docstring = s.extractDocComment(node, source)

	// Extract attributes
	meta.Decorators = s.extractAttributes(node, source)
}

// extractEnumMetadata extracts metadata from an enum item.
func (s *RustStrategy) extractEnumMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Check visibility
	visibility := s.findChild(node, "visibility_modifier")
	if visibility != nil {
		visText := string(source[visibility.StartByte():visibility.EndByte()])
		if strings.HasPrefix(visText, "pub") {
			meta.IsExported = true
			meta.Visibility = "public"
		}
	} else {
		meta.Visibility = "private"
	}

	// Find enum name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_identifier" {
			meta.ClassName = string(source[child.StartByte():child.EndByte()])
			break
		}
	}

	// Extract doc comments
	meta.Docstring = s.extractDocComment(node, source)

	// Extract attributes
	meta.Decorators = s.extractAttributes(node, source)
}

// extractTraitMetadata extracts metadata from a trait item.
func (s *RustStrategy) extractTraitMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Check visibility
	visibility := s.findChild(node, "visibility_modifier")
	if visibility != nil {
		visText := string(source[visibility.StartByte():visibility.EndByte()])
		if strings.HasPrefix(visText, "pub") {
			meta.IsExported = true
			meta.Visibility = "public"
		}
	} else {
		meta.Visibility = "private"
	}

	// Find trait name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_identifier" {
			meta.ClassName = string(source[child.StartByte():child.EndByte()])
			break
		}
	}

	// Extract supertraits as implements
	bounds := s.findChild(node, "trait_bounds")
	if bounds != nil {
		for i := 0; i < int(bounds.ChildCount()); i++ {
			child := bounds.Child(i)
			if child.Type() == "type_identifier" {
				meta.Implements = append(meta.Implements, string(source[child.StartByte():child.EndByte()]))
			}
		}
	}

	// Extract doc comments
	meta.Docstring = s.extractDocComment(node, source)

	// Extract attributes
	meta.Decorators = s.extractAttributes(node, source)
}

// extractImplMetadata extracts metadata from an impl item.
func (s *RustStrategy) extractImplMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Find the type being implemented
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_identifier" || child.Type() == "generic_type" {
			meta.ClassName = string(source[child.StartByte():child.EndByte()])
			break
		}
	}

	// Check if this is a trait impl
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_identifier" && i > 0 {
			// Second type identifier is the trait being implemented
			prevChild := node.Child(i - 1)
			if prevChild != nil {
				prevText := string(source[prevChild.StartByte():prevChild.EndByte()])
				if prevText == "for" {
					// The previous type_identifier was the trait
					for j := 0; j < i; j++ {
						traitChild := node.Child(j)
						if traitChild.Type() == "type_identifier" {
							meta.Implements = append(meta.Implements, string(source[traitChild.StartByte():traitChild.EndByte()]))
							break
						}
					}
				}
			}
		}
	}

	meta.Visibility = "private" // impl blocks don't have visibility

	// Extract doc comments
	meta.Docstring = s.extractDocComment(node, source)

	// Extract attributes
	meta.Decorators = s.extractAttributes(node, source)
}

// extractTypeMetadata extracts metadata from a type alias item.
func (s *RustStrategy) extractTypeMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Check visibility
	visibility := s.findChild(node, "visibility_modifier")
	if visibility != nil {
		visText := string(source[visibility.StartByte():visibility.EndByte()])
		if strings.HasPrefix(visText, "pub") {
			meta.IsExported = true
			meta.Visibility = "public"
		}
	} else {
		meta.Visibility = "private"
	}

	// Find type name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_identifier" {
			meta.ClassName = string(source[child.StartByte():child.EndByte()])
			break
		}
	}

	// Extract doc comments
	meta.Docstring = s.extractDocComment(node, source)
}

// extractConstMetadata extracts metadata from a const/static item.
func (s *RustStrategy) extractConstMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Check visibility
	visibility := s.findChild(node, "visibility_modifier")
	if visibility != nil {
		visText := string(source[visibility.StartByte():visibility.EndByte()])
		if strings.HasPrefix(visText, "pub") {
			meta.IsExported = true
			meta.Visibility = "public"
		}
	} else {
		meta.Visibility = "private"
	}

	// Find const/static name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			meta.FunctionName = string(source[child.StartByte():child.EndByte()])
			break
		}
	}

	// Extract doc comments
	meta.Docstring = s.extractDocComment(node, source)
}

// extractModMetadata extracts metadata from a mod item.
func (s *RustStrategy) extractModMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Check visibility
	visibility := s.findChild(node, "visibility_modifier")
	if visibility != nil {
		visText := string(source[visibility.StartByte():visibility.EndByte()])
		if strings.HasPrefix(visText, "pub") {
			meta.IsExported = true
			meta.Visibility = "public"
		}
	} else {
		meta.Visibility = "private"
	}

	// Find mod name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			meta.Namespace = string(source[child.StartByte():child.EndByte()])
			break
		}
	}
}

// extractParameters extracts parameter names from a parameters node.
func (s *RustStrategy) extractParameters(params *sitter.Node, source []byte) []string {
	var result []string

	for i := 0; i < int(params.ChildCount()); i++ {
		child := params.Child(i)
		switch child.Type() {
		case "parameter":
			var paramName string
			var paramType string
			for j := 0; j < int(child.ChildCount()); j++ {
				paramChild := child.Child(j)
				switch paramChild.Type() {
				case "identifier":
					paramName = string(source[paramChild.StartByte():paramChild.EndByte()])
				case "type_identifier", "reference_type", "generic_type", "primitive_type":
					paramType = string(source[paramChild.StartByte():paramChild.EndByte()])
				}
			}
			if paramName != "" {
				if paramType != "" {
					result = append(result, paramName+": "+paramType)
				} else {
					result = append(result, paramName)
				}
			}
		case "self_parameter":
			selfText := string(source[child.StartByte():child.EndByte()])
			result = append(result, selfText)
		}
	}

	return result
}

// extractAttributes extracts attribute names from a node.
func (s *RustStrategy) extractAttributes(node *sitter.Node, source []byte) []string {
	var attributes []string

	// Look for attribute items before this node
	prev := node.PrevSibling()
	for prev != nil && prev.Type() == "attribute_item" {
		// Extract the attribute name
		for i := 0; i < int(prev.ChildCount()); i++ {
			child := prev.Child(i)
			if child.Type() == "attribute" {
				for j := 0; j < int(child.ChildCount()); j++ {
					attrChild := child.Child(j)
					if attrChild.Type() == "identifier" || attrChild.Type() == "scoped_identifier" {
						attributes = append([]string{string(source[attrChild.StartByte():attrChild.EndByte()])}, attributes...)
						break
					}
				}
			}
		}
		prev = prev.PrevSibling()
	}

	return attributes
}

// extractDocComment extracts doc comments (///) preceding a node.
func (s *RustStrategy) extractDocComment(node *sitter.Node, source []byte) string {
	var docLines []string

	prev := node.PrevSibling()
	for prev != nil {
		if prev.Type() == "line_comment" {
			comment := string(source[prev.StartByte():prev.EndByte()])
			// Check for doc comment ///
			if strings.HasPrefix(comment, "///") {
				docLine := strings.TrimPrefix(comment, "///")
				docLine = strings.TrimSpace(docLine)
				docLines = append([]string{docLine}, docLines...)
			} else {
				break
			}
		} else if prev.Type() == "attribute_item" {
			// Skip attributes
		} else {
			break
		}
		prev = prev.PrevSibling()
	}

	return strings.Join(docLines, " ")
}

// buildFunctionSignature builds a function signature string.
func (s *RustStrategy) buildFunctionSignature(meta *chunkers.CodeMetadata) string {
	var sig strings.Builder

	if meta.IsExported {
		sig.WriteString("pub ")
	}
	if meta.IsAsync {
		sig.WriteString("async ")
	}
	sig.WriteString("fn ")
	sig.WriteString(meta.FunctionName)
	sig.WriteString("(")
	sig.WriteString(strings.Join(meta.Parameters, ", "))
	sig.WriteString(")")

	if meta.ReturnType != "" {
		sig.WriteString(" -> ")
		sig.WriteString(meta.ReturnType)
	}

	return sig.String()
}

// findChild finds the first child with the given type.
func (s *RustStrategy) findChild(node *sitter.Node, nodeType string) *sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == nodeType {
			return child
		}
	}
	return nil
}

// Ensure RustStrategy implements LanguageStrategy.
var _ code.LanguageStrategy = (*RustStrategy)(nil)
