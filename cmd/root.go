package cmd

import (
	"fmt"
	"os"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/spf13/cobra"
)

var memorizerCmd = &cobra.Command{
	Use:   "memorizer",
	Short: "A Knowledge Graph Based Memorization Tool for AI Agents",
	Long: "Agentic Memorizer provides automatic awareness and analysis capabilities for files contained in registered directories.\n\n" +
		"A background daemon monitors registered directories and, optionally, their subdirectories for file additions, updates, moves, and deletions. " +
		"When changes are detected, the tool performs metadata extraction, semantic analysis, and embeddings generation for the affected files, updating its knowledge graph accordingly.\n\n",
	PersistentPreRunE: runInitializeConfig,
}

func init() {

}

func runInitializeConfig(cmd *cobra.Command, args []string) error {
	return config.Init()
}

func Execute() error {
	memorizerCmd.SilenceErrors = true
	memorizerCmd.SilenceUsage = true

	err := memorizerCmd.Execute()

	if err != nil {
		cmd, _, _ := memorizerCmd.Find(os.Args[1:])
		if cmd == nil {
			cmd = memorizerCmd
		}

		fmt.Printf("Error: %v\n", err)
		if !cmd.SilenceUsage {
			fmt.Printf("\n")
			cmd.SetOut(os.Stdout)
			_ = cmd.Usage()
		}

		return err
	}

	return nil
}
