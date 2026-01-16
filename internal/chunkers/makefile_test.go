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

func TestMakefileChunker_DoubleColonRules(t *testing.T) {
	c := NewMakefileChunker()
	// Note: The current implementation treats :: as : which is acceptable
	// This test documents the behavior
	content := `clean::
	rm -f *.o

clean::
	rm -f *.tmp
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should parse without error, even if double-colon isn't fully supported
	if result.TotalChunks < 1 {
		t.Errorf("expected at least 1 chunk, got %d", result.TotalChunks)
	}
}

func TestMakefileChunker_TargetWithNoRecipe(t *testing.T) {
	c := NewMakefileChunker()
	content := `all: build test lint

build:
	go build

test:
	go test
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 3 {
		t.Errorf("expected 3 chunks, got %d", result.TotalChunks)
	}

	// First target (all) should have dependencies but minimal recipe
	if result.TotalChunks >= 1 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Build.TargetName != "all" {
			t.Errorf("expected target name 'all', got %q", chunk.Metadata.Build.TargetName)
		}
		if len(chunk.Metadata.Build.Dependencies) != 3 {
			t.Errorf("expected 3 dependencies, got %d", len(chunk.Metadata.Build.Dependencies))
		}
	}
}

func TestMakefileChunker_OrderOnlyPrerequisites(t *testing.T) {
	c := NewMakefileChunker()
	content := `output/file: input.txt | output
	cp $< $@

output:
	mkdir -p output
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should parse, even if order-only (|) isn't specially handled
	if result.TotalChunks < 1 {
		t.Errorf("expected at least 1 chunk, got %d", result.TotalChunks)
	}
}

func TestMakefileChunker_OnlyVariables(t *testing.T) {
	c := NewMakefileChunker()
	content := `# Just variable definitions
CC := gcc
CFLAGS := -Wall -O2
LDFLAGS := -lm
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return content as a chunk even without targets
	if result.TotalChunks != 1 {
		t.Errorf("expected 1 chunk for variables-only file, got %d", result.TotalChunks)
	}
}

func TestMakefileChunker_SpecialTargets(t *testing.T) {
	c := NewMakefileChunker()
	content := `.DEFAULT_GOAL := build

.SUFFIXES:

build:
	go build
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have at least the build target
	if result.TotalChunks < 1 {
		t.Errorf("expected at least 1 chunk, got %d", result.TotalChunks)
	}

	// Find build target
	found := false
	for _, chunk := range result.Chunks {
		if chunk.Metadata.Build != nil && chunk.Metadata.Build.TargetName == "build" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find build target")
	}
}

func TestMakefileChunker_OriginalSizeTracked(t *testing.T) {
	c := NewMakefileChunker()
	content := `build:
	go build
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.OriginalSize != len(content) {
		t.Errorf("expected OriginalSize %d, got %d", len(content), result.OriginalSize)
	}
}

func TestMakefileChunker_EscapedNewlineInVariable(t *testing.T) {
	c := NewMakefileChunker()
	content := `SOURCES := \
	main.c \
	util.c \
	helper.c

build: $(SOURCES)
	$(CC) -o app $^
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should parse and include the variable in preamble
	if result.TotalChunks < 1 {
		t.Errorf("expected at least 1 chunk, got %d", result.TotalChunks)
	}

	// First chunk should contain the SOURCES variable
	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if !strings.Contains(chunk.Content, "SOURCES") {
			t.Error("expected chunk to contain SOURCES variable")
		}
	}
}

func TestMakefileChunker_TokenEstimatePopulated(t *testing.T) {
	c := NewMakefileChunker()
	content := `build:
	go build -o app ./...
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.Metadata.TokenEstimate <= 0 {
			t.Error("expected TokenEstimate to be positive")
		}
	}
}

func TestMakefileChunker_ChunkIndexes(t *testing.T) {
	c := NewMakefileChunker()
	content := `build:
	go build

test:
	go test

clean:
	rm -rf ./build
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify chunk indexes are sequential starting from 0
	for i, chunk := range result.Chunks {
		if chunk.Index != i {
			t.Errorf("expected chunk index %d, got %d", i, chunk.Index)
		}
	}
}

func TestMakefileChunker_StartEndOffsets(t *testing.T) {
	c := NewMakefileChunker()
	content := `build:
	go build
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if chunk.EndOffset <= chunk.StartOffset {
			t.Errorf("expected EndOffset > StartOffset, got StartOffset=%d EndOffset=%d",
				chunk.StartOffset, chunk.EndOffset)
		}
	}
}

func TestMakefileChunker_LargeTargetSplitting(t *testing.T) {
	c := NewMakefileChunker()

	// Create a makefile with a very large recipe
	var largeRecipe strings.Builder
	largeRecipe.WriteString("build:\n")
	for i := range 50 {
		largeRecipe.WriteString("\t@echo 'Step " + string(rune('0'+i%10)) + "'\n")
	}

	content := largeRecipe.String()

	// Use a small max chunk size to trigger splitting
	opts := ChunkOptions{MaxChunkSize: 300}

	result, err := c.Chunk(context.Background(), []byte(content), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be split into multiple chunks
	if result.TotalChunks < 2 {
		t.Errorf("expected content to be split into multiple chunks, got %d", result.TotalChunks)
	}

	// All chunks should have Build metadata
	for i, chunk := range result.Chunks {
		if chunk.Metadata.Build == nil {
			t.Errorf("chunk %d missing Build metadata", i)
		}
	}
}

func TestMakefileChunker_TargetWithDotInName(t *testing.T) {
	c := NewMakefileChunker()
	content := `app.out: main.o util.o
	$(LD) -o $@ $^

main.o: main.c
	$(CC) -c $<
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks != 2 {
		t.Errorf("expected 2 chunks, got %d", result.TotalChunks)
	}

	// First target should be app.out
	if result.TotalChunks >= 1 {
		chunk := result.Chunks[0]
		if chunk.Metadata.Build.TargetName != "app.out" {
			t.Errorf("expected target name 'app.out', got %q", chunk.Metadata.Build.TargetName)
		}
	}
}

func TestMakefileChunker_EmptyDependencies(t *testing.T) {
	c := NewMakefileChunker()
	content := `clean:
	rm -rf ./build
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
		if len(chunk.Metadata.Build.Dependencies) != 0 {
			t.Errorf("expected 0 dependencies, got %d", len(chunk.Metadata.Build.Dependencies))
		}
	}
}

func TestMakefileChunker_CommentsPreserved(t *testing.T) {
	c := NewMakefileChunker()
	content := `# Main build target
# This compiles the application
build:
	go build
`

	result, err := c.Chunk(context.Background(), []byte(content), DefaultChunkOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalChunks > 0 {
		chunk := result.Chunks[0]
		if !strings.Contains(chunk.Content, "# Main build target") {
			t.Error("expected chunk to contain comments")
		}
	}
}
