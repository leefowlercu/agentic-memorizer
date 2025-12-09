package subcommands

import (
	"fmt"
	"os"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/spf13/cobra"
)

var SetupCmd = &cobra.Command{
	Use:   "setup <integration-name>",
	Short: "Setup a specific integration",
	Long: "\nSetup integration with a specific agent framework.\n\n" +
		"Configures the framework to use memorizer for memory indexing. The setup process " +
		"varies by framework but typically involves adding hooks or tools to the framework's " +
		"configuration files.",
	Example: `  # Setup Claude Code integration
  memorizer integrations setup claude-code-hook

  # Setup with custom binary path
  memorizer integrations setup claude-code-hook --binary-path /custom/path/memorizer`,
	Args:    cobra.ExactArgs(1),
	PreRunE: validateSetup,
	RunE:    runSetup,
}

func init() {
	SetupCmd.Flags().String("binary-path", "", "Custom path to memorizer binary (auto-detected if not specified)")
}

func validateSetup(cmd *cobra.Command, args []string) error {
	integrationName := args[0]

	// Validate integration exists
	registry := integrations.GlobalRegistry()
	if _, err := registry.Get(integrationName); err != nil {
		return fmt.Errorf("integration %q not found; %w\n\nRun 'memorizer integrations list' to see available integrations", integrationName, err)
	}

	// Validate binary path if provided
	binaryPath, _ := cmd.Flags().GetString("binary-path")
	if binaryPath != "" {
		if _, err := os.Stat(binaryPath); err != nil {
			return fmt.Errorf("binary-path %q is not accessible; %w", binaryPath, err)
		}
		// Check if executable
		info, _ := os.Stat(binaryPath)
		if info.Mode()&0111 == 0 {
			return fmt.Errorf("binary-path %q is not executable", binaryPath)
		}
	}

	// All validation passed - errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runSetup(cmd *cobra.Command, args []string) error {
	integrationName := args[0]
	binaryPath, _ := cmd.Flags().GetString("binary-path")

	// Find binary path if not specified
	if binaryPath == "" {
		path, err := FindBinaryPath()
		if err != nil {
			return fmt.Errorf("could not auto-detect binary path; %w\nPlease specify with --binary-path flag", err)
		}
		binaryPath = path
	}

	registry := integrations.GlobalRegistry()
	integration, err := registry.Get(integrationName)
	if err != nil {
		return fmt.Errorf("integration %q not found; %w\n\nRun 'memorizer integrations list' to see available integrations", integrationName, err)
	}

	// Check if framework is installed (skip for generic adapters)
	detected, err := integration.Detect()
	if err != nil {
		return fmt.Errorf("failed to detect %s; %w", integrationName, err)
	}
	if !detected {
		// Try to setup anyway - generic adapters will provide helpful manual instructions
		fmt.Printf("Warning: %s does not appear to be installed (auto-detection may not work for all frameworks)\n", integration.GetName())
		fmt.Printf("Attempting setup anyway...\n\n")
	}

	// Check if already configured
	enabled, _ := integration.IsEnabled()
	if enabled {
		fmt.Printf("%s is already configured.\n", integration.GetName())
		fmt.Printf("To reconfigure, first remove the integration:\n")
		fmt.Printf("  memorizer integrations remove %s\n", integrationName)
		fmt.Printf("Then setup again:\n")
		fmt.Printf("  memorizer integrations setup %s\n", integrationName)
		return nil
	}

	// Setup integration
	fmt.Printf("Setting up %s integration...\n", integration.GetName())
	fmt.Printf("Binary path: %s\n", binaryPath)
	fmt.Println()

	if err := integration.Setup(binaryPath); err != nil {
		return fmt.Errorf("failed to setup %s; %w", integrationName, err)
	}

	// Update config to track this integration as enabled
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config; %w", err)
	}

	// Add integration to enabled list if not already present
	integrationExists := false
	for _, name := range cfg.Integrations.Enabled {
		if name == integrationName {
			integrationExists = true
			break
		}
	}
	if !integrationExists {
		cfg.Integrations.Enabled = append(cfg.Integrations.Enabled, integrationName)
		configPath := config.GetConfigPath()
		if err := config.WriteConfig(configPath, cfg); err != nil {
			return fmt.Errorf("failed to update config with enabled integration; %w", err)
		}
	}

	fmt.Printf("✓ %s integration configured successfully\n", integration.GetName())
	fmt.Println()
	fmt.Printf("The integration will be active on your next %s session.\n", integration.GetName())

	return nil
}
