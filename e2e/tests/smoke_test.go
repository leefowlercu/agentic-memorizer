// go:build e2e

package tests

import (
	"testing"

	"github.com/leefowlercu/agentic-memorizer/e2e/harness"
)

// TestSmokeBasicHarness is a basic smoke test to verify the harness works
func TestSmokeBasicHarness(t *testing.T) {
	h := harness.New(t)

	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Verify basic paths exist
	harness.AssertNotEmpty(t, h.AppDir, "AppDir")
	harness.AssertNotEmpty(t, h.MemoryRoot, "MemoryRoot")
	harness.AssertNotEmpty(t, h.BinaryPath, "BinaryPath")

	t.Log("✅ Smoke test: harness initialization successful")
}

// TestSmokeVersionCommand is a smoke test for the version command
func TestSmokeVersionCommand(t *testing.T) {
	h := harness.New(t)

	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Run version command
	stdout, stderr, exitCode := h.RunCommand("version")

	// Should succeed
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Should contain version info
	harness.AssertContains(t, stdout, "Version:")
	harness.AssertContains(t, stdout, "Commit:")

	t.Log("✅ Smoke test: version command successful")
}

// TestSmokeHelpCommand is a smoke test for the help command
func TestSmokeHelpCommand(t *testing.T) {
	h := harness.New(t)

	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Run help command
	stdout, stderr, exitCode := h.RunCommand("--help")

	// Should succeed
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Should contain usage info
	harness.AssertContains(t, stdout, "Usage:")
	harness.AssertContains(t, stdout, "Available Commands:")

	t.Log("✅ Smoke test: help command successful")
}
