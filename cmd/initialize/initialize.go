package initialize

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/docker"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/adapters/claude" // Register Claude adapter
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/adapters/codex"  // Register Codex adapter
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/adapters/gemini" // Register Gemini adapter
	tuiinit "github.com/leefowlercu/agentic-memorizer/internal/tui/initialize"
	"github.com/spf13/cobra"
)

var InitializeCmd = &cobra.Command{
	Use:   "initialize",
	Short: "Initialize configuration and memory directory",
	Long: "\nCreates default configuration file and memory directory.\n\n" +
		"The initialize command sets up the Agentic Memorizer by creating a default configuration " +
		"file and the memory directory where you'll store files for analysis and indexing.\n\n" +
		"By default, runs an interactive TUI wizard. Use --unattended for scripted setup.\n\n" +
		"After initialization, start the daemon manually with 'agentic-memorizer daemon start' " +
		"or set up as a system service for automatic management (recommended for production).\n\n" +
		"By default, configuration and data files are stored in ~/.agentic-memorizer/. " +
		"You can customize this location by setting the MEMORIZER_APP_DIR environment variable " +
		"before running initialize.",
	Example: `  # Interactive initialization (TUI wizard)
  agentic-memorizer initialize

  # Unattended initialization with required flags
  agentic-memorizer initialize --unattended \
    --use-env-anthropic-api-key \
    --start-falkordb \
    --integrations claude-code-hook,claude-code-mcp

  # Unattended with explicit API keys
  agentic-memorizer initialize --unattended \
    --anthropic-api-key sk-ant-... \
    --openai-api-key sk-... \
    --enable-embeddings \
    --graph-host localhost \
    --graph-port 6379

  # Force overwrite existing config
  agentic-memorizer initialize --force`,
	PreRunE: validateInit,
	RunE:    runInit,
}

func init() {
	// Directory options
	InitializeCmd.Flags().String("memory-root", config.DefaultConfig.MemoryRoot, "Memory directory")
	InitializeCmd.Flags().String("cache-dir", "", "Cache directory (default: <memory-root>/.cache)")
	InitializeCmd.Flags().Bool("force", false, "Overwrite existing config")

	// Mode selection
	InitializeCmd.Flags().Bool("unattended", false, "Run in unattended mode without interactive prompts")

	// Claude API configuration
	InitializeCmd.Flags().String("anthropic-api-key", "", "Anthropic API key value")
	InitializeCmd.Flags().Bool("use-env-anthropic-api-key", false, "Use ANTHROPIC_API_KEY from environment")

	// HTTP API configuration
	InitializeCmd.Flags().Int("http-port", -1, "HTTP API port (0 to disable, -1 for wizard default)")

	// FalkorDB configuration
	InitializeCmd.Flags().String("graph-host", config.DefaultConfig.Graph.Host, "FalkorDB host")
	InitializeCmd.Flags().Int("graph-port", config.DefaultConfig.Graph.Port, "FalkorDB port")
	InitializeCmd.Flags().String("graph-password", "", "FalkorDB password")
	InitializeCmd.Flags().Bool("start-falkordb", false, "Start FalkorDB in Docker")
	InitializeCmd.Flags().Bool("skip-falkordb-check", false, "Skip FalkorDB connectivity verification")

	// Embeddings configuration
	InitializeCmd.Flags().Bool("enable-embeddings", false, "Enable embeddings")
	InitializeCmd.Flags().Bool("disable-embeddings", false, "Disable embeddings (default)")
	InitializeCmd.Flags().String("openai-api-key", "", "OpenAI API key for embeddings")
	InitializeCmd.Flags().Bool("use-env-openai-api-key", false, "Use OPENAI_API_KEY from environment")

	// Integration configuration
	InitializeCmd.Flags().StringSlice("integrations", []string{}, "Integrations to setup (comma-separated)")

	// Deprecated flags (kept for backward compatibility)
	InitializeCmd.Flags().Bool("setup-integrations", false, "Deprecated: use --integrations instead")
	InitializeCmd.Flags().Bool("skip-integrations", false, "Deprecated: omit --integrations instead")
	InitializeCmd.Flags().MarkDeprecated("setup-integrations", "use --integrations flag instead")
	InitializeCmd.Flags().MarkDeprecated("skip-integrations", "simply omit --integrations flag")

	InitializeCmd.Flags().SortFlags = false
}

func validateInit(cmd *cobra.Command, args []string) error {
	unattended, _ := cmd.Flags().GetBool("unattended")

	// Validate mutually exclusive flags
	enableEmbeddings, _ := cmd.Flags().GetBool("enable-embeddings")
	disableEmbeddings, _ := cmd.Flags().GetBool("disable-embeddings")
	if enableEmbeddings && disableEmbeddings {
		return fmt.Errorf("--enable-embeddings and --disable-embeddings are mutually exclusive")
	}

	anthropicKey, _ := cmd.Flags().GetString("anthropic-api-key")
	useEnvAnthropic, _ := cmd.Flags().GetBool("use-env-anthropic-api-key")
	if anthropicKey != "" && useEnvAnthropic {
		return fmt.Errorf("--anthropic-api-key and --use-env-anthropic-api-key are mutually exclusive")
	}

	openaiKey, _ := cmd.Flags().GetString("openai-api-key")
	useEnvOpenai, _ := cmd.Flags().GetBool("use-env-openai-api-key")
	if openaiKey != "" && useEnvOpenai {
		return fmt.Errorf("--openai-api-key and --use-env-openai-api-key are mutually exclusive")
	}

	// Validate http-port flag if provided
	httpPort, _ := cmd.Flags().GetInt("http-port")
	if httpPort < -1 || httpPort > 65535 {
		return fmt.Errorf("--http-port must be -1 (default), 0 (disabled), or 1-65535")
	}

	// Unattended mode validation
	if unattended {
		// Must have either API key or use-env flag
		if anthropicKey == "" && !useEnvAnthropic {
			envKey := os.Getenv(config.ClaudeAPIKeyEnv)
			if envKey == "" {
				return fmt.Errorf("unattended mode requires --anthropic-api-key or --use-env-anthropic-api-key (or %s environment variable)", config.ClaudeAPIKeyEnv)
			}
		}

		// FalkorDB must be addressed
		startFalkorDB, _ := cmd.Flags().GetBool("start-falkordb")
		skipCheck, _ := cmd.Flags().GetBool("skip-falkordb-check")
		if !startFalkorDB && !skipCheck {
			// Check if FalkorDB is already running
			graphPort, _ := cmd.Flags().GetInt("graph-port")
			if !docker.IsFalkorDBRunning(graphPort) {
				return fmt.Errorf("unattended mode requires FalkorDB to be running, use --start-falkordb to auto-start, or --skip-falkordb-check to bypass")
			}
		}

		// Embeddings validation
		if enableEmbeddings {
			if openaiKey == "" && !useEnvOpenai {
				envKey := os.Getenv(config.EmbeddingsAPIKeyEnv)
				if envKey == "" {
					return fmt.Errorf("--enable-embeddings requires --openai-api-key or --use-env-openai-api-key (or %s environment variable)", config.EmbeddingsAPIKeyEnv)
				}
			}
		}
	}

	// All validation passed - errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runInit(cmd *cobra.Command, args []string) error {
	unattended, _ := cmd.Flags().GetBool("unattended")

	if unattended {
		return runUnattended(cmd)
	}
	return runInteractive(cmd)
}

func runInteractive(cmd *cobra.Command) error {
	force, _ := cmd.Flags().GetBool("force")
	memoryRoot, _ := cmd.Flags().GetString("memory-root")
	cacheDir, _ := cmd.Flags().GetString("cache-dir")

	// Get app directory
	appDir, err := config.GetAppDir()
	if err != nil {
		return fmt.Errorf("failed to get app directory; %w", err)
	}
	configPath := filepath.Join(appDir, config.ConfigFile)

	// Check for existing config
	if !force {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("config file already exists at %s (use --force to overwrite)", configPath)
		}
	}

	// Build initial config from defaults and flags
	cfg := config.DefaultConfig
	if memoryRoot != "" {
		cfg.MemoryRoot = config.ExpandHome(memoryRoot)
	} else {
		cfg.MemoryRoot = config.ExpandHome(cfg.MemoryRoot)
	}
	if cacheDir != "" {
		cfg.Analysis.CacheDir = config.ExpandHome(cacheDir)
	} else {
		cfg.Analysis.CacheDir = config.ExpandHome(cfg.Analysis.CacheDir)
	}

	// Run the TUI wizard
	result, err := tuiinit.RunWizard(&cfg)
	if err != nil {
		return fmt.Errorf("initialization wizard error; %w", err)
	}

	if result.Cancelled {
		fmt.Println("Initialization cancelled.")
		return nil
	}

	if !result.Confirmed {
		fmt.Println("Initialization not confirmed.")
		return nil
	}

	// Finalize configuration
	return finalizeInit(configPath, result.Config)
}

func runUnattended(cmd *cobra.Command) error {
	force, _ := cmd.Flags().GetBool("force")
	memoryRoot, _ := cmd.Flags().GetString("memory-root")
	cacheDir, _ := cmd.Flags().GetString("cache-dir")

	// Get app directory
	appDir, err := config.GetAppDir()
	if err != nil {
		return fmt.Errorf("failed to get app directory; %w", err)
	}
	configPath := filepath.Join(appDir, config.ConfigFile)

	// Check for existing config
	if !force {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("config file already exists at %s (use --force to overwrite)", configPath)
		}
	}

	// Build config from flags
	cfg := config.DefaultConfig

	// Memory root
	if memoryRoot != "" {
		cfg.MemoryRoot = config.ExpandHome(memoryRoot)
	} else {
		cfg.MemoryRoot = config.ExpandHome(cfg.MemoryRoot)
	}

	// Cache dir
	if cacheDir != "" {
		cfg.Analysis.CacheDir = config.ExpandHome(cacheDir)
	} else {
		cfg.Analysis.CacheDir = config.ExpandHome(cfg.Analysis.CacheDir)
	}

	// Anthropic API key
	anthropicKey, _ := cmd.Flags().GetString("anthropic-api-key")
	useEnvAnthropic, _ := cmd.Flags().GetBool("use-env-anthropic-api-key")
	if anthropicKey != "" {
		cfg.Claude.APIKey = anthropicKey
	} else if useEnvAnthropic || os.Getenv(config.ClaudeAPIKeyEnv) != "" {
		cfg.Claude.APIKey = "" // Will use env var
	}

	// HTTP port
	httpPort, _ := cmd.Flags().GetInt("http-port")
	if httpPort >= 0 {
		cfg.Daemon.HTTPPort = httpPort
		if httpPort > 0 {
			cfg.MCP.DaemonPort = httpPort
			cfg.MCP.DaemonHost = "localhost"
		}
	}

	// FalkorDB
	graphHost, _ := cmd.Flags().GetString("graph-host")
	graphPort, _ := cmd.Flags().GetInt("graph-port")
	graphPassword, _ := cmd.Flags().GetString("graph-password")
	cfg.Graph.Host = graphHost
	cfg.Graph.Port = graphPort
	cfg.Graph.Password = graphPassword

	// Start FalkorDB if requested
	startFalkorDBFlag, _ := cmd.Flags().GetBool("start-falkordb")
	if startFalkorDBFlag {
		fmt.Println("Starting FalkorDB in Docker...")
		opts := docker.StartOptions{
			Port:    graphPort,
			DataDir: fmt.Sprintf("%s/falkordb", appDir),
			Detach:  true,
		}
		if err := docker.StartFalkorDB(opts); err != nil {
			return fmt.Errorf("failed to start FalkorDB; %w", err)
		}
		fmt.Println("FalkorDB started successfully.")
		fmt.Printf("  Redis port: %d\n", graphPort)
		fmt.Printf("  Browser UI: http://localhost:3000\n")
	}

	// Embeddings
	enableEmbeddings, _ := cmd.Flags().GetBool("enable-embeddings")
	openaiKey, _ := cmd.Flags().GetString("openai-api-key")
	useEnvOpenai, _ := cmd.Flags().GetBool("use-env-openai-api-key")
	if enableEmbeddings {
		cfg.Embeddings.Enabled = true
		if openaiKey != "" {
			cfg.Embeddings.APIKey = openaiKey
		} else if useEnvOpenai || os.Getenv(config.EmbeddingsAPIKeyEnv) != "" {
			cfg.Embeddings.APIKey = "" // Will use env var
		}
	}

	// Integrations
	integrationNames, _ := cmd.Flags().GetStringSlice("integrations")
	cfg.Integrations.Enabled = integrationNames

	// Finalize configuration
	return finalizeInit(configPath, &cfg)
}

func finalizeInit(configPath string, cfg *config.Config) error {
	// Create directories
	if err := os.MkdirAll(cfg.MemoryRoot, 0755); err != nil {
		return fmt.Errorf("failed to create memory directory; %w", err)
	}

	if err := os.MkdirAll(cfg.Analysis.CacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory; %w", err)
	}

	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory; %w", err)
	}

	// Write config
	if err := config.WriteMinimalConfig(configPath, cfg); err != nil {
		return fmt.Errorf("failed to write config; %w", err)
	}

	// Initialize config system
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config; %w", err)
	}

	// Print summary
	fmt.Printf("\nConfiguration:\n")
	fmt.Printf("  Created configuration file: %s\n", configPath)
	fmt.Printf("  Created memory directory: %s\n", cfg.MemoryRoot)
	fmt.Printf("  Created cache directory: %s\n", cfg.Analysis.CacheDir)
	fmt.Printf("  FalkorDB: %s:%d (database: %s)\n", cfg.Graph.Host, cfg.Graph.Port, config.GraphDatabase)
	if cfg.Embeddings.Enabled {
		fmt.Printf("  Embeddings: %s (%s)\n", config.EmbeddingsProvider, config.EmbeddingsModel)
	} else {
		fmt.Printf("  Embeddings: disabled\n")
	}

	// Setup integrations
	if len(cfg.Integrations.Enabled) > 0 {
		enabledIntegrations, err := setupIntegrations(cfg.Integrations.Enabled)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n\n", err)
		}

		// Update config with successfully enabled integrations
		if len(enabledIntegrations) > 0 {
			cfg.Integrations.Enabled = enabledIntegrations
			if err := config.WriteMinimalConfig(configPath, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to update config with enabled integrations; %v\n", err)
			}
			if err := config.InitConfig(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to reload config; %v\n", err)
			}
		}
	}

	// Print next steps
	printNextSteps(cfg)

	return nil
}

func setupIntegrations(integrationNames []string) ([]string, error) {
	if len(integrationNames) == 0 {
		return nil, nil
	}

	binaryPath, err := findBinaryPath()
	if err != nil {
		return nil, fmt.Errorf("could not auto-detect binary path; %w\nPlease manually configure integrations", err)
	}

	fmt.Printf("\nConfigured integrations:\n")
	var enabledIntegrations []string
	registry := integrations.GlobalRegistry()

	for _, name := range integrationNames {
		integration, err := registry.Get(name)
		if err != nil {
			fmt.Printf("  Warning: Integration %s not found\n", name)
			continue
		}

		if err := integration.Setup(binaryPath); err != nil {
			fmt.Printf("  Warning: Failed to setup %s: %v\n", name, err)
			continue
		}

		fmt.Printf("  Integration %s configured\n", name)
		enabledIntegrations = append(enabledIntegrations, name)
	}

	if len(enabledIntegrations) > 0 {
		fmt.Printf("\nBinary path: %s\n", binaryPath)
	}

	return enabledIntegrations, nil
}

func printNextSteps(cfg *config.Config) {
	apiKeyConfigured := cfg.Claude.APIKey != "" || os.Getenv(config.ClaudeAPIKeyEnv) != ""
	falkorDBRunning := docker.IsFalkorDBRunning(cfg.Graph.Port)

	fmt.Printf("\nNext steps:\n")
	stepNum := 1

	if !apiKeyConfigured {
		fmt.Printf("%d. Set your Claude API key: export ANTHROPIC_API_KEY=\"your-key-here\"\n", stepNum)
		stepNum++
	}

	if cfg.Embeddings.Enabled && os.Getenv(config.EmbeddingsAPIKeyEnv) == "" && cfg.Embeddings.APIKey == "" {
		fmt.Printf("%d. Set your OpenAI API key: export OPENAI_API_KEY=\"your-key-here\"\n", stepNum)
		stepNum++
	}

	if !falkorDBRunning {
		fmt.Printf("%d. Start FalkorDB (required):\n", stepNum)
		fmt.Printf("   docker run -d --name memorizer-falkordb -p %d:6379 falkordb/falkordb\n", cfg.Graph.Port)
		stepNum++
	}

	fmt.Printf("%d. Add files to %s\n", stepNum, cfg.MemoryRoot)
	stepNum++

	fmt.Printf("%d. Start the daemon:\n", stepNum)
	fmt.Printf("   # Option A: Manual (foreground)\n")
	fmt.Printf("   agentic-memorizer daemon start\n\n")
	fmt.Printf("   # Option B: Manual (background)\n")
	fmt.Printf("   nohup agentic-memorizer daemon start &\n\n")
	fmt.Printf("   # Option C: System service (background, recommended)\n")
	fmt.Printf("   agentic-memorizer daemon systemctl  # Linux\n")
	fmt.Printf("   agentic-memorizer daemon launchctl  # macOS\n")
}

// Helper functions

func findBinaryPath() (string, error) {
	// Try to get the current executable path
	execPath, err := os.Executable()
	if err == nil {
		if filepath.Base(execPath) == "agentic-memorizer" {
			return execPath, nil
		}
	}

	// Try common installation paths
	home, err := os.UserHomeDir()
	if err == nil {
		commonPath := filepath.Join(home, ".local", "bin", "agentic-memorizer")
		if _, err := os.Stat(commonPath); err == nil {
			return commonPath, nil
		}
	}

	// Try PATH
	pathBinary, err := exec.LookPath("agentic-memorizer")
	if err == nil {
		return pathBinary, nil
	}

	return "", fmt.Errorf("could not locate agentic-memorizer binary")
}
