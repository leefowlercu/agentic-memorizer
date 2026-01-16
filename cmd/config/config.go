// Package config provides the config parent command and subcommands.
package config

import (
	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/cmd/config/subcommands"
)

// ConfigCmd is the parent command for all config-related subcommands.
var ConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage memorizer configuration",
	Long: "Manage memorizer configuration.\n\n" +
		"The config command allows you to view, edit, and reset the memorizer " +
		"configuration. Configuration is stored in a YAML file located at " +
		"~/.config/memorizer/config.yaml by default.",
}

func init() {
	// Register subcommands
	ConfigCmd.AddCommand(subcommands.ShowCmd)
	ConfigCmd.AddCommand(subcommands.EditCmd)
	ConfigCmd.AddCommand(subcommands.ResetCmd)
	ConfigCmd.AddCommand(subcommands.ValidateCmd)
}
