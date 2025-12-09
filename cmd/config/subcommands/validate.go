package subcommands

import (
	"encoding/json"
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var validateFormat string

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
  memorizer config validate

  # Validate and show full config in YAML
  memorizer config validate --format yaml

  # Validate and show full config in JSON
  memorizer config validate --format json`,
	PreRunE: validateValidate,
	RunE:    runValidate,
}

func init() {
	ValidateCmd.Flags().StringVar(&validateFormat, "format", "",
		"Output format for full configuration (yaml, json)")
}

func validateValidate(cmd *cobra.Command, args []string) error {
	// Validate format if specified
	if validateFormat != "" {
		validFormats := []string{"yaml", "json"}
		found := false
		for _, f := range validFormats {
			if validateFormat == f {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid format %q; valid formats are: yaml, json", validateFormat)
		}
	}

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
		// Format validation error using format package
		status := format.NewStatus(format.StatusError, "Configuration validation failed")
		status.AddDetail(err.Error())

		formatter, fmtErr := format.GetFormatter("text")
		if fmtErr != nil {
			return fmt.Errorf("failed to get formatter; %w", fmtErr)
		}
		output, fmtErr := formatter.Format(status)
		if fmtErr != nil {
			return fmt.Errorf("failed to format output; %w", fmtErr)
		}
		fmt.Println(output)
		return fmt.Errorf("validation failed")
	}

	// Format success message using format package
	status := format.NewStatus(format.StatusSuccess, "Configuration is valid")

	formatter, err := format.GetFormatter("text")
	if err != nil {
		return fmt.Errorf("failed to get formatter; %w", err)
	}
	output, err := formatter.Format(status)
	if err != nil {
		return fmt.Errorf("failed to format output; %w", err)
	}
	fmt.Println(output)

	// If format specified, print full config
	if validateFormat != "" {
		fmt.Println()
		switch validateFormat {
		case "yaml":
			return printConfigYAML(cfg)
		case "json":
			return printConfigJSON(cfg)
		}
	}

	return nil
}

func printConfigYAML(cfg *config.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML; %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func printConfigJSON(cfg *config.Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config to JSON; %w", err)
	}
	fmt.Println(string(data))
	return nil
}
