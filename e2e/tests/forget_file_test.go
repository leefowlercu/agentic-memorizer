//go:build e2e

package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/e2e/harness"
)

// TestForgetFile_SingleFile tests forgetting a single file
func TestForgetFile_SingleFile(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create a file in memory directory
	memFile := filepath.Join(h.MemoryRoot, "to-forget.md")
	if err := os.WriteFile(memFile, []byte("# Content to forget"), 0644); err != nil {
		t.Fatalf("Failed to write memory file: %v", err)
	}

	// Forget the file
	stdout, stderr, exitCode := h.RunCommand("forget", "file", memFile)

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "moved to")

	// Verify file is gone from memory directory
	if _, err := os.Stat(memFile); !os.IsNotExist(err) {
		t.Error("File should be removed from memory directory")
	}

	// Verify file exists in forgotten directory
	forgottenDir := filepath.Join(h.AppDir, ".forgotten")
	expectedPath := filepath.Join(forgottenDir, "to-forget.md")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("File not found in forgotten directory: %s", expectedPath)
	}

	// Verify content is preserved
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("Failed to read forgotten file: %v", err)
	}
	if string(content) != "# Content to forget" {
		t.Errorf("File content not preserved; got %q", string(content))
	}
}

// TestForgetFile_Directory tests forgetting a directory recursively
func TestForgetFile_Directory(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create a directory with files in memory
	memDir := filepath.Join(h.MemoryRoot, "old-project")
	files := map[string]string{
		"readme.md":       "# Old Project",
		"docs/guide.md":   "# Guide",
		"docs/api.md":     "# API",
		"src/main.go":     "package main",
		"src/lib/util.go": "package lib",
	}
	for name, content := range files {
		path := filepath.Join(memDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create dir for %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", name, err)
		}
	}

	// Forget the directory
	stdout, stderr, exitCode := h.RunCommand("forget", "file", memDir)

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "moved")
	harness.AssertContains(t, stdout, "files to")

	// Verify directory is gone from memory
	if _, err := os.Stat(memDir); !os.IsNotExist(err) {
		t.Error("Directory should be removed from memory")
	}

	// Verify files exist in forgotten directory with preserved structure
	forgottenDir := filepath.Join(h.AppDir, ".forgotten")
	for name := range files {
		expectedPath := filepath.Join(forgottenDir, "old-project", name)
		if _, err := os.Stat(expectedPath); err != nil {
			t.Errorf("File not found in forgotten directory: %s", expectedPath)
		}
	}
}

// TestForgetFile_WithDryRunFlag tests dry-run mode
func TestForgetFile_WithDryRunFlag(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create a file in memory directory
	memFile := filepath.Join(h.MemoryRoot, "dryrun-forget.md")
	if err := os.WriteFile(memFile, []byte("# Dry Run Test"), 0644); err != nil {
		t.Fatalf("Failed to write memory file: %v", err)
	}

	// Forget with --dry-run
	stdout, stderr, exitCode := h.RunCommand("forget", "file", "--dry-run", memFile)

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "would move to")

	// Verify file still exists in memory directory
	if _, err := os.Stat(memFile); err != nil {
		t.Error("File should still exist in memory directory during dry-run")
	}

	// Verify file was NOT moved to forgotten directory
	forgottenDir := filepath.Join(h.AppDir, ".forgotten")
	forgottenPath := filepath.Join(forgottenDir, "dryrun-forget.md")
	if _, err := os.Stat(forgottenPath); !os.IsNotExist(err) {
		t.Error("File should not exist in forgotten directory during dry-run")
	}
}

// TestForgetFile_ConflictResolution tests automatic renaming on conflicts in .forgotten
func TestForgetFile_ConflictResolution(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create existing files in forgotten directory
	forgottenDir := filepath.Join(h.AppDir, ".forgotten")
	if err := os.MkdirAll(forgottenDir, 0755); err != nil {
		t.Fatalf("Failed to create forgotten dir: %v", err)
	}
	for i := 0; i <= 2; i++ {
		var name string
		if i == 0 {
			name = "conflict.md"
		} else {
			name = "conflict-" + string('0'+byte(i)) + ".md"
		}
		path := filepath.Join(forgottenDir, name)
		if err := os.WriteFile(path, []byte("Old forgotten"), 0644); err != nil {
			t.Fatalf("Failed to write forgotten file %s: %v", name, err)
		}
	}

	// Create a file in memory with conflicting name
	memFile := filepath.Join(h.MemoryRoot, "conflict.md")
	if err := os.WriteFile(memFile, []byte("New to forget"), 0644); err != nil {
		t.Fatalf("Failed to write memory file: %v", err)
	}

	// Forget the file (should auto-rename)
	stdout, stderr, exitCode := h.RunCommand("forget", "file", memFile)

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "renamed due to conflict")

	// Verify file was renamed (should be conflict-3.md since 0-2 exist)
	expectedPath := filepath.Join(forgottenDir, "conflict-3.md")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("Renamed file not found: %s", expectedPath)
	}

	// Verify content
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("Failed to read renamed file: %v", err)
	}
	if string(content) != "New to forget" {
		t.Errorf("File content mismatch; got %q", string(content))
	}
}

// TestForgetFile_PathOutsideMemory tests error when path is outside memory directory
func TestForgetFile_PathOutsideMemory(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create a file outside memory directory
	outsideFile := filepath.Join(h.AppDir, "outside", "external.md")
	if err := os.MkdirAll(filepath.Dir(outsideFile), 0755); err != nil {
		t.Fatalf("Failed to create outside dir: %v", err)
	}
	if err := os.WriteFile(outsideFile, []byte("External file"), 0644); err != nil {
		t.Fatalf("Failed to write outside file: %v", err)
	}

	// Try to forget the file
	stdout, stderr, exitCode := h.RunCommand("forget", "file", outsideFile)

	if exitCode == 0 {
		t.Error("Expected command to fail for path outside memory directory")
	}

	output := stdout + stderr
	harness.AssertContains(t, output, "not in memory directory")
	// Should show usage since this is an input validation error
	harness.AssertContains(t, output, "Usage:")
}

// TestForgetFile_NonExistentPath tests validation for non-existent paths
func TestForgetFile_NonExistentPath(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Try to forget non-existent file (within memory directory path)
	nonExistentPath := filepath.Join(h.MemoryRoot, "does-not-exist.md")
	stdout, stderr, exitCode := h.RunCommand("forget", "file", nonExistentPath)

	if exitCode == 0 {
		t.Error("Expected command to fail for non-existent path")
	}

	output := stdout + stderr
	harness.AssertContains(t, output, "does not exist")
	// Should show usage since this is an input validation error
	harness.AssertContains(t, output, "Usage:")
}

// TestForgetFile_BatchOperationWithErrors tests batch operations with some failures
func TestForgetFile_BatchOperationWithErrors(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create valid file in memory
	validFile := filepath.Join(h.MemoryRoot, "valid.md")
	if err := os.WriteFile(validFile, []byte("Valid content"), 0644); err != nil {
		t.Fatalf("Failed to write valid file: %v", err)
	}

	// Non-existent file
	nonExistent := filepath.Join(h.MemoryRoot, "does-not-exist.md")

	// Run batch operation
	stdout, stderr, exitCode := h.RunCommand("forget", "file", validFile, nonExistent)

	// Should fail because one file doesn't exist
	if exitCode == 0 {
		t.Error("Expected command to fail when one file doesn't exist")
	}

	output := stdout + stderr
	harness.AssertContains(t, output, "does not exist")
	// Should show usage since this is an input validation error
	harness.AssertContains(t, output, "Usage:")
}

// TestForgetFile_MultipleFiles tests forgetting multiple files in one command
func TestForgetFile_MultipleFiles(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create files in memory directory
	files := []string{"file1.md", "file2.md", "file3.md"}
	var memPaths []string
	for _, name := range files {
		path := filepath.Join(h.MemoryRoot, name)
		if err := os.WriteFile(path, []byte("Content of "+name), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", name, err)
		}
		memPaths = append(memPaths, path)
	}

	// Forget multiple files
	args := append([]string{"forget", "file"}, memPaths...)
	stdout, stderr, exitCode := h.RunCommand(args...)

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "3 forgotten")

	// Verify all files removed from memory
	for _, name := range files {
		memPath := filepath.Join(h.MemoryRoot, name)
		if _, err := os.Stat(memPath); !os.IsNotExist(err) {
			t.Errorf("File should be removed from memory: %s", memPath)
		}
	}

	// Verify all files in forgotten directory
	forgottenDir := filepath.Join(h.AppDir, ".forgotten")
	for _, name := range files {
		forgottenPath := filepath.Join(forgottenDir, name)
		if _, err := os.Stat(forgottenPath); err != nil {
			t.Errorf("File not found in forgotten directory: %s", forgottenPath)
		}
	}
}

// TestForgetFile_PreservesRelativePath tests that nested files preserve their path structure
func TestForgetFile_PreservesRelativePath(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create nested file in memory directory
	nestedPath := filepath.Join(h.MemoryRoot, "project", "docs", "guide.md")
	if err := os.MkdirAll(filepath.Dir(nestedPath), 0755); err != nil {
		t.Fatalf("Failed to create nested dir: %v", err)
	}
	if err := os.WriteFile(nestedPath, []byte("# Guide"), 0644); err != nil {
		t.Fatalf("Failed to write nested file: %v", err)
	}

	// Forget the file
	stdout, stderr, exitCode := h.RunCommand("forget", "file", nestedPath)

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Verify file is in forgotten with preserved path structure
	forgottenDir := filepath.Join(h.AppDir, ".forgotten")
	expectedPath := filepath.Join(forgottenDir, "project", "docs", "guide.md")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("File not found with preserved path structure: %s", expectedPath)
	}
}

// TestForgetFile_CommandHelp tests help output for forget file command
func TestForgetFile_CommandHelp(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	stdout, stderr, exitCode := h.RunCommand("forget", "file", "--help")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "Usage:")
	harness.AssertContains(t, stdout, "--dry-run")
	harness.AssertContains(t, stdout, ".forgotten")
}

// TestForgetFile_NoArgs tests error when no arguments provided
func TestForgetFile_NoArgs(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	stdout, stderr, exitCode := h.RunCommand("forget", "file")

	if exitCode == 0 {
		t.Error("Expected command to fail with no arguments")
	}

	output := stdout + stderr
	// Should mention minimum args required
	if !strings.Contains(output, "requires at least 1 arg") && !strings.Contains(output, "minimum") {
		t.Logf("Output: %s", output)
	}
	harness.AssertContains(t, output, "Usage:")
}

// TestForgetFile_RecoveryWorkflow tests that forgotten files can be recovered with remember
func TestForgetFile_RecoveryWorkflow(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create a file in memory
	memFile := filepath.Join(h.MemoryRoot, "recoverable.md")
	originalContent := "# Recoverable Content\n\nThis should be recoverable."
	if err := os.WriteFile(memFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to write memory file: %v", err)
	}

	// Forget the file
	stdout, stderr, exitCode := h.RunCommand("forget", "file", memFile)
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Verify file is gone from memory
	if _, err := os.Stat(memFile); !os.IsNotExist(err) {
		t.Error("File should be removed from memory")
	}

	// Find the file in forgotten directory
	forgottenDir := filepath.Join(h.AppDir, ".forgotten")
	forgottenPath := filepath.Join(forgottenDir, "recoverable.md")
	if _, err := os.Stat(forgottenPath); err != nil {
		t.Fatalf("File not found in forgotten directory: %s", forgottenPath)
	}

	// Recover using remember file command
	stdout, stderr, exitCode = h.RunCommand("remember", "file", forgottenPath)
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Verify file is back in memory with original content
	content, err := os.ReadFile(memFile)
	if err != nil {
		t.Fatalf("Failed to read recovered file: %v", err)
	}
	if string(content) != originalContent {
		t.Errorf("Recovered content mismatch; got %q, want %q", string(content), originalContent)
	}
}
