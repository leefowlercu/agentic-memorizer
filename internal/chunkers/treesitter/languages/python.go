package languages

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/chunkers/treesitter"
)

// PythonStrategy implements tree-sitter parsing for Python code.
type PythonStrategy struct{}

// NewPythonStrategy creates a new Python language strategy.
func NewPythonStrategy() *PythonStrategy {
	return &PythonStrategy{}
}

// Language returns the language identifier.
func (s *PythonStrategy) Language() string {
	return "python"
}

// Extensions returns file extensions this strategy handles.
func (s *PythonStrategy) Extensions() []string {
	return []string{".py", ".pyw", ".pyi"}
}

// MIMETypes returns MIME types this strategy handles.
func (s *PythonStrategy) MIMETypes() []string {
	return []string{"text/x-python", "application/x-python"}
}

// GetLanguage returns the tree-sitter Language for Python.
func (s *PythonStrategy) GetLanguage() *sitter.Language {
	return python.GetLanguage()
}

// NodeTypes returns Python-specific node type configuration.
func (s *PythonStrategy) NodeTypes() treesitter.NodeTypeConfig {
	return treesitter.NodeTypeConfig{
		Functions: []string{
			"function_definition",
		},
		Methods: []string{}, // Methods are function_definition inside class_definition
		Classes: []string{
			"class_definition",
		},
		Declarations: []string{},
		TopLevel:     []string{},
	}
}

// ShouldChunk determines if a node should be its own chunk.
func (s *PythonStrategy) ShouldChunk(node *sitter.Node) bool {
	nodeType := node.Type()
	switch nodeType {
	case "function_definition":
		// Chunk top-level functions and methods
		parent := node.Parent()
		if parent == nil {
			return true
		}
		switch parent.Type() {
		case "module", "block":
			// Check if block's parent is class_definition
			grandparent := parent.Parent()
			if grandparent != nil && grandparent.Type() == "class_definition" {
				return true // It's a method
			}
			return parent.Type() == "module" // Top-level function
		}
		return false
	case "class_definition":
		return true
	}
	return false
}

// ExtractMetadata extracts Python-specific metadata from an AST node.
func (s *PythonStrategy) ExtractMetadata(node *sitter.Node, source []byte) *chunkers.CodeMetadata {
	meta := &chunkers.CodeMetadata{
		Language:  "python",
		LineStart: int(node.StartPoint().Row) + 1,
		LineEnd:   int(node.EndPoint().Row) + 1,
	}

	switch node.Type() {
	case "function_definition":
		s.extractFunctionMetadata(node, source, meta)
	case "class_definition":
		s.extractClassMetadata(node, source, meta)
	}

	return meta
}

// extractFunctionMetadata extracts metadata from a function definition.
func (s *PythonStrategy) extractFunctionMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Check if this is a method (inside a class)
	parent := node.Parent()
	if parent != nil && parent.Type() == "block" {
		grandparent := parent.Parent()
		if grandparent != nil && grandparent.Type() == "class_definition" {
			// Extract class name
			for i := 0; i < int(grandparent.ChildCount()); i++ {
				child := grandparent.Child(i)
				if child.Type() == "identifier" {
					meta.ClassName = string(source[child.StartByte():child.EndByte()])
					break
				}
			}
		}
	}

	// Find function name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			meta.FunctionName = string(source[child.StartByte():child.EndByte()])
			// Python visibility convention: _private, __dunder__
			meta.IsExported = !strings.HasPrefix(meta.FunctionName, "_") ||
				(strings.HasPrefix(meta.FunctionName, "__") && strings.HasSuffix(meta.FunctionName, "__"))
			meta.Visibility = "public"
			if strings.HasPrefix(meta.FunctionName, "_") && !strings.HasPrefix(meta.FunctionName, "__") {
				meta.Visibility = "private"
			}
			break
		}
	}

	// Check for async
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "async" {
			meta.IsAsync = true
			break
		}
	}

	// Extract decorators
	meta.Decorators = s.extractDecorators(node, source)

	// Check for @staticmethod or @classmethod
	for _, dec := range meta.Decorators {
		if dec == "staticmethod" {
			meta.IsStatic = true
		}
	}

	// Extract parameters
	params := s.findChild(node, "parameters")
	if params != nil {
		meta.Parameters = s.extractParameters(params, source)
	}

	// Extract return type annotation
	returnType := s.findChild(node, "type")
	if returnType != nil {
		meta.ReturnType = string(source[returnType.StartByte():returnType.EndByte()])
	}

	// Extract docstring
	body := s.findChild(node, "block")
	if body != nil {
		meta.Docstring = s.extractDocstring(body, source)
	}

	// Build signature
	meta.Signature = s.buildSignature(meta)
}

// extractClassMetadata extracts metadata from a class definition.
func (s *PythonStrategy) extractClassMetadata(node *sitter.Node, source []byte, meta *chunkers.CodeMetadata) {
	// Find class name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			meta.ClassName = string(source[child.StartByte():child.EndByte()])
			meta.IsExported = !strings.HasPrefix(meta.ClassName, "_")
			meta.Visibility = "public"
			if strings.HasPrefix(meta.ClassName, "_") {
				meta.Visibility = "private"
			}
			break
		}
	}

	// Extract decorators
	meta.Decorators = s.extractDecorators(node, source)

	// Extract parent classes
	argList := s.findChild(node, "argument_list")
	if argList != nil {
		meta.Implements = s.extractBaseClasses(argList, source)
		if len(meta.Implements) > 0 {
			meta.ParentClass = meta.Implements[0]
		}
	}

	// Extract docstring
	body := s.findChild(node, "block")
	if body != nil {
		meta.Docstring = s.extractDocstring(body, source)
	}
}

// extractDecorators extracts decorator names from a decorated definition.
func (s *PythonStrategy) extractDecorators(node *sitter.Node, source []byte) []string {
	var decorators []string

	// Look for decorator siblings before this node
	prev := node.PrevSibling()
	for prev != nil && prev.Type() == "decorator" {
		// Extract decorator name
		for i := 0; i < int(prev.ChildCount()); i++ {
			child := prev.Child(i)
			switch child.Type() {
			case "identifier":
				decorators = append([]string{string(source[child.StartByte():child.EndByte()])}, decorators...)
			case "call":
				// Decorator with arguments - get the function name
				for j := 0; j < int(child.ChildCount()); j++ {
					callChild := child.Child(j)
					if callChild.Type() == "identifier" || callChild.Type() == "attribute" {
						decorators = append([]string{string(source[callChild.StartByte():callChild.EndByte()])}, decorators...)
						break
					}
				}
			}
		}
		prev = prev.PrevSibling()
	}

	return decorators
}

// extractParameters extracts parameter names from a parameters node.
func (s *PythonStrategy) extractParameters(params *sitter.Node, source []byte) []string {
	var result []string

	for i := 0; i < int(params.ChildCount()); i++ {
		child := params.Child(i)
		switch child.Type() {
		case "identifier":
			result = append(result, string(source[child.StartByte():child.EndByte()]))
		case "typed_parameter", "default_parameter", "typed_default_parameter":
			// Get the identifier within
			for j := 0; j < int(child.ChildCount()); j++ {
				paramChild := child.Child(j)
				if paramChild.Type() == "identifier" {
					result = append(result, string(source[paramChild.StartByte():paramChild.EndByte()]))
					break
				}
			}
		case "list_splat_pattern", "dictionary_splat_pattern":
			// *args, **kwargs
			for j := 0; j < int(child.ChildCount()); j++ {
				paramChild := child.Child(j)
				if paramChild.Type() == "identifier" {
					name := string(source[paramChild.StartByte():paramChild.EndByte()])
					if child.Type() == "list_splat_pattern" {
						result = append(result, "*"+name)
					} else {
						result = append(result, "**"+name)
					}
					break
				}
			}
		}
	}

	return result
}

// extractBaseClasses extracts base class names from an argument list.
func (s *PythonStrategy) extractBaseClasses(argList *sitter.Node, source []byte) []string {
	var bases []string

	for i := 0; i < int(argList.ChildCount()); i++ {
		child := argList.Child(i)
		switch child.Type() {
		case "identifier":
			bases = append(bases, string(source[child.StartByte():child.EndByte()]))
		case "attribute":
			bases = append(bases, string(source[child.StartByte():child.EndByte()]))
		}
	}

	return bases
}

// extractDocstring extracts the docstring from a block.
func (s *PythonStrategy) extractDocstring(block *sitter.Node, source []byte) string {
	// First statement in block might be a string (docstring)
	if block.ChildCount() == 0 {
		return ""
	}

	// Only check the first child - docstring must be first statement
	child := block.Child(0)
	if child.Type() != "expression_statement" {
		return ""
	}

	// Check if it contains a string
	for j := 0; j < int(child.ChildCount()); j++ {
		exprChild := child.Child(j)
		if exprChild.Type() == "string" {
			docstring := string(source[exprChild.StartByte():exprChild.EndByte()])
			// Remove quotes
			docstring = strings.Trim(docstring, `"'`)
			docstring = strings.TrimPrefix(docstring, `""`)
			docstring = strings.TrimSuffix(docstring, `""`)
			return strings.TrimSpace(docstring)
		}
	}
	return ""
}

// buildSignature builds a signature string from metadata.
func (s *PythonStrategy) buildSignature(meta *chunkers.CodeMetadata) string {
	var sig strings.Builder

	if meta.IsAsync {
		sig.WriteString("async ")
	}
	sig.WriteString("def ")
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
func (s *PythonStrategy) findChild(node *sitter.Node, nodeType string) *sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == nodeType {
			return child
		}
	}
	return nil
}

// Ensure PythonStrategy implements LanguageStrategy.
var _ treesitter.LanguageStrategy = (*PythonStrategy)(nil)
