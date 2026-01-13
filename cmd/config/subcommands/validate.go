package subcommands

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/spf13/cobra"
)

// ValidateCmd validates the current configuration.
var ValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the current configuration",
	Long: "Validate the current configuration.\n\n" +
		"Checks the configuration file for syntax errors and validates that all " +
		"settings have valid values. Returns exit code 0 if valid, 1 if invalid.",
	Example: `  # Validate the configuration
  memorizer config validate`,
	PreRunE: validateValidate,
	RunE:    runValidate,
}

func validateValidate(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runValidate(cmd *cobra.Command, args []string) error {
	configPath := config.GetConfigPath()

	// Check if config file exists
	if !config.ConfigExists() {
		fmt.Printf("No configuration file found at %s\n", configPath)
		fmt.Println("Using default configuration values.")
		return nil
	}

	// Load the configuration (this also validates it)
	_, err := config.LoadFromPath(configPath)
	if err != nil {
		fmt.Println("Configuration validation failed:")
		fmt.Printf("  %v\n", err)
		return fmt.Errorf("configuration is invalid")
	}

	fmt.Printf("Configuration is valid: %s\n", configPath)
	return nil
}
