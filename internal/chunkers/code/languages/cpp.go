package languages

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/cpp"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/chunkers/code"
)

// CPPStrategy implements tree-sitter parsing for C++ code.
type CPPStrategy struct{}

// NewCPPStrategy creates a new C++ language strategy.
func NewCPPStrategy() *CPPStrategy {
	return &CPPStrategy{}
}

// Language returns the language identifier.
func (s *CPPStrategy) Language() string {
	return "cpp"
}

// Extensions returns file extensions this strategy handles.
func (s *CPPStrategy) Extensions() []string {
	return []string{".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx", ".h++", ".c++"}
}

// MIMETypes returns MIME types this strategy handles.
func (s *CPPStrategy) MIMETypes() []string {
	return []string{
		"text/x-c++",
		"text/x-c++src",
		"text/x-c++hdr",
	}
}

// GetLanguage returns the tree-sitter Language for C++.
func (s *CPPStrategy) GetLanguage() *sitter.Language {
	return cpp.GetLanguage()
}

// NodeTypes returns C++-specific node type configuration.
func (s *CPPStrategy) NodeTypes() code.NodeTypeConfig {
	return code.NodeTypeConfig{
		Functions: []string{
			"function_definition",
		},
		Methods: []string{},
		Classes: []string{
			"class_specifier",
			"struct_specifier",
			"union_specifier",
			"enum_specifier",
			"namespace_definition",
		},
		Declarations: []string{
			"template_declaration",
			"alias_declaration",
		},
		TopLevel: []string{},
	}
}

// ShouldChunk determines if a node should be its own chunk.
func (s *CPPStrategy) ShouldChunk(node *sitter.Node) bool {
	nodeType := node.Type()
	switch nodeType {
	case "function_definition":
		return true
	case "class_specifier", "struct_specifier", "union_specifier":
		// Only chunk if it has a body
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "field_declaration_list" {
				return true
			}
		}
		return false
	case "enum_specifier":
		// Only chunk if it has a body
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "enumerator_list" {
				return true
			}
		}
		return false
	case "namespace_definition":
		return true
	case "template_declaration":
		// Chunk if it contains a function or class
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			switch child.Type() {
			case "function_definition", "class_specifier", "struct_specifier":
				return true
			}
		}
		return false
	}
	return false
}

// ExtractMetadata extracts C++-specific metadata from an AST node.
func (s *CPPStrategy) ExtractMetadata(node *sitter.Node, source []byte) *chunkers.CodeMetadata {
	meta := &chunkers.CodeMetadata{
		Language:  "cpp",
		LineStart: int(node.StartPoint().Row) + 1,
		LineEnd:   int(node.EndPoint().Row) + 1,
	}

	switch node.Type() {
	case "function_definition":
		s.extractFunctionMetadata(node, source, meta)
	case "class_specifier":
		s.extractClassMetadata(node, source, meta)
	case "struct_specifier":
		s.extractStructMetadata(node, source, meta)
	case "union_specifier":
		s.extractUnionMetadata(node, source, meta)
	case "enum_specifier":
		s.extractEnumMetadata(node, source, meta)
	case "namespace_definition":
		s.extractNamespaceMetadata(node, source, meta)
	case "template_declaration":
		s.extractTemplateMetadata(node, source, meta)
	}

	return meta
}

// extractFunctionMetadata extracts metadata from a function definition.
func (s *CPPStrategy) extractFunctionMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Check if this is a method (inside a class body)
	parent := node.Parent()
	if parent != nil && parent.Type() == "field_declaration_list" {
		grandparent := parent.Parent()
		if grandparent != nil {
			switch grandparent.Type() {
			case "class_specifier", "struct_specifier":
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

	// Look for declarator which contains the function name
	declarator := s.findChild(node, "function_declarator")
	if declarator == nil {
		// Try to find it in a reference_declarator or pointer_declarator
		refDecl := s.findChild(node, "reference_declarator")
		if refDecl != nil {
			declarator = s.findChild(refDecl, "function_declarator")
		}
		if declarator == nil {
			ptrDecl := s.findChild(node, "pointer_declarator")
			if ptrDecl != nil {
				declarator = s.findChild(ptrDecl, "function_declarator")
			}
		}
	}

	if declarator != nil {
		// Get function name from declarator
		for i := 0; i < int(declarator.ChildCount()); i++ {
			child := declarator.Child(i)
			switch child.Type() {
			case "identifier":
				meta.FunctionName = string(source[child.StartByte():child.EndByte()])
			case "qualified_identifier", "field_identifier":
				meta.FunctionName = string(source[child.StartByte():child.EndByte()])
			case "destructor_name":
				meta.FunctionName = string(source[child.StartByte():child.EndByte()])
			case "operator_name":
				meta.FunctionName = string(source[child.StartByte():child.EndByte()])
			}
		}

		// Extract parameters
		params := s.findChild(declarator, "parameter_list")
		if params != nil {
			meta.Parameters = s.extractParameters(params, source)
		}
	}

	// Check for virtual, static, const qualifiers
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "virtual_function_specifier":
			// Could add IsVirtual field
		case "storage_class_specifier":
			text := string(source[child.StartByte():child.EndByte()])
			if text == "static" {
				meta.IsStatic = true
			}
		case "primitive_type", "type_identifier", "qualified_identifier":
			meta.ReturnType = string(source[child.StartByte():child.EndByte()])
		}
	}

	// Check for access specifier in parent context
	meta.Visibility = s.getAccessSpecifier(node, source)

	// Build signature
	meta.Signature = s.buildFunctionSignature(meta)

	// Extract doc comment
	meta.Docstring = s.extractComment(node, source)
}

// extractClassMetadata extracts metadata from a class specifier.
func (s *CPPStrategy) extractClassMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Find class name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_identifier" {
			meta.ClassName = string(source[child.StartByte():child.EndByte()])
			break
		}
	}

	// Extract base classes
	baseClause := s.findChild(node, "base_class_clause")
	if baseClause != nil {
		for i := 0; i < int(baseClause.ChildCount()); i++ {
			child := baseClause.Child(i)
			if child.Type() == "base_class_name" || child.Type() == "type_identifier" {
				baseName := string(source[child.StartByte():child.EndByte()])
				if meta.ParentClass == "" {
					meta.ParentClass = baseName
				}
				meta.Implements = append(meta.Implements, baseName)
			}
		}
	}

	meta.Visibility = "public"
	meta.IsExported = true

	// Extract doc comment
	meta.Docstring = s.extractComment(node, source)
}

// extractStructMetadata extracts metadata from a struct specifier.
func (s *CPPStrategy) extractStructMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Structs are similar to classes in C++
	s.extractClassMetadata(node, source, meta)
}

// extractUnionMetadata extracts metadata from a union specifier.
func (s *CPPStrategy) extractUnionMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
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
func (s *CPPStrategy) extractEnumMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
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

// extractNamespaceMetadata extracts metadata from a namespace definition.
func (s *CPPStrategy) extractNamespaceMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Find namespace name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "identifier", "namespace_identifier":
			meta.Namespace = string(source[child.StartByte():child.EndByte()])
		}
	}

	meta.Visibility = "public"
	meta.IsExported = true
}

// extractTemplateMetadata extracts metadata from a template declaration.
func (s *CPPStrategy) extractTemplateMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Extract template parameters
	params := s.findChild(node, "template_parameter_list")
	if params != nil {
		for i := 0; i < int(params.ChildCount()); i++ {
			child := params.Child(i)
			switch child.Type() {
			case "type_parameter_declaration":
				for j := 0; j < int(child.ChildCount()); j++ {
					paramChild := child.Child(j)
					if paramChild.Type() == "type_identifier" {
						meta.Parameters = append(meta.Parameters, string(source[paramChild.StartByte():paramChild.EndByte()]))
					}
				}
			}
		}
	}

	// Process the templated declaration
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "function_definition":
			s.extractFunctionMetadata(child, source, meta)
		case "class_specifier":
			s.extractClassMetadata(child, source, meta)
		case "struct_specifier":
			s.extractStructMetadata(child, source, meta)
		}
	}

	// Add template prefix to signature
	if meta.Signature != "" && len(meta.Parameters) > 0 {
		meta.Signature = "template<" + strings.Join(meta.Parameters, ", ") + "> " + meta.Signature
		meta.Parameters = nil // Clear parameters as they're now in signature
	}
}

// getAccessSpecifier determines the access specifier for a node.
func (s *CPPStrategy) getAccessSpecifier(node *sitter.Node, source []byte) string {
	// Look for access specifier in previous siblings
	prev := node.PrevSibling()
	for prev != nil {
		if prev.Type() == "access_specifier" {
			text := string(source[prev.StartByte():prev.EndByte()])
			text = strings.TrimSuffix(text, ":")
			return text
		}
		prev = prev.PrevSibling()
	}

	// Check parent type for default access
	parent := node.Parent()
	if parent != nil {
		grandparent := parent.Parent()
		if grandparent != nil {
			switch grandparent.Type() {
			case "class_specifier":
				return "private" // Default for class
			case "struct_specifier":
				return "public" // Default for struct
			}
		}
	}

	return "public"
}

// extractParameters extracts parameter names from a parameter list.
func (s *CPPStrategy) extractParameters(params *sitter.Node, source []byte) []string {
	var result []string

	for i := 0; i < int(params.ChildCount()); i++ {
		child := params.Child(i)
		if child.Type() == "parameter_declaration" || child.Type() == "optional_parameter_declaration" {
			var paramName string
			var paramType string

			for j := 0; j < int(child.ChildCount()); j++ {
				paramChild := child.Child(j)
				switch paramChild.Type() {
				case "identifier":
					paramName = string(source[paramChild.StartByte():paramChild.EndByte()])
				case "primitive_type", "type_identifier", "qualified_identifier":
					paramType = string(source[paramChild.StartByte():paramChild.EndByte()])
				case "reference_declarator", "pointer_declarator":
					// Get name from declarator
					for k := 0; k < int(paramChild.ChildCount()); k++ {
						declChild := paramChild.Child(k)
						if declChild.Type() == "identifier" {
							if paramChild.Type() == "reference_declarator" {
								paramName = "&" + string(source[declChild.StartByte():declChild.EndByte()])
							} else {
								paramName = "*" + string(source[declChild.StartByte():declChild.EndByte()])
							}
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
				result = append(result, paramType)
			}
		}
	}

	return result
}

// extractComment extracts comment preceding a node.
func (s *CPPStrategy) extractComment(node *sitter.Node, source []byte) string {
	prev := node.PrevSibling()
	if prev == nil {
		return ""
	}

	if prev.Type() == "comment" {
		comment := string(source[prev.StartByte():prev.EndByte()])
		// Handle /* */ comments and Doxygen comments
		if strings.HasPrefix(comment, "/*") {
			comment = strings.TrimPrefix(comment, "/**")
			comment = strings.TrimPrefix(comment, "/*")
			comment = strings.TrimSuffix(comment, "*/")
			lines := strings.Split(comment, "\n")
			var cleaned []string
			for _, line := range lines {
				line = strings.TrimSpace(line)
				line = strings.TrimPrefix(line, "*")
				line = strings.TrimSpace(line)
				// Skip Doxygen tags for summary
				if !strings.HasPrefix(line, "@") && !strings.HasPrefix(line, "\\") && line != "" {
					cleaned = append(cleaned, line)
				}
			}
			return strings.Join(cleaned, " ")
		}
		// Handle // comments
		if strings.HasPrefix(comment, "///") {
			return strings.TrimSpace(strings.TrimPrefix(comment, "///"))
		}
		if strings.HasPrefix(comment, "//") {
			return strings.TrimSpace(strings.TrimPrefix(comment, "//"))
		}
	}

	return ""
}

// buildFunctionSignature builds a function signature string.
func (s *CPPStrategy) buildFunctionSignature(meta *chunkers.CodeMetadata) string {
	var sig strings.Builder

	if meta.Visibility != "" && meta.Visibility != "public" {
		sig.WriteString(meta.Visibility)
		sig.WriteString(" ")
	}
	if meta.IsStatic {
		sig.WriteString("static ")
	}
	if meta.ReturnType != "" {
		sig.WriteString(meta.ReturnType)
		sig.WriteString(" ")
	}
	if meta.ClassName != "" {
		sig.WriteString(meta.ClassName)
		sig.WriteString("::")
	}
	sig.WriteString(meta.FunctionName)
	sig.WriteString("(")
	sig.WriteString(strings.Join(meta.Parameters, ", "))
	sig.WriteString(")")

	return sig.String()
}

// findChild finds the first child with the given type.
func (s *CPPStrategy) findChild(node *sitter.Node, nodeType string) *sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == nodeType {
			return child
		}
	}
	return nil
}

// Ensure CPPStrategy implements LanguageStrategy.
var _ code.LanguageStrategy = (*CPPStrategy)(nil)
