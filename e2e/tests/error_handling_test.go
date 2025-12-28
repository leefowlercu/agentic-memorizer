//go:build e2e

package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/e2e/harness"
)

// TestErrorHandling_MissingMemoryRoot tests daemon behavior with missing memory directory
func TestErrorHandling_MissingMemoryRoot(t *testing.T) {
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

	// Remove memory directory
	if err := os.RemoveAll(h.MemoryRoot); err != nil {
		t.Fatalf("Failed to remove memory directory: %v", err)
	}

	// Try to start daemon
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Log("Daemon started despite missing memory root (it may create it)")
		t.Logf("Output: %s", string(output))

		// Wait for daemon to potentially create directory
		time.Sleep(2 * time.Second)

		// Check if directory was created
		if _, statErr := os.Stat(h.MemoryRoot); statErr == nil {
			t.Log("Daemon created missing memory directory")
		}

		// Stop daemon
		stopCmd := exec.Command(h.BinaryPath, "daemon", "stop")
		stopCmd.Env = append(stopCmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)
		stopCmd.Run()
	} else {
		t.Logf("Daemon failed to start with missing memory root (expected): %v", err)
		t.Logf("Output: %s", string(output))
	}
}

// TestErrorHandling_CorruptedConfigFile tests daemon behavior with corrupted config
func TestErrorHandling_CorruptedConfigFile(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Corrupt the config file
	if err := os.WriteFile(h.ConfigPath, []byte("invalid: yaml: :::"), 0644); err != nil {
		t.Fatalf("Failed to write corrupted config: %v", err)
	}

	// Try to start daemon
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("Daemon started with corrupted config (should have failed)")

		// Stop daemon
		stopCmd := exec.Command(h.BinaryPath, "daemon", "stop")
		stopCmd.Env = append(stopCmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)
		stopCmd.Run()
	} else {
		t.Logf("Daemon correctly failed to start with corrupted config: %v", err)
		t.Logf("Output: %s", string(output))
	}
}

// TestErrorHandling_GracefulDegradation tests daemon fails without FalkorDB
func TestErrorHandling_GracefulDegradation(t *testing.T) {
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

	// Configure with non-existent graph host
	configContent := `# E2E Test Configuration
memory_root: ` + h.MemoryRoot + `

semantic:
  enabled: false
  provider: claude
  timeout: 30
  max_file_size: 10485760
  skip_extensions: []
  skip_files: []
  cache_dir: ` + filepath.Join(h.MemoryRoot, ".cache") + `
  rate_limit_per_min: 20

daemon:
  workers: 2
  debounce_ms: 200
  full_rebuild_interval_minutes: 60
  http_port: 8080
  log_file: ` + h.LogPath + `
  log_level: info

graph:
  host: nonexistent-host
  port: 9999

mcp:
  log_file: ` + filepath.Join(h.AppDir, "mcp.log") + `
  log_level: info
  daemon_host: localhost
  daemon_port: 8080
`
	if err := os.WriteFile(h.ConfigPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Add test file
	if err := h.AddMemoryFile("test.md", "# Test File\n"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Try to start daemon (should fail without working graph DB)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("Daemon started without FalkorDB (should have failed)")
		t.Logf("Output: %s", string(output))

		// Stop daemon if it somehow started
		stopCmd := exec.Command(h.BinaryPath, "daemon", "stop")
		stopCmd.Env = append(stopCmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)
		stopCmd.Run()
	} else {
		t.Log("Daemon correctly failed to start without FalkorDB")
		t.Logf("Error: %v", err)
		t.Logf("Output: %s", string(output))
	}
}

// TestErrorHandling_NonWritableDirectory tests behavior with permission issues
func TestErrorHandling_NonWritableDirectory(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

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

	// Create a subdirectory and make it read-only
	readOnlyDir := filepath.Join(h.MemoryRoot, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Add a file in the readonly directory before making it readonly
	testFile := filepath.Join(readOnlyDir, "test.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Make directory read-only
	if err := os.Chmod(readOnlyDir, 0555); err != nil {
		t.Fatalf("Failed to change permissions: %v", err)
	}
	defer os.Chmod(readOnlyDir, 0755) // Restore permissions for cleanup

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

	// Wait for processing attempt
	time.Sleep(5 * time.Second)

	// Verify daemon is still healthy despite permission issues
	health, err := h.HTTPClient.Health()
	if err != nil {
		t.Errorf("Health check failed: %v", err)
	} else {
		t.Log("Daemon remains healthy despite readonly directory")
		t.Logf("Health: %+v", health)
	}
}

// TestErrorHandling_DaemonRestart tests daemon restart behavior
func TestErrorHandling_DaemonRestart(t *testing.T) {
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
	if err := h.AddMemoryFile("restart-test.md", "# Restart Test\n"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Start daemon
	ctx1, cancel1 := context.WithCancel(context.Background())

	cmd1 := exec.CommandContext(ctx1, h.BinaryPath, "daemon", "start")
	cmd1.Env = append(cmd1.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd1.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	t.Log("First daemon instance started successfully")

	// Stop first instance
	cancel1()
	cmd1.Wait()

	// Wait for shutdown
	time.Sleep(2 * time.Second)

	// Start second instance
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	cmd2 := exec.CommandContext(ctx2, h.BinaryPath, "daemon", "start")
	cmd2.Env = append(cmd2.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd2.Start(); err != nil {
		t.Fatalf("Failed to restart daemon: %v", err)
	}

	defer func() {
		cancel2()
		cmd2.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy after restart: %v", err)
	}

	t.Log("Daemon successfully restarted")

	// Verify data persisted across restart
	index, err := h.HTTPClient.GetIndex()
	if err != nil {
		t.Fatalf("Failed to get index: %v", err)
	}

	indexMap, ok := index.(map[string]any)
	if !ok {
		t.Fatalf("Unexpected index format: %T", index)
	}

	if files, ok := indexMap["files"].([]any); ok {
		t.Logf("Index contains %d files after restart", len(files))
	} else {
		t.Error("Failed to get files from index")
	}
}

// TestErrorHandling_MultipleDaemonStarts tests prevention of multiple daemon instances
func TestErrorHandling_MultipleDaemonStarts(t *testing.T) {
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

	// Start first daemon
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	cmd1 := exec.CommandContext(ctx1, h.BinaryPath, "daemon", "start")
	cmd1.Env = append(cmd1.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd1.Start(); err != nil {
		t.Fatalf("Failed to start first daemon: %v", err)
	}

	defer func() {
		cancel1()
		cmd1.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("First daemon failed to become healthy: %v", err)
	}

	t.Log("First daemon started successfully")

	// Try to start second daemon (should fail)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()

	cmd2 := exec.CommandContext(ctx2, h.BinaryPath, "daemon", "start")
	cmd2.Env = append(cmd2.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	output, err := cmd2.CombinedOutput()
	if err == nil {
		t.Error("Second daemon started (should have been prevented by PID lock)")
		t.Logf("Output: %s", string(output))
	} else {
		t.Log("Second daemon correctly prevented from starting")
		t.Logf("Error: %v", err)
		t.Logf("Output: %s", string(output))
	}
}
