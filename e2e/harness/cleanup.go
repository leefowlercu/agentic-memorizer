package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Cleanup handles cleanup operations for E2E tests
type Cleanup struct {
	t        testing.TB
	harness  *E2EHarness
	cleanups []func() error
}

// NewCleanup creates a new cleanup manager
func NewCleanup(t testing.TB, h *E2EHarness) *Cleanup {
	return &Cleanup{
		t:       t,
		harness: h,
	}
}

// Register adds a cleanup function to be called on teardown
func (c *Cleanup) Register(fn func() error) {
	c.cleanups = append(c.cleanups, fn)
}

// Run executes all registered cleanup functions
func (c *Cleanup) Run() {
	for i := len(c.cleanups) - 1; i >= 0; i-- {
		if err := c.cleanups[i](); err != nil {
			c.t.Logf("Cleanup error: %v", err)
		}
	}
}

// StopDaemon ensures the daemon is stopped
func (c *Cleanup) StopDaemon() error {
	return c.harness.StopDaemon()
}

// ClearGraph clears all data from the test graph
func (c *Cleanup) ClearGraph() error {
	if c.harness.GraphClient == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return c.harness.GraphClient.Clear(ctx)
}

// RemoveTestFiles removes all test files from memory directory
func (c *Cleanup) RemoveTestFiles() error {
	if c.harness.MemoryRoot == "" {
		return nil
	}

	// Remove all files except .cache directory
	entries, err := os.ReadDir(c.harness.MemoryRoot)
	if err != nil {
		return fmt.Errorf("failed to read memory directory; %w", err)
	}

	for _, entry := range entries {
		if entry.Name() == ".cache" {
			continue
		}

		path := filepath.Join(c.harness.MemoryRoot, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("failed to remove %s; %w", path, err)
		}
	}

	return nil
}

// WaitForDaemonStop waits for the daemon to stop gracefully
func (c *Cleanup) WaitForDaemonStop(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if _, err := os.Stat(c.harness.PIDPath); os.IsNotExist(err) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("daemon did not stop within %v", timeout)
}

// KillDaemonProcess forcefully kills the daemon process (last resort)
func (c *Cleanup) KillDaemonProcess() error {
	// Read PID file
	pidData, err := os.ReadFile(c.harness.PIDPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No PID file, daemon not running
		}
		return fmt.Errorf("failed to read PID file; %w", err)
	}

	var pid int
	if _, err := fmt.Sscanf(string(pidData), "%d", &pid); err != nil {
		return fmt.Errorf("failed to parse PID; %w", err)
	}

	// Find and kill process
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process; %w", err)
	}

	if err := process.Kill(); err != nil {
		return fmt.Errorf("failed to kill process; %w", err)
	}

	return nil
}

// CleanupAll performs all cleanup operations
func (c *Cleanup) CleanupAll() {
	// Stop daemon gracefully
	if err := c.StopDaemon(); err != nil {
		c.t.Logf("Failed to stop daemon gracefully: %v", err)

		// Force kill if graceful stop failed
		if err := c.KillDaemonProcess(); err != nil {
			c.t.Logf("Failed to force kill daemon: %v", err)
		}
	}

	// Wait for daemon to stop
	if err := c.WaitForDaemonStop(5 * time.Second); err != nil {
		c.t.Logf("Daemon did not stop in time: %v", err)
	}

	// Clear graph data
	if err := c.ClearGraph(); err != nil {
		c.t.Logf("Failed to clear graph: %v", err)
	}

	// Remove test files
	if err := c.RemoveTestFiles(); err != nil {
		c.t.Logf("Failed to remove test files: %v", err)
	}

	// Run custom cleanup functions
	c.Run()
}

// MustCleanup creates a cleanup manager and registers it with t.Cleanup()
func MustCleanup(t testing.TB, h *E2EHarness) *Cleanup {
	t.Helper()

	cleanup := NewCleanup(t, h)
	t.Cleanup(cleanup.CleanupAll)

	return cleanup
}
