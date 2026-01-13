// Package daemon provides the daemon parent command and subcommands.
package daemon

import (
	"github.com/leefowlercu/agentic-memorizer/cmd/daemon/subcommands"
	"github.com/spf13/cobra"
)

// DaemonCmd is the parent command for all daemon-related subcommands.
var DaemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the memorizer daemon",
	Long: "Manage the memorizer daemon.\n\n" +
		"The daemon command allows you to start, stop, and check the status of the " +
		"background memorizer service. The daemon coordinates various subsystems and " +
		"exposes health check endpoints for monitoring.",
}

func init() {
	// Register subcommands
	DaemonCmd.AddCommand(subcommands.StartCmd)
	DaemonCmd.AddCommand(subcommands.StopCmd)
	DaemonCmd.AddCommand(subcommands.StatusCmd)
	DaemonCmd.AddCommand(subcommands.RebuildCmd)
}
