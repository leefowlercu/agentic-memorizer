//go:build e2e

package tests

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/e2e/harness"
)

// TestMetadata_MarkdownExtraction tests markdown file metadata extraction
func TestMetadata_MarkdownExtraction(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Add markdown file with specific content
	markdownContent := `# Test Markdown Document

## Introduction

This is a test markdown file with approximately 100 words. Lorem ipsum dolor sit amet,
consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna
aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut
aliquip ex ea commodo consequat.

## Section 2

More content here to increase word count. Duis aute irure dolor in reprehenderit in
voluptate velit esse cillum dolore eu fugiat nulla pariatur.

### Subsection

Additional text to reach target word count milestone.
`
	if err := h.AddMemoryFile("test.md", markdownContent); err != nil {
		t.Fatalf("Failed to add markdown file: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for file processing
	time.Sleep(5 * time.Second)

	// Verify file metadata via API
	metadata, err := h.HTTPClient.GetFileMetadata("test.md")
	if err != nil {
		t.Fatalf("Failed to get file metadata: %v", err)
	}

	// Verify basic metadata fields
	metadataMap, ok := metadata.(map[string]any)
	if !ok {
		t.Fatalf("Unexpected metadata format: %T", metadata)
	}

	if file, ok := metadataMap["file"].(map[string]any); ok {
		if fileType, ok := file["type"].(string); !ok || fileType != "markdown" {
			t.Errorf("Expected type 'markdown', got %v", file["type"])
		}
		if category, ok := file["category"].(string); !ok || category != "documents" {
			t.Errorf("Expected category 'documents', got %v", file["category"])
		}
		t.Logf("Markdown metadata: type=%v, category=%v", file["type"], file["category"])
	} else {
		t.Error("Missing 'file' field in metadata response")
	}
}

// TestMetadata_ImageExtraction tests image file metadata extraction
func TestMetadata_ImageExtraction(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Create a simple 1x1 PNG image (minimal valid PNG)
	// PNG signature + IHDR chunk for 1x1 image + IEND chunk
	pngData := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
		0x00, 0x00, 0x00, 0x0d, // IHDR length
		0x49, 0x48, 0x44, 0x52, // IHDR
		0x00, 0x00, 0x00, 0x01, // Width: 1
		0x00, 0x00, 0x00, 0x01, // Height: 1
		0x08, 0x02, 0x00, 0x00, 0x00, // Bit depth, color type, etc.
		0x90, 0x77, 0x53, 0xde, // CRC
		0x00, 0x00, 0x00, 0x00, // IEND length
		0x49, 0x45, 0x4e, 0x44, // IEND
		0xae, 0x42, 0x60, 0x82, // CRC
	}

	if err := h.AddMemoryFile("test.png", string(pngData)); err != nil {
		t.Fatalf("Failed to add image file: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for file processing
	time.Sleep(5 * time.Second)

	// Verify file metadata via API
	metadata, err := h.HTTPClient.GetFileMetadata("test.png")
	if err != nil {
		t.Fatalf("Failed to get file metadata: %v", err)
	}

	// Verify basic metadata fields
	metadataMap, ok := metadata.(map[string]any)
	if !ok {
		t.Fatalf("Unexpected metadata format: %T", metadata)
	}

	if file, ok := metadataMap["file"].(map[string]any); ok {
		if fileType, ok := file["type"].(string); !ok || fileType != "png" {
			t.Errorf("Expected type 'png', got %v", file["type"])
		}
		if category, ok := file["category"].(string); !ok || category != "images" {
			t.Errorf("Expected category 'images', got %v", file["category"])
		}
		t.Logf("Image metadata: type=%v, category=%v", file["type"], file["category"])
	} else {
		t.Error("Missing 'file' field in metadata response")
	}
}

// TestMetadata_CodeExtraction tests code file metadata extraction
func TestMetadata_CodeExtraction(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Add Go code file
	codeContent := `package main

import "fmt"

// main is the entry point
func main() {
	fmt.Println("Hello, World!")
}

// helper function
func add(a, b int) int {
	return a + b
}
`
	if err := h.AddMemoryFile("test.go", codeContent); err != nil {
		t.Fatalf("Failed to add code file: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for file processing
	time.Sleep(5 * time.Second)

	// Verify file metadata via API
	metadata, err := h.HTTPClient.GetFileMetadata("test.go")
	if err != nil {
		t.Fatalf("Failed to get file metadata: %v", err)
	}

	// Verify basic metadata fields
	metadataMap, ok := metadata.(map[string]any)
	if !ok {
		t.Fatalf("Unexpected metadata format: %T", metadata)
	}

	if file, ok := metadataMap["file"].(map[string]any); ok {
		if fileType, ok := file["type"].(string); !ok || fileType != "code" {
			t.Errorf("Expected type 'code', got %v", file["type"])
		}
		if category, ok := file["category"].(string); !ok || category != "code" {
			t.Errorf("Expected category 'code', got %v", file["category"])
		}
		t.Logf("Code metadata: type=%v, category=%v", file["type"], file["category"])
	} else {
		t.Error("Missing 'file' field in metadata response")
	}
}

// TestMetadata_JSONExtraction tests JSON file metadata extraction
func TestMetadata_JSONExtraction(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Add JSON file
	jsonContent := `{
  "name": "test-package",
  "version": "1.0.0",
  "description": "A test package for E2E testing",
  "keywords": ["test", "e2e", "metadata"],
  "dependencies": {
    "example": "^1.0.0"
  }
}`
	if err := h.AddMemoryFile("package.json", jsonContent); err != nil {
		t.Fatalf("Failed to add JSON file: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for file processing
	time.Sleep(5 * time.Second)

	// Verify file metadata via API
	metadata, err := h.HTTPClient.GetFileMetadata("package.json")
	if err != nil {
		t.Fatalf("Failed to get file metadata: %v", err)
	}

	// Verify basic metadata fields
	metadataMap, ok := metadata.(map[string]any)
	if !ok {
		t.Fatalf("Unexpected metadata format: %T", metadata)
	}

	if file, ok := metadataMap["file"].(map[string]any); ok {
		if fileType, ok := file["type"].(string); !ok || fileType != "data" {
			t.Errorf("Expected type 'data', got %v", file["type"])
		}
		if category, ok := file["category"].(string); !ok || category != "data" {
			t.Errorf("Expected category 'data', got %v", file["category"])
		}
		t.Logf("JSON metadata: type=%v, category=%v", file["type"], file["category"])
	} else {
		t.Error("Missing 'file' field in metadata response")
	}
}
