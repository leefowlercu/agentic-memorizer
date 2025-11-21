package subcommands

import (
	"fmt"
	"os"
	"syscall"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/daemon"
	"github.com/spf13/cobra"
)

var StopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running daemon",
	Long: "\nStop the running background indexing daemon by sending a SIGTERM signal.\n\n" +
		"The daemon will gracefully shut down, completing any in-progress operations before exiting.",
	PreRunE: validateStop,
	RunE:    runStop,
}

func validateStop(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runStop(cmd *cobra.Command, args []string) error {
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config; %w", err)
	}

	pidFile, err := config.GetPIDPath()
	if err != nil {
		return fmt.Errorf("failed to get PID path; %w", err)
	}

	// Read PID file
	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("daemon is not running (PID file not found)%s", daemon.GetNotRunningHelp())
		}
		return fmt.Errorf("failed to read PID file; %w", err)
	}

	var pid int
	_, err = fmt.Sscanf(string(data), "%d", &pid)
	if err != nil {
		return fmt.Errorf("invalid PID file; %w", err)
	}

	// Find process
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("daemon process not found; %w", err)
	}

	// Send SIGTERM
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to signal daemon; %w%s", err, daemon.GetSignalErrorHelp(pid))
	}

	fmt.Printf("Sent stop signal to daemon (PID %d)\n", pid)
	return nil
}
