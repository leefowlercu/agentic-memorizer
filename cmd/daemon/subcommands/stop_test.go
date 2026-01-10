package subcommands

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// T057: Tests for stop command PID reading

func TestReadPIDFile_ValidPID(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "daemon.pid")

	// Write a valid PID
	expectedPID := 12345
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(expectedPID)), 0o644); err != nil {
		t.Fatalf("Failed to write PID file: %v", err)
	}

	pid, err := readPIDFile(pidPath)
	if err != nil {
		t.Fatalf("readPIDFile() error = %v", err)
	}

	if pid != expectedPID {
		t.Errorf("readPIDFile() = %d, want %d", pid, expectedPID)
	}
}

func TestReadPIDFile_NotExists(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "nonexistent.pid")

	_, err := readPIDFile(pidPath)
	if err == nil {
		t.Error("readPIDFile() expected error for nonexistent file")
	}
}

func TestReadPIDFile_InvalidContent(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "daemon.pid")

	// Write invalid content
	if err := os.WriteFile(pidPath, []byte("not-a-number"), 0o644); err != nil {
		t.Fatalf("Failed to write PID file: %v", err)
	}

	_, err := readPIDFile(pidPath)
	if err == nil {
		t.Error("readPIDFile() expected error for invalid content")
	}
}

// T058: Tests for process checking

func TestIsProcessRunning_CurrentProcess(t *testing.T) {
	// Current process should be running
	pid := os.Getpid()
	if !isProcessRunning(pid) {
		t.Errorf("isProcessRunning(%d) = false, want true for current process", pid)
	}
}

func TestIsProcessRunning_DeadProcess(t *testing.T) {
	// Very high PID that almost certainly doesn't exist
	pid := 99999999
	if isProcessRunning(pid) {
		t.Errorf("isProcessRunning(%d) = true, want false for dead process", pid)
	}
}

// T059: Tests for stop command when no daemon running

func TestStopCmd_NoDaemonRunning(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "nonexistent.pid")

	// This should return ErrNoDaemonRunning
	err := stopDaemon(pidPath)
	if err != ErrNoDaemonRunning {
		t.Errorf("stopDaemon() error = %v, want ErrNoDaemonRunning", err)
	}
}

func TestStopCmd_StalePIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "daemon.pid")

	// Write a stale PID (process that doesn't exist)
	stalePID := 99999999
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(stalePID)), 0o644); err != nil {
		t.Fatalf("Failed to write PID file: %v", err)
	}

	err := stopDaemon(pidPath)
	if err != ErrStalePIDFile {
		t.Errorf("stopDaemon() error = %v, want ErrStalePIDFile", err)
	}

	// PID file should be cleaned up
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("stopDaemon() should have cleaned up stale PID file")
	}
}
