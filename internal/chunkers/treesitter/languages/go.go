package languages

import (
	"strings"
	"unicode"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/chunkers/treesitter"
)

// GoStrategy implements tree-sitter parsing for Go code.
type GoStrategy struct{}

// NewGoStrategy creates a new Go language strategy.
func NewGoStrategy() *GoStrategy {
	return &GoStrategy{}
}

// Language returns the language identifier.
func (s *GoStrategy) Language() string {
	return "go"
}

// Extensions returns file extensions this strategy handles.
func (s *GoStrategy) Extensions() []string {
	return []string{".go"}
}

// MIMETypes returns MIME types this strategy handles.
func (s *GoStrategy) MIMETypes() []string {
	return []string{"text/x-go", "application/x-go"}
}

// GetLanguage returns the tree-sitter Language for Go.
func (s *GoStrategy) GetLanguage() *sitter.Language {
	return golang.GetLanguage()
}

// NodeTypes returns Go-specific node type configuration.
func (s *GoStrategy) NodeTypes() treesitter.NodeTypeConfig {
	return treesitter.NodeTypeConfig{
		Functions: []string{
			"function_declaration",
		},
		Methods: []string{
			"method_declaration",
		},
		Classes: []string{
			"type_declaration",
		},
		Declarations: []string{
			"const_declaration",
			"var_declaration",
		},
		TopLevel: []string{},
	}
}

// ShouldChunk determines if a node should be its own chunk.
func (s *GoStrategy) ShouldChunk(node *sitter.Node) bool {
	// Always chunk functions, methods, and type declarations
	nodeType := node.Type()
	switch nodeType {
	case "function_declaration", "method_declaration", "type_declaration":
		return true
	case "const_declaration", "var_declaration":
		// Only chunk top-level var/const declarations
		parent := node.Parent()
		return parent != nil && parent.Type() == "source_file"
	}
	return false
}

// ExtractMetadata extracts Go-specific metadata from an AST node.
func (s *GoStrategy) ExtractMetadata(node *sitter.Node, source []byte) *chunkers.CodeMetadata {
	meta := &chunkers.CodeMetadata{
		Language:  "go",
		LineStart: int(node.StartPoint().Row) + 1,
		LineEnd:   int(node.EndPoint().Row) + 1,
	}

	switch node.Type() {
	case "function_declaration":
		s.extractFunctionMetadata(node, source, meta)
	case "method_declaration":
		s.extractMethodMetadata(node, source, meta)
	case "type_declaration":
		s.extractTypeMetadata(node, source, meta)
	}

	return meta
}

// extractFunctionMetadata extracts metadata from a function declaration.
func (s *GoStrategy) extractFunctionMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Find function name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			meta.FunctionName = string(source[child.StartByte():child.EndByte()])
			meta.IsExported = isExported(meta.FunctionName)
			meta.Visibility = "package"
			if meta.IsExported {
				meta.Visibility = "public"
			}
			break
		}
	}

	// Extract signature
	meta.Signature = s.extractSignature(node, source)

	// Extract parameters
	params := s.findChild(node, "parameter_list")
	if params != nil {
		meta.Parameters = s.extractParameters(params, source)
	}

	// Extract return type
	result := s.findChild(node, "result")
	if result != nil {
		meta.ReturnType = strings.TrimSpace(string(source[result.StartByte():result.EndByte()]))
	}

	// Check for preceding doc comment
	meta.Docstring = s.extractDocComment(node, source)
}

// extractMethodMetadata extracts metadata from a method declaration.
func (s *GoStrategy) extractMethodMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// First extract as function
	s.extractFunctionMetadata(node, source, meta)

	// Extract receiver (class name)
	receiver := s.findChild(node, "parameter_list")
	if receiver != nil {
		// First parameter_list is the receiver
		receiverText := string(source[receiver.StartByte():receiver.EndByte()])
		// Extract type from receiver, handling pointer types
		receiverText = strings.Trim(receiverText, "()")
		parts := strings.Fields(receiverText)
		if len(parts) >= 2 {
			typeName := parts[len(parts)-1]
			typeName = strings.TrimPrefix(typeName, "*")
			meta.ClassName = typeName
		} else if len(parts) == 1 {
			typeName := parts[0]
			typeName = strings.TrimPrefix(typeName, "*")
			meta.ClassName = typeName
		}
	}
}

// extractTypeMetadata extracts metadata from a type declaration.
func (s *GoStrategy) extractTypeMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Find type_spec within type_declaration
	typeSpec := s.findChild(node, "type_spec")
	if typeSpec == nil {
		return
	}

	// Get type name
	for i := 0; i < int(typeSpec.ChildCount()); i++ {
		child := typeSpec.Child(i)
		if child.Type() == "type_identifier" {
			meta.ClassName = string(source[child.StartByte():child.EndByte()])
			meta.IsExported = isExported(meta.ClassName)
			meta.Visibility = "package"
			if meta.IsExported {
				meta.Visibility = "public"
			}
			break
		}
	}

	// Check for preceding doc comment
	meta.Docstring = s.extractDocComment(node, source)
}

// extractSignature extracts the function/method signature.
func (s *GoStrategy) extractSignature(node *sitter.Node, source []byte) string {
	// Build signature from func name + params + result
	var sig strings.Builder
	sig.WriteString("func ")

	// Get name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "identifier":
			sig.WriteString(string(source[child.StartByte():child.EndByte()]))
		case "parameter_list":
			sig.WriteString(string(source[child.StartByte():child.EndByte()]))
		case "result":
			sig.WriteString(" ")
			sig.WriteString(string(source[child.StartByte():child.EndByte()]))
		}
	}

	return sig.String()
}

// extractParameters extracts parameter names from a parameter list.
func (s *GoStrategy) extractParameters(params *sitter.Node, source []byte) []string {
	var result []string

	cursor := sitter.NewTreeCursor(params)
	defer cursor.Close()

	if cursor.GoToFirstChild() {
		for {
			node := cursor.CurrentNode()
			if node.Type() == "parameter_declaration" {
				// Get parameter name(s)
				for i := 0; i < int(node.ChildCount()); i++ {
					child := node.Child(i)
					if child.Type() == "identifier" {
						result = append(result, string(source[child.StartByte():child.EndByte()]))
					}
				}
			}
			if !cursor.GoToNextSibling() {
				break
			}
		}
	}

	return result
}

// extractDocComment extracts the doc comment preceding a node.
func (s *GoStrategy) extractDocComment(node *sitter.Node, source []byte) string {
	// Look for comment nodes immediately before this node
	prev := node.PrevSibling()
	if prev == nil {
		return ""
	}

	if prev.Type() == "comment" {
		comment := string(source[prev.StartByte():prev.EndByte()])
		// Check if it's directly adjacent (doc comment)
		if int(node.StartPoint().Row)-int(prev.EndPoint().Row) <= 1 {
			return strings.TrimPrefix(strings.TrimPrefix(comment, "//"), " ")
		}
	}

	return ""
}

// findChild finds the first child with the given type.
func (s *GoStrategy) findChild(node *sitter.Node, nodeType string) *sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == nodeType {
			return child
		}
	}
	return nil
}

// isExported returns true if the identifier is exported (starts with uppercase).
func isExported(name string) bool {
	if name == "" {
		return false
	}
	r := []rune(name)
	return unicode.IsUpper(r[0])
}

// Ensure GoStrategy implements LanguageStrategy.
var _ treesitter.LanguageStrategy = (*GoStrategy)(nil)
