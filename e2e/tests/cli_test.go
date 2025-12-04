//go:build e2e

package tests

import (
	"strings"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/e2e/harness"
)

// TestCLI_Version tests the version command
func TestCLI_Version(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	stdout, stderr, exitCode := h.RunCommand("version")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "Version:")
	harness.AssertContains(t, stdout, "Commit:")
	harness.AssertContains(t, stdout, "Built:")
	harness.AssertEmpty(t, stderr, "stderr")
}

// TestCLI_Help tests the --help flag
func TestCLI_Help(t *testing.T) {
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
		{"root help", []string{"--help"}},
		{"help flag", []string{"-h"}},
		{"daemon help", []string{"daemon", "--help"}},
		{"graph help", []string{"graph", "--help"}},
		{"integrations help", []string{"integrations", "--help"}},
		{"config help", []string{"config", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, exitCode := h.RunCommand(tt.args...)

			harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
			harness.AssertContains(t, stdout, "Usage:")

			// Help should go to stdout, not stderr
			if stderr != "" && !strings.Contains(stderr, "level=") {
				t.Errorf("Unexpected stderr output: %s", stderr)
			}
		})
	}
}

// TestCLI_InvalidCommand tests invalid command handling
func TestCLI_InvalidCommand(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	stdout, stderr, exitCode := h.RunCommand("nonexistent")

	// Should fail with non-zero exit code
	if exitCode == 0 {
		t.Errorf("Expected non-zero exit code for invalid command, got 0")
	}

	// Should show error message
	output := stdout + stderr
	harness.AssertContains(t, output, "Error:")
	harness.AssertContains(t, output, "unknown command")
}

// TestCLI_DaemonStatus tests daemon status when not running
func TestCLI_DaemonStatus(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	stdout, stderr, _ := h.RunCommand("daemon", "status")

	// Status when daemon is not running should show that
	output := stdout + stderr
	outputLower := strings.ToLower(output)
	if !strings.Contains(outputLower, "not running") && !strings.Contains(outputLower, "stopped") {
		t.Errorf("Expected status to indicate daemon is not running, got: %s", output)
	}
}

// TestCLI_ConfigValidate tests config validation command
func TestCLI_ConfigValidate(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	stdout, stderr, exitCode := h.RunCommand("config", "validate")

	// Should succeed with valid config created by harness
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	output := stdout + stderr
	if !strings.Contains(output, "valid") && !strings.Contains(output, "✅") {
		t.Logf("Config validation output: %s", output)
	}
}

// TestCLI_GraphStatus tests graph status command
func TestCLI_GraphStatus(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	stdout, stderr, exitCode := h.RunCommand("graph", "status")

	// Graph status should work (may show not running or running depending on test env)
	// Just verify command executes
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Unexpected exit code %d for graph status. Stdout: %s, Stderr: %s",
			exitCode, stdout, stderr)
	}
}

// TestCLI_IntegrationsList tests integrations list command
func TestCLI_IntegrationsList(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	stdout, stderr, exitCode := h.RunCommand("integrations", "list")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Should list known integrations
	harness.AssertContains(t, stdout, "claude-code")
	harness.AssertContains(t, stdout, "gemini")
}

// TestCLI_IntegrationsDetect tests integrations detect command
func TestCLI_IntegrationsDetect(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	stdout, stderr, exitCode := h.RunCommand("integrations", "detect")

	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)

	// Detect should run successfully even if no integrations found
	harness.AssertNotEmpty(t, stdout, "stdout")
}

// TestCLI_Read tests the read command
func TestCLI_Read(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Add a test file to memory
	if err := h.AddMemoryFile("test.md", "# Test File\n\nThis is a test."); err != nil {
		t.Fatalf("Failed to add memory file: %v", err)
	}

	// Note: read command requires daemon to have processed files
	// For now, just verify command syntax works
	stdout, stderr, _ := h.RunCommand("read")

	// May fail if no index exists yet, but should not have syntax errors
	output := stdout + stderr
	if strings.Contains(output, "unknown flag") || strings.Contains(output, "unknown command") {
		t.Errorf("Unexpected command error: %s", output)
	}
}

// TestCLI_DaemonStartStop tests daemon start and stop commands
func TestCLI_DaemonStartStop(t *testing.T) {
	t.Skip("Daemon start runs in foreground and blocks - requires background process management")

	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Start daemon
	stdout, stderr, exitCode := h.RunCommand("daemon", "start")
	if exitCode != 0 {
		t.Logf("Daemon start output: stdout=%s, stderr=%s", stdout, stderr)
		// May fail due to missing dependencies, but verify error message is clear
		output := stdout + stderr
		if output == "" {
			t.Error("Expected error message when daemon start fails")
		}
		t.Skip("Daemon start failed (expected in minimal test environment)")
	}

	// If start succeeded, verify status shows running
	stdout, stderr, exitCode = h.RunCommand("daemon", "status")
	if exitCode == 0 {
		output := stdout + stderr
		outputLower := strings.ToLower(output)
		if !strings.Contains(outputLower, "running") && !strings.Contains(outputLower, "active") {
			t.Errorf("Expected status to show running, got: %s", output)
		}
	}

	// Stop daemon
	stdout, stderr, exitCode = h.RunCommand("daemon", "stop")
	harness.LogOutput(t, stdout, stderr)

	// Verify status shows stopped
	stdout, stderr, exitCode = h.RunCommand("daemon", "status")
	output := stdout + stderr
	outputLower := strings.ToLower(output)
	if !strings.Contains(outputLower, "not running") && !strings.Contains(outputLower, "stopped") {
		t.Logf("Status after stop: %s", output)
	}
}

// TestCLI_IntegrationsSetupInvalidIntegration tests setup with invalid integration
func TestCLI_IntegrationsSetupInvalidIntegration(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	stdout, stderr, exitCode := h.RunCommand("integrations", "setup", "nonexistent-integration")

	// Should fail with non-zero exit code
	if exitCode == 0 {
		t.Error("Expected setup to fail for nonexistent integration")
	}

	// Should show error and usage
	output := stdout + stderr
	harness.AssertContains(t, output, "Error:")
	harness.AssertContains(t, output, "Usage:")
}

// TestCLI_IntegrationsRemoveNonexistent tests remove on non-configured integration
func TestCLI_IntegrationsRemoveNonexistent(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Try to remove an integration that doesn't exist
	stdout, stderr, exitCode := h.RunCommand("integrations", "remove", "nonexistent-integration")

	// Should fail
	if exitCode == 0 {
		t.Error("Expected remove to fail for nonexistent integration")
	}

	output := stdout + stderr
	harness.AssertContains(t, output, "Error:")
}

// TestCLI_ConfigReloadWithoutDaemon tests config reload when daemon not running
func TestCLI_ConfigReloadWithoutDaemon(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	stdout, stderr, exitCode := h.RunCommand("config", "reload")

	// Command should execute (behavior may vary - might validate config or check daemon status)
	// Just verify it doesn't crash with unexpected errors
	output := stdout + stderr
	if strings.Contains(output, "unknown flag") || strings.Contains(output, "unknown command") {
		t.Errorf("Unexpected command error: %s", output)
	}

	t.Logf("Config reload when daemon not running (exit=%d): %s", exitCode, output)
}

// TestCLI_DaemonLogs tests daemon logs command
func TestCLI_DaemonLogs(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Logs command should work even if no logs exist yet
	_, _, exitCode := h.RunCommand("daemon", "logs")

	// Command should execute (may show "no logs" or actual log content)
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Unexpected exit code %d for daemon logs", exitCode)
	}
}

// TestCLI_Initialize tests initialize command
func TestCLI_Initialize(t *testing.T) {
	t.Skip("Initialize requires interactive input - tested manually")

	// This test is skipped because initialize has interactive prompts
	// It would need special handling to provide input
}

// TestCLI_MCPStart tests mcp start command
func TestCLI_MCPStart(t *testing.T) {
	t.Skip("MCP start is a long-running process - tested in mcp_test.go")

	// This test is skipped because mcp start runs indefinitely
	// It's tested properly in the MCP-specific test file
}

// TestCLI_DaemonSystemctl tests daemon systemctl command
func TestCLI_DaemonSystemctl(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	stdout, stderr, exitCode := h.RunCommand("daemon", "systemctl")

	// Should generate systemd unit file
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "[Unit]")
	harness.AssertContains(t, stdout, "[Service]")
	harness.AssertContains(t, stdout, "Type=notify")
}

// TestCLI_DaemonLaunchctl tests daemon launchctl command
func TestCLI_DaemonLaunchctl(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	stdout, stderr, exitCode := h.RunCommand("daemon", "launchctl")

	// Should generate launchd plist file
	harness.AssertExitCode(t, 0, exitCode, stdout, stderr)
	harness.AssertContains(t, stdout, "<?xml")
	harness.AssertContains(t, stdout, "plist")
	harness.AssertContains(t, stdout, "Label")
}
