package subcommands

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/spf13/cobra"
)

var ValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	Long: "\nValidate the configuration file for errors.\n\n" +
		"Performs comprehensive validation including:\n" +
		"- Required fields are present\n" +
		"- Values are within valid ranges\n" +
		"- Paths are safe and accessible\n" +
		"- Enums have valid values\n" +
		"- Cross-field dependencies are satisfied",
	Example: `  # Validate current configuration
  agentic-memorizer config validate

  # Validate specific config file
  agentic-memorizer config validate --config /path/to/config.yaml`,
	PreRunE: validateConfigValidate,
	RunE:    runValidate,
}

func validateConfigValidate(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runValidate(cmd *cobra.Command, args []string) error {
	// Initialize and load config
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to load config; %w", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get config; %w", err)
	}

	// Validate configuration
	if err := config.ValidateConfig(cfg); err != nil {
		fmt.Println("❌ Configuration validation failed:")
		fmt.Println()
		fmt.Println(err.Error())
		return fmt.Errorf("validation failed")
	}

	fmt.Println("✓ Configuration is valid")
	fmt.Println()
	fmt.Printf("Configuration file: %s\n", config.GetConfigPath())
	fmt.Printf("Memory root: %s\n", cfg.MemoryRoot)
	fmt.Printf("Cache directory: %s\n", cfg.Analysis.CacheDir)
	fmt.Printf("Output format: %s\n", cfg.Output.Format)
	fmt.Printf("Daemon enabled: %v\n", cfg.Daemon.Enabled)

	return nil
}
