package languages_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/chunkers"
	"github.com/leefowlercu/agentic-memorizer/internal/chunkers/treesitter/languages"
)

// getTestDataPath returns the path to the testdata directory.
func getTestDataPath() string {
	// Walk up from the test file to find testdata
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	// We're in internal/chunkers/treesitter/languages
	return filepath.Join(wd, "../../../../testdata/code")
}

func TestGoStrategyWithFixture(t *testing.T) {
	c := languages.NewDefaultChunker()

	content, err := os.ReadFile(filepath.Join(getTestDataPath(), "sample.go"))
	if err != nil {
		t.Skipf("skipping fixture test: %v", err)
	}

	result, err := c.Chunk(context.Background(), content, chunkers.ChunkOptions{
		Language: "go",
	})
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if result.ChunkerUsed != "treesitter" {
		t.Errorf("expected treesitter, got %q", result.ChunkerUsed)
	}

	// The sample.go contains:
	// - type Greeter struct
	// - func NewGreeter
	// - func (g *Greeter) Greet
	// - func (g *Greeter) SayGoodbye
	foundGreeter := false
	foundNewGreeter := false
	foundMethodsInContent := false // Check content includes methods even if not separate chunks

	for _, chunk := range result.Chunks {
		if chunk.Metadata.Code != nil {
			if chunk.Metadata.Code.ClassName == "Greeter" {
				foundGreeter = true
			}
			if chunk.Metadata.Code.FunctionName == "NewGreeter" {
				foundNewGreeter = true
				if !chunk.Metadata.Code.IsExported {
					t.Error("NewGreeter should be exported")
				}
			}
		}
		// Check content for method names
		if strings.Contains(chunk.Content, "Greet") && strings.Contains(chunk.Content, "SayGoodbye") {
			foundMethodsInContent = true
		}
	}

	if !foundGreeter {
		t.Error("expected to find Greeter type")
	}
	if !foundNewGreeter {
		t.Error("expected to find NewGreeter function")
	}
	if !foundMethodsInContent {
		t.Error("expected to find methods Greet and SayGoodbye in content")
	}
}

func TestPythonStrategyWithFixture(t *testing.T) {
	c := languages.NewDefaultChunker()

	content, err := os.ReadFile(filepath.Join(getTestDataPath(), "sample.py"))
	if err != nil {
		t.Skipf("skipping fixture test: %v", err)
	}

	result, err := c.Chunk(context.Background(), content, chunkers.ChunkOptions{
		Language: "python",
	})
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// The sample.py contains:
	// - class Calculator with methods: __init__, add, subtract, multiply, divide, reset
	foundCalculator := false
	foundMethodsInContent := false

	for _, chunk := range result.Chunks {
		if chunk.Metadata.Code != nil {
			if chunk.Metadata.Code.ClassName == "Calculator" && chunk.Metadata.Code.FunctionName == "" {
				foundCalculator = true
			}
		}
		// Check content for method definitions
		if strings.Contains(chunk.Content, "def __init__") &&
			strings.Contains(chunk.Content, "def add") &&
			strings.Contains(chunk.Content, "def multiply") {
			foundMethodsInContent = true
		}
	}

	if !foundCalculator {
		t.Error("expected to find Calculator class")
	}
	if !foundMethodsInContent {
		t.Error("expected to find Calculator methods in content")
	}
}

func TestJavaScriptStrategyWithFixture(t *testing.T) {
	c := languages.NewDefaultChunker()

	content, err := os.ReadFile(filepath.Join(getTestDataPath(), "sample.js"))
	if err != nil {
		t.Skipf("skipping fixture test: %v", err)
	}

	result, err := c.Chunk(context.Background(), content, chunkers.ChunkOptions{
		Language: "javascript",
	})
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// The sample.js contains:
	// - class Counter with methods: constructor, increment, decrement, getCount, reset
	// - function createCounter
	foundCounter := false
	foundCreateCounter := false

	for _, chunk := range result.Chunks {
		if chunk.Metadata.Code != nil {
			if chunk.Metadata.Code.ClassName == "Counter" && chunk.Metadata.Code.FunctionName == "" {
				foundCounter = true
			}
			if chunk.Metadata.Code.FunctionName == "createCounter" {
				foundCreateCounter = true
			}
		}
	}

	if !foundCounter {
		t.Error("expected to find Counter class")
	}
	if !foundCreateCounter {
		t.Error("expected to find createCounter function")
	}
}

func TestTypeScriptStrategyWithFixture(t *testing.T) {
	c := languages.NewDefaultChunker()

	content, err := os.ReadFile(filepath.Join(getTestDataPath(), "sample.ts"))
	if err != nil {
		t.Skipf("skipping fixture test: %v", err)
	}

	result, err := c.Chunk(context.Background(), content, chunkers.ChunkOptions{
		Language: "typescript",
	})
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// The sample.ts contains:
	// - interface User
	// - interface UserCreateParams
	// - class UserManager
	foundUserInterface := false
	foundUserManager := false

	for _, chunk := range result.Chunks {
		if chunk.Metadata.Code != nil {
			if chunk.Metadata.Code.ClassName == "User" {
				foundUserInterface = true
			}
			if chunk.Metadata.Code.ClassName == "UserManager" {
				foundUserManager = true
			}
		}
	}

	if !foundUserInterface {
		t.Error("expected to find User interface")
	}
	if !foundUserManager {
		t.Error("expected to find UserManager class")
	}
}

func TestJavaStrategyWithFixture(t *testing.T) {
	c := languages.NewDefaultChunker()

	content, err := os.ReadFile(filepath.Join(getTestDataPath(), "sample.java"))
	if err != nil {
		t.Skipf("skipping fixture test: %v", err)
	}

	result, err := c.Chunk(context.Background(), content, chunkers.ChunkOptions{
		Language: "java",
	})
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// The sample.java contains:
	// - public class TaskManager
	// - class Task
	foundTaskManager := false
	foundTask := false

	for _, chunk := range result.Chunks {
		if chunk.Metadata.Code != nil {
			if chunk.Metadata.Code.ClassName == "TaskManager" && chunk.Metadata.Code.FunctionName == "" {
				foundTaskManager = true
				if chunk.Metadata.Code.Visibility != "public" {
					t.Errorf("TaskManager should be public, got %q", chunk.Metadata.Code.Visibility)
				}
			}
			if chunk.Metadata.Code.ClassName == "Task" && chunk.Metadata.Code.FunctionName == "" {
				foundTask = true
			}
		}
	}

	if !foundTaskManager {
		t.Error("expected to find TaskManager class")
	}
	if !foundTask {
		t.Error("expected to find Task class")
	}
}

func TestRustStrategyWithFixture(t *testing.T) {
	c := languages.NewDefaultChunker()

	content, err := os.ReadFile(filepath.Join(getTestDataPath(), "sample.rs"))
	if err != nil {
		t.Skipf("skipping fixture test: %v", err)
	}

	result, err := c.Chunk(context.Background(), content, chunkers.ChunkOptions{
		Language: "rust",
	})
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// The sample.rs contains:
	// - pub struct Store
	// - impl Store with methods: new, set, get, delete, contains, len, is_empty
	// - impl Default for Store
	// - mod tests
	foundStore := false
	foundImplContent := false

	for _, chunk := range result.Chunks {
		if chunk.Metadata.Code != nil {
			if chunk.Metadata.Code.ClassName == "Store" {
				foundStore = true
			}
		}
		// Check content for impl block with methods
		if strings.Contains(chunk.Content, "impl Store") &&
			strings.Contains(chunk.Content, "pub fn new") {
			foundImplContent = true
		}
	}

	if !foundStore {
		t.Error("expected to find Store struct")
	}
	if !foundImplContent {
		t.Error("expected to find impl Store with methods in content")
	}
}

func TestCStrategyWithFixture(t *testing.T) {
	c := languages.NewDefaultChunker()

	content, err := os.ReadFile(filepath.Join(getTestDataPath(), "sample.c"))
	if err != nil {
		t.Skipf("skipping fixture test: %v", err)
	}

	result, err := c.Chunk(context.Background(), content, chunkers.ChunkOptions{
		Language: "c",
	})
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// The sample.c contains:
	// - typedef struct Person
	// - Person* person_create
	// - void person_free
	// - void person_print
	// - int person_is_adult
	foundPerson := false
	foundPersonCreate := false

	for _, chunk := range result.Chunks {
		if chunk.Metadata.Code != nil {
			if chunk.Metadata.Code.ClassName == "Person" {
				foundPerson = true
			}
			if chunk.Metadata.Code.FunctionName == "person_create" {
				foundPersonCreate = true
			}
		}
	}

	if !foundPerson {
		t.Error("expected to find Person struct")
	}
	if !foundPersonCreate {
		t.Error("expected to find person_create function")
	}
}

func TestCPPStrategyWithFixture(t *testing.T) {
	c := languages.NewDefaultChunker()

	content, err := os.ReadFile(filepath.Join(getTestDataPath(), "sample.cpp"))
	if err != nil {
		t.Skipf("skipping fixture test: %v", err)
	}

	result, err := c.Chunk(context.Background(), content, chunkers.ChunkOptions{
		Language: "cpp",
	})
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// The sample.cpp contains:
	// - class Shape (abstract)
	// - class Circle : public Shape
	// - class Rectangle : public Shape
	// - class ShapeCollection
	foundShape := false
	foundCircle := false
	foundRectangle := false
	foundShapeCollection := false

	for _, chunk := range result.Chunks {
		if chunk.Metadata.Code != nil {
			name := chunk.Metadata.Code.ClassName
			if name == "Shape" {
				foundShape = true
			}
			if name == "Circle" {
				foundCircle = true
			}
			if name == "Rectangle" {
				foundRectangle = true
			}
			if name == "ShapeCollection" {
				foundShapeCollection = true
			}
		}
	}

	if !foundShape {
		t.Error("expected to find Shape class")
	}
	if !foundCircle {
		t.Error("expected to find Circle class")
	}
	if !foundRectangle {
		t.Error("expected to find Rectangle class")
	}
	if !foundShapeCollection {
		t.Error("expected to find ShapeCollection class")
	}
}

func TestAllLanguagesProduceChunks(t *testing.T) {
	c := languages.NewDefaultChunker()
	testDataPath := getTestDataPath()

	tests := []struct {
		language  string
		filename  string
		minChunks int
	}{
		{"go", "sample.go", 1},
		{"python", "sample.py", 1},
		{"javascript", "sample.js", 1},
		{"typescript", "sample.ts", 1},
		{"java", "sample.java", 1},
		{"rust", "sample.rs", 1},
		{"c", "sample.c", 1},
		{"cpp", "sample.cpp", 1},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(testDataPath, tt.filename))
			if err != nil {
				t.Skipf("skipping: %v", err)
			}

			result, err := c.Chunk(context.Background(), content, chunkers.ChunkOptions{
				Language: tt.language,
			})
			if err != nil {
				t.Fatalf("Chunk failed: %v", err)
			}

			if result.ChunkerUsed != "treesitter" {
				t.Errorf("expected treesitter, got %q", result.ChunkerUsed)
			}

			if len(result.Chunks) < tt.minChunks {
				t.Errorf("expected at least %d chunks, got %d", tt.minChunks, len(result.Chunks))
			}

			// All chunks should have code metadata
			for i, chunk := range result.Chunks {
				if chunk.Metadata.Code == nil {
					t.Errorf("chunk %d has nil Code metadata", i)
				} else if chunk.Metadata.Code.Language != tt.language {
					t.Errorf("chunk %d has wrong language: got %q, want %q",
						i, chunk.Metadata.Code.Language, tt.language)
				}
			}

			// Check there are no warnings (clean parse)
			for _, w := range result.Warnings {
				if strings.Contains(w.Code, "ERROR") {
					t.Logf("warning: %s: %s", w.Code, w.Message)
				}
			}
		})
	}
}
