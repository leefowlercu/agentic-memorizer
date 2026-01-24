package subcommands

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
)

// Errors for stop command
var (
	ErrNoDaemonRunning = errors.New("no daemon running")
	ErrStalePIDFile    = errors.New("stale PID file found and cleaned up")
)

// StopCmd stops a running daemon.
var StopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running daemon gracefully",
	Long: "Stop the running daemon gracefully.\n\n" +
		"Sends SIGTERM to the running daemon process and waits for it to shut down. " +
		"If the daemon does not respond within the timeout period, a warning is logged.",
	Example: `  # Stop the daemon
  memorizer daemon stop`,
	PreRunE: validateStop,
	RunE:    runStop,
}

var (
	stopTimeout time.Duration
)

func init() {
	StopCmd.Flags().DurationVar(&stopTimeout, "timeout", 30*time.Second,
		"Maximum time to wait for daemon to stop")
}

func validateStop(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runStop(cmd *cobra.Command, args []string) error {
	pidPath := config.ExpandPath(config.Get().Daemon.PIDFile)

	if err := stopDaemon(pidPath); err != nil {
		if errors.Is(err, ErrNoDaemonRunning) {
			fmt.Println("No daemon is running")
			return nil
		}
		if errors.Is(err, ErrStalePIDFile) {
			fmt.Println("Found stale PID file, cleaned up")
			return nil
		}
		return fmt.Errorf("failed to stop daemon; %w", err)
	}

	fmt.Println("Daemon stopped")
	return nil
}

// stopDaemon attempts to stop the daemon by reading the PID file and sending SIGTERM.
func stopDaemon(pidPath string) error {
	// Read PID from file
	pid, err := readPIDFile(pidPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNoDaemonRunning
		}
		return fmt.Errorf("failed to read PID file; %w", err)
	}

	// Check if process is running
	if !isProcessRunning(pid) {
		// Stale PID file - clean it up
		os.Remove(pidPath)
		return ErrStalePIDFile
	}

	slog.Debug("sending SIGTERM to daemon", "pid", pid)

	// Send SIGTERM
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM; %w", err)
	}

	// Wait for process to exit
	deadline := time.Now().Add(stopTimeout)
	for time.Now().Before(deadline) {
		if !isProcessRunning(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	slog.Debug("daemon did not stop within timeout", "pid", pid, "timeout", stopTimeout)
	return nil
}

// readPIDFile reads and parses the PID from the given file.
func readPIDFile(path string) (int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(string(content))
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file; %w", err)
	}

	return pid, nil
}

// isProcessRunning checks if a process with the given PID is running.
func isProcessRunning(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
