package daemon

import (
	"fmt"

	"github.com/spf13/cobra"
)

var DaemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the background indexing daemon",
	Long: "\nManage the background indexing daemon that maintains a precomputed index.\n\n" +
		"The daemon watches the memory directory for changes and automatically rebuilds " +
		"the index, ensuring fast startup times for the read command.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("please specify a subcommand: start, stop, status, restart, rebuild, or logs")
	},
}

func init() {
	DaemonCmd.AddCommand(startCmd)
	DaemonCmd.AddCommand(stopCmd)
	DaemonCmd.AddCommand(statusCmd)
	DaemonCmd.AddCommand(restartCmd)
	DaemonCmd.AddCommand(rebuildCmd)
	DaemonCmd.AddCommand(logsCmd)
}
