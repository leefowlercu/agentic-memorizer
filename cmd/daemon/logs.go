package daemon

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/spf13/cobra"
)

var (
	followLogs bool
	tailLines  int
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show daemon logs",
	Long:  "\nShow logs from the background indexing daemon.",
	RunE:  runLogs,
}

func init() {
	logsCmd.Flags().BoolVarP(&followLogs, "follow", "f", false, "Follow log output")
	logsCmd.Flags().IntVarP(&tailLines, "tail", "n", 50, "Number of lines to show from the end")
}

func runLogs(cmd *cobra.Command, args []string) error {
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	logFile := cfg.Daemon.LogFile
	if logFile == "" {
		return fmt.Errorf("no log file configured")
	}

	// Check if log file exists
	if _, err := os.Stat(logFile); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("log file does not exist: %s", logFile)
		}
		return fmt.Errorf("failed to access log file: %w", err)
	}

	// Use tail command to show logs
	var tailCmd *exec.Cmd
	if followLogs {
		tailCmd = exec.Command("tail", "-f", "-n", fmt.Sprintf("%d", tailLines), logFile)
	} else {
		tailCmd = exec.Command("tail", "-n", fmt.Sprintf("%d", tailLines), logFile)
	}

	tailCmd.Stdout = os.Stdout
	tailCmd.Stderr = os.Stderr

	return tailCmd.Run()
}
