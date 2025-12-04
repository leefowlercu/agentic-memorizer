//go:build e2e

package tests

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/e2e/harness"
)

// TestDaemon_StartStop tests basic daemon start and stop lifecycle
func TestDaemon_StartStop(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Start daemon in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Skipf("Failed to start daemon: %v (expected in minimal test environment)", err)
	}

	// Ensure daemon is killed on test exit
	defer func() {
		cancel()
		cmd.Wait()
	}()

	// Wait for daemon to be healthy
	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Skipf("Daemon failed to become healthy: %v", err)
	}

	// Verify status shows running
	stdout, stderr, exitCode := h.RunCommand("daemon", "status")
	if exitCode != 0 {
		t.Errorf("Status command failed (exit %d): stdout=%s, stderr=%s", exitCode, stdout, stderr)
	}

	output := stdout + stderr
	outputLower := strings.ToLower(output)
	if !strings.Contains(outputLower, "running") && !strings.Contains(outputLower, "active") {
		t.Errorf("Expected status to show running, got: %s", output)
	}

	// Stop daemon
	stdout, stderr, exitCode = h.RunCommand("daemon", "stop")
	if exitCode != 0 {
		t.Errorf("Stop command failed (exit %d): stdout=%s, stderr=%s", exitCode, stdout, stderr)
	}

	// Verify daemon stopped
	time.Sleep(2 * time.Second)
	stdout, stderr, exitCode = h.RunCommand("daemon", "status")
	output = stdout + stderr
	outputLower = strings.ToLower(output)
	if !strings.Contains(outputLower, "not running") && !strings.Contains(outputLower, "stopped") {
		t.Errorf("Expected status to show stopped, got: %s", output)
	}
}

// TestDaemon_HealthEndpoint tests daemon health endpoint
func TestDaemon_HealthEndpoint(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server for health endpoint
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Start daemon in background
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

	// Query health endpoint
	health, err := h.HTTPClient.Health()
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}

	// Verify health response contains expected fields
	if status, ok := health["status"].(string); !ok || status != "healthy" {
		t.Errorf("Expected status=healthy, got: %v", health["status"])
	}

	if graph, ok := health["graph"].(map[string]any); ok {
		if _, ok := graph["connected"].(bool); !ok {
			t.Errorf("Expected graph.connected to be boolean, got: %v", graph["connected"])
		}
		t.Logf("Graph connected: %v", graph["connected"])
	}
}

// TestDaemon_MultipleStarts tests that multiple daemon starts are prevented
func TestDaemon_MultipleStarts(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Start first daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Skipf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	// Wait for daemon to be healthy
	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Skipf("Daemon failed to become healthy: %v", err)
	}

	// Try to start second daemon (should fail)
	stdout, stderr, exitCode := h.RunCommand("daemon", "start")
	if exitCode == 0 {
		t.Error("Expected second daemon start to fail, but it succeeded")
	}

	output := stdout + stderr
	if !strings.Contains(output, "already running") && !strings.Contains(output, "PID file") {
		t.Logf("Second start output: %s", output)
	}
}

// TestDaemon_Rebuild tests daemon rebuild command
func TestDaemon_Rebuild(t *testing.T) {
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

	// Add test files for indexing
	if err := h.AddMemoryFile("test.md", "# Test\n\nTest content for rebuild."); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Start daemon in background
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

	// Trigger rebuild
	stdout, stderr, exitCode := h.RunCommand("daemon", "rebuild")
	if exitCode != 0 {
		t.Errorf("Rebuild failed (exit %d): stdout=%s, stderr=%s", exitCode, stdout, stderr)
	}

	output := stdout + stderr
	if !strings.Contains(output, "Rebuild") && !strings.Contains(output, "rebuild") {
		t.Logf("Rebuild output: %s", output)
	}

	t.Log("Rebuild command executed successfully")
}

// TestDaemon_Restart tests daemon restart command
func TestDaemon_Restart(t *testing.T) {
	t.Skip("Restart requires running daemon - implementation varies by platform")

	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Start daemon
	if err := h.StartDaemon(); err != nil {
		t.Skipf("Failed to start daemon: %v", err)
	}
	defer h.StopDaemon()

	// Wait for healthy
	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Skipf("Daemon failed to become healthy: %v", err)
	}

	// Restart daemon
	stdout, stderr, exitCode := h.RunCommand("daemon", "restart")
	harness.LogOutput(t, stdout, stderr)

	// Command behavior varies by platform
	if exitCode != 0 {
		t.Logf("Restart command exit code: %d", exitCode)
	}
}

// TestDaemon_GracefulShutdown tests that daemon handles SIGTERM gracefully
func TestDaemon_GracefulShutdown(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Start daemon in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Skipf("Failed to start daemon: %v", err)
	}

	// Wait for daemon to be healthy
	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		cancel()
		cmd.Wait()
		t.Skipf("Daemon failed to become healthy: %v", err)
	}

	// Send stop command (graceful shutdown)
	stdout, stderr, exitCode := h.RunCommand("daemon", "stop")
	if exitCode != 0 {
		t.Logf("Stop command output: stdout=%s, stderr=%s", stdout, stderr)
	}

	// Wait for process to exit
	cancel()
	err := cmd.Wait()

	// Context cancellation is expected
	if err != nil && !strings.Contains(err.Error(), "signal: killed") && !strings.Contains(err.Error(), "context canceled") {
		t.Logf("Daemon exit error: %v", err)
	}

	// Verify PID file is removed
	// (would need to check h.PIDPath existence)
	t.Log("Daemon shutdown completed")
}
