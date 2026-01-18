// Package subcommands provides the daemon subcommands (start, stop, status).
package subcommands

import "github.com/spf13/cobra"

// Helper functions shared across daemon subcommands.

func isQuiet(cmd *cobra.Command) bool {
	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		return false
	}
	return quiet
}
