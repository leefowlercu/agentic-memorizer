// Package initialize implements the initialize command for first-time setup.
package initialize

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize/steps"
)

// Flag variables for the initialize command.
var (
	initializeUnattended bool
	initializeForce      bool
)

// InitializeCmd is the initialize command for first-time setup.
var InitializeCmd = &cobra.Command{
	Use:   "initialize",
	Short: "Run the interactive setup wizard",
	Long: "Run the interactive setup wizard to configure memorizer.\n\n" +
		"This command launches a terminal-based wizard that guides you through the initial " +
		"configuration, including FalkorDB setup, LLM provider selection, and daemon settings. " +
		"The configuration is saved to the default config file location.",
	Example: `  # Run the interactive setup wizard
  memorizer initialize

  # Run in unattended mode with environment-based configuration
  memorizer initialize --unattended

  # Force re-initialization even if config exists
  memorizer initialize --force`,
	PreRunE: validateInitialize,
	RunE:    runInitialize,
}

func init() {
	InitializeCmd.Flags().BoolVar(&initializeUnattended, "unattended", false,
		"Run in non-interactive mode using environment variables and defaults")
	InitializeCmd.Flags().BoolVar(&initializeForce, "force", false,
		"Force re-initialization even if configuration already exists")
}

func validateInitialize(cmd *cobra.Command, args []string) error {
	configPath := config.GetConfigPath()
	slog.Debug("validating initialize command", "config_path", configPath, "force", initializeForce)

	// Check if config already exists
	if !initializeForce && config.ConfigExists() {
		slog.Info("configuration already exists", "path", configPath)
		return fmt.Errorf("configuration already exists at %s; use --force to reinitialize", configPath)
	}

	cmd.SilenceUsage = true
	return nil
}

func runInitialize(cmd *cobra.Command, args []string) error {
	slog.Info("starting initialization wizard", "unattended", initializeUnattended)

	// Create config directory if needed
	if err := config.EnsureConfigDir(); err != nil {
		slog.Error("failed to create config directory", "error", err)
		return fmt.Errorf("failed to create config directory; %w", err)
	}

	// Start with default configuration
	defaultCfg := config.NewDefaultConfig()
	cfg := &defaultCfg

	// Define wizard steps
	stepList := []initialize.Step{
		steps.NewFalkorDBStep(),
		steps.NewSemanticProviderStep(),
		steps.NewEmbeddingsStep(),
		steps.NewHTTPPortStep(),
		steps.NewConfirmStep(),
	}
	slog.Debug("wizard steps initialized", "step_count", len(stepList))

	if initializeUnattended {
		return runUnattended(cfg, stepList)
	}

	return runInteractive(cfg, stepList)
}

func runInteractive(cfg *config.Config, stepList []initialize.Step) error {
	slog.Debug("starting interactive wizard")
	result, err := initialize.RunWizard(cfg, stepList)
	if err != nil {
		slog.Error("wizard failed", "error", err)
		return fmt.Errorf("wizard failed; %w", err)
	}

	if result.Cancelled {
		slog.Info("wizard cancelled by user")
		return nil
	}

	if result.Err != nil {
		slog.Error("wizard completed with error", "error", result.Err)
		return result.Err
	}

	if !result.Confirmed {
		slog.Info("wizard completed without confirmation")
		return nil
	}

	slog.Info("wizard completed successfully, writing configuration")

	// Write configuration
	if err := writeConfig(result.Config); err != nil {
		return err
	}

	fmt.Println("Configuration saved successfully.")
	fmt.Println("\nTo start the daemon, run:")
	fmt.Println("  memorizer daemon start")

	return nil
}

func runUnattended(cfg *config.Config, stepList []initialize.Step) error {
	slog.Debug("starting unattended initialization")

	// Initialize each step with defaults and apply
	for _, step := range stepList {
		slog.Debug("processing step", "step", step.Title())
		step.Init(cfg)

		if err := step.Validate(); err != nil {
			slog.Error("step validation failed", "step", step.Title(), "error", err)
			return fmt.Errorf("validation failed for %s; %w", step.Title(), err)
		}

		if err := step.Apply(cfg); err != nil {
			slog.Error("step apply failed", "step", step.Title(), "error", err)
			return fmt.Errorf("configuration failed for %s; %w", step.Title(), err)
		}
		slog.Debug("step completed", "step", step.Title())
	}

	slog.Info("unattended initialization completed, writing configuration")

	// Write configuration
	if err := writeConfig(cfg); err != nil {
		return err
	}

	fmt.Println("Configuration saved successfully (unattended mode).")
	return nil
}

func writeConfig(cfg *config.Config) error {
	configPath := config.GetConfigPath()
	slog.Debug("writing configuration file", "path", configPath)

	if err := config.Write(cfg, configPath); err != nil {
		slog.Error("failed to write config", "path", configPath, "error", err)
		return fmt.Errorf("failed to write config; %w", err)
	}

	slog.Info("configuration written successfully", "path", configPath)
	return nil
}
