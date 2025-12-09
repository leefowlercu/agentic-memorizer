package graph

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/cmd/graph/subcommands"
	"github.com/spf13/cobra"
)

var GraphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Manage the FalkorDB knowledge graph",
	Long: "\nManage the FalkorDB knowledge graph that stores file relationships and enables semantic search.\n\n" +
		"FalkorDB runs as a Docker container and provides graph-based storage for file metadata, " +
		"semantic analysis, and embeddings. The daemon connects to FalkorDB to store and query the knowledge graph.\n\n" +
		"To rebuild the graph, use 'memorizer daemon rebuild'.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("please specify a subcommand: start, stop, or status")
	},
}

func init() {
	GraphCmd.AddCommand(subcommands.StartCmd)
	GraphCmd.AddCommand(subcommands.StopCmd)
	GraphCmd.AddCommand(subcommands.StatusCmd)
}
