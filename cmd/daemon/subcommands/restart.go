package subcommands

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/spf13/cobra"
)

var RestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the daemon with graceful shutdown",
	Long: "\nRestart the background indexing daemon by stopping and starting it.\n\n" +
		"Performs a graceful shutdown by sending SIGTERM to the running daemon, " +
		"then waits up to 3 seconds for the daemon to stop before starting a new instance.",
	Example: `  # Restart the daemon
  memorizer daemon restart`,
	PreRunE: validateRestart,
	RunE:    runRestart,
}

func validateRestart(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runRestart(cmd *cobra.Command, args []string) error {
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config; %w", err)
	}

	pidFile, err := config.GetPIDPath()
	if err != nil {
		return fmt.Errorf("failed to get PID path; %w", err)
	}

	// Try to stop existing daemon
	data, err := os.ReadFile(pidFile)
	if err == nil {
		var pid int
		fmt.Sscanf(string(data), "%d", &pid)

		process, err := os.FindProcess(pid)
		if err == nil {
			status := format.NewStatus(format.StatusRunning, fmt.Sprintf("Stopping daemon (PID %d)", pid))
			if outputErr := outputStatus(status); outputErr != nil {
				return outputErr
			}

			if err := process.Signal(syscall.SIGTERM); err == nil {
				// Wait for daemon to stop
				for i := 0; i < 30; i++ {
					if err := process.Signal(syscall.Signal(0)); err != nil {
						// Process stopped
						break
					}
					time.Sleep(100 * time.Millisecond)
				}
			}
		}
	}

	// Start daemon
	startStatus := format.NewStatus(format.StatusRunning, "Starting daemon")
	if err := outputStatus(startStatus); err != nil {
		return err
	}
	return runStart(cmd, args)
}
