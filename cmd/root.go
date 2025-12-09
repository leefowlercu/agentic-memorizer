package cmd

import (
	"fmt"
	"os"

	"github.com/leefowlercu/agentic-memorizer/cmd/cache"
	"github.com/leefowlercu/agentic-memorizer/cmd/config"
	"github.com/leefowlercu/agentic-memorizer/cmd/daemon"
	"github.com/leefowlercu/agentic-memorizer/cmd/graph"
	"github.com/leefowlercu/agentic-memorizer/cmd/initialize"
	"github.com/leefowlercu/agentic-memorizer/cmd/integrations"
	"github.com/leefowlercu/agentic-memorizer/cmd/mcp"
	"github.com/leefowlercu/agentic-memorizer/cmd/read"
	"github.com/leefowlercu/agentic-memorizer/cmd/version"
	configint "github.com/leefowlercu/agentic-memorizer/internal/config"
	versionint "github.com/leefowlercu/agentic-memorizer/internal/version"
	"github.com/spf13/cobra"
)

var memorizerCmd = &cobra.Command{
	Use:   "memorizer",
	Short: "Local file memorizer for AI agent frameworks",
	Long: "\nA local file 'memorizer' for AI agent frameworks that provides automatic " +
		"awareness and understanding of files in your memory directory through AI-powered semantic analysis.\n\n" +
		"The background daemon continuously maintains a knowledge graph of your memory directory. Files are automatically indexed and semantically analyzed. " +
		"The analysis is stored in a graph database with relationship information.",
	PersistentPreRunE: runInit,
	// Set custom version output
	Version: getVersionString(),
}

func init() {
	memorizerCmd.AddCommand(initialize.InitializeCmd)
	memorizerCmd.AddCommand(daemon.DaemonCmd)
	memorizerCmd.AddCommand(graph.GraphCmd)
	memorizerCmd.AddCommand(cache.CacheCmd)
	memorizerCmd.AddCommand(read.ReadCmd)
	memorizerCmd.AddCommand(integrations.IntegrationsCmd)
	memorizerCmd.AddCommand(config.ConfigCmd)
	memorizerCmd.AddCommand(mcp.McpCmd)
	memorizerCmd.AddCommand(version.VersionCmd)

	// Customize version output template to use multi-line format
	memorizerCmd.SetVersionTemplate(getVersionString() + "\n")
}

// getVersionString returns the version information in multi-line format
func getVersionString() string {
	return fmt.Sprintf("Version: %s\nCommit:  %s\nBuilt:   %s",
		versionint.GetShortVersion(),
		versionint.GetGitCommit(),
		versionint.GetBuildDate())
}

func runInit(cmd *cobra.Command, args []string) error {
	// Skip config init for 'initialize' command - it creates config, so can't load it first.
	// All other commands require existing config and will fail if not initialized.
	if cmd.Name() == "initialize" {
		return nil
	}

	err := configint.InitConfig()
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
