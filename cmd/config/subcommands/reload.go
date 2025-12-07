package subcommands

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/format"
	_ "github.com/leefowlercu/agentic-memorizer/internal/format/formatters" // Register formatters
	"github.com/spf13/cobra"
)

var ReloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reload configuration",
	Long: "\nReload the configuration file and apply changes to running daemon.\n\n" +
		"This command validates the new configuration and applies hot-reloadable " +
		"changes to the daemon if it is running. If the daemon is not running, " +
		"the configuration is validated and the next daemon start will use the new settings.\n\n" +
		"Hot-reloadable settings:\n" +
		"- Claude API settings (key, model, tokens, vision, timeout)\n" +
		"- Worker pool size and rate limits\n" +
		"- Debounce interval\n" +
		"- Log level\n" +
		"- Health check port\n" +
		"- Rebuild interval\n" +
		"- Skip patterns\n\n" +
		"Settings that require daemon restart:\n" +
		"- memory_root\n" +
		"- analysis.cache_dir\n" +
		"- daemon.log_file",
	Example: `  # Reload configuration
  agentic-memorizer config reload

  # Check what's changed without applying
  agentic-memorizer config validate`,
	PreRunE: validateReload,
	RunE:    runReload,
}

func validateReload(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runReload(cmd *cobra.Command, args []string) error {
	// Initialize and load new config
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to load config; %w", err)
	}

	newCfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get config; %w", err)
	}

	// Validate new configuration
	if err := config.ValidateConfig(newCfg); err != nil {
		status := format.NewStatus(format.StatusError, "Configuration validation failed")
		status.AddDetail(err.Error())
		if outputErr := outputStatus(status); outputErr != nil {
			return outputErr
		}
		return fmt.Errorf("validation failed")
	}

	validStatus := format.NewStatus(format.StatusSuccess, "Configuration is valid")
	if err := outputStatus(validStatus); err != nil {
		return err
	}

	// Check if daemon is running
	pidFile := getPIDFilePath()
	pid, running := isDaemonRunning(pidFile)

	if !running {
		status := format.NewStatus(format.StatusInfo, "Daemon is not running")
		status.AddDetail("New configuration will be used on next daemon start")
		return outputStatus(status)
	}

	// Daemon is running - send reload signal
	runningStatus := format.NewStatus(format.StatusInfo, fmt.Sprintf("Daemon is running (PID: %d)", pid))
	runningStatus.AddDetail("Sending SIGHUP signal to reload configuration")
	if err := outputStatus(runningStatus); err != nil {
		return err
	}

	// Send SIGHUP to daemon
	if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
		return fmt.Errorf("failed to send SIGHUP signal; %w", err)
	}

	successStatus := format.NewStatus(format.StatusSuccess, "Configuration reload signal sent successfully")
	if newCfg.Daemon.LogFile != "" {
		successStatus.AddDetail(fmt.Sprintf("Check daemon logs: tail -f %s", newCfg.Daemon.LogFile))
	} else {
		successStatus.AddDetail("Check daemon output (logging to stdout)")
	}
	return outputStatus(successStatus)
}

// outputStatus formats and outputs a status message
func outputStatus(status *format.Status) error {
	formatter, err := format.GetFormatter("text")
	if err != nil {
		return fmt.Errorf("failed to get formatter; %w", err)
	}
	output, err := formatter.Format(status)
	if err != nil {
		return fmt.Errorf("failed to format status; %w", err)
	}
	fmt.Println(output)
	return nil
}

// getPIDFilePath returns the daemon PID file path
func getPIDFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".agentic-memorizer", "daemon.pid")
}

// isDaemonRunning checks if the daemon is running by checking PID file
func isDaemonRunning(pidFile string) (int, bool) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, false
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, false
	}

	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return 0, false
	}

	// Send signal 0 to check if process is alive
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return 0, false
	}

	return pid, true
}
