//go:build e2e

package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/e2e/harness"
)

// TestWalker_InitialScan tests walker initialization during daemon rebuild
func TestWalker_InitialScan(t *testing.T) {
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

	// Create multiple files for walker to discover
	testFiles := []string{
		"file1.md",
		"file2.txt",
		"subdir/file3.md",
		"subdir/nested/file4.json",
		"another/file5.go",
	}

	for _, f := range testFiles {
		if err := h.AddMemoryFile(f, "# Test Content\n\nContent for "+f); err != nil {
			t.Fatalf("Failed to add file %s: %v", f, err)
		}
	}

	// Start daemon (will trigger initial walk) - let cleanup handle stopping
	cmd := exec.Command(h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for initial walk and processing
	time.Sleep(10 * time.Second)

	// Verify all files were discovered via HTTP API
	for _, f := range testFiles {
		metadata, err := h.HTTPClient.GetFileMetadata(filepath.Base(f))
		if err != nil {
			t.Errorf("File %s not indexed after initial walk: %v", f, err)
		} else {
			t.Logf("File %s discovered and indexed", f)
			_ = metadata
		}
	}
}

// TestWalker_SkipDotDirectories tests that walker skips .cache, .git, etc.
func TestWalker_SkipDotDirectories(t *testing.T) {
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

	// Create files in both visible and hidden directories
	testFiles := map[string]bool{
		"visible.md":        true,  // Should be indexed
		".hidden.md":        false, // Hidden file - should be skipped
		".git/config":       false, // .git directory - should be skipped
		".cache/cache.json": false, // .cache directory - should be skipped
		"subdir/normal.txt": true,  // Should be indexed
		"subdir/.DS_Store":  false, // Hidden file in subdir - should be skipped
	}

	for f := range testFiles {
		if err := h.AddMemoryFile(f, "Content for "+f); err != nil {
			t.Fatalf("Failed to add file %s: %v", f, err)
		}
	}

	// Start daemon - let cleanup handle stopping
	cmd := exec.Command(h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for processing
	time.Sleep(10 * time.Second)

	// Verify files were indexed or skipped as expected
	for f, shouldExist := range testFiles {
		basename := filepath.Base(f)
		_, err := h.HTTPClient.GetFileMetadata(basename)

		if shouldExist {
			if err != nil {
				t.Errorf("File %s should have been indexed but wasn't: %v", f, err)
			} else {
				t.Logf("File %s correctly indexed", f)
			}
		} else {
			if err == nil {
				t.Errorf("File %s should have been skipped but was indexed", f)
			} else {
				t.Logf("File %s correctly skipped", f)
			}
		}
	}
}

// TestWalker_ConfiguredSkipPatterns tests walker respecting configured skip patterns
func TestWalker_ConfiguredSkipPatterns(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server for metadata queries
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Modify config to skip specific extensions and files
	graphHost := os.Getenv("FALKORDB_HOST")
	if graphHost == "" {
		graphHost = "localhost"
	}
	graphPort := os.Getenv("FALKORDB_PORT")
	if graphPort == "" {
		graphPort = "6379"
	}

	configContent := `memory_root: ` + h.MemoryRoot + `

analysis:
  max_file_size: 10485760
  skip_extensions: [".log", ".tmp"]
  skip_files: ["SKIP_ME.txt"]
  cache_dir: ` + filepath.Join(h.MemoryRoot, ".cache") + `

daemon:
  workers: 2
  rate_limit_per_min: 20
  debounce_ms: 200
  full_rebuild_interval_minutes: 60
  http_port: 8080
  log_file: ` + h.LogPath + `
  log_level: info

graph:
  host: ` + graphHost + `
  port: ` + graphPort + `
  database: ` + h.GraphName + `

mcp:
  log_file: ` + filepath.Join(h.AppDir, "mcp.log") + `
  log_level: info
  daemon_host: localhost
  daemon_port: 8080
`

	if err := os.WriteFile(h.ConfigPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Create files with various extensions
	testFiles := map[string]bool{
		"normal.md":   true,  // Should be indexed
		"data.json":   true,  // Should be indexed
		"debug.log":   false, // .log extension - should be skipped
		"temp.tmp":    false, // .tmp extension - should be skipped
		"SKIP_ME.txt": false, // Explicitly skipped file
		"allowed.txt": true,  // Should be indexed
	}

	for f := range testFiles {
		if err := h.AddMemoryFile(f, "Content for "+f); err != nil {
			t.Fatalf("Failed to add file %s: %v", f, err)
		}
	}

	// Start daemon - let cleanup handle stopping
	cmd := exec.Command(h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for processing
	time.Sleep(10 * time.Second)

	// Verify skip patterns were applied
	for f, shouldExist := range testFiles {
		_, err := h.HTTPClient.GetFileMetadata(f)

		if shouldExist {
			if err != nil {
				t.Errorf("File %s should have been indexed but wasn't: %v", f, err)
			} else {
				t.Logf("File %s correctly indexed", f)
			}
		} else {
			if err == nil {
				t.Errorf("File %s should have been skipped but was indexed", f)
			} else {
				t.Logf("File %s correctly skipped due to skip pattern", f)
			}
		}
	}
}

// TestWalker_DeepNestedDirectories tests walker handling deep directory structures
func TestWalker_DeepNestedDirectories(t *testing.T) {
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

	// Create deeply nested directory structure
	deepPath := "level1/level2/level3/level4/level5/level6/level7/level8/deep.md"
	if err := h.AddMemoryFile(deepPath, "# Deep File\n\nContent at depth 8"); err != nil {
		t.Fatalf("Failed to add deep file: %v", err)
	}

	// Also create files at various depths
	testFiles := []string{
		"root.md",
		"level1/l1.md",
		"level1/level2/l2.md",
		"level1/level2/level3/l3.md",
		deepPath,
	}

	for _, f := range testFiles {
		if f == deepPath {
			continue // Already created
		}
		if err := h.AddMemoryFile(f, "Content for "+f); err != nil {
			t.Fatalf("Failed to add file %s: %v", f, err)
		}
	}

	// Start daemon - let cleanup handle stopping
	cmd := exec.Command(h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for processing
	time.Sleep(10 * time.Second)

	// Verify all files at all depths were discovered
	for _, f := range testFiles {
		basename := filepath.Base(f)
		_, err := h.HTTPClient.GetFileMetadata(basename)
		if err != nil {
			t.Errorf("File %s at depth not indexed: %v", f, err)
		} else {
			t.Logf("File %s at depth correctly indexed", f)
		}
	}
}

// TestWalker_SymbolicLinks tests walker handling of symbolic links
func TestWalker_SymbolicLinks(t *testing.T) {
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
	realFile := "real.md"
	if err := h.AddMemoryFile(realFile, "# Real File\n\nThis is the real file"); err != nil {
		t.Fatalf("Failed to add real file: %v", err)
	}

	// Create directory outside memory root
	externalDir := t.TempDir()
	externalFile := filepath.Join(externalDir, "external.md")
	if err := os.WriteFile(externalFile, []byte("# External\n\nExternal content"), 0644); err != nil {
		t.Fatalf("Failed to create external file: %v", err)
	}

	// Create symlink to external file inside memory root
	symlinkPath := filepath.Join(h.MemoryRoot, "symlink.md")
	if err := os.Symlink(externalFile, symlinkPath); err != nil {
		t.Skipf("Failed to create symlink (may not be supported): %v", err)
	}

	// Start daemon - let cleanup handle stopping
	cmd := exec.Command(h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for processing
	time.Sleep(10 * time.Second)

	// Verify real file is indexed
	_, err := h.HTTPClient.GetFileMetadata("real.md")
	if err != nil {
		t.Errorf("Real file should be indexed: %v", err)
	} else {
		t.Log("Real file correctly indexed")
	}

	// Verify symlink handling (behavior depends on implementation)
	// The walker follows symlinks by default via filepath.Walk
	_, err = h.HTTPClient.GetFileMetadata("symlink.md")
	if err != nil {
		t.Logf("Symlink not indexed (expected behavior if symlinks are not followed): %v", err)
	} else {
		t.Log("Symlink indexed (walker follows symlinks)")
	}
}

// TestWalker_LargeFileCount tests walker performance with many files
func TestWalker_LargeFileCount(t *testing.T) {
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

	// Create 50 files (reasonable for E2E test, larger for stress test)
	fileCount := 50
	t.Logf("Creating %d files for walker to discover", fileCount)

	for i := 0; i < fileCount; i++ {
		filename := filepath.Join("batch", "file"+string(rune('A'+i%26))+".md")
		content := "# File " + string(rune('A'+i%26)) + "\n\nContent for file " + string(rune('A'+i%26))
		if err := h.AddMemoryFile(filename, content); err != nil {
			t.Fatalf("Failed to add file %d: %v", i, err)
		}
	}

	// Start daemon - let cleanup handle stopping
	startTime := time.Now()

	cmd := exec.Command(h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for processing (may take longer with many files)
	time.Sleep(15 * time.Second)

	walkDuration := time.Since(startTime)
	t.Logf("Walker completed initial scan in %v", walkDuration)

	// Sample check - verify some files were indexed
	sampleFiles := []string{"fileA.md", "fileM.md", "fileZ.md"}
	indexedCount := 0

	for _, f := range sampleFiles {
		_, err := h.HTTPClient.GetFileMetadata(f)
		if err == nil {
			indexedCount++
		}
	}

	if indexedCount == 0 {
		t.Error("No files were indexed from large batch")
	} else {
		t.Logf("Successfully indexed at least %d sample files from batch of %d", indexedCount, fileCount)
	}

	// Performance check - walker should complete within reasonable time
	if walkDuration > 60*time.Second {
		t.Logf("Warning: Walker took %v to process %d files (>60s)", walkDuration, fileCount)
	}
}
