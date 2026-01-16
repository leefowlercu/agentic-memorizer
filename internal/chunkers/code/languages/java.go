package languages

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/chunkers/code"
)

// JavaStrategy implements tree-sitter parsing for Java code.
type JavaStrategy struct{}

// NewJavaStrategy creates a new Java language strategy.
func NewJavaStrategy() *JavaStrategy {
	return &JavaStrategy{}
}

// Language returns the language identifier.
func (s *JavaStrategy) Language() string {
	return "java"
}

// Extensions returns file extensions this strategy handles.
func (s *JavaStrategy) Extensions() []string {
	return []string{".java"}
}

// MIMETypes returns MIME types this strategy handles.
func (s *JavaStrategy) MIMETypes() []string {
	return []string{
		"text/x-java",
		"text/x-java-source",
		"application/x-java",
	}
}

// GetLanguage returns the tree-sitter Language for Java.
func (s *JavaStrategy) GetLanguage() *sitter.Language {
	return java.GetLanguage()
}

// NodeTypes returns Java-specific node type configuration.
func (s *JavaStrategy) NodeTypes() code.NodeTypeConfig {
	return code.NodeTypeConfig{
		Functions: []string{},
		Methods: []string{
			"method_declaration",
			"constructor_declaration",
		},
		Classes: []string{
			"class_declaration",
			"interface_declaration",
			"enum_declaration",
			"record_declaration",
			"annotation_type_declaration",
		},
		Declarations: []string{
			"field_declaration",
		},
		TopLevel: []string{},
	}
}

// ShouldChunk determines if a node should be its own chunk.
func (s *JavaStrategy) ShouldChunk(node *sitter.Node) bool {
	nodeType := node.Type()
	switch nodeType {
	case "class_declaration", "interface_declaration", "enum_declaration",
		"record_declaration", "annotation_type_declaration":
		return true
	case "method_declaration", "constructor_declaration":
		// Only chunk if inside a class body
		parent := node.Parent()
		if parent != nil && parent.Type() == "class_body" {
			return true
		}
		return false
	}
	return false
}

// ExtractMetadata extracts Java-specific metadata from an AST node.
func (s *JavaStrategy) ExtractMetadata(node *sitter.Node, source []byte) *chunkers.CodeMetadata {
	meta := &chunkers.CodeMetadata{
		Language:  "java",
		LineStart: int(node.StartPoint().Row) + 1,
		LineEnd:   int(node.EndPoint().Row) + 1,
	}

	switch node.Type() {
	case "class_declaration":
		s.extractClassMetadata(node, source, meta)
	case "interface_declaration":
		s.extractInterfaceMetadata(node, source, meta)
	case "enum_declaration":
		s.extractEnumMetadata(node, source, meta)
	case "record_declaration":
		s.extractRecordMetadata(node, source, meta)
	case "method_declaration":
		s.extractMethodMetadata(node, source, meta)
	case "constructor_declaration":
		s.extractConstructorMetadata(node, source, meta)
	}

	return meta
}

// extractClassMetadata extracts metadata from a class declaration.
func (s *JavaStrategy) extractClassMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Extract modifiers
	modifiers := s.findChild(node, "modifiers")
	if modifiers != nil {
		s.extractModifiers(modifiers, source, meta)
	}

	// Find class name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			meta.ClassName = string(source[child.StartByte():child.EndByte()])
			break
		}
	}

	// Extract superclass
	superclass := s.findChild(node, "superclass")
	if superclass != nil {
		for i := 0; i < int(superclass.ChildCount()); i++ {
			child := superclass.Child(i)
			if child.Type() == "type_identifier" {
				meta.ParentClass = string(source[child.StartByte():child.EndByte()])
				break
			}
		}
	}

	// Extract interfaces
	interfaces := s.findChild(node, "super_interfaces")
	if interfaces != nil {
		s.extractInterfaceList(interfaces, source, meta)
	}

	// Extract JavaDoc
	meta.Docstring = s.extractJavaDoc(node, source)

	// Extract annotations as decorators
	meta.Decorators = s.extractAnnotations(node, source)
}

// extractInterfaceMetadata extracts metadata from an interface declaration.
func (s *JavaStrategy) extractInterfaceMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Extract modifiers
	modifiers := s.findChild(node, "modifiers")
	if modifiers != nil {
		s.extractModifiers(modifiers, source, meta)
	}

	// Find interface name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			meta.ClassName = string(source[child.StartByte():child.EndByte()])
			break
		}
	}

	// Extract extended interfaces
	extendsInterfaces := s.findChild(node, "extends_interfaces")
	if extendsInterfaces != nil {
		s.extractInterfaceList(extendsInterfaces, source, meta)
	}

	// Extract JavaDoc
	meta.Docstring = s.extractJavaDoc(node, source)

	// Extract annotations
	meta.Decorators = s.extractAnnotations(node, source)
}

// extractEnumMetadata extracts metadata from an enum declaration.
func (s *JavaStrategy) extractEnumMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Extract modifiers
	modifiers := s.findChild(node, "modifiers")
	if modifiers != nil {
		s.extractModifiers(modifiers, source, meta)
	}

	// Find enum name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			meta.ClassName = string(source[child.StartByte():child.EndByte()])
			break
		}
	}

	// Extract interfaces
	interfaces := s.findChild(node, "super_interfaces")
	if interfaces != nil {
		s.extractInterfaceList(interfaces, source, meta)
	}

	// Extract JavaDoc
	meta.Docstring = s.extractJavaDoc(node, source)

	// Extract annotations
	meta.Decorators = s.extractAnnotations(node, source)
}

// extractRecordMetadata extracts metadata from a record declaration.
func (s *JavaStrategy) extractRecordMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Extract modifiers
	modifiers := s.findChild(node, "modifiers")
	if modifiers != nil {
		s.extractModifiers(modifiers, source, meta)
	}

	// Find record name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			meta.ClassName = string(source[child.StartByte():child.EndByte()])
			break
		}
	}

	// Extract interfaces
	interfaces := s.findChild(node, "super_interfaces")
	if interfaces != nil {
		s.extractInterfaceList(interfaces, source, meta)
	}

	// Extract JavaDoc
	meta.Docstring = s.extractJavaDoc(node, source)

	// Extract annotations
	meta.Decorators = s.extractAnnotations(node, source)
}

// extractMethodMetadata extracts metadata from a method declaration.
func (s *JavaStrategy) extractMethodMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Get class name from parent
	parent := node.Parent()
	if parent != nil && parent.Type() == "class_body" {
		grandparent := parent.Parent()
		if grandparent != nil {
			for i := 0; i < int(grandparent.ChildCount()); i++ {
				child := grandparent.Child(i)
				if child.Type() == "identifier" {
					meta.ClassName = string(source[child.StartByte():child.EndByte()])
					break
				}
			}
		}
	}

	// Extract modifiers
	modifiers := s.findChild(node, "modifiers")
	if modifiers != nil {
		s.extractModifiers(modifiers, source, meta)
	}

	// Find method name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			meta.FunctionName = string(source[child.StartByte():child.EndByte()])
			break
		}
	}

	// Extract return type
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "type_identifier", "void_type", "generic_type", "array_type", "integral_type", "floating_point_type", "boolean_type":
			meta.ReturnType = string(source[child.StartByte():child.EndByte()])
		}
	}

	// Extract parameters
	params := s.findChild(node, "formal_parameters")
	if params != nil {
		meta.Parameters = s.extractParameters(params, source)
	}

	// Build signature
	meta.Signature = s.buildMethodSignature(meta)

	// Extract JavaDoc
	meta.Docstring = s.extractJavaDoc(node, source)

	// Extract annotations
	meta.Decorators = s.extractAnnotations(node, source)
}

// extractConstructorMetadata extracts metadata from a constructor declaration.
func (s *JavaStrategy) extractConstructorMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Get class name from parent
	parent := node.Parent()
	if parent != nil && parent.Type() == "class_body" {
		grandparent := parent.Parent()
		if grandparent != nil {
			for i := 0; i < int(grandparent.ChildCount()); i++ {
				child := grandparent.Child(i)
				if child.Type() == "identifier" {
					meta.ClassName = string(source[child.StartByte():child.EndByte()])
					meta.FunctionName = meta.ClassName // Constructor has same name as class
					break
				}
			}
		}
	}

	meta.IsConstructor = true

	// Extract modifiers
	modifiers := s.findChild(node, "modifiers")
	if modifiers != nil {
		s.extractModifiers(modifiers, source, meta)
	}

	// Extract parameters
	params := s.findChild(node, "formal_parameters")
	if params != nil {
		meta.Parameters = s.extractParameters(params, source)
	}

	// Build signature
	meta.Signature = s.buildConstructorSignature(meta)

	// Extract JavaDoc
	meta.Docstring = s.extractJavaDoc(node, source)

	// Extract annotations
	meta.Decorators = s.extractAnnotations(node, source)
}

// extractModifiers extracts visibility and other modifiers.
func (s *JavaStrategy) extractModifiers(modifiers *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	for i := 0; i < int(modifiers.ChildCount()); i++ {
		child := modifiers.Child(i)
		text := string(source[child.StartByte():child.EndByte()])
		switch text {
		case "public":
			meta.Visibility = "public"
			meta.IsExported = true
		case "private":
			meta.Visibility = "private"
		case "protected":
			meta.Visibility = "protected"
		case "static":
			meta.IsStatic = true
		case "abstract":
			// Could add IsAbstract field
		case "final":
			// Could add IsFinal field
		case "synchronized":
			meta.IsAsync = true // Using IsAsync for synchronized
		}
	}
	if meta.Visibility == "" {
		meta.Visibility = "package" // Default package-private
	}
}

// extractInterfaceList extracts implemented/extended interfaces.
func (s *JavaStrategy) extractInterfaceList(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "type_list" {
			for j := 0; j < int(child.ChildCount()); j++ {
				typeChild := child.Child(j)
				if typeChild.Type() == "type_identifier" || typeChild.Type() == "generic_type" {
					meta.Implements = append(meta.Implements, string(source[typeChild.StartByte():typeChild.EndByte()]))
				}
			}
		}
	}
}

// extractParameters extracts parameter names from formal parameters.
func (s *JavaStrategy) extractParameters(params *sitter.Node, source []byte) []string {
	var result []string

	for i := 0; i < int(params.ChildCount()); i++ {
		child := params.Child(i)
		if child.Type() == "formal_parameter" || child.Type() == "spread_parameter" {
			var paramName string
			var paramType string

			for j := 0; j < int(child.ChildCount()); j++ {
				paramChild := child.Child(j)
				switch paramChild.Type() {
				case "identifier":
					paramName = string(source[paramChild.StartByte():paramChild.EndByte()])
				case "type_identifier", "generic_type", "array_type", "integral_type", "floating_point_type", "boolean_type":
					paramType = string(source[paramChild.StartByte():paramChild.EndByte()])
				}
			}

			if paramName != "" {
				if paramType != "" {
					result = append(result, paramType+" "+paramName)
				} else {
					result = append(result, paramName)
				}
			}
		}
	}

	return result
}

// extractAnnotations extracts annotation names preceding a declaration.
func (s *JavaStrategy) extractAnnotations(node *sitter.Node, source []byte) []string {
	var annotations []string

	// Look for modifiers containing annotations
	modifiers := s.findChild(node, "modifiers")
	if modifiers != nil {
		for i := 0; i < int(modifiers.ChildCount()); i++ {
			child := modifiers.Child(i)
			if child.Type() == "marker_annotation" || child.Type() == "annotation" {
				for j := 0; j < int(child.ChildCount()); j++ {
					annChild := child.Child(j)
					if annChild.Type() == "identifier" {
						annotations = append(annotations, string(source[annChild.StartByte():annChild.EndByte()]))
						break
					}
				}
			}
		}
	}

	return annotations
}

// extractJavaDoc extracts JavaDoc comment preceding a node.
func (s *JavaStrategy) extractJavaDoc(node *sitter.Node, source []byte) string {
	prev := node.PrevSibling()
	if prev == nil {
		return ""
	}

	// Look for block comment that looks like JavaDoc
	if prev.Type() == "block_comment" {
		comment := string(source[prev.StartByte():prev.EndByte()])
		if strings.HasPrefix(comment, "/**") {
			// Clean up the comment
			comment = strings.TrimPrefix(comment, "/**")
			comment = strings.TrimSuffix(comment, "*/")
			lines := strings.Split(comment, "\n")
			var cleaned []string
			for _, line := range lines {
				line = strings.TrimSpace(line)
				line = strings.TrimPrefix(line, "*")
				line = strings.TrimSpace(line)
				// Skip @param, @return tags for summary
				if !strings.HasPrefix(line, "@") && line != "" {
					cleaned = append(cleaned, line)
				}
			}
			return strings.Join(cleaned, " ")
		}
	}

	return ""
}

// buildMethodSignature builds a method signature string.
func (s *JavaStrategy) buildMethodSignature(meta *chunkers.CodeMetadata) string {
	var sig strings.Builder

	if meta.Visibility != "" && meta.Visibility != "package" {
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
	sig.WriteString(meta.FunctionName)
	sig.WriteString("(")
	sig.WriteString(strings.Join(meta.Parameters, ", "))
	sig.WriteString(")")

	return sig.String()
}

// buildConstructorSignature builds a constructor signature string.
func (s *JavaStrategy) buildConstructorSignature(meta *chunkers.CodeMetadata) string {
	var sig strings.Builder

	if meta.Visibility != "" && meta.Visibility != "package" {
		sig.WriteString(meta.Visibility)
		sig.WriteString(" ")
	}
	sig.WriteString(meta.ClassName)
	sig.WriteString("(")
	sig.WriteString(strings.Join(meta.Parameters, ", "))
	sig.WriteString(")")

	return sig.String()
}

// findChild finds the first child with the given type.
func (s *JavaStrategy) findChild(node *sitter.Node, nodeType string) *sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == nodeType {
			return child
		}
	}
	return nil
}

// Ensure JavaStrategy implements LanguageStrategy.
var _ code.LanguageStrategy = (*JavaStrategy)(nil)
