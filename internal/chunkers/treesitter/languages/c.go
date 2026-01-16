package languages

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/chunkers/treesitter"
)

// CStrategy implements tree-sitter parsing for C code.
type CStrategy struct{}

// NewCStrategy creates a new C language strategy.
func NewCStrategy() *CStrategy {
	return &CStrategy{}
}

// Language returns the language identifier.
func (s *CStrategy) Language() string {
	return "c"
}

// Extensions returns file extensions this strategy handles.
func (s *CStrategy) Extensions() []string {
	return []string{".c", ".h"}
}

// MIMETypes returns MIME types this strategy handles.
func (s *CStrategy) MIMETypes() []string {
	return []string{
		"text/x-c",
		"text/x-csrc",
		"text/x-chdr",
	}
}

// GetLanguage returns the tree-sitter Language for C.
func (s *CStrategy) GetLanguage() *sitter.Language {
	return c.GetLanguage()
}

// NodeTypes returns C-specific node type configuration.
func (s *CStrategy) NodeTypes() treesitter.NodeTypeConfig {
	return treesitter.NodeTypeConfig{
		Functions: []string{
			"function_definition",
		},
		Methods: []string{},
		Classes: []string{
			"struct_specifier",
			"union_specifier",
			"enum_specifier",
		},
		Declarations: []string{
			"declaration",
			"type_definition",
		},
		TopLevel: []string{
			"preproc_function_def",
		},
	}
}

// ShouldChunk determines if a node should be its own chunk.
func (s *CStrategy) ShouldChunk(node *sitter.Node) bool {
	nodeType := node.Type()
	switch nodeType {
	case "function_definition":
		return true
	case "struct_specifier", "union_specifier", "enum_specifier":
		// Only chunk if it has a body (not just a forward declaration)
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			switch child.Type() {
			case "field_declaration_list", "enumerator_list":
				return true
			}
		}
		return false
	case "type_definition":
		// Chunk typedef if it defines a struct/union/enum
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			switch child.Type() {
			case "struct_specifier", "union_specifier", "enum_specifier":
				return true
			}
		}
		return false
	case "preproc_function_def":
		// Macro function definitions
		return true
	}
	return false
}

// ExtractMetadata extracts C-specific metadata from an AST node.
func (s *CStrategy) ExtractMetadata(node *sitter.Node, source []byte) *chunkers.CodeMetadata {
	meta := &chunkers.CodeMetadata{
		Language:  "c",
		LineStart: int(node.StartPoint().Row) + 1,
		LineEnd:   int(node.EndPoint().Row) + 1,
	}

	switch node.Type() {
	case "function_definition":
		s.extractFunctionMetadata(node, source, meta)
	case "struct_specifier":
		s.extractStructMetadata(node, source, meta)
	case "union_specifier":
		s.extractUnionMetadata(node, source, meta)
	case "enum_specifier":
		s.extractEnumMetadata(node, source, meta)
	case "type_definition":
		s.extractTypedefMetadata(node, source, meta)
	case "preproc_function_def":
		s.extractMacroMetadata(node, source, meta)
	}

	return meta
}

// extractFunctionMetadata extracts metadata from a function definition.
func (s *CStrategy) extractFunctionMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Look for declarator which contains the function name
	declarator := s.findChild(node, "function_declarator")
	if declarator == nil {
		// Try to find it in a pointer_declarator
		ptrDecl := s.findChild(node, "pointer_declarator")
		if ptrDecl != nil {
			declarator = s.findChild(ptrDecl, "function_declarator")
		}
	}

	if declarator != nil {
		// Get function name from declarator
		for i := 0; i < int(declarator.ChildCount()); i++ {
			child := declarator.Child(i)
			if child.Type() == "identifier" {
				meta.FunctionName = string(source[child.StartByte():child.EndByte()])
				break
			}
			// Handle pointer to function
			if child.Type() == "parenthesized_declarator" {
				for j := 0; j < int(child.ChildCount()); j++ {
					subChild := child.Child(j)
					if subChild.Type() == "pointer_declarator" {
						for k := 0; k < int(subChild.ChildCount()); k++ {
							ptrChild := subChild.Child(k)
							if ptrChild.Type() == "identifier" {
								meta.FunctionName = string(source[ptrChild.StartByte():ptrChild.EndByte()])
								break
							}
						}
					}
				}
			}
		}

		// Extract parameters
		params := s.findChild(declarator, "parameter_list")
		if params != nil {
			meta.Parameters = s.extractParameters(params, source)
		}
	}

	// Extract return type (everything before the declarator)
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "primitive_type", "type_identifier", "sized_type_specifier":
			meta.ReturnType = string(source[child.StartByte():child.EndByte()])
		case "storage_class_specifier":
			text := string(source[child.StartByte():child.EndByte()])
			if text == "static" {
				meta.IsStatic = true
				meta.Visibility = "file"
			}
		}
	}

	// Default visibility for C is public (global)
	if meta.Visibility == "" {
		meta.Visibility = "public"
		meta.IsExported = true
	}

	// Build signature
	meta.Signature = s.buildFunctionSignature(meta)

	// Extract doc comment
	meta.Docstring = s.extractComment(node, source)
}

// extractStructMetadata extracts metadata from a struct specifier.
func (s *CStrategy) extractStructMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Find struct name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_identifier" {
			meta.ClassName = string(source[child.StartByte():child.EndByte()])
			break
		}
	}

	meta.Visibility = "public"
	meta.IsExported = true

	// Extract doc comment
	meta.Docstring = s.extractComment(node, source)
}

// extractUnionMetadata extracts metadata from a union specifier.
func (s *CStrategy) extractUnionMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Find union name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_identifier" {
			meta.ClassName = string(source[child.StartByte():child.EndByte()])
			break
		}
	}

	meta.Visibility = "public"
	meta.IsExported = true

	// Extract doc comment
	meta.Docstring = s.extractComment(node, source)
}

// extractEnumMetadata extracts metadata from an enum specifier.
func (s *CStrategy) extractEnumMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Find enum name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_identifier" {
			meta.ClassName = string(source[child.StartByte():child.EndByte()])
			break
		}
	}

	meta.Visibility = "public"
	meta.IsExported = true

	// Extract doc comment
	meta.Docstring = s.extractComment(node, source)
}

// extractTypedefMetadata extracts metadata from a typedef.
func (s *CStrategy) extractTypedefMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Find the new type name (last identifier before ;)
	var lastName string
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_identifier" {
			lastName = string(source[child.StartByte():child.EndByte()])
		}
	}
	meta.ClassName = lastName

	// Check if it typedefs a struct/union/enum
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "struct_specifier":
			s.extractStructMetadata(child, source, meta)
			if meta.ClassName == "" {
				meta.ClassName = lastName
			}
		case "union_specifier":
			s.extractUnionMetadata(child, source, meta)
			if meta.ClassName == "" {
				meta.ClassName = lastName
			}
		case "enum_specifier":
			s.extractEnumMetadata(child, source, meta)
			if meta.ClassName == "" {
				meta.ClassName = lastName
			}
		}
	}

	meta.Visibility = "public"
	meta.IsExported = true
}

// extractMacroMetadata extracts metadata from a macro function definition.
func (s *CStrategy) extractMacroMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Find macro name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			meta.FunctionName = string(source[child.StartByte():child.EndByte()])
			break
		}
	}

	// Extract parameters from preproc_params
	params := s.findChild(node, "preproc_params")
	if params != nil {
		for i := 0; i < int(params.ChildCount()); i++ {
			child := params.Child(i)
			if child.Type() == "identifier" {
				meta.Parameters = append(meta.Parameters, string(source[child.StartByte():child.EndByte()]))
			}
		}
	}

	meta.Visibility = "public"
	meta.IsExported = true

	// Build signature
	meta.Signature = "#define " + meta.FunctionName + "(" + strings.Join(meta.Parameters, ", ") + ")"
}

// extractParameters extracts parameter names from a parameter list.
func (s *CStrategy) extractParameters(params *sitter.Node, source []byte) []string {
	var result []string

	for i := 0; i < int(params.ChildCount()); i++ {
		child := params.Child(i)
		if child.Type() == "parameter_declaration" {
			var paramName string
			var paramType string

			for j := 0; j < int(child.ChildCount()); j++ {
				paramChild := child.Child(j)
				switch paramChild.Type() {
				case "identifier":
					paramName = string(source[paramChild.StartByte():paramChild.EndByte()])
				case "primitive_type", "type_identifier", "sized_type_specifier":
					paramType = string(source[paramChild.StartByte():paramChild.EndByte()])
				case "pointer_declarator":
					// Get name from pointer declarator
					for k := 0; k < int(paramChild.ChildCount()); k++ {
						ptrChild := paramChild.Child(k)
						if ptrChild.Type() == "identifier" {
							paramName = "*" + string(source[ptrChild.StartByte():ptrChild.EndByte()])
							break
						}
					}
				case "array_declarator":
					// Get name from array declarator
					for k := 0; k < int(paramChild.ChildCount()); k++ {
						arrChild := paramChild.Child(k)
						if arrChild.Type() == "identifier" {
							paramName = string(source[arrChild.StartByte():arrChild.EndByte()]) + "[]"
							break
						}
					}
				}
			}

			if paramName != "" {
				if paramType != "" {
					result = append(result, paramType+" "+paramName)
				} else {
					result = append(result, paramName)
				}
			} else if paramType != "" {
				// Parameter with just type (no name)
				result = append(result, paramType)
			}
		}
	}

	return result
}

// extractComment extracts comment preceding a node.
func (s *CStrategy) extractComment(node *sitter.Node, source []byte) string {
	prev := node.PrevSibling()
	if prev == nil {
		return ""
	}

	if prev.Type() == "comment" {
		comment := string(source[prev.StartByte():prev.EndByte()])
		// Handle /* */ comments
		if strings.HasPrefix(comment, "/*") {
			comment = strings.TrimPrefix(comment, "/*")
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
		// Handle // comments
		if strings.HasPrefix(comment, "//") {
			return strings.TrimSpace(strings.TrimPrefix(comment, "//"))
		}
	}

	return ""
}

// buildFunctionSignature builds a function signature string.
func (s *CStrategy) buildFunctionSignature(meta *chunkers.CodeMetadata) string {
	var sig strings.Builder

	if meta.IsStatic {
		sig.WriteString("static ")
	}
	if meta.ReturnType != "" {
		sig.WriteString(meta.ReturnType)
		sig.WriteString(" ")
	}
	sig.WriteString(meta.FunctionName)
	sig.WriteString("(")
	sig.WriteString(strings.Join(meta.Parameters, ", "))
	sig.WriteString(")")

	return sig.String()
}

// findChild finds the first child with the given type.
func (s *CStrategy) findChild(node *sitter.Node, nodeType string) *sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == nodeType {
			return child
		}
	}
	return nil
}

// Ensure CStrategy implements LanguageStrategy.
var _ treesitter.LanguageStrategy = (*CStrategy)(nil)
