package daemon

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// T016: Tests for PIDFile Write/Read/Remove

func TestPIDFile_Write(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	pf := NewPIDFile(pidPath)

	err := pf.Write()
	if err != nil {
		t.Fatalf("PIDFile.Write() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(pidPath); os.IsNotExist(err) {
		t.Error("PIDFile.Write() did not create file")
	}

	// Verify content is current PID
	content, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("Failed to read PID file: %v", err)
	}

	expectedPID := strconv.Itoa(os.Getpid())
	if string(content) != expectedPID {
		t.Errorf("PIDFile content = %q, want %q", string(content), expectedPID)
	}
}

func TestPIDFile_Read(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write a known PID
	expectedPID := 12345
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(expectedPID)), 0o644); err != nil {
		t.Fatalf("Failed to create test PID file: %v", err)
	}

	pf := NewPIDFile(pidPath)

	pid, err := pf.Read()
	if err != nil {
		t.Fatalf("PIDFile.Read() error = %v", err)
	}

	if pid != expectedPID {
		t.Errorf("PIDFile.Read() = %d, want %d", pid, expectedPID)
	}
}

func TestPIDFile_Read_NotExists(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "nonexistent.pid")

	pf := NewPIDFile(pidPath)

	_, err := pf.Read()
	if err == nil {
		t.Error("PIDFile.Read() expected error for nonexistent file")
	}
}

func TestPIDFile_Remove(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Create file first
	if err := os.WriteFile(pidPath, []byte("12345"), 0o644); err != nil {
		t.Fatalf("Failed to create test PID file: %v", err)
	}

	pf := NewPIDFile(pidPath)

	err := pf.Remove()
	if err != nil {
		t.Fatalf("PIDFile.Remove() error = %v", err)
	}

	// Verify file is removed
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("PIDFile.Remove() did not remove file")
	}
}

func TestPIDFile_Remove_NotExists(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "nonexistent.pid")

	pf := NewPIDFile(pidPath)

	// Should not error when file doesn't exist
	err := pf.Remove()
	if err != nil {
		t.Errorf("PIDFile.Remove() error = %v, want nil for nonexistent file", err)
	}
}

// T017: Tests for PIDFile IsStale detection

func TestPIDFile_IsStale_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "nonexistent.pid")

	pf := NewPIDFile(pidPath)

	stale, err := pf.IsStale()
	if err != nil {
		t.Fatalf("PIDFile.IsStale() error = %v", err)
	}

	if stale {
		t.Error("PIDFile.IsStale() = true for nonexistent file, want false")
	}
}

func TestPIDFile_IsStale_CurrentProcess(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write current process PID
	currentPID := os.Getpid()
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(currentPID)), 0o644); err != nil {
		t.Fatalf("Failed to create test PID file: %v", err)
	}

	pf := NewPIDFile(pidPath)

	stale, err := pf.IsStale()
	if err != nil {
		t.Fatalf("PIDFile.IsStale() error = %v", err)
	}

	if stale {
		t.Error("PIDFile.IsStale() = true for current process, want false")
	}
}

func TestPIDFile_IsStale_DeadProcess(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write a PID that almost certainly doesn't exist
	// Use a very high PID that's unlikely to be in use
	deadPID := 99999999
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(deadPID)), 0o644); err != nil {
		t.Fatalf("Failed to create test PID file: %v", err)
	}

	pf := NewPIDFile(pidPath)

	stale, err := pf.IsStale()
	if err != nil {
		t.Fatalf("PIDFile.IsStale() error = %v", err)
	}

	if !stale {
		t.Error("PIDFile.IsStale() = false for dead process PID, want true")
	}
}

// T018: Tests for PIDFile CheckAndClaim

func TestPIDFile_CheckAndClaim_NoExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	pf := NewPIDFile(pidPath)

	err := pf.CheckAndClaim()
	if err != nil {
		t.Fatalf("PIDFile.CheckAndClaim() error = %v", err)
	}

	// Verify file was created with current PID
	content, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("Failed to read PID file: %v", err)
	}

	expectedPID := strconv.Itoa(os.Getpid())
	if string(content) != expectedPID {
		t.Errorf("PIDFile content = %q, want %q", string(content), expectedPID)
	}
}

func TestPIDFile_CheckAndClaim_StaleFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write a stale PID
	deadPID := 99999999
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(deadPID)), 0o644); err != nil {
		t.Fatalf("Failed to create test PID file: %v", err)
	}

	pf := NewPIDFile(pidPath)

	err := pf.CheckAndClaim()
	if err != nil {
		t.Fatalf("PIDFile.CheckAndClaim() error = %v, want nil for stale file", err)
	}

	// Verify file was updated with current PID
	content, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("Failed to read PID file: %v", err)
	}

	expectedPID := strconv.Itoa(os.Getpid())
	if string(content) != expectedPID {
		t.Errorf("PIDFile content = %q, want %q", string(content), expectedPID)
	}
}

func TestPIDFile_CheckAndClaim_ActiveProcess(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write current process PID (simulating already running daemon)
	currentPID := os.Getpid()
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(currentPID)), 0o644); err != nil {
		t.Fatalf("Failed to create test PID file: %v", err)
	}

	pf := NewPIDFile(pidPath)

	err := pf.CheckAndClaim()
	if err == nil {
		t.Error("PIDFile.CheckAndClaim() expected error for active process")
	}
}
