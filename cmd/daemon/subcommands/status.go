package subcommands

import (
	"fmt"
	"os"
	"syscall"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/daemon"
	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/spf13/cobra"
)

var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	Long: "\nShow the current status of the background indexing daemon.\n\n" +
		"Displays whether the daemon is running and configuration details.",
	PreRunE: validateStatus,
	RunE:    runStatus,
}

func validateStatus(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config; %w", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config; %w", err)
	}

	pidFile, err := config.GetPIDPath()
	if err != nil {
		return fmt.Errorf("failed to get PID path; %w", err)
	}

	// Build main section
	section := format.NewSection("Daemon Status").AddDivider()

	// Check if daemon is running
	var pid int
	var running bool

	data, err := os.ReadFile(pidFile)
	if err == nil {
		fmt.Sscanf(string(data), "%d", &pid)

		// Check if process exists
		process, err := os.FindProcess(pid)
		if err == nil {
			err = process.Signal(syscall.Signal(0))
			running = err == nil
		}
	}

	if running {
		managed := daemon.IsServiceManaged()
		if managed {
			section.AddKeyValue("Status", fmt.Sprintf("Running (PID %d, service-managed)", pid))
		} else {
			section.AddKeyValue("Status", fmt.Sprintf("Running (PID %d, foreground)", pid))
		}
	} else {
		section.AddKeyValue("Status", "Not running")
	}

	// Add Configuration subsection
	configSection := format.NewSection("Configuration").SetLevel(1).AddDivider()
	configSection.AddKeyValuef("Debounce Period", "%d ms", cfg.Daemon.DebounceMs)
	configSection.AddKeyValuef("Workers", "%d", cfg.Daemon.Workers)
	configSection.AddKeyValuef("Rate Limit", "%d/min", cfg.Daemon.RateLimitPerMin)
	configSection.AddKeyValuef("Rebuild Interval", "%d minutes", cfg.Daemon.FullRebuildIntervalMinutes)

	// HTTP Port with "(disabled)" if 0
	httpPortValue := fmt.Sprintf("%d", cfg.Daemon.HTTPPort)
	if cfg.Daemon.HTTPPort == 0 {
		httpPortValue = "0 (disabled)"
	}
	configSection.AddKeyValue("HTTP Port", httpPortValue)

	configSection.AddKeyValue("Log File", cfg.Daemon.LogFile)
	configSection.AddKeyValue("Log Level", cfg.Daemon.LogLevel)
	section.AddSubsection(configSection)

	// Format and write output
	formatter, err := format.GetFormatter("text")
	if err != nil {
		return fmt.Errorf("failed to get formatter; %w", err)
	}
	output, err := formatter.Format(section)
	if err != nil {
		return fmt.Errorf("failed to format output; %w", err)
	}
	fmt.Println(output)

	// Add note if daemon is not running
	if !running {
		fmt.Printf("\nTo start the daemon, run: agentic-memorizer daemon start\n")
	}

	return nil
}
