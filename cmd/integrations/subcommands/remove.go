package subcommands

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/spf13/cobra"
)

var RemoveCmd = &cobra.Command{
	Use:   "remove <integration-name>",
	Short: "Remove an integration",
	Long: "\nRemove integration configuration from an agent framework.\n\n" +
		"Removes hooks, tools, or other configuration entries that were added by the setup command.",
	Example: `  # Remove Claude Code integration
  memorizer integrations remove claude-code-hook`,
	Args:    cobra.ExactArgs(1),
	PreRunE: validateRemove,
	RunE:    runRemove,
}

func validateRemove(cmd *cobra.Command, args []string) error {
	integrationName := args[0]

	// Validate integration exists
	registry := integrations.GlobalRegistry()
	if _, err := registry.Get(integrationName); err != nil {
		return fmt.Errorf("integration %q not found; %w\n\nRun 'memorizer integrations list' to see available integrations", integrationName, err)
	}

	// All validation passed - errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runRemove(cmd *cobra.Command, args []string) error {
	integrationName := args[0]

	registry := integrations.GlobalRegistry()
	integration, err := registry.Get(integrationName)
	if err != nil {
		return fmt.Errorf("integration %q not found; %w", integrationName, err)
	}

	// Check if configured
	enabled, err := integration.IsEnabled()
	if err != nil {
		return fmt.Errorf("failed to check integration status; %w", err)
	}
	if !enabled {
		fmt.Printf("%s is not currently configured\n", integration.GetName())
		return nil
	}

	// Remove integration
	fmt.Printf("Removing %s integration...\n", integration.GetName())

	if err := integration.Remove(); err != nil {
		return fmt.Errorf("failed to remove %s; %w", integrationName, err)
	}

	// Update config to remove this integration from enabled list
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config; %w", err)
	}

	// Remove integration from enabled list
	newEnabled := []string{}
	for _, name := range cfg.Integrations.Enabled {
		if name != integrationName {
			newEnabled = append(newEnabled, name)
		}
	}
	cfg.Integrations.Enabled = newEnabled
	configPath := config.GetConfigPath()
	if err := config.WriteConfig(configPath, cfg); err != nil {
		return fmt.Errorf("failed to update config after removing integration; %w", err)
	}

	fmt.Printf("✓ %s integration removed successfully\n", integration.GetName())

	return nil
}
