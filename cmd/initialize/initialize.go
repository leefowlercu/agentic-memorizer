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

	// Unattended mode configuration flags
	initializeGraphHost          string
	initializeGraphPort          int
	initializeSemanticProvider   string
	initializeSemanticModel      string
	initializeSemanticAPIKey     string
	initializeNoEmbeddings       bool
	initializeEmbeddingsProvider string
	initializeEmbeddingsModel    string
	initializeEmbeddingsAPIKey   string
	initializeHTTPPort           int
	initializeOutput             string
)

// InitializeCmd is the initialize command for first-time setup.
var InitializeCmd = &cobra.Command{
	Use:   "initialize",
	Short: "Run the interactive setup wizard",
	Long: "Run the interactive setup wizard to configure memorizer.\n\n" +
		"This command launches a terminal-based wizard that guides you through the initial " +
		"configuration, including FalkorDB setup, LLM provider selection, and daemon settings. " +
		"The configuration is saved to the default config file location.\n\n" +
		"In unattended mode (--unattended), configuration values are resolved in priority order:\n" +
		"1. CLI flags (highest priority)\n" +
		"2. MEMORIZER_* environment variables\n" +
		"3. Provider-native environment variables (for API keys)\n" +
		"4. Default values (lowest priority)",
	Example: `  # Run the interactive setup wizard
  memorizer initialize

  # Run in unattended mode with defaults
  memorizer initialize --unattended

  # Unattended mode with custom FalkorDB and provider
  memorizer initialize --unattended \
      --graph-host=redis.example.com \
      --graph-port=6380 \
      --semantic-provider=openai

  # Unattended mode with embeddings disabled
  memorizer initialize --unattended --no-embeddings

  # Full unattended configuration with JSON output
  memorizer initialize --unattended --force \
      --graph-host=falkordb.local \
      --semantic-provider=google \
      --semantic-api-key="$GOOGLE_API_KEY" \
      --embeddings-provider=voyage \
      --embeddings-api-key="$VOYAGE_API_KEY" \
      --http-port=8080 \
      --output=json

  # Using environment variables for unattended mode
  MEMORIZER_GRAPH_HOST=falkordb.local \
  MEMORIZER_SEMANTIC_PROVIDER=openai \
  OPENAI_API_KEY=sk-... \
  memorizer initialize --unattended

  # Force re-initialization even if config exists
  memorizer initialize --force`,
	PreRunE: validateInitialize,
	RunE:    runInitialize,
}

func init() {
	// Mode flags
	InitializeCmd.Flags().BoolVar(&initializeUnattended, "unattended", false,
		"Run in non-interactive mode using environment variables and defaults")
	InitializeCmd.Flags().BoolVar(&initializeForce, "force", false,
		"Force re-initialization even if configuration already exists")

	// FalkorDB configuration flags (unattended mode)
	InitializeCmd.Flags().StringVar(&initializeGraphHost, "graph-host", "",
		"FalkorDB server hostname (default: localhost)")
	InitializeCmd.Flags().IntVar(&initializeGraphPort, "graph-port", 0,
		"FalkorDB server port (default: 6379)")

	// Semantic provider configuration flags (unattended mode)
	InitializeCmd.Flags().StringVar(&initializeSemanticProvider, "semantic-provider", "",
		"LLM provider: anthropic, openai, google (default: anthropic)")
	InitializeCmd.Flags().StringVar(&initializeSemanticModel, "semantic-model", "",
		"Model identifier (default: provider's default model)")
	InitializeCmd.Flags().StringVar(&initializeSemanticAPIKey, "semantic-api-key", "",
		"API key for semantic provider")

	// Embeddings configuration flags (unattended mode)
	InitializeCmd.Flags().BoolVar(&initializeNoEmbeddings, "no-embeddings", false,
		"Disable vector embeddings")
	InitializeCmd.Flags().StringVar(&initializeEmbeddingsProvider, "embeddings-provider", "",
		"Embeddings provider: openai, voyage, google (default: openai)")
	InitializeCmd.Flags().StringVar(&initializeEmbeddingsModel, "embeddings-model", "",
		"Embeddings model (default: provider's default model)")
	InitializeCmd.Flags().StringVar(&initializeEmbeddingsAPIKey, "embeddings-api-key", "",
		"API key for embeddings provider")

	// Daemon configuration flags (unattended mode)
	InitializeCmd.Flags().IntVar(&initializeHTTPPort, "http-port", 0,
		"HTTP API port (default: 7600)")

	// Output control flags (unattended mode)
	InitializeCmd.Flags().StringVar(&initializeOutput, "output", "text",
		"Output format: text, json (default: text)")
}

func validateInitialize(cmd *cobra.Command, args []string) error {
	configPath := config.GetConfigPath()
	slog.Debug("validating initialize command", "config_path", configPath, "force", initializeForce)

	// Check if config already exists
	if !initializeForce && config.ConfigExists() {
		slog.Info("configuration already exists", "path", configPath)
		return fmt.Errorf("configuration already exists at %s; use --force to reinitialize", configPath)
	}

	// Validate unattended mode flags before silencing usage
	if initializeUnattended {
		if err := validateUnattendedFlags(cmd); err != nil {
			return err
		}
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
		return runUnattended(cmd, cfg)
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

func runUnattended(cmd *cobra.Command, cfg *config.Config) error {
	slog.Debug("starting unattended initialization")

	// Resolve configuration values from flags, env vars, and defaults
	resolved := resolveUnattendedConfig(cmd)

	// FR-006: Validate required API keys are present
	if err := validateRequiredAPIKeys(resolved); err != nil {
		slog.Error("validation failed", "error", err)
		return err
	}

	// Apply resolved values to config
	applyResolvedConfig(cfg, resolved)

	slog.Info("unattended initialization completed, writing configuration")

	// Write configuration
	if err := writeConfig(cfg); err != nil {
		return err
	}

	// Output result based on format
	if initializeOutput == "json" {
		jsonOutput, err := formatJSONOutput(resolved)
		if err != nil {
			return err
		}
		fmt.Println(jsonOutput)
		return nil
	}

	fmt.Print(formatTextOutput(resolved))
	return nil
}

// applyResolvedConfig applies the resolved unattended configuration to the config struct.
func applyResolvedConfig(cfg *config.Config, resolved *UnattendedConfig) {
	// Graph configuration
	cfg.Graph.Host = resolved.GraphHost
	cfg.Graph.Port = resolved.GraphPort

	// Semantic configuration
	cfg.Semantic.Provider = resolved.SemanticProvider
	cfg.Semantic.Model = resolved.SemanticModel
	if resolved.SemanticAPIKey != "" {
		cfg.Semantic.APIKey = &resolved.SemanticAPIKey
	}

	// Embeddings configuration
	cfg.Embeddings.Enabled = resolved.EmbeddingsEnabled
	if resolved.EmbeddingsEnabled {
		cfg.Embeddings.Provider = resolved.EmbeddingsProvider
		cfg.Embeddings.Model = resolved.EmbeddingsModel
		cfg.Embeddings.Dimensions = resolved.EmbeddingsDimensions
		if resolved.EmbeddingsAPIKey != "" {
			cfg.Embeddings.APIKey = &resolved.EmbeddingsAPIKey
		}
	}

	// Daemon configuration
	cfg.Daemon.HTTPPort = resolved.HTTPPort
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

