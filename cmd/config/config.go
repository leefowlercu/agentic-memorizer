package cmdconfig

import (
	"github.com/leefowlercu/agentic-memorizer/cmd/config/subcommands"
	"github.com/spf13/cobra"
)

var ConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long: "\nManage and validate Agentic Memorizer configuration.\n\n" +
		"The config command group provides tools for validating and managing the " +
		"configuration file.",
}

func init() {
	ConfigCmd.AddCommand(subcommands.ValidateCmd)
}
