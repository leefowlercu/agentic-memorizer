// Package providers provides the providers parent command and subcommands.
package providers

import (
	"github.com/leefowlercu/agentic-memorizer/cmd/providers/subcommands"
	"github.com/spf13/cobra"
)

// ProvidersCmd is the parent command for all provider-related subcommands.
var ProvidersCmd = &cobra.Command{
	Use:   "providers",
	Short: "Manage AI providers for semantic analysis and embeddings",
	Long: "Manage AI providers for semantic analysis and embeddings.\n\n" +
		"Providers are the AI services that perform semantic analysis and generate " +
		"vector embeddings for memorized content. This command allows you to list " +
		"available providers and test their connectivity.",
}

func init() {
	// Register subcommands
	ProvidersCmd.AddCommand(subcommands.ListCmd)
	ProvidersCmd.AddCommand(subcommands.TestCmd)
}
