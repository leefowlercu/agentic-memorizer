// Package integrations provides the integrations command.
package integrations

import (
	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/cmd/integrations/subcommands"
)

// IntegrationsCmd is the parent command for integration management.
var IntegrationsCmd = &cobra.Command{
	Use:   "integrations",
	Short: "Manage AI harness integrations",
	Long: `Manage AI harness integrations.

The integrations command allows you to configure various AI coding assistants
(Claude Code, Codex CLI, Gemini CLI, OpenCode) to use the memorizer knowledge graph.
Integrations can inject knowledge graph data via hooks, expose data via MCP protocol,
or install plugins for native integration.`,
}

func init() {
	IntegrationsCmd.AddCommand(subcommands.ListCmd)
	IntegrationsCmd.AddCommand(subcommands.SetupCmd)
	IntegrationsCmd.AddCommand(subcommands.RemoveCmd)
	IntegrationsCmd.AddCommand(subcommands.StatusCmd)
}
