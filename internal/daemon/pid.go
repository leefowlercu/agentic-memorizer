package daemon

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// writePIDFile writes the current process PID to the specified file
func writePIDFile(path string) error {
	pid := os.Getpid()
	content := fmt.Sprintf("%d\n", pid)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	return nil
}

// checkPIDFile checks if a PID file exists and if the process is running
// Returns error if daemon is already running
func checkPIDFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No PID file, daemon not running
			return nil
		}
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		// Invalid PID file, remove it
		os.Remove(path)
		return nil
	}

	// Check if process is running
	process, err := os.FindProcess(pid)
	if err != nil {
		// Process not found, remove stale PID file
		os.Remove(path)
		return nil
	}

	// Try to send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		// Process doesn't exist, remove stale PID file
		os.Remove(path)
		return nil
	}

	return fmt.Errorf("daemon already running with PID %d", pid)
}

// removePIDFile removes the PID file
func removePIDFile(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove PID file: %w", err)
	}
	return nil
}

// readPIDFile reads the PID from the PID file
func readPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("failed to read PID file: %w", err)
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file: %w", err)
	}

	return pid, nil
}

// isProcessRunning checks if a process with the given PID is running
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}
