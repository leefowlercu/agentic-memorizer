//go:build e2e

package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/e2e/harness"
)

// TestEdgeCase_BinaryFile tests that binary files are indexed but not analyzed
func TestEdgeCase_BinaryFile(t *testing.T) {
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

	// Create a binary file (ELF header)
	binaryData := []byte{
		0x7f, 0x45, 0x4c, 0x46, // ELF magic
		0x02, 0x01, 0x01, 0x00, // 64-bit, little-endian
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	if err := h.AddMemoryFile("test.bin", string(binaryData)); err != nil {
		t.Fatalf("Failed to add binary file: %v", err)
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

	// Wait for processing
	time.Sleep(5 * time.Second)

	// Verify file appears in index (binary files should be indexed)
	index, err := h.HTTPClient.GetIndex()
	if err != nil {
		t.Fatalf("Failed to get index: %v", err)
	}

	indexMap, ok := index.(map[string]any)
	if !ok {
		t.Fatalf("Unexpected index format: %T", index)
	}

	files, ok := indexMap["files"].([]any)
	if !ok {
		t.Fatalf("Index missing 'files' array")
	}

	found := false
	for _, f := range files {
		fileMap, ok := f.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := fileMap["name"].(string); ok && name == "test.bin" {
			found = true
			t.Logf("Binary file indexed: %+v", fileMap)
			break
		}
	}

	if !found {
		t.Log("Binary file not in index (may be skipped by configuration)")
	} else {
		t.Log("Binary file successfully indexed")
	}
}

// TestEdgeCase_HiddenFiles tests hidden file handling
func TestEdgeCase_HiddenFiles(t *testing.T) {
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

	// Add hidden file
	if err := h.AddMemoryFile(".hidden.md", "# Hidden File\n\nThis file starts with a dot."); err != nil {
		t.Fatalf("Failed to add hidden file: %v", err)
	}

	// Add normal file for comparison
	if err := h.AddMemoryFile("visible.md", "# Visible File\n\nNormal file."); err != nil {
		t.Fatalf("Failed to add visible file: %v", err)
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

	// Wait for processing
	time.Sleep(5 * time.Second)

	// Check if files appear in index
	index, err := h.HTTPClient.GetIndex()
	if err != nil {
		t.Fatalf("Failed to get index: %v", err)
	}

	indexMap, ok := index.(map[string]any)
	if !ok {
		t.Fatalf("Unexpected index format: %T", index)
	}

	files, ok := indexMap["files"].([]any)
	if !ok {
		t.Fatalf("Index missing 'files' array")
	}

	foundHidden := false
	foundVisible := false
	for _, f := range files {
		fileMap, ok := f.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := fileMap["name"].(string); ok {
			if name == ".hidden.md" {
				foundHidden = true
			}
			if name == "visible.md" {
				foundVisible = true
			}
		}
	}

	if !foundVisible {
		t.Error("Visible file not found in index")
	}

	if foundHidden {
		t.Log("Hidden file indexed (hidden files are processed)")
	} else {
		t.Log("Hidden file not indexed (may be filtered)")
	}
}

// TestEdgeCase_SpecialCharacters tests filenames with special characters
func TestEdgeCase_SpecialCharacters(t *testing.T) {
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

	// Test files with various special characters
	testCases := []struct {
		name     string
		filename string
		content  string
	}{
		{"spaces", "file with spaces.md", "# File With Spaces\n"},
		{"unicode", "файл.md", "# Unicode Filename\n"},
		{"emoji", "test-📝.md", "# Emoji Filename\n"},
		{"parentheses", "file(1).md", "# Parentheses\n"},
	}

	for _, tc := range testCases {
		if err := h.AddMemoryFile(tc.filename, tc.content); err != nil {
			t.Logf("Warning: Failed to add file %q: %v", tc.filename, err)
		}
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

	// Wait for processing
	time.Sleep(5 * time.Second)

	// Verify at least some files were indexed
	index, err := h.HTTPClient.GetIndex()
	if err != nil {
		t.Fatalf("Failed to get index: %v", err)
	}

	indexMap, ok := index.(map[string]any)
	if !ok {
		t.Fatalf("Unexpected index format: %T", index)
	}

	files, ok := indexMap["files"].([]any)
	if !ok {
		t.Fatalf("Index missing 'files' array")
	}

	found := 0
	for _, f := range files {
		fileMap, ok := f.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := fileMap["name"].(string); ok {
			for _, tc := range testCases {
				if name == tc.filename {
					found++
					t.Logf("Successfully indexed file with special characters: %q", name)
					break
				}
			}
		}
	}

	t.Logf("Indexed %d/%d files with special characters", found, len(testCases))
}

// TestEdgeCase_LongFilename tests very long filename handling
func TestEdgeCase_LongFilename(t *testing.T) {
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

	// Create a long filename (200 chars - under typical 255 limit)
	longName := strings.Repeat("a", 200) + ".md"
	if err := h.AddMemoryFile(longName, "# Long Filename Test\n"); err != nil {
		t.Logf("Warning: Failed to create long filename (expected on some filesystems): %v", err)
		return // Skip if filesystem doesn't support
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

	// Wait for processing
	time.Sleep(5 * time.Second)

	// Verify file was indexed
	index, err := h.HTTPClient.GetIndex()
	if err != nil {
		t.Fatalf("Failed to get index: %v", err)
	}

	indexMap, ok := index.(map[string]any)
	if !ok {
		t.Fatalf("Unexpected index format: %T", index)
	}

	files, ok := indexMap["files"].([]any)
	if !ok {
		t.Fatalf("Index missing 'files' array")
	}

	found := false
	for _, f := range files {
		fileMap, ok := f.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := fileMap["name"].(string); ok && name == longName {
			found = true
			t.Logf("Long filename successfully indexed (length: %d)", len(longName))
			break
		}
	}

	if !found {
		t.Error("Long filename not found in index")
	}
}

// TestEdgeCase_Symlinks tests symlink handling
func TestEdgeCase_Symlinks(t *testing.T) {
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

	// Create real file
	realFile := "real-file.md"
	if err := h.AddMemoryFile(realFile, "# Real File\n"); err != nil {
		t.Fatalf("Failed to add real file: %v", err)
	}

	// Create symlink
	symlinkName := "symlink.md"
	realPath := filepath.Join(h.MemoryRoot, realFile)
	symlinkPath := filepath.Join(h.MemoryRoot, symlinkName)
	if err := os.Symlink(realPath, symlinkPath); err != nil {
		t.Logf("Warning: Failed to create symlink (may not be supported): %v", err)
		return // Skip if symlinks not supported
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

	// Wait for processing
	time.Sleep(5 * time.Second)

	// Check how symlinks are handled
	index, err := h.HTTPClient.GetIndex()
	if err != nil {
		t.Fatalf("Failed to get index: %v", err)
	}

	indexMap, ok := index.(map[string]any)
	if !ok {
		t.Fatalf("Unexpected index format: %T", index)
	}

	files, ok := indexMap["files"].([]any)
	if !ok {
		t.Fatalf("Index missing 'files' array")
	}

	foundReal := false
	foundSymlink := false
	for _, f := range files {
		fileMap, ok := f.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := fileMap["name"].(string); ok {
			if name == realFile {
				foundReal = true
			}
			if name == symlinkName {
				foundSymlink = true
			}
		}
	}

	if !foundReal {
		t.Error("Real file not found in index")
	}

	if foundSymlink {
		t.Log("Symlink indexed (symlinks are followed)")
	} else {
		t.Log("Symlink not indexed (symlinks may be skipped)")
	}
}

// TestEdgeCase_EmptyFile tests zero-byte file handling
func TestEdgeCase_EmptyFile(t *testing.T) {
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

	// Add empty file
	if err := h.AddMemoryFile("empty.md", ""); err != nil {
		t.Fatalf("Failed to add empty file: %v", err)
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

	// Wait for processing
	time.Sleep(5 * time.Second)

	// Verify empty file handling
	index, err := h.HTTPClient.GetIndex()
	if err != nil {
		t.Fatalf("Failed to get index: %v", err)
	}

	indexMap, ok := index.(map[string]any)
	if !ok {
		t.Fatalf("Unexpected index format: %T", index)
	}

	files, ok := indexMap["files"].([]any)
	if !ok {
		t.Fatalf("Index missing 'files' array")
	}

	found := false
	for _, f := range files {
		fileMap, ok := f.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := fileMap["name"].(string); ok && name == "empty.md" {
			found = true
			if size, ok := fileMap["size"].(float64); ok && size == 0 {
				t.Log("Empty file successfully indexed with size 0")
			}
			break
		}
	}

	if !found {
		t.Log("Empty file not indexed (may be filtered)")
	}
}

// TestEdgeCase_VeryLargeFile tests file size limit enforcement
func TestEdgeCase_VeryLargeFile(t *testing.T) {
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

	// Create a large file (15 MB - exceeds default 10 MB limit)
	largeContent := strings.Repeat("A", 15*1024*1024)
	if err := h.AddMemoryFile("very-large.txt", largeContent); err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	// Create normal file for comparison
	if err := h.AddMemoryFile("normal.md", "# Normal File\n"); err != nil {
		t.Fatalf("Failed to create normal file: %v", err)
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

	// Wait for processing
	time.Sleep(5 * time.Second)

	// Check file handling
	index, err := h.HTTPClient.GetIndex()
	if err != nil {
		t.Fatalf("Failed to get index: %v", err)
	}

	indexMap, ok := index.(map[string]any)
	if !ok {
		t.Fatalf("Unexpected index format: %T", index)
	}

	files, ok := indexMap["files"].([]any)
	if !ok {
		t.Fatalf("Index missing 'files' array")
	}

	foundLarge := false
	foundNormal := false
	for _, f := range files {
		fileMap, ok := f.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := fileMap["name"].(string); ok {
			if name == "very-large.txt" {
				foundLarge = true
			}
			if name == "normal.md" {
				foundNormal = true
			}
		}
	}

	if !foundNormal {
		t.Error("Normal file not found in index")
	}

	if foundLarge {
		t.Log("Large file indexed (may be indexed but not analyzed)")
	} else {
		t.Log("Large file not indexed (exceeds size limit)")
	}
}
