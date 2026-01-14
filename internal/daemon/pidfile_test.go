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

// Edge case tests for PIDFile

func TestPIDFile_Read_NonNumericContent(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write non-numeric content
	if err := os.WriteFile(pidPath, []byte("not-a-number"), 0o644); err != nil {
		t.Fatalf("Failed to create test PID file: %v", err)
	}

	pf := NewPIDFile(pidPath)

	_, err := pf.Read()
	if err == nil {
		t.Error("PIDFile.Read() expected error for non-numeric content")
	}
}

func TestPIDFile_Read_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write empty content
	if err := os.WriteFile(pidPath, []byte(""), 0o644); err != nil {
		t.Fatalf("Failed to create test PID file: %v", err)
	}

	pf := NewPIDFile(pidPath)

	_, err := pf.Read()
	if err == nil {
		t.Error("PIDFile.Read() expected error for empty file")
	}
}

func TestPIDFile_Read_WhitespaceContent(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write PID with trailing newline (common case)
	expectedPID := 12345
	if err := os.WriteFile(pidPath, []byte("12345\n"), 0o644); err != nil {
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

func TestPIDFile_Write_CreatesParentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	// Nested path that doesn't exist
	pidPath := filepath.Join(tmpDir, "subdir", "nested", "test.pid")

	pf := NewPIDFile(pidPath)

	err := pf.Write()
	if err != nil {
		t.Fatalf("PIDFile.Write() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(pidPath); os.IsNotExist(err) {
		t.Error("PIDFile.Write() did not create file in nested directory")
	}
}

func TestPIDFile_Write_Permissions(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	pf := NewPIDFile(pidPath)

	err := pf.Write()
	if err != nil {
		t.Fatalf("PIDFile.Write() error = %v", err)
	}

	// Check file permissions (should be readable)
	info, err := os.Stat(pidPath)
	if err != nil {
		t.Fatalf("Failed to stat PID file: %v", err)
	}

	// File should be readable by owner at minimum
	perm := info.Mode().Perm()
	if perm&0o400 == 0 {
		t.Errorf("PID file is not readable, permissions = %o", perm)
	}
}

func TestPIDFile_IsStale_InvalidContent(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write invalid content
	if err := os.WriteFile(pidPath, []byte("invalid"), 0o644); err != nil {
		t.Fatalf("Failed to create test PID file: %v", err)
	}

	pf := NewPIDFile(pidPath)

	// Should return error for invalid content
	_, err := pf.IsStale()
	if err == nil {
		t.Error("PIDFile.IsStale() expected error for invalid content")
	}
}

func TestPIDFile_CheckAndClaim_InvalidContent(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write invalid content
	if err := os.WriteFile(pidPath, []byte("not-a-pid"), 0o644); err != nil {
		t.Fatalf("Failed to create test PID file: %v", err)
	}

	pf := NewPIDFile(pidPath)

	// Should return error for invalid content
	err := pf.CheckAndClaim()
	if err == nil {
		t.Error("PIDFile.CheckAndClaim() expected error for invalid content")
	}
}

func TestPIDFile_CheckAndClaim_NegativePID(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write negative PID
	if err := os.WriteFile(pidPath, []byte("-1"), 0o644); err != nil {
		t.Fatalf("Failed to create test PID file: %v", err)
	}

	pf := NewPIDFile(pidPath)

	// Negative PID should cause an error during stale check (invalid PID)
	err := pf.CheckAndClaim()
	if err == nil {
		t.Error("PIDFile.CheckAndClaim() expected error for negative PID")
	}
}

func TestPIDFile_Read_NegativePID(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write negative PID
	if err := os.WriteFile(pidPath, []byte("-1"), 0o644); err != nil {
		t.Fatalf("Failed to create test PID file: %v", err)
	}

	pf := NewPIDFile(pidPath)

	_, err := pf.Read()
	if err == nil {
		t.Error("PIDFile.Read() expected error for negative PID")
	}
}

func TestPIDFile_Read_ZeroPID(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write zero PID
	if err := os.WriteFile(pidPath, []byte("0"), 0o644); err != nil {
		t.Fatalf("Failed to create test PID file: %v", err)
	}

	pf := NewPIDFile(pidPath)

	_, err := pf.Read()
	if err == nil {
		t.Error("PIDFile.Read() expected error for zero PID")
	}
}

func TestPIDFile_Path(t *testing.T) {
	expectedPath := "/some/path/daemon.pid"
	pf := NewPIDFile(expectedPath)

	if pf.Path() != expectedPath {
		t.Errorf("PIDFile.Path() = %q, want %q", pf.Path(), expectedPath)
	}
}

func TestPIDFile_Read_LargePID(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write a large but valid PID (Linux max is 4194304, but can be higher)
	largePID := 4194304
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(largePID)), 0o644); err != nil {
		t.Fatalf("Failed to create test PID file: %v", err)
	}

	pf := NewPIDFile(pidPath)

	pid, err := pf.Read()
	if err != nil {
		t.Fatalf("PIDFile.Read() error = %v", err)
	}

	if pid != largePID {
		t.Errorf("PIDFile.Read() = %d, want %d", pid, largePID)
	}
}

func TestPIDFile_Read_VeryLargePID(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write a very large PID (still valid int)
	veryLargePID := 2147483647 // max int32
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(veryLargePID)), 0o644); err != nil {
		t.Fatalf("Failed to create test PID file: %v", err)
	}

	pf := NewPIDFile(pidPath)

	pid, err := pf.Read()
	if err != nil {
		t.Fatalf("PIDFile.Read() error = %v", err)
	}

	if pid != veryLargePID {
		t.Errorf("PIDFile.Read() = %d, want %d", pid, veryLargePID)
	}
}

func TestPIDFile_IsStale_LargePID(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write a very large PID that definitely doesn't exist
	veryLargePID := 2147483647
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(veryLargePID)), 0o644); err != nil {
		t.Fatalf("Failed to create test PID file: %v", err)
	}

	pf := NewPIDFile(pidPath)

	stale, err := pf.IsStale()
	if err != nil {
		t.Fatalf("PIDFile.IsStale() error = %v", err)
	}

	// Large PID should be stale (process doesn't exist)
	if !stale {
		t.Error("PIDFile.IsStale() = false for very large PID, want true")
	}
}

func TestPIDFile_Read_LeadingWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write PID with leading whitespace
	if err := os.WriteFile(pidPath, []byte("  12345"), 0o644); err != nil {
		t.Fatalf("Failed to create test PID file: %v", err)
	}

	pf := NewPIDFile(pidPath)

	pid, err := pf.Read()
	if err != nil {
		t.Fatalf("PIDFile.Read() error = %v", err)
	}

	if pid != 12345 {
		t.Errorf("PIDFile.Read() = %d, want 12345", pid)
	}
}

func TestPIDFile_Read_SurroundingWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write PID with whitespace on both sides
	if err := os.WriteFile(pidPath, []byte("  12345  \n"), 0o644); err != nil {
		t.Fatalf("Failed to create test PID file: %v", err)
	}

	pf := NewPIDFile(pidPath)

	pid, err := pf.Read()
	if err != nil {
		t.Fatalf("PIDFile.Read() error = %v", err)
	}

	if pid != 12345 {
		t.Errorf("PIDFile.Read() = %d, want 12345", pid)
	}
}

func TestPIDFile_Write_Overwrite(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write initial content
	if err := os.WriteFile(pidPath, []byte("99999"), 0o644); err != nil {
		t.Fatalf("Failed to create initial PID file: %v", err)
	}

	pf := NewPIDFile(pidPath)

	// Write should overwrite
	err := pf.Write()
	if err != nil {
		t.Fatalf("PIDFile.Write() error = %v", err)
	}

	// Verify content is now current PID
	content, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("Failed to read PID file: %v", err)
	}

	expectedPID := strconv.Itoa(os.Getpid())
	if string(content) != expectedPID {
		t.Errorf("PIDFile content = %q, want %q", string(content), expectedPID)
	}
}
