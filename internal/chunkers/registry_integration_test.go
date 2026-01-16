package chunkers_test

import (
	"context"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	// Import to register TreeSitter factory via init()
	_ "github.com/leefowlercu/agentic-memorizer/internal/chunkers/code/languages"
)

func TestDefaultRegistryUsesTreeSitter(t *testing.T) {
	registry := chunkers.DefaultRegistry()

	// Check for Go code handling - should use treesitter
	goChunker := registry.Get("", "go")
	if goChunker == nil {
		t.Fatal("expected chunker for Go code")
	}
	if goChunker.Name() != "treesitter" {
		t.Errorf("expected treesitter chunker for Go, got %q", goChunker.Name())
	}

	// Check for Python code handling - should use treesitter
	pyChunker := registry.Get("", "python")
	if pyChunker == nil {
		t.Fatal("expected chunker for Python code")
	}
	if pyChunker.Name() != "treesitter" {
		t.Errorf("expected treesitter chunker for Python, got %q", pyChunker.Name())
	}

	// Check for JavaScript code handling - should use treesitter
	jsChunker := registry.Get("", "javascript")
	if jsChunker == nil {
		t.Fatal("expected chunker for JavaScript code")
	}
	if jsChunker.Name() != "treesitter" {
		t.Errorf("expected treesitter chunker for JavaScript, got %q", jsChunker.Name())
	}
}

func TestDefaultRegistryChunksGoCode(t *testing.T) {
	registry := chunkers.DefaultRegistry()

	goCode := []byte(`package main

func hello() string {
	return "hello"
}

func world() string {
	return "world"
}
`)
	result, err := registry.Chunk(context.Background(), goCode, chunkers.ChunkOptions{
		Language: "go",
	})
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}
	if result.ChunkerUsed != "treesitter" {
		t.Errorf("expected treesitter, got %q", result.ChunkerUsed)
	}

	// Should have extracted functions
	foundHello := false
	foundWorld := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Code != nil {
			if chunk.Metadata.Code.FunctionName == "hello" {
				foundHello = true
			}
			if chunk.Metadata.Code.FunctionName == "world" {
				foundWorld = true
			}
		}
	}

	if !foundHello {
		t.Error("expected to find function 'hello'")
	}
	if !foundWorld {
		t.Error("expected to find function 'world'")
	}
}

func TestDefaultRegistryChunksPythonCode(t *testing.T) {
	registry := chunkers.DefaultRegistry()

	pyCode := []byte(`def add(a, b):
    """Add two numbers."""
    return a + b

class Calculator:
    """A calculator class."""
    def multiply(self, x, y):
        return x * y
`)
	result, err := registry.Chunk(context.Background(), pyCode, chunkers.ChunkOptions{
		Language: "python",
	})
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}
	if result.ChunkerUsed != "treesitter" {
		t.Errorf("expected treesitter, got %q", result.ChunkerUsed)
	}

	// Should have extracted functions and classes
	foundAdd := false
	foundCalculator := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Code != nil {
			if chunk.Metadata.Code.FunctionName == "add" {
				foundAdd = true
				// Check docstring was extracted
				if chunk.Metadata.Code.Docstring != "Add two numbers." {
					t.Errorf("expected docstring 'Add two numbers.', got %q", chunk.Metadata.Code.Docstring)
				}
			}
			if chunk.Metadata.Code.ClassName == "Calculator" && chunk.Metadata.Code.FunctionName == "" {
				foundCalculator = true
			}
		}
	}

	if !foundAdd {
		t.Error("expected to find function 'add'")
	}
	if !foundCalculator {
		t.Error("expected to find class 'Calculator'")
	}
}

func TestDefaultRegistryChunksMultipleLanguages(t *testing.T) {
	registry := chunkers.DefaultRegistry()

	languages := []struct {
		name     string
		code     string
		funcName string
	}{
		{"go", "package main\nfunc test() {}\n", "test"},
		{"python", "def test(): pass\n", "test"},
		{"javascript", "function test() {}\n", "test"},
		{"typescript", "function test(): void {}\n", "test"},
		{"java", "public class Test { void test() {} }\n", ""},
		{"rust", "fn test() {}\n", "test"},
		{"c", "void test() {}\n", "test"},
		{"cpp", "void test() {}\n", "test"},
	}

	for _, lang := range languages {
		t.Run(lang.name, func(t *testing.T) {
			result, err := registry.Chunk(context.Background(), []byte(lang.code), chunkers.ChunkOptions{
				Language: lang.name,
			})
			if err != nil {
				t.Fatalf("Chunk failed: %v", err)
			}
			if result.ChunkerUsed != "treesitter" {
				t.Errorf("expected treesitter, got %q", result.ChunkerUsed)
			}
			if len(result.Chunks) == 0 {
				t.Error("expected at least one chunk")
			}
		})
	}
}

func TestDefaultRegistryChunksRST(t *testing.T) {
	registry := chunkers.DefaultRegistry()

	// Verify RST chunker is selected for .rst files
	rstChunker := registry.Get("", "test.rst")
	if rstChunker == nil {
		t.Fatal("expected chunker for RST files")
	}
	if rstChunker.Name() != "rst" {
		t.Errorf("expected rst chunker, got %q", rstChunker.Name())
	}

	// Test chunking RST content
	rstContent := []byte(`Introduction
============

This is the introduction.

Getting Started
---------------

Here is how to get started.
`)
	result, err := registry.Chunk(context.Background(), rstContent, chunkers.ChunkOptions{
		Language: "test.rst",
	})
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}
	if result.ChunkerUsed != "rst" {
		t.Errorf("expected rst, got %q", result.ChunkerUsed)
	}
	if len(result.Chunks) < 2 {
		t.Errorf("expected at least 2 chunks, got %d", len(result.Chunks))
	}

	// Verify heading extraction
	foundIntro := false
	foundGettingStarted := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil {
			if chunk.Metadata.Document.Heading == "Introduction" {
				foundIntro = true
				if chunk.Metadata.Document.HeadingLevel != 1 {
					t.Errorf("expected level 1 for Introduction, got %d", chunk.Metadata.Document.HeadingLevel)
				}
			}
			if chunk.Metadata.Document.Heading == "Getting Started" {
				foundGettingStarted = true
				if chunk.Metadata.Document.HeadingLevel != 2 {
					t.Errorf("expected level 2 for Getting Started, got %d", chunk.Metadata.Document.HeadingLevel)
				}
			}
		}
	}
	if !foundIntro {
		t.Error("expected to find 'Introduction' heading")
	}
	if !foundGettingStarted {
		t.Error("expected to find 'Getting Started' heading")
	}
}

func TestDefaultRegistryChunksAsciiDoc(t *testing.T) {
	registry := chunkers.DefaultRegistry()

	// Verify AsciiDoc chunker is selected for .adoc files
	adocChunker := registry.Get("", "test.adoc")
	if adocChunker == nil {
		t.Fatal("expected chunker for AsciiDoc files")
	}
	if adocChunker.Name() != "asciidoc" {
		t.Errorf("expected asciidoc chunker, got %q", adocChunker.Name())
	}

	// Test chunking AsciiDoc content
	adocContent := []byte(`= Document Title

Introduction text.

== First Section

Section content.

=== Subsection

Subsection content.
`)
	result, err := registry.Chunk(context.Background(), adocContent, chunkers.ChunkOptions{
		Language: "test.adoc",
	})
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}
	if result.ChunkerUsed != "asciidoc" {
		t.Errorf("expected asciidoc, got %q", result.ChunkerUsed)
	}
	if len(result.Chunks) < 3 {
		t.Errorf("expected at least 3 chunks, got %d", len(result.Chunks))
	}

	// Verify heading levels
	levelsSeen := make(map[int]bool)
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil && chunk.Metadata.Document.HeadingLevel > 0 {
			levelsSeen[chunk.Metadata.Document.HeadingLevel] = true
		}
	}
	// Should have levels 1 (=), 2 (==), and 3 (===)
	if !levelsSeen[1] || !levelsSeen[2] || !levelsSeen[3] {
		t.Errorf("expected levels 1, 2, and 3, got %v", levelsSeen)
	}
}

func TestDefaultRegistryChunksLaTeX(t *testing.T) {
	registry := chunkers.DefaultRegistry()

	// Verify LaTeX chunker is selected for .tex files
	texChunker := registry.Get("", "test.tex")
	if texChunker == nil {
		t.Fatal("expected chunker for LaTeX files")
	}
	if texChunker.Name() != "latex" {
		t.Errorf("expected latex chunker, got %q", texChunker.Name())
	}

	// Test chunking LaTeX content
	texContent := []byte(`\section{Introduction}

This is the introduction.

\subsection{Background}

Background information.

\begin{equation}
E = mc^2
\end{equation}

\section{Methods}

Description of methods.
`)
	result, err := registry.Chunk(context.Background(), texContent, chunkers.ChunkOptions{
		Language: "test.tex",
	})
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}
	if result.ChunkerUsed != "latex" {
		t.Errorf("expected latex, got %q", result.ChunkerUsed)
	}
	if len(result.Chunks) < 2 {
		t.Errorf("expected at least 2 chunks, got %d", len(result.Chunks))
	}

	// Verify heading extraction
	foundIntro := false
	foundMethods := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Document != nil {
			if chunk.Metadata.Document.Heading == "Introduction" {
				foundIntro = true
			}
			if chunk.Metadata.Document.Heading == "Methods" {
				foundMethods = true
			}
		}
	}
	if !foundIntro {
		t.Error("expected to find 'Introduction' section")
	}
	if !foundMethods {
		t.Error("expected to find 'Methods' section")
	}
}

func TestDefaultRegistryTextFormatChunkerPriorities(t *testing.T) {
	registry := chunkers.DefaultRegistry()
	chunkers := registry.List()

	// Build a map of chunker priorities
	priorities := make(map[string]int)
	for _, c := range chunkers {
		priorities[c.Name()] = c.Priority()
	}

	// Verify text format chunker priorities
	if priorities["rst"] != 55 {
		t.Errorf("expected rst priority 55, got %d", priorities["rst"])
	}
	if priorities["asciidoc"] != 54 {
		t.Errorf("expected asciidoc priority 54, got %d", priorities["asciidoc"])
	}
	if priorities["latex"] != 53 {
		t.Errorf("expected latex priority 53, got %d", priorities["latex"])
	}

	// Text format chunkers should be between markdown (50) and ODT (71)
	if priorities["markdown"] >= priorities["rst"] {
		t.Error("rst should have higher priority than markdown")
	}
	if priorities["rst"] >= priorities["odt"] {
		t.Error("odt should have higher priority than rst")
	}
}
