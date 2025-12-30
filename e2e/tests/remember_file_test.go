//go:build e2e

package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/e2e/harness"
)

// TestRememberFile_SingleFile tests remembering a single file
func TestRememberFile_SingleFile(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create a source file outside memory directory
	srcFile := filepath.Join(h.AppDir, "source", "test-file.md")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	if err := os.WriteFile(srcFile, []byte("# Test Content\n\nThis is a test file."), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Remember the file
	stdout, stderr, exitCode := h.RunCommand("remember", "file", srcFile)

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "copied to")

	// Verify file exists in memory directory
	expectedPath := filepath.Join(h.MemoryRoot, "test-file.md")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("File not found in memory directory: %s", expectedPath)
	}

	// Verify original file still exists (copy not move)
	if _, err := os.Stat(srcFile); err != nil {
		t.Errorf("Original file was removed (should be copied, not moved): %s", srcFile)
	}
}

// TestRememberFile_Directory tests remembering a directory recursively
func TestRememberFile_Directory(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create a source directory with files
	srcDir := filepath.Join(h.AppDir, "source", "docs")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	files := map[string]string{
		"readme.md":         "# Readme\n\nProject readme.",
		"guide.md":          "# Guide\n\nUser guide.",
		"nested/chapter.md": "# Chapter 1\n\nContent here.",
	}
	for name, content := range files {
		path := filepath.Join(srcDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create dir for %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", name, err)
		}
	}

	// Remember the directory
	stdout, stderr, exitCode := h.RunCommand("remember", "file", srcDir)

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "copied")
	harness.AssertContains(t, stdout, "files to")

	// Verify files exist in memory directory
	for name := range files {
		expectedPath := filepath.Join(h.MemoryRoot, "docs", name)
		if _, err := os.Stat(expectedPath); err != nil {
			t.Errorf("File not found in memory directory: %s", expectedPath)
		}
	}
}

// TestRememberFile_WithDirFlag tests remembering files into a subdirectory
func TestRememberFile_WithDirFlag(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create a source file
	srcFile := filepath.Join(h.AppDir, "source", "notes.md")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	if err := os.WriteFile(srcFile, []byte("# Notes\n\nSome notes."), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Remember with --dir flag
	stdout, stderr, exitCode := h.RunCommand("remember", "file", "--dir", "work/projects", srcFile)

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "copied to")
	harness.AssertContains(t, stdout, "work/projects")

	// Verify file exists in subdirectory
	expectedPath := filepath.Join(h.MemoryRoot, "work", "projects", "notes.md")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("File not found in subdirectory: %s", expectedPath)
	}
}

// TestRememberFile_WithDirFlagInvalid tests validation of invalid --dir values
func TestRememberFile_WithDirFlagInvalid(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create a source file
	srcFile := filepath.Join(h.AppDir, "source", "test.md")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	if err := os.WriteFile(srcFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	tests := []struct {
		name     string
		dirValue string
		contains string
	}{
		{"parent traversal", "../outside", ".."},
		{"absolute path", "/etc/passwd", "absolute"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, exitCode := h.RunCommand("remember", "file", "--dir", tt.dirValue, srcFile)

			if exitCode == 0 {
				t.Error("Expected command to fail for invalid --dir value")
			}

			output := stdout + stderr
			harness.AssertContains(t, output, "invalid")
			// Should show usage since this is an input validation error
			harness.AssertContains(t, output, "Usage:")
		})
	}
}

// TestRememberFile_WithForceFlag tests overwriting existing files
func TestRememberFile_WithForceFlag(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create an existing file in memory directory
	existingFile := filepath.Join(h.MemoryRoot, "existing.md")
	if err := os.WriteFile(existingFile, []byte("Original content"), 0644); err != nil {
		t.Fatalf("Failed to write existing file: %v", err)
	}

	// Create a source file with same name but different content
	srcFile := filepath.Join(h.AppDir, "source", "existing.md")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	if err := os.WriteFile(srcFile, []byte("New content"), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Remember with --force flag
	stdout, stderr, exitCode := h.RunCommand("remember", "file", "--force", srcFile)

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "copied to")

	// Verify content was overwritten
	content, err := os.ReadFile(existingFile)
	if err != nil {
		t.Fatalf("Failed to read existing file: %v", err)
	}
	if string(content) != "New content" {
		t.Errorf("File content was not overwritten; got %q, want %q", string(content), "New content")
	}
}

// TestRememberFile_WithDryRunFlag tests dry-run mode
func TestRememberFile_WithDryRunFlag(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create a source file
	srcFile := filepath.Join(h.AppDir, "source", "dryrun.md")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	if err := os.WriteFile(srcFile, []byte("# Dry Run Test"), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Remember with --dry-run flag
	stdout, stderr, exitCode := h.RunCommand("remember", "file", "--dry-run", srcFile)

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "would copy to")

	// Verify file was NOT copied
	expectedPath := filepath.Join(h.MemoryRoot, "dryrun.md")
	if _, err := os.Stat(expectedPath); !os.IsNotExist(err) {
		t.Error("File should not exist in memory directory during dry-run")
	}
}

// TestRememberFile_ConflictResolution tests automatic renaming on conflicts
func TestRememberFile_ConflictResolution(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create existing files in memory directory
	for i := 0; i <= 2; i++ {
		var name string
		if i == 0 {
			name = "conflict.md"
		} else {
			name = "conflict-" + string('0'+byte(i)) + ".md"
		}
		path := filepath.Join(h.MemoryRoot, name)
		if err := os.WriteFile(path, []byte("Existing"), 0644); err != nil {
			t.Fatalf("Failed to write existing file %s: %v", name, err)
		}
	}

	// Create a source file with conflicting name
	srcFile := filepath.Join(h.AppDir, "source", "conflict.md")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	if err := os.WriteFile(srcFile, []byte("New content"), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Remember without --force (should auto-rename)
	stdout, stderr, exitCode := h.RunCommand("remember", "file", srcFile)

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "renamed due to conflict")

	// Verify file was renamed (should be conflict-3.md since 0-2 exist)
	expectedPath := filepath.Join(h.MemoryRoot, "conflict-3.md")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("Renamed file not found: %s", expectedPath)
	}

	// Verify content
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("Failed to read renamed file: %v", err)
	}
	if string(content) != "New content" {
		t.Errorf("File content mismatch; got %q, want %q", string(content), "New content")
	}
}

// TestRememberFile_BatchOperationWithErrors tests batch operations with some failures
func TestRememberFile_BatchOperationWithErrors(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create valid source files
	srcDir := filepath.Join(h.AppDir, "source")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	validFile := filepath.Join(srcDir, "valid.md")
	if err := os.WriteFile(validFile, []byte("Valid content"), 0644); err != nil {
		t.Fatalf("Failed to write valid file: %v", err)
	}

	// Non-existent file
	nonExistent := filepath.Join(srcDir, "does-not-exist.md")

	// Run batch operation
	stdout, stderr, exitCode := h.RunCommand("remember", "file", validFile, nonExistent)

	// Should fail because one file doesn't exist
	if exitCode == 0 {
		t.Error("Expected command to fail when one file doesn't exist")
	}

	output := stdout + stderr
	harness.AssertContains(t, output, "does not exist")
	// Should show usage since this is an input validation error
	harness.AssertContains(t, output, "Usage:")
}

// TestRememberFile_AlreadyInMemory tests error when file is already in memory directory
func TestRememberFile_AlreadyInMemory(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create a file already in memory directory
	existingFile := filepath.Join(h.MemoryRoot, "already-here.md")
	if err := os.WriteFile(existingFile, []byte("Already in memory"), 0644); err != nil {
		t.Fatalf("Failed to write existing file: %v", err)
	}

	// Try to remember the file that's already in memory
	stdout, stderr, exitCode := h.RunCommand("remember", "file", existingFile)

	// Should fail
	if exitCode == 0 {
		t.Error("Expected command to fail for file already in memory")
	}

	output := stdout + stderr
	harness.AssertContains(t, output, "already in memory")
}

// TestRememberFile_NonExistentPath tests validation for non-existent paths
func TestRememberFile_NonExistentPath(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Try to remember non-existent file
	stdout, stderr, exitCode := h.RunCommand("remember", "file", "/path/that/does/not/exist.md")

	if exitCode == 0 {
		t.Error("Expected command to fail for non-existent path")
	}

	output := stdout + stderr
	harness.AssertContains(t, output, "does not exist")
	// Should show usage since this is an input validation error
	harness.AssertContains(t, output, "Usage:")
}

// TestRememberFile_LargeBatchWithoutForce tests warning for large batches
func TestRememberFile_LargeBatchWithoutForce(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create a directory with more than 100 files
	srcDir := filepath.Join(h.AppDir, "source", "large-batch")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	for i := 0; i < 105; i++ {
		path := filepath.Join(srcDir, "file-"+string('0'+byte(i/100))+string('0'+byte((i/10)%10))+string('0'+byte(i%10))+".txt")
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to write file %d: %v", i, err)
		}
	}

	// Try to remember without --force
	stdout, stderr, exitCode := h.RunCommand("remember", "file", srcDir)

	if exitCode == 0 {
		t.Error("Expected command to fail for large batch without --force")
	}

	output := stdout + stderr
	harness.AssertContains(t, output, "large batch")
	harness.AssertContains(t, output, "--force")
}

// TestRememberFile_LargeBatchWithForce tests that --force allows large batches
func TestRememberFile_LargeBatchWithForce(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create a directory with more than 100 files
	srcDir := filepath.Join(h.AppDir, "source", "large-batch")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	for i := 0; i < 105; i++ {
		path := filepath.Join(srcDir, "file-"+string('0'+byte(i/100))+string('0'+byte((i/10)%10))+string('0'+byte(i%10))+".txt")
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to write file %d: %v", i, err)
		}
	}

	// Remember with --force
	stdout, stderr, exitCode := h.RunCommand("remember", "file", "--force", srcDir)

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "copied")
}

// TestRememberFile_MultipleFiles tests remembering multiple files in one command
func TestRememberFile_MultipleFiles(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create source files
	srcDir := filepath.Join(h.AppDir, "source")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}

	files := []string{"file1.md", "file2.md", "file3.md"}
	var srcPaths []string
	for _, name := range files {
		path := filepath.Join(srcDir, name)
		if err := os.WriteFile(path, []byte("Content of "+name), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", name, err)
		}
		srcPaths = append(srcPaths, path)
	}

	// Remember multiple files
	args := append([]string{"remember", "file"}, srcPaths...)
	stdout, stderr, exitCode := h.RunCommand(args...)

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "3 succeeded")

	// Verify all files exist in memory directory
	for _, name := range files {
		expectedPath := filepath.Join(h.MemoryRoot, name)
		if _, err := os.Stat(expectedPath); err != nil {
			t.Errorf("File not found in memory directory: %s", expectedPath)
		}
	}
}

// TestRememberFile_UnsupportedFileTypeWarning tests warning for unsupported file types
func TestRememberFile_UnsupportedFileTypeWarning(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Create a file with unsupported extension
	srcFile := filepath.Join(h.AppDir, "source", "data.xyz")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	if err := os.WriteFile(srcFile, []byte("binary data"), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Remember the file
	stdout, stderr, exitCode := h.RunCommand("remember", "file", srcFile)

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	// Should show warning about unsupported file type
	harness.AssertContains(t, stdout, ".xyz")
	harness.AssertContains(t, stdout, "may not be fully analyzed")
	// But should still copy
	harness.AssertContains(t, stdout, "copied to")
}

// TestRememberFile_CommandHelp tests help output for remember file command
func TestRememberFile_CommandHelp(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	stdout, stderr, exitCode := h.RunCommand("remember", "file", "--help")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "Usage:")
	harness.AssertContains(t, stdout, "--dir")
	harness.AssertContains(t, stdout, "--force")
	harness.AssertContains(t, stdout, "--dry-run")
}

// TestRememberFile_NoArgs tests error when no arguments provided
func TestRememberFile_NoArgs(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	stdout, stderr, exitCode := h.RunCommand("remember", "file")

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
