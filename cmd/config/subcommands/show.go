package subcommands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
)

var (
	showRaw bool
)

// ShowCmd displays the current configuration.
var ShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display the current configuration",
	Long: "Display the current configuration.\n\n" +
		"Shows the current memorizer configuration values. By default, shows " +
		"the effective configuration with defaults applied. Use --raw to show " +
		"only the values explicitly set in the config file.",
	Example: `  # Show effective configuration
  memorizer config show

  # Show only explicitly set values
  memorizer config show --raw`,
	PreRunE: validateShow,
	RunE:    runShow,
}

func init() {
	ShowCmd.Flags().BoolVar(&showRaw, "raw", false, "Show only explicitly configured values (no defaults)")
}

func validateShow(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runShow(cmd *cobra.Command, args []string) error {
	if showRaw {
		return showRawConfig()
	}
	return showEffectiveConfig()
}

func showRawConfig() error {
	// Read the config file directly
	configPath := config.GetConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("# No configuration file found")
			fmt.Printf("# Default location: %s\n", configPath)
			return nil
		}
		return fmt.Errorf("failed to read config file; %w", err)
	}

	fmt.Printf("# Configuration file: %s\n", configPath)
	fmt.Println(string(data))
	return nil
}

func showEffectiveConfig() error {
	// Get all configuration settings
	settings := config.GetAllSettings()

	// Convert to YAML
	data, err := yaml.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to format configuration; %w", err)
	}

	fmt.Println("# Effective configuration (with defaults)")
	fmt.Printf("# Config file: %s\n", config.GetConfigPath())
	fmt.Println(string(data))
	return nil
}
