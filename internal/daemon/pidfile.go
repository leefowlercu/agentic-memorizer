package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// ErrDaemonAlreadyRunning indicates that another daemon process is already running.
var ErrDaemonAlreadyRunning = errors.New("daemon already running")

// PIDFile manages a process ID file for the daemon.
type PIDFile struct {
	path string
}

// NewPIDFile creates a new PIDFile instance with the given path.
func NewPIDFile(path string) *PIDFile {
	return &PIDFile{path: path}
}

// Path returns the path to the PID file.
func (p *PIDFile) Path() string {
	return p.path
}

// Write writes the current process's PID to the file atomically.
// It uses a temporary file and rename to ensure atomic write.
func (p *PIDFile) Write() error {
	pid := os.Getpid()
	pidStr := strconv.Itoa(pid)

	// Ensure directory exists
	dir := filepath.Dir(p.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create PID file directory; %w", err)
	}

	// Write to temporary file first
	tmpPath := p.path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(pidStr), 0o644); err != nil {
		return fmt.Errorf("failed to write temporary PID file; %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, p.path); err != nil {
		os.Remove(tmpPath) // Clean up temp file on error
		return fmt.Errorf("failed to rename PID file; %w", err)
	}

	return nil
}

// Read reads and returns the PID from the file.
func (p *PIDFile) Read() (int, error) {
	content, err := os.ReadFile(p.path)
	if err != nil {
		return 0, fmt.Errorf("failed to read PID file; %w", err)
	}

	// Trim whitespace (PID files often have trailing newlines)
	pidStr := strings.TrimSpace(string(content))
	if pidStr == "" {
		return 0, errors.New("empty PID file")
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file; %w", err)
	}

	if pid <= 0 {
		return 0, fmt.Errorf("invalid PID %d; must be positive", pid)
	}

	return pid, nil
}

// Remove removes the PID file if it exists.
// It does not return an error if the file does not exist.
func (p *PIDFile) Remove() error {
	err := os.Remove(p.path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove PID file; %w", err)
	}
	return nil
}

// IsStale checks if the PID file references a process that is no longer running.
// Returns false if the file does not exist (not stale because there's nothing to be stale).
// Returns true if the file exists but the process is not running.
// Returns false if the file exists and the process is running.
func (p *PIDFile) IsStale() (bool, error) {
	pid, err := p.Read()
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		// If we got a read error (not NotExist), verify file actually exists
		if _, statErr := os.Stat(p.path); os.IsNotExist(statErr) {
			// File was deleted between Read() and Stat(), treat as not stale
			return false, nil
		}
		// File exists but couldn't be read - propagate error
		return false, fmt.Errorf("PID file exists but unreadable; %w", err)
	}

	// Check if process exists by sending signal 0
	// This doesn't actually send a signal, just checks if the process exists
	err = syscall.Kill(pid, 0)
	if err != nil {
		if errors.Is(err, syscall.ESRCH) {
			// Process does not exist - PID file is stale
			return true, nil
		}
		if errors.Is(err, syscall.EPERM) {
			// Process exists but we don't have permission to signal it
			// This means the process is running
			return false, nil
		}
		return false, fmt.Errorf("failed to check process; %w", err)
	}

	// Process exists
	return false, nil
}

// CheckAndClaim checks if the daemon can claim the PID file.
// If no PID file exists, it creates one with the current PID.
// If a stale PID file exists, it removes it and creates a new one.
// If an active PID file exists, it returns ErrDaemonAlreadyRunning.
func (p *PIDFile) CheckAndClaim() error {
	// Check if PID file exists
	if _, err := os.Stat(p.path); os.IsNotExist(err) {
		// No PID file, claim it
		return p.Write()
	}

	// PID file exists, check if stale
	stale, err := p.IsStale()
	if err != nil {
		return fmt.Errorf("failed to check if PID file is stale; %w", err)
	}

	if stale {
		// Remove stale PID file and claim
		if err := p.Remove(); err != nil {
			return fmt.Errorf("failed to remove stale PID file; %w", err)
		}
		return p.Write()
	}

	// PID file exists and process is running
	return ErrDaemonAlreadyRunning
}
