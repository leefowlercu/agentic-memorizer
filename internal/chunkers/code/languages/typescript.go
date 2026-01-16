package languages

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/typescript"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/chunkers/code"
)

// TypeScriptStrategy implements tree-sitter parsing for TypeScript code.
type TypeScriptStrategy struct{}

// NewTypeScriptStrategy creates a new TypeScript language strategy.
func NewTypeScriptStrategy() *TypeScriptStrategy {
	return &TypeScriptStrategy{}
}

// Language returns the language identifier.
func (s *TypeScriptStrategy) Language() string {
	return "typescript"
}

// Extensions returns file extensions this strategy handles.
func (s *TypeScriptStrategy) Extensions() []string {
	return []string{".ts", ".tsx", ".mts", ".cts"}
}

// MIMETypes returns MIME types this strategy handles.
func (s *TypeScriptStrategy) MIMETypes() []string {
	return []string{
		"text/typescript",
		"application/typescript",
		"text/tsx",
	}
}

// GetLanguage returns the tree-sitter Language for TypeScript.
func (s *TypeScriptStrategy) GetLanguage() *sitter.Language {
	return typescript.GetLanguage()
}

// NodeTypes returns TypeScript-specific node type configuration.
func (s *TypeScriptStrategy) NodeTypes() code.NodeTypeConfig {
	return code.NodeTypeConfig{
		Functions: []string{
			"function_declaration",
			"arrow_function",
			"function_expression",
			"generator_function_declaration",
		},
		Methods: []string{
			"method_definition",
			"method_signature",
		},
		Classes: []string{
			"class_declaration",
			"interface_declaration",
			"type_alias_declaration",
			"enum_declaration",
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
func (s *TypeScriptStrategy) ShouldChunk(node *sitter.Node) bool {
	nodeType := node.Type()
	switch nodeType {
	case "function_declaration", "generator_function_declaration":
		return true
	case "class_declaration", "interface_declaration", "type_alias_declaration", "enum_declaration":
		return true
	case "method_definition", "method_signature":
		// Only chunk methods that are inside classes/interfaces
		parent := node.Parent()
		if parent != nil {
			switch parent.Type() {
			case "class_body", "interface_body", "object_type":
				return true
			}
		}
		return false
	case "arrow_function", "function_expression":
		// Only chunk if assigned to a variable at top level
		parent := node.Parent()
		if parent == nil {
			return false
		}
		if parent.Type() == "variable_declarator" {
			grandparent := parent.Parent()
			if grandparent != nil {
				greatgrandparent := grandparent.Parent()
				if greatgrandparent != nil && greatgrandparent.Type() == "program" {
					return true
				}
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
			case "function_declaration", "class_declaration", "interface_declaration",
				"type_alias_declaration", "enum_declaration", "lexical_declaration",
				"variable_declaration":
				return true
			}
		}
		return false
	}
	return false
}

// ExtractMetadata extracts TypeScript-specific metadata from an AST node.
func (s *TypeScriptStrategy) ExtractMetadata(node *sitter.Node, source []byte) *chunkers.CodeMetadata {
	meta := &chunkers.CodeMetadata{
		Language:  "typescript",
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
	case "interface_declaration":
		s.extractInterfaceMetadata(node, source, meta)
	case "type_alias_declaration":
		s.extractTypeAliasMetadata(node, source, meta)
	case "enum_declaration":
		s.extractEnumMetadata(node, source, meta)
	case "method_definition", "method_signature":
		s.extractMethodMetadata(node, source, meta)
	case "export_statement":
		s.extractExportMetadata(node, source, meta)
	}

	return meta
}

// extractFunctionMetadata extracts metadata from a function declaration.
func (s *TypeScriptStrategy) extractFunctionMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
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

	// Extract return type
	returnType := s.findChild(node, "type_annotation")
	if returnType != nil {
		meta.ReturnType = strings.TrimPrefix(
			strings.TrimSpace(string(source[returnType.StartByte():returnType.EndByte()])),
			":",
		)
		meta.ReturnType = strings.TrimSpace(meta.ReturnType)
	}

	// Build signature
	meta.Signature = s.buildFunctionSignature(meta)

	// Extract JSDoc comment
	meta.Docstring = s.extractJSDocComment(node, source)
}

// extractArrowFunctionMetadata extracts metadata from an arrow function.
func (s *TypeScriptStrategy) extractArrowFunctionMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
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

		// Check for type annotation on variable
		typeAnnotation := s.findChild(parent, "type_annotation")
		if typeAnnotation != nil {
			meta.ReturnType = strings.TrimPrefix(
				strings.TrimSpace(string(source[typeAnnotation.StartByte():typeAnnotation.EndByte()])),
				":",
			)
			meta.ReturnType = strings.TrimSpace(meta.ReturnType)
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
func (s *TypeScriptStrategy) extractClassMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Find class name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_identifier" {
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
			if child.Type() == "extends_clause" {
				for j := 0; j < int(child.ChildCount()); j++ {
					extChild := child.Child(j)
					if extChild.Type() == "type_identifier" || extChild.Type() == "identifier" {
						meta.ParentClass = string(source[extChild.StartByte():extChild.EndByte()])
						break
					}
				}
			}
			if child.Type() == "implements_clause" {
				// Extract implemented interfaces
				for j := 0; j < int(child.ChildCount()); j++ {
					implChild := child.Child(j)
					if implChild.Type() == "type_identifier" || implChild.Type() == "identifier" {
						meta.Implements = append(meta.Implements, string(source[implChild.StartByte():implChild.EndByte()]))
					}
				}
			}
		}
	}

	// Extract JSDoc
	meta.Docstring = s.extractJSDocComment(node, source)
}

// extractInterfaceMetadata extracts metadata from an interface declaration.
func (s *TypeScriptStrategy) extractInterfaceMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Find interface name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_identifier" {
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

	// Extract extended interfaces
	heritage := s.findChild(node, "extends_type_clause")
	if heritage != nil {
		for i := 0; i < int(heritage.ChildCount()); i++ {
			child := heritage.Child(i)
			if child.Type() == "type_identifier" {
				meta.Implements = append(meta.Implements, string(source[child.StartByte():child.EndByte()]))
			}
		}
	}

	// Extract JSDoc
	meta.Docstring = s.extractJSDocComment(node, source)
}

// extractTypeAliasMetadata extracts metadata from a type alias declaration.
func (s *TypeScriptStrategy) extractTypeAliasMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Find type name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_identifier" {
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

	// Extract JSDoc
	meta.Docstring = s.extractJSDocComment(node, source)
}

// extractEnumMetadata extracts metadata from an enum declaration.
func (s *TypeScriptStrategy) extractEnumMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Find enum name
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

	// Extract JSDoc
	meta.Docstring = s.extractJSDocComment(node, source)
}

// extractMethodMetadata extracts metadata from a method definition/signature.
func (s *TypeScriptStrategy) extractMethodMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Get class/interface name from parent
	parent := node.Parent()
	if parent != nil {
		grandparent := parent.Parent()
		if grandparent != nil {
			switch grandparent.Type() {
			case "class_declaration", "interface_declaration":
				for i := 0; i < int(grandparent.ChildCount()); i++ {
					child := grandparent.Child(i)
					if child.Type() == "type_identifier" {
						meta.ClassName = string(source[child.StartByte():child.EndByte()])
						break
					}
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

	// Check for accessibility modifiers, static, async, get, set
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "accessibility_modifier":
			text := string(source[child.StartByte():child.EndByte()])
			meta.Visibility = text
		case "static":
			meta.IsStatic = true
		case "async":
			meta.IsAsync = true
		case "readonly":
			// Could add IsReadonly field if needed
		}
		text := string(source[child.StartByte():child.EndByte()])
		switch text {
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

	if meta.Visibility == "" {
		meta.Visibility = "public"
	}

	// Extract parameters
	params := s.findChild(node, "formal_parameters")
	if params != nil {
		meta.Parameters = s.extractParameters(params, source)
	}

	// Extract return type
	returnType := s.findChild(node, "type_annotation")
	if returnType != nil {
		meta.ReturnType = strings.TrimPrefix(
			strings.TrimSpace(string(source[returnType.StartByte():returnType.EndByte()])),
			":",
		)
		meta.ReturnType = strings.TrimSpace(meta.ReturnType)
	}

	// Build signature
	meta.Signature = s.buildMethodSignature(meta)
}

// extractExportMetadata extracts metadata from an export statement.
func (s *TypeScriptStrategy) extractExportMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
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
		case "interface_declaration":
			s.extractInterfaceMetadata(child, source, meta)
			meta.IsExported = true
		case "type_alias_declaration":
			s.extractTypeAliasMetadata(child, source, meta)
			meta.IsExported = true
		case "enum_declaration":
			s.extractEnumMetadata(child, source, meta)
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

// extractParameters extracts parameter names and types from formal parameters.
func (s *TypeScriptStrategy) extractParameters(params *sitter.Node, source []byte) []string {
	var result []string

	for i := 0; i < int(params.ChildCount()); i++ {
		child := params.Child(i)
		switch child.Type() {
		case "identifier":
			result = append(result, string(source[child.StartByte():child.EndByte()]))
		case "required_parameter", "optional_parameter":
			// Get the identifier and optional type
			var paramName string
			var paramType string
			for j := 0; j < int(child.ChildCount()); j++ {
				paramChild := child.Child(j)
				switch paramChild.Type() {
				case "identifier":
					paramName = string(source[paramChild.StartByte():paramChild.EndByte()])
				case "type_annotation":
					paramType = strings.TrimPrefix(
						strings.TrimSpace(string(source[paramChild.StartByte():paramChild.EndByte()])),
						":",
					)
					paramType = strings.TrimSpace(paramType)
				}
			}
			if paramName != "" {
				if paramType != "" {
					result = append(result, paramName+": "+paramType)
				} else {
					result = append(result, paramName)
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
			// Destructured parameter
			result = append(result, string(source[child.StartByte():child.EndByte()]))
		}
	}

	return result
}

// extractJSDocComment extracts JSDoc comment preceding a node.
func (s *TypeScriptStrategy) extractJSDocComment(node *sitter.Node, source []byte) string {
	prev := node.PrevSibling()
	if prev == nil {
		return ""
	}

	if prev.Type() == "comment" {
		comment := string(source[prev.StartByte():prev.EndByte()])
		if strings.HasPrefix(comment, "/**") {
			comment = strings.TrimPrefix(comment, "/**")
			comment = strings.TrimSuffix(comment, "*/")
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
func (s *TypeScriptStrategy) buildFunctionSignature(meta *chunkers.CodeMetadata) string {
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

	if meta.ReturnType != "" {
		sig.WriteString(": ")
		sig.WriteString(meta.ReturnType)
	}

	return sig.String()
}

// buildMethodSignature builds a method signature string.
func (s *TypeScriptStrategy) buildMethodSignature(meta *chunkers.CodeMetadata) string {
	var sig strings.Builder

	if meta.Visibility != "" && meta.Visibility != "public" {
		sig.WriteString(meta.Visibility)
		sig.WriteString(" ")
	}
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

	if meta.ReturnType != "" {
		sig.WriteString(": ")
		sig.WriteString(meta.ReturnType)
	}

	return sig.String()
}

// findChild finds the first child with the given type.
func (s *TypeScriptStrategy) findChild(node *sitter.Node, nodeType string) *sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == nodeType {
			return child
		}
	}
	return nil
}

// Ensure TypeScriptStrategy implements LanguageStrategy.
var _ code.LanguageStrategy = (*TypeScriptStrategy)(nil)
