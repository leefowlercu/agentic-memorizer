package subcommands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// SetupCmd sets up an integration.
var SetupCmd = &cobra.Command{
	Use:   "setup <integration-name>",
	Short: "Set up an integration",
	Long: `Set up an integration.

Configures the specified AI harness to use the memorizer knowledge graph.
This may involve modifying configuration files, adding hooks, or installing plugins.
A backup of any modified configuration is created automatically.`,
	Example: `  # Set up Claude Code hook integration
  memorizer integrations setup claude-code-hook

  # Set up Claude Code MCP integration
  memorizer integrations setup claude-code-mcp

  # Set up Gemini CLI hook integration
  memorizer integrations setup gemini-cli-hook`,
	Args:    cobra.ExactArgs(1),
	PreRunE: validateSetup,
	RunE:    runSetup,
}

func validateSetup(cmd *cobra.Command, args []string) error {
	integrationName := args[0]

	// Check if integration exists
	integration, err := lookupIntegration(integrationName)
	if err != nil {
		return err
	}

	// Validate the integration can be set up
	if err := integration.Validate(); err != nil {
		return fmt.Errorf("integration %q cannot be set up; %w", integrationName, err)
	}

	cmd.SilenceUsage = true
	return nil
}

func runSetup(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	integrationName := args[0]

	integration, err := lookupIntegration(integrationName)
	if err != nil {
		return err
	}

	// Check if already installed
	installed, err := integration.IsInstalled()
	if err != nil {
		return fmt.Errorf("failed to check installation status; %w", err)
	}

	if installed {
		fmt.Printf("Integration %q is already installed\n", integrationName)
		return nil
	}

	fmt.Printf("Setting up %s integration for %s...\n", formatIntegrationType(integration.Type()), integration.Harness())

	if err := integration.Setup(ctx); err != nil {
		return fmt.Errorf("setup failed; %w", err)
	}

	// Get status for config path
	status, _ := integration.Status()
	if status != nil && status.ConfigPath != "" {
		fmt.Printf("Integration %q installed successfully\n", integrationName)
		fmt.Printf("Configuration: %s\n", status.ConfigPath)
	} else {
		fmt.Printf("Integration %q installed successfully\n", integrationName)
	}

	return nil
}
