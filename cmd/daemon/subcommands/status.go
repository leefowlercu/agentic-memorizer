package subcommands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/daemon"
	"github.com/leefowlercu/agentic-memorizer/internal/daemonclient"
)

// DaemonStatus holds the status information about the daemon.
type DaemonStatus struct {
	Running      bool                 `json:"running"`
	PID          int                  `json:"pid,omitempty"`
	StalePIDFile bool                 `json:"stale_pid_file,omitempty"`
	Health       *daemon.HealthStatus `json:"health,omitempty"`
}

// StatusCmd shows the daemon status.
var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status and health metrics",
	Long: "Show daemon status and health metrics.\n\n" +
		"Displays whether the daemon is running, its PID, and health status " +
		"including component health when available.",
	Example: `  # Check daemon status
  memorizer daemon status`,
	PreRunE: validateStatus,
	RunE:    runStatus,
}

func validateStatus(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	pidPath := config.ExpandPath(config.Get().Daemon.PIDFile)
	out := cmd.OutOrStdout()
	quiet := isQuiet(cmd)

	status, err := getDaemonStatus(pidPath)
	if err != nil {
		return fmt.Errorf("failed to get daemon status; %w", err)
	}

	if !quiet {
		fmt.Fprintln(out, formatStatus(status))
	}
	return nil
}

// getDaemonStatus retrieves the current status of the daemon.
func getDaemonStatus(pidPath string) (*DaemonStatus, error) {
	status := &DaemonStatus{}

	// Check PID file
	pid, err := readPIDFile(pidPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No PID file means daemon not running
			return status, nil
		}
		return nil, fmt.Errorf("failed to read PID file; %w", err)
	}

	status.PID = pid

	// Check if process is running
	if !isProcessRunning(pid) {
		status.StalePIDFile = true
		return status, nil
	}

	status.Running = true

	// Try to fetch health from the HTTP endpoint
	health, err := fetchHealth()
	if err == nil {
		status.Health = health
	}

	return status, nil
}

// fetchHealth attempts to fetch health status from the daemon's HTTP endpoint.
func fetchHealth() (*daemon.HealthStatus, error) {
	client, err := daemonclient.NewFromConfig(config.Get())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize daemon client; %w", err)
	}
	return client.Ready(context.Background())
}

// formatStatus formats the daemon status for display.
func formatStatus(status *DaemonStatus) string {
	var sb strings.Builder

	if !status.Running {
		sb.WriteString("Daemon: not running")
		if status.StalePIDFile {
			sb.WriteString(fmt.Sprintf(" (stale PID file with PID %d)", status.PID))
		}
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("Daemon: running (PID %d)", status.PID))

	if status.Health != nil {
		sb.WriteString(fmt.Sprintf("\nHealth: %s", status.Health.Status))
		sb.WriteString(fmt.Sprintf("\nReady: %v", status.Health.Ready))

		if len(status.Health.Components) > 0 {
			sb.WriteString("\nComponents:")
			for name, health := range status.Health.Components {
				sb.WriteString(fmt.Sprintf("\n  - %s: %s", name, health.Status))
				if health.Error != "" {
					sb.WriteString(fmt.Sprintf(" (%s)", health.Error))
				}
			}
		}
	}

	return sb.String()
}
