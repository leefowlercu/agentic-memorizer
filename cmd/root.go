package cmd

import (
	"fmt"
	"os"

	cmdconfig "github.com/leefowlercu/agentic-memorizer/cmd/config"
	cmddaemon "github.com/leefowlercu/agentic-memorizer/cmd/daemon"
	cmdinit "github.com/leefowlercu/agentic-memorizer/cmd/init"
	cmdintegrations "github.com/leefowlercu/agentic-memorizer/cmd/integrations"
	cmdread "github.com/leefowlercu/agentic-memorizer/cmd/read"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/spf13/cobra"
)

var memorizerCmd = &cobra.Command{
	Use:   "agentic-memorizer",
	Short: "Local file memorizer for AI agent frameworks",
	Long: "\nA local file 'memorizer' for AI agent frameworks that provides automatic " +
		"awareness and understanding of files in your memory directory through AI-powered semantic analysis.\n\n" +
		"The background daemon continuously maintains a precomputed index of your memory directory, enabling " +
		"quick startup for agent sessions. Files are automatically indexed and semantically analyzed " +
		"in the background, with the index available via the 'read' command for framework hooks or tools.",
	PersistentPreRunE: runInit,
}

func init() {
	memorizerCmd.AddCommand(cmdinit.InitCmd)
	memorizerCmd.AddCommand(cmddaemon.DaemonCmd)
	memorizerCmd.AddCommand(cmdread.ReadCmd)
	memorizerCmd.AddCommand(cmdintegrations.IntegrationsCmd)
	memorizerCmd.AddCommand(cmdconfig.ConfigCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	if cmd.Name() == "init" {
		return nil
	}

	err := config.InitConfig()
	if err != nil {
		return fmt.Errorf("failed to initialize configuration; %w", err)
	}

	return nil
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
			cmd.Usage()
		}

		return err
	}

	return nil
}
