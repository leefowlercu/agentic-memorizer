package chunkers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMakefileChunker_Name(t *testing.T) {
	c := NewMakefileChunker()
	if c.Name() != "makefile" {
		t.Errorf("expected name 'makefile', got %q", c.Name())
	}
}

func TestMakefileChunker_Priority(t *testing.T) {
	c := NewMakefileChunker()
	if c.Priority() != 44 {
		t.Errorf("expected priority 44, got %d", c.Priority())
	}
}

func TestMakefileChunker_CanHandle(t *testing.T) {
	c := NewMakefileChunker()

	tests := []struct {
		mimeType string
		language string
		want     bool
	}{
		{"text/x-makefile", "", true},
		{"application/x-makefile", "", true},
		{"", "makefile", true},
		{"", "Makefile", true},
		{"", "GNUmakefile", true},
		{"", "/path/to/Makefile", true},
		{"", "build.mk", true},
		{"", "config.mk", true},
		{"text/plain", "", false},
		{"", "go", false},
		{"", "python", false},
	}

	for _, tt := range tests {
		got := c.CanHandle(tt.mimeType, tt.language)
		if got != tt.want {
			t.Errorf("CanHandle(%q, %q) = %v, want %v", tt.mimeType, tt.language, got, tt.want)
		}
	}
}

func TestMakefileChunker_EmptyContent(t *testing.T) {
	c := NewMakefileChunker()
	result, err := c.Chunk(context.Background(), []byte{}, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalChunks != 0 {
		t.Errorf("expected 0 chunks, got %d", result.TotalChunks)
	}
	if result.ChunkerUsed != "makefile" {
		t.Errorf("expected chunker name 'makefile', got %q", result.ChunkerUsed)
	}
}

func TestMakefileChunker_SingleTarget(t *testing.T) {
	c := NewMakefileChunker()
	content := `all:
	echo "building"
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Build == nil {
			t.Fatal("expected Build metadata to be set")
		}
		if chunk.Metadata.Build.TargetName != "all" {
			t.Errorf("expected target name 'all', got %q", chunk.Metadata.Build.TargetName)
		}
	}
}

func TestMakefileChunker_MultipleTargets(t *testing.T) {
	c := NewMakefileChunker()
	content := `build:
	go build ./...

test: build
	go test ./...

clean:
	rm -rf ./build
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 3 {
		t.Errorf("expected 3 chunks, got %d", result.TotalChunks)
	}

	// Verify first target
	if result.TotalChunks >= 1 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Build.TargetName != "build" {
			t.Errorf("expected target name 'build', got %q", chunk.Metadata.Build.TargetName)
		}
	}

	// Verify second target has dependency
	if result.TotalChunks >= 2 {
		chunk := result.Chunks[1]
		if chunk.Metadata.Build.TargetName != "test" {
			t.Errorf("expected target name 'test', got %q", chunk.Metadata.Build.TargetName)
		}
		if len(chunk.Metadata.Build.Dependencies) != 1 || chunk.Metadata.Build.Dependencies[0] != "build" {
			t.Errorf("expected dependencies ['build'], got %v", chunk.Metadata.Build.Dependencies)
		}
	}

	// Verify third target
	if result.TotalChunks >= 3 {
		chunk := result.Chunks[2]
		if chunk.Metadata.Build.TargetName != "clean" {
			t.Errorf("expected target name 'clean', got %q", chunk.Metadata.Build.TargetName)
		}
	}
}

func TestMakefileChunker_MultipleDependencies(t *testing.T) {
	c := NewMakefileChunker()
	content := `all: build test lint
	echo "done"
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Build.TargetName != "all" {
			t.Errorf("expected target name 'all', got %q", chunk.Metadata.Build.TargetName)
		}
		expectedDeps := []string{"build", "test", "lint"}
		if len(chunk.Metadata.Build.Dependencies) != len(expectedDeps) {
			t.Errorf("expected %d dependencies, got %d", len(expectedDeps), len(chunk.Metadata.Build.Dependencies))
		}
		for i, dep := range expectedDeps {
			if i < len(chunk.Metadata.Build.Dependencies) && chunk.Metadata.Build.Dependencies[i] != dep {
				t.Errorf("expected dependency %q, got %q", dep, chunk.Metadata.Build.Dependencies[i])
			}
		}
	}
}

func TestMakefileChunker_VariablesAndPreamble(t *testing.T) {
	c := NewMakefileChunker()
	content := `# Makefile variables
BINARY_NAME := myapp
GO := go

build:
	$(GO) build -o $(BINARY_NAME)
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		// Preamble should be included with first target
		if !strings.Contains(chunk.Content, "BINARY_NAME") {
			t.Error("expected chunk to contain preamble variable BINARY_NAME")
		}
		if !strings.Contains(chunk.Content, "build:") {
			t.Error("expected chunk to contain build target")
		}
	}
}

func TestMakefileChunker_PhonyTargets(t *testing.T) {
	c := NewMakefileChunker()
	content := `.PHONY: build test clean

build:
	go build

test:
	go test
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// .PHONY should not create a separate chunk, targets should still be parsed
	if result.TotalChunks != 2 {
		t.Errorf("expected 2 chunks, got %d", result.TotalChunks)
	}

	// First chunk should be build (with .PHONY in preamble)
	if result.TotalChunks >= 1 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Build.TargetName != "build" {
			t.Errorf("expected target name 'build', got %q", chunk.Metadata.Build.TargetName)
		}
	}
}

func TestMakefileChunker_TestdataFixture(t *testing.T) {
	c := NewMakefileChunker()

	// Read the testdata fixture
	fixturePath := filepath.Join("..", "..", "testdata", "devops", "Makefile")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("testdata fixture not found: %v", err)
	}

	result, err := c.Chunk(context.Background(), content, DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fixture has multiple targets: all, build, test, test-coverage, lint, fmt, clean, help
	// The default all: target depends on build, so should have 8 targets total
	if result.TotalChunks < 7 {
		t.Errorf("expected at least 7 chunks for fixture, got %d", result.TotalChunks)
	}

	// Verify first target includes preamble
	if result.TotalChunks >= 1 {
		chunk := result.Chunks[0]
		if !strings.Contains(chunk.Content, "BINARY_NAME") {
			t.Error("expected first chunk to contain preamble with BINARY_NAME")
		}
	}
}

func TestMakefileChunker_ContextCancellation(t *testing.T) {
	c := NewMakefileChunker()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	content := `build:
	echo "test"
`

	_, err := c.Chunk(ctx, []byte(content), DefaultChunkOptions())
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestMakefileChunker_ChunkType(t *testing.T) {
	c := NewMakefileChunker()
	content := `build:
	echo "test"
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Type != ChunkTypeStructured {
			t.Errorf("expected ChunkTypeStructured, got %q", chunk.Metadata.Type)
		}
	}
}

func TestMakefileChunker_MultilineRecipe(t *testing.T) {
	c := NewMakefileChunker()
	content := `build:
	@echo "Step 1" && \
	echo "Step 2" && \
	echo "Step 3"
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	// Verify the multiline recipe is kept together
	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if !strings.Contains(chunk.Content, "Step 1") {
			t.Error("expected chunk to contain Step 1")
		}
		if !strings.Contains(chunk.Content, "Step 3") {
			t.Error("expected chunk to contain Step 3")
		}
	}
}

func TestMakefileChunker_PatternRules(t *testing.T) {
	c := NewMakefileChunker()
	content := `%.o: %.c
	$(CC) -c $< -o $@
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk, got %d", result.TotalChunks)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Build.TargetName != "%.o" {
			t.Errorf("expected target name '%%.o', got %q", chunk.Metadata.Build.TargetName)
		}
		if len(chunk.Metadata.Build.Dependencies) != 1 || chunk.Metadata.Build.Dependencies[0] != "%.c" {
			t.Errorf("expected dependencies ['%%.c'], got %v", chunk.Metadata.Build.Dependencies)
		}
	}
}
