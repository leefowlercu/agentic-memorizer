package subcommands

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/testutil"
)

// T068: Tests for status command output formats

func TestFormatStatus_Running(t *testing.T) {
	status := &DaemonStatus{
		Running: true,
		PID:     12345,
	}

	output := formatStatus(status)

	if output == "" {
		t.Error("formatStatus() returned empty string")
	}

	// Should contain running indicator and PID
	expectedContains := []string{"running", "12345"}
	for _, want := range expectedContains {
		if !containsString(output, want) {
			t.Errorf("formatStatus() output missing %q", want)
		}
	}
}

func TestFormatStatus_NotRunning(t *testing.T) {
	status := &DaemonStatus{
		Running: false,
	}

	output := formatStatus(status)

	if output == "" {
		t.Error("formatStatus() returned empty string")
	}

	if !containsString(output, "not running") {
		t.Error("formatStatus() should indicate daemon not running")
	}
}

// T069: Tests for status command with running daemon

func TestGetDaemonStatus_Running(t *testing.T) {
	// Initialize config for fetchHealth() to work
	_ = testutil.NewTestEnv(t)

	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "daemon.pid")

	// Write current process PID (simulating running daemon)
	currentPID := os.Getpid()
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(currentPID)), 0o644); err != nil {
		t.Fatalf("Failed to write PID file: %v", err)
	}

	status, err := getDaemonStatus(pidPath)
	if err != nil {
		t.Fatalf("getDaemonStatus() error = %v", err)
	}

	if !status.Running {
		t.Error("getDaemonStatus().Running = false, want true")
	}

	if status.PID != currentPID {
		t.Errorf("getDaemonStatus().PID = %d, want %d", status.PID, currentPID)
	}
}

// T070: Tests for status command with stale PID file

func TestGetDaemonStatus_StalePIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "daemon.pid")

	// Write a stale PID
	stalePID := 99999999
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(stalePID)), 0o644); err != nil {
		t.Fatalf("Failed to write PID file: %v", err)
	}

	status, err := getDaemonStatus(pidPath)
	if err != nil {
		t.Fatalf("getDaemonStatus() error = %v", err)
	}

	if status.Running {
		t.Error("getDaemonStatus().Running = true for stale PID, want false")
	}

	if !status.StalePIDFile {
		t.Error("getDaemonStatus().StalePIDFile = false, want true")
	}
}

func TestGetDaemonStatus_NoPIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "nonexistent.pid")

	status, err := getDaemonStatus(pidPath)
	if err != nil {
		t.Fatalf("getDaemonStatus() error = %v", err)
	}

	if status.Running {
		t.Error("getDaemonStatus().Running = true for missing PID file, want false")
	}
}

// Helper function for string contains
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
