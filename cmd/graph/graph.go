package graph

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/cmd/graph/subcommands"
	"github.com/spf13/cobra"
)

var GraphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Check FalkorDB knowledge graph status",
	Long: "\nCheck the status of the FalkorDB knowledge graph.\n\n" +
		"FalkorDB provides graph-based storage for file metadata, semantic analysis, and embeddings. " +
		"The daemon connects to FalkorDB to store and query the knowledge graph. " +
		"Use 'status' to check connectivity and view graph statistics.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("please specify a subcommand: status")
	},
}

func init() {
	GraphCmd.AddCommand(subcommands.StatusCmd)
}
