package daemon

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/cmd/daemon/subcommands"
	"github.com/spf13/cobra"
)

var DaemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the background indexing daemon",
	Long: "\nManage the background indexing daemon that maintains the knowledge graph.\n\n" +
		"The daemon watches the memory directory for changes and automatically updates " +
		"the graph database, enabling quick access via the read command.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("please specify a subcommand: start, stop, status, restart, rebuild, or logs")
	},
}

func init() {
	DaemonCmd.AddCommand(subcommands.StartCmd)
	DaemonCmd.AddCommand(subcommands.StopCmd)
	DaemonCmd.AddCommand(subcommands.StatusCmd)
	DaemonCmd.AddCommand(subcommands.RestartCmd)
	DaemonCmd.AddCommand(subcommands.RebuildCmd)
	DaemonCmd.AddCommand(subcommands.LogsCmd)
}
