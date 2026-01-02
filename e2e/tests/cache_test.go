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

// TestCache_DirectoryCreation tests that cache directory is created
func TestCache_DirectoryCreation(t *testing.T) {
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

	// Add test file
	if err := h.AddMemoryFile("cache-test.md", "# Cache Test\n\nTest file for cache."); err != nil {
		t.Fatalf("Failed to add file: %v", err)
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

	// Verify cache directory was created
	cacheDir := filepath.Join(h.MemoryRoot, ".cache")
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		t.Errorf("Cache directory was not created: %s", cacheDir)
	} else {
		t.Logf("Cache directory exists: %s", cacheDir)
	}
}

// TestCache_FileProcessing tests that files are processed and indexed
func TestCache_FileProcessing(t *testing.T) {
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

	// Add test file
	content := "# Test Document\n\nThis is test content for cache processing."
	if err := h.AddMemoryFile("process-test.md", content); err != nil {
		t.Fatalf("Failed to add file: %v", err)
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

	// Verify file appears in index
	files, err := h.HTTPClient.GetIndexFiles()
	if err != nil {
		t.Fatalf("Failed to get index files: %v", err)
	}

	found := false
	for _, fileMap := range files {
		if name, ok := fileMap["name"].(string); ok && name == "process-test.md" {
			found = true
			// Verify hash field exists (content-addressable)
			if hash, ok := fileMap["hash"].(string); !ok || hash == "" {
				t.Error("File missing hash field")
			} else {
				t.Logf("File hash: %s", hash)
			}
			break
		}
	}

	if !found {
		t.Error("File not found in index after processing")
	}
}

// TestCache_ModifiedFile tests that modified files trigger reprocessing
func TestCache_ModifiedFile(t *testing.T) {
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

	// Add initial file
	initialContent := "# Original Content\n\nThis is the initial version."
	if err := h.AddMemoryFile("modify-test.md", initialContent); err != nil {
		t.Fatalf("Failed to add file: %v", err)
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

	// Wait for initial processing
	time.Sleep(5 * time.Second)

	// Get initial file metadata
	metadata1, err := h.HTTPClient.GetFileMetadata("modify-test.md")
	if err != nil {
		t.Fatalf("Failed to get initial metadata: %v", err)
	}

	// Extract initial hash
	var initialHash string
	if m1, ok := metadata1.(map[string]any); ok {
		if file, ok := m1["file"].(map[string]any); ok {
			if hash, ok := file["hash"].(string); ok {
				initialHash = hash
				t.Logf("Initial hash: %s", initialHash)
			}
		}
	}

	// Modify the file (different content = different hash)
	modifiedContent := "# Modified Content\n\nThis is the updated version with different text."
	if err := h.AddMemoryFile("modify-test.md", modifiedContent); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	// Wait for reprocessing
	time.Sleep(3 * time.Second)

	// Get modified file metadata
	metadata2, err := h.HTTPClient.GetFileMetadata("modify-test.md")
	if err != nil {
		t.Fatalf("Failed to get modified metadata: %v", err)
	}

	// Extract modified hash
	var modifiedHash string
	if m2, ok := metadata2.(map[string]any); ok {
		if file, ok := m2["file"].(map[string]any); ok {
			if hash, ok := file["hash"].(string); ok {
				modifiedHash = hash
				t.Logf("Modified hash: %s", modifiedHash)
			}
		}
	}

	// Verify hash changed (cache invalidation occurred)
	if initialHash == "" || modifiedHash == "" {
		t.Error("Failed to extract hashes for comparison")
	} else if initialHash == modifiedHash {
		t.Error("Hash did not change after file modification (cache invalidation failed)")
	} else {
		t.Logf("Cache invalidation verified: hash changed from %s to %s", initialHash, modifiedHash)
	}
}

// TestCache_IdenticalContent tests that identical content gets same hash
func TestCache_IdenticalContent(t *testing.T) {
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

	// Add two files with identical content
	identicalContent := "# Identical Content\n\nThis content is the same in both files."
	if err := h.AddMemoryFile("file1.md", identicalContent); err != nil {
		t.Fatalf("Failed to add file1: %v", err)
	}
	if err := h.AddMemoryFile("file2.md", identicalContent); err != nil {
		t.Fatalf("Failed to add file2: %v", err)
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

	// Get metadata for both files
	metadata1, err := h.HTTPClient.GetFileMetadata("file1.md")
	if err != nil {
		t.Fatalf("Failed to get file1 metadata: %v", err)
	}

	metadata2, err := h.HTTPClient.GetFileMetadata("file2.md")
	if err != nil {
		t.Fatalf("Failed to get file2 metadata: %v", err)
	}

	// Extract hashes
	var hash1, hash2 string
	if m1, ok := metadata1.(map[string]any); ok {
		if file, ok := m1["file"].(map[string]any); ok {
			if h, ok := file["hash"].(string); ok {
				hash1 = h
			}
		}
	}
	if m2, ok := metadata2.(map[string]any); ok {
		if file, ok := m2["file"].(map[string]any); ok {
			if h, ok := file["hash"].(string); ok {
				hash2 = h
			}
		}
	}

	// Verify identical content produces identical hash
	if hash1 == "" || hash2 == "" {
		t.Error("Failed to extract hashes")
	} else if hash1 != hash2 {
		t.Errorf("Identical content produced different hashes: %s vs %s", hash1, hash2)
	} else {
		t.Logf("Content-addressable cache verified: identical content = identical hash (%s)", hash1)
	}
}

// TestCache_StatusCommand tests the cache status command output
func TestCache_StatusCommand(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Test cache status with empty cache
	stdout, stderr, exitCode := h.RunCommand("cache", "status")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "Cache Status")
	harness.AssertContains(t, stdout, "Current Version:")
	harness.AssertContains(t, stdout, "Total Entries:")
	harness.AssertContains(t, stdout, "Total Size:")
}

// TestCache_StatusWithEntries tests cache status output format
// Note: Without Claude API key, no cache entries are created, so we just verify the output format
func TestCache_StatusWithEntries(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Test cache status command output format (doesn't require daemon)
	stdout, stderr, exitCode := h.RunCommand("cache", "status")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "Cache Status")
	harness.AssertContains(t, stdout, "Current Version:")
	harness.AssertContains(t, stdout, "Statistics")
	harness.AssertContains(t, stdout, "Total Entries:")
	harness.AssertContains(t, stdout, "Total Size:")
	harness.AssertContains(t, stdout, "Legacy Entries:")

	t.Logf("Cache status output:\n%s", stdout)
}

// TestCache_ClearRequiresFlag tests that cache clear requires a flag
func TestCache_ClearRequiresFlag(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Try to clear without flag
	stdout, stderr, exitCode := h.RunCommand("cache", "clear")

	// Should fail and show usage
	if exitCode == 0 {
		t.Errorf("Expected non-zero exit code when no flag provided")
	}

	output := stdout + stderr
	harness.AssertContains(t, output, "--all")
	harness.AssertContains(t, output, "--stale")
}

// TestCache_ClearAll tests the cache clear --all command
func TestCache_ClearAll(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Clear all cache entries (may be empty, which is fine)
	stdout, stderr, exitCode := h.RunCommand("cache", "clear", "--all")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	// Accept either "Cache cleared successfully" or "Cache is already empty"
	output := stdout + stderr
	if !containsAny(output, "Cache cleared successfully", "Cache is already empty") {
		t.Errorf("Expected cache clear message, got: %s", output)
	}

	// Verify cache is empty
	stdout2, stderr2, exitCode2 := h.RunCommand("cache", "status")
	harness.AssertExitCode(t, 0, exitCode2, stdout2, stderr2)
	harness.AssertContains(t, stdout2, "Total Entries:  0")
}

// TestCache_ClearStale tests the cache clear --stale command
func TestCache_ClearStale(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Clear stale entries (should have no effect on empty/current-only cache)
	stdout, stderr, exitCode := h.RunCommand("cache", "clear", "--stale")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	// Should indicate no stale entries to clear
	output := stdout + stderr
	if !containsAny(output, "No stale entries", "Removed 0", "All", "current") {
		t.Logf("Output: %s", output)
	}
}

// TestCache_ClearMutuallyExclusive tests that --all and --stale are mutually exclusive
func TestCache_ClearMutuallyExclusive(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Try to use both flags
	stdout, stderr, exitCode := h.RunCommand("cache", "clear", "--all", "--stale")

	// Should fail
	if exitCode == 0 {
		t.Errorf("Expected non-zero exit code when both flags provided")
	}

	output := stdout + stderr
	harness.AssertContains(t, output, "cannot use both")
}

// TestCache_HelpCommand tests cache command help
func TestCache_HelpCommand(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	tests := []struct {
		name string
		args []string
	}{
		{"cache help", []string{"cache", "--help"}},
		{"cache status help", []string{"cache", "status", "--help"}},
		{"cache clear help", []string{"cache", "clear", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, exitCode := h.RunCommand(tt.args...)

			harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
			harness.AssertContains(t, stdout, "Usage:")
		})
	}
}

// containsAny checks if s contains any of the substrings
func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
