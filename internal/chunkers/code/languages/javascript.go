package languages

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/chunkers/code"
)

// JavaScriptStrategy implements tree-sitter parsing for JavaScript code.
type JavaScriptStrategy struct{}

// NewJavaScriptStrategy creates a new JavaScript language strategy.
func NewJavaScriptStrategy() *JavaScriptStrategy {
	return &JavaScriptStrategy{}
}

// Language returns the language identifier.
func (s *JavaScriptStrategy) Language() string {
	return "javascript"
}

// Extensions returns file extensions this strategy handles.
func (s *JavaScriptStrategy) Extensions() []string {
	return []string{".js", ".mjs", ".cjs", ".jsx"}
}

// MIMETypes returns MIME types this strategy handles.
func (s *JavaScriptStrategy) MIMETypes() []string {
	return []string{
		"text/javascript",
		"application/javascript",
		"text/jsx",
	}
}

// GetLanguage returns the tree-sitter Language for JavaScript.
func (s *JavaScriptStrategy) GetLanguage() *sitter.Language {
	return javascript.GetLanguage()
}

// NodeTypes returns JavaScript-specific node type configuration.
func (s *JavaScriptStrategy) NodeTypes() code.NodeTypeConfig {
	return code.NodeTypeConfig{
		Functions: []string{
			"function_declaration",
			"arrow_function",
			"function_expression",
			"generator_function_declaration",
		},
		Methods: []string{
			"method_definition",
		},
		Classes: []string{
			"class_declaration",
		},
		Declarations: []string{
			"lexical_declaration",
			"variable_declaration",
		},
		TopLevel: []string{
			"export_statement",
		},
	}
}

// ShouldChunk determines if a node should be its own chunk.
func (s *JavaScriptStrategy) ShouldChunk(node *sitter.Node) bool {
	nodeType := node.Type()
	switch nodeType {
	case "function_declaration", "generator_function_declaration":
		return true
	case "class_declaration":
		return true
	case "method_definition":
		// Only chunk methods that are inside classes
		parent := node.Parent()
		if parent != nil && parent.Type() == "class_body" {
			return true
		}
		return false
	case "arrow_function", "function_expression":
		// Only chunk if assigned to a variable at top level
		parent := node.Parent()
		if parent == nil {
			return false
		}
		// Check if this is a variable declarator's value
		if parent.Type() == "variable_declarator" {
			grandparent := parent.Parent()
			if grandparent != nil {
				greatgrandparent := grandparent.Parent()
				if greatgrandparent != nil && greatgrandparent.Type() == "program" {
					return true
				}
				// Check for export statement
				if greatgrandparent != nil && greatgrandparent.Type() == "export_statement" {
					return true
				}
			}
		}
		return false
	case "export_statement":
		// Only chunk export statements that contain declarations
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			childType := child.Type()
			switch childType {
			case "function_declaration", "class_declaration", "lexical_declaration", "variable_declaration":
				return true
			}
		}
		return false
	}
	return false
}

// ExtractMetadata extracts JavaScript-specific metadata from an AST node.
func (s *JavaScriptStrategy) ExtractMetadata(node *sitter.Node, source []byte) *chunkers.CodeMetadata {
	meta := &chunkers.CodeMetadata{
		Language:  "javascript",
		LineStart: int(node.StartPoint().Row) + 1,
		LineEnd:   int(node.EndPoint().Row) + 1,
	}

	switch node.Type() {
	case "function_declaration", "generator_function_declaration":
		s.extractFunctionMetadata(node, source, meta)
	case "arrow_function", "function_expression":
		s.extractArrowFunctionMetadata(node, source, meta)
	case "class_declaration":
		s.extractClassMetadata(node, source, meta)
	case "method_definition":
		s.extractMethodMetadata(node, source, meta)
	case "export_statement":
		s.extractExportMetadata(node, source, meta)
	}

	return meta
}

// extractFunctionMetadata extracts metadata from a function declaration.
func (s *JavaScriptStrategy) extractFunctionMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Find function name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			meta.FunctionName = string(source[child.StartByte():child.EndByte()])
			break
		}
	}

	// Check if async
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "async" {
			meta.IsAsync = true
			break
		}
	}

	// Check if generator
	if node.Type() == "generator_function_declaration" {
		meta.IsGenerator = true
	}

	// Check if exported
	parent := node.Parent()
	if parent != nil && parent.Type() == "export_statement" {
		meta.IsExported = true
	}
	meta.Visibility = "module"

	// Extract parameters
	params := s.findChild(node, "formal_parameters")
	if params != nil {
		meta.Parameters = s.extractParameters(params, source)
	}

	// Build signature
	meta.Signature = s.buildFunctionSignature(meta)

	// Extract JSDoc comment
	meta.Docstring = s.extractJSDocComment(node, source)
}

// extractArrowFunctionMetadata extracts metadata from an arrow function.
func (s *JavaScriptStrategy) extractArrowFunctionMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Arrow functions get their name from the variable they're assigned to
	parent := node.Parent()
	if parent != nil && parent.Type() == "variable_declarator" {
		for i := 0; i < int(parent.ChildCount()); i++ {
			child := parent.Child(i)
			if child.Type() == "identifier" {
				meta.FunctionName = string(source[child.StartByte():child.EndByte()])
				break
			}
		}
	}

	// Check for async
	prevSibling := node.PrevSibling()
	if prevSibling != nil && string(source[prevSibling.StartByte():prevSibling.EndByte()]) == "async" {
		meta.IsAsync = true
	}

	// Check if exported
	grandparent := parent
	if grandparent != nil {
		grandparent = grandparent.Parent()
	}
	if grandparent != nil {
		greatgrandparent := grandparent.Parent()
		if greatgrandparent != nil && greatgrandparent.Type() == "export_statement" {
			meta.IsExported = true
		}
	}
	meta.Visibility = "module"

	// Extract parameters
	params := s.findChild(node, "formal_parameters")
	if params != nil {
		meta.Parameters = s.extractParameters(params, source)
	}

	// Build signature
	meta.Signature = s.buildFunctionSignature(meta)
}

// extractClassMetadata extracts metadata from a class declaration.
func (s *JavaScriptStrategy) extractClassMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Find class name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			meta.ClassName = string(source[child.StartByte():child.EndByte()])
			break
		}
	}

	// Check if exported
	parent := node.Parent()
	if parent != nil && parent.Type() == "export_statement" {
		meta.IsExported = true
	}
	meta.Visibility = "module"

	// Extract parent class (extends)
	heritage := s.findChild(node, "class_heritage")
	if heritage != nil {
		for i := 0; i < int(heritage.ChildCount()); i++ {
			child := heritage.Child(i)
			if child.Type() == "identifier" {
				meta.ParentClass = string(source[child.StartByte():child.EndByte()])
				break
			}
		}
	}

	// Extract JSDoc
	meta.Docstring = s.extractJSDocComment(node, source)
}

// extractMethodMetadata extracts metadata from a method definition.
func (s *JavaScriptStrategy) extractMethodMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Get class name from parent
	parent := node.Parent() // class_body
	if parent != nil {
		grandparent := parent.Parent() // class_declaration
		if grandparent != nil && grandparent.Type() == "class_declaration" {
			for i := 0; i < int(grandparent.ChildCount()); i++ {
				child := grandparent.Child(i)
				if child.Type() == "identifier" {
					meta.ClassName = string(source[child.StartByte():child.EndByte()])
					break
				}
			}
		}
	}

	// Find method name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "property_identifier" {
			meta.FunctionName = string(source[child.StartByte():child.EndByte()])
			break
		}
	}

	// Check for static, async, get, set
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		text := string(source[child.StartByte():child.EndByte()])
		switch text {
		case "static":
			meta.IsStatic = true
		case "async":
			meta.IsAsync = true
		case "get":
			meta.IsGetter = true
		case "set":
			meta.IsSetter = true
		}
	}

	// Constructor is special
	if meta.FunctionName == "constructor" {
		meta.IsConstructor = true
	}

	meta.Visibility = "public"

	// Extract parameters
	params := s.findChild(node, "formal_parameters")
	if params != nil {
		meta.Parameters = s.extractParameters(params, source)
	}

	// Build signature
	meta.Signature = s.buildMethodSignature(meta)
}

// extractExportMetadata extracts metadata from an export statement.
func (s *JavaScriptStrategy) extractExportMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Extract the declaration inside
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "function_declaration":
			s.extractFunctionMetadata(child, source, meta)
			meta.IsExported = true
		case "class_declaration":
			s.extractClassMetadata(child, source, meta)
			meta.IsExported = true
		case "lexical_declaration", "variable_declaration":
			// Extract variable names
			var names []string
			for j := 0; j < int(child.ChildCount()); j++ {
				declarator := child.Child(j)
				if declarator.Type() == "variable_declarator" {
					for k := 0; k < int(declarator.ChildCount()); k++ {
						name := declarator.Child(k)
						if name.Type() == "identifier" {
							names = append(names, string(source[name.StartByte():name.EndByte()]))
							break
						}
					}
				}
			}
			if len(names) > 0 {
				meta.FunctionName = strings.Join(names, ", ")
			}
			meta.IsExported = true
		}
	}
}

// extractParameters extracts parameter names from formal parameters.
func (s *JavaScriptStrategy) extractParameters(params *sitter.Node, source []byte) []string {
	var result []string

	for i := 0; i < int(params.ChildCount()); i++ {
		child := params.Child(i)
		switch child.Type() {
		case "identifier":
			result = append(result, string(source[child.StartByte():child.EndByte()]))
		case "assignment_pattern":
			// Default parameter - get the name
			for j := 0; j < int(child.ChildCount()); j++ {
				paramChild := child.Child(j)
				if paramChild.Type() == "identifier" {
					result = append(result, string(source[paramChild.StartByte():paramChild.EndByte()]))
					break
				}
			}
		case "rest_pattern":
			// ...args
			for j := 0; j < int(child.ChildCount()); j++ {
				paramChild := child.Child(j)
				if paramChild.Type() == "identifier" {
					result = append(result, "..."+string(source[paramChild.StartByte():paramChild.EndByte()]))
					break
				}
			}
		case "object_pattern", "array_pattern":
			// Destructured parameter - just note it
			result = append(result, string(source[child.StartByte():child.EndByte()]))
		}
	}

	return result
}

// extractJSDocComment extracts JSDoc comment preceding a node.
func (s *JavaScriptStrategy) extractJSDocComment(node *sitter.Node, source []byte) string {
	prev := node.PrevSibling()
	if prev == nil {
		return ""
	}

	if prev.Type() == "comment" {
		comment := string(source[prev.StartByte():prev.EndByte()])
		// Check if it's a JSDoc comment
		if strings.HasPrefix(comment, "/**") {
			// Clean up the comment
			comment = strings.TrimPrefix(comment, "/**")
			comment = strings.TrimSuffix(comment, "*/")
			// Remove leading asterisks from lines
			lines := strings.Split(comment, "\n")
			var cleaned []string
			for _, line := range lines {
				line = strings.TrimSpace(line)
				line = strings.TrimPrefix(line, "*")
				line = strings.TrimSpace(line)
				if line != "" {
					cleaned = append(cleaned, line)
				}
			}
			return strings.Join(cleaned, " ")
		}
	}

	return ""
}

// buildFunctionSignature builds a function signature string.
func (s *JavaScriptStrategy) buildFunctionSignature(meta *chunkers.CodeMetadata) string {
	var sig strings.Builder

	if meta.IsAsync {
		sig.WriteString("async ")
	}
	sig.WriteString("function ")
	if meta.IsGenerator {
		sig.WriteString("* ")
	}
	sig.WriteString(meta.FunctionName)
	sig.WriteString("(")
	sig.WriteString(strings.Join(meta.Parameters, ", "))
	sig.WriteString(")")

	return sig.String()
}

// buildMethodSignature builds a method signature string.
func (s *JavaScriptStrategy) buildMethodSignature(meta *chunkers.CodeMetadata) string {
	var sig strings.Builder

	if meta.IsStatic {
		sig.WriteString("static ")
	}
	if meta.IsAsync {
		sig.WriteString("async ")
	}
	if meta.IsGetter {
		sig.WriteString("get ")
	}
	if meta.IsSetter {
		sig.WriteString("set ")
	}
	sig.WriteString(meta.FunctionName)
	sig.WriteString("(")
	sig.WriteString(strings.Join(meta.Parameters, ", "))
	sig.WriteString(")")

	return sig.String()
}

// findChild finds the first child with the given type.
func (s *JavaScriptStrategy) findChild(node *sitter.Node, nodeType string) *sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == nodeType {
			return child
		}
	}
	return nil
}

// Ensure JavaScriptStrategy implements LanguageStrategy.
var _ code.LanguageStrategy = (*JavaScriptStrategy)(nil)
