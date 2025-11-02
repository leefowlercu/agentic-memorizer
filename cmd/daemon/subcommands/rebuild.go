package subcommands

import (
	"fmt"
	"os"
	"syscall"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/spf13/cobra"
)

var RebuildCmd = &cobra.Command{
	Use:   "rebuild",
	Short: "Force immediate index rebuild",
	Long: "\nForce the daemon to perform an immediate full index rebuild.\n\n" +
		"This sends a SIGUSR1 signal to the running daemon. If the daemon is not running, " +
		"this command will return an error.",
	PreRunE: validateRebuild,
	RunE:    runRebuild,
}

func validateRebuild(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runRebuild(cmd *cobra.Command, args []string) error {
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	pidFile, err := config.GetPIDPath()
	if err != nil {
		return fmt.Errorf("failed to get PID path: %w", err)
	}

	// Read PID file
	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("daemon is not running (PID file not found)")
		}
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	var pid int
	_, err = fmt.Sscanf(string(data), "%d", &pid)
	if err != nil {
		return fmt.Errorf("invalid PID file: %w", err)
	}

	// Find process
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("daemon process not found: %w", err)
	}

	// Send SIGUSR1 to trigger rebuild
	if err := process.Signal(syscall.SIGUSR1); err != nil {
		return fmt.Errorf("failed to signal daemon: %w", err)
	}

	fmt.Printf("Sent rebuild signal to daemon (PID %d)\n", pid)
	fmt.Println("Check daemon logs for rebuild progress")
	return nil
}
