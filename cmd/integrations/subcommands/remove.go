package subcommands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// RemoveCmd removes an integration.
var RemoveCmd = &cobra.Command{
	Use:   "remove <integration-name>",
	Short: "Remove an integration",
	Long: `Remove an integration.

Removes the memorizer configuration from the specified AI harness.
If a backup was created during setup, it will be restored.`,
	Example: `  # Remove Claude Code hook integration
  memorizer integrations remove claude-code-hook

  # Remove Gemini CLI MCP integration
  memorizer integrations remove gemini-cli-mcp`,
	Args:    cobra.ExactArgs(1),
	PreRunE: validateRemove,
	RunE:    runRemove,
}

func validateRemove(cmd *cobra.Command, args []string) error {
	integrationName := args[0]

	// Check if integration exists
	if _, err := lookupIntegration(integrationName); err != nil {
		return err
	}

	cmd.SilenceUsage = true
	return nil
}

func runRemove(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	integrationName := args[0]

	integration, err := lookupIntegration(integrationName)
	if err != nil {
		return err
	}

	// Check if installed
	installed, err := integration.IsInstalled()
	if err != nil {
		return fmt.Errorf("failed to check installation status; %w", err)
	}

	if !installed {
		fmt.Printf("Integration %q is not installed\n", integrationName)
		return nil
	}

	fmt.Printf("Removing %s integration from %s...\n", formatIntegrationType(integration.Type()), integration.Harness())

	if err := integration.Teardown(ctx); err != nil {
		return fmt.Errorf("removal failed; %w", err)
	}

	fmt.Printf("Integration %q removed successfully\n", integrationName)
	return nil
}
