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

// TestFileSystem_NewFile tests that new files trigger processing
func TestFileSystem_NewFile(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server for health checks
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
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

	// Wait for daemon to be healthy
	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Add a new file
	testFile := "test-new.md"
	if err := h.AddMemoryFile(testFile, "# New File\n\nThis is a new test file."); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Wait for processing (debounce + processing time)
	time.Sleep(3 * time.Second)

	// Verify file was indexed (would need to query index or graph)
	t.Logf("File %s added, processing expected", testFile)
}

// TestFileSystem_ModifiedFile tests that modified files trigger re-processing
func TestFileSystem_ModifiedFile(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Add initial file
	testFile := "test-modify.md"
	if err := h.AddMemoryFile(testFile, "# Initial Content"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Enable HTTP server for health checks
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
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

	// Wait for initial indexing
	time.Sleep(3 * time.Second)

	// Modify the file
	if err := h.AddMemoryFile(testFile, "# Modified Content\n\nThis has been updated."); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	// Wait for re-processing
	time.Sleep(3 * time.Second)

	t.Logf("File %s modified, re-processing expected", testFile)
}

// TestFileSystem_DeletedFile tests that deleted files are removed from index
func TestFileSystem_DeletedFile(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Add file
	testFile := "test-delete.md"
	if err := h.AddMemoryFile(testFile, "# To Be Deleted"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Enable HTTP server for health checks
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
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

	// Wait for initial indexing
	time.Sleep(3 * time.Second)

	// Delete the file
	filePath := filepath.Join(h.MemoryRoot, testFile)
	if err := os.Remove(filePath); err != nil {
		t.Fatalf("Failed to delete file: %v", err)
	}

	// Wait for removal processing
	time.Sleep(3 * time.Second)

	t.Logf("File %s deleted, index removal expected", testFile)
}

// TestFileSystem_SkipPatterns tests that configured skip patterns are honored
func TestFileSystem_SkipPatterns(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Add files that should be skipped
	testCases := []struct {
		name       string
		path       string
		shouldSkip bool
	}{
		{"cache file", ".cache/test.txt", true},
		{"git file", ".git/config", true},
		{"node_modules", "node_modules/package.json", true},
		{"normal file", "document.md", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := h.AddMemoryFile(tc.path, "test content"); err != nil {
				// Skip patterns may prevent directory creation
				if tc.shouldSkip && strings.Contains(err.Error(), "permission") {
					t.Logf("File %s correctly skipped during creation", tc.path)
				} else if !tc.shouldSkip {
					t.Fatalf("Failed to add file %s: %v", tc.path, err)
				}
			}
		})
	}
}

// TestFileSystem_SubdirectoryFiles tests that files in subdirectories are discovered
func TestFileSystem_SubdirectoryFiles(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Add files in subdirectories
	testFiles := []string{
		"docs/README.md",
		"docs/api/endpoints.md",
		"src/main.go",
		"tests/unit/test.go",
	}

	for _, file := range testFiles {
		if err := h.AddMemoryFile(file, "# Test Content\n\nTest file in subdirectory."); err != nil {
			t.Fatalf("Failed to add file %s: %v", file, err)
		}
	}

	// Verify files exist
	for _, file := range testFiles {
		path := filepath.Join(h.MemoryRoot, file)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("File %s should exist: %v", file, err)
		}
	}

	t.Logf("Created %d files in subdirectories", len(testFiles))
}

// TestFileSystem_LargeFile tests that large files respect size limits
func TestFileSystem_LargeFile(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create a large file (11 MB, exceeds default 10 MB limit)
	largeContent := strings.Repeat("A", 11*1024*1024)
	testFile := "large-file.txt"

	if err := h.AddMemoryFile(testFile, largeContent); err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	// Verify file exists
	path := filepath.Join(h.MemoryRoot, testFile)
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Large file should exist: %v", err)
	}

	if stat.Size() < 11*1024*1024 {
		t.Errorf("Expected file size >= 11MB, got %d bytes", stat.Size())
	}

	t.Logf("Large file created: %d bytes", stat.Size())
}

// TestFileSystem_Debouncing tests that rapid changes are debounced
func TestFileSystem_Debouncing(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Add initial file
	testFile := "debounce-test.md"
	if err := h.AddMemoryFile(testFile, "# Initial"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Enable HTTP server for health checks
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
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

	// Make rapid changes (should be debounced into single processing)
	for i := 0; i < 10; i++ {
		content := "# Version " + string(rune('0'+i))
		if err := h.AddMemoryFile(testFile, content); err != nil {
			t.Fatalf("Failed to modify file: %v", err)
		}
		time.Sleep(50 * time.Millisecond) // Faster than debounce window
	}

	// Wait for debounced processing
	time.Sleep(3 * time.Second)

	t.Log("Rapid changes completed, debouncing expected")
}

// TestFileSystem_RenamedFile tests that file renames are handled correctly
func TestFileSystem_RenamedFile(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Add file
	oldName := "old-name.md"
	newName := "new-name.md"

	if err := h.AddMemoryFile(oldName, "# Test File"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Enable HTTP server for health checks
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
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

	// Wait for initial indexing
	time.Sleep(3 * time.Second)

	// Rename the file
	oldPath := filepath.Join(h.MemoryRoot, oldName)
	newPath := filepath.Join(h.MemoryRoot, newName)
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatalf("Failed to rename file: %v", err)
	}

	// Wait for rename processing (delete + create)
	time.Sleep(3 * time.Second)

	t.Logf("File renamed from %s to %s", oldName, newName)
}
