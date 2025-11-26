package integrations

import (
	"github.com/leefowlercu/agentic-memorizer/cmd/integrations/subcommands"
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/adapters/claude"  // Register Claude adapter
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/adapters/codex"   // Register Codex adapter
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/adapters/gemini"  // Register Gemini adapter
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/adapters/generic" // Register generic adapters
	"github.com/spf13/cobra"
)

var IntegrationsCmd = &cobra.Command{
	Use:   "integrations",
	Short: "Manage agent framework integrations",
	Long: "\nManage integrations with agent frameworks like Claude Code, Continue.dev, Cline, etc.\n\n" +
		"The integrations command group provides tools for discovering, configuring, and managing " +
		"integrations with various AI agent frameworks.",
	Example: `  # List all available integrations
  agentic-memorizer integrations list

  # Detect installed agent frameworks
  agentic-memorizer integrations detect

  # Setup a specific integration
  agentic-memorizer integrations setup claude-code-hook

  # Remove an integration
  agentic-memorizer integrations remove claude-code-hook

  # Validate configuration
  agentic-memorizer integrations validate`,
}

func init() {
	IntegrationsCmd.AddCommand(subcommands.ListCmd)
	IntegrationsCmd.AddCommand(subcommands.DetectCmd)
	IntegrationsCmd.AddCommand(subcommands.SetupCmd)
	IntegrationsCmd.AddCommand(subcommands.RemoveCmd)
	IntegrationsCmd.AddCommand(subcommands.ValidateCmd)
	IntegrationsCmd.AddCommand(subcommands.HealthCmd)
}
