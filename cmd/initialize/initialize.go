package initialize

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/container"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/adapters/claude" // Register Claude adapter
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/adapters/codex"  // Register Codex adapter
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/adapters/gemini" // Register Gemini adapter
	"github.com/leefowlercu/agentic-memorizer/internal/servicemanager"
	tuiinit "github.com/leefowlercu/agentic-memorizer/internal/tui/initialize"
	"github.com/spf13/cobra"
)

var (
	// Directory options
	initializeMemoryRoot string
	initializeCacheDir   string
	initializeForce      bool

	// Mode selection
	initializeUnattended bool

	// Semantic provider configuration
	initializeSemanticProvider     string
	initializeSemanticModel        string
	initializeSemanticAPIKey       string
	initializeUseEnvSemanticAPIKey bool
	initializeSkipSemantic         bool

	// HTTP API configuration
	initializeHTTPPort int

	// FalkorDB configuration
	initializeGraphHost           string
	initializeGraphPort           int
	initializeGraphDatabase       string
	initializeGraphPassword       string
	initializeStartFalkorDBDocker bool
	initializeStartFalkorDBPodman bool
	initializeSkipFalkorDBCheck   bool

	// Embeddings configuration
	initializeEnableEmbeddings   bool
	initializeDisableEmbeddings  bool
	initializeOpenAIAPIKey       string
	initializeUseEnvOpenAIAPIKey bool

	// Integration configuration
	initializeIntegrations []string
)

var InitializeCmd = &cobra.Command{
	Use:   "initialize",
	Short: "Initialize configuration and memory directory",
	Long: "\nCreates default configuration file and memory directory.\n\n" +
		"The initialize command sets up the Agentic Memorizer by creating a default configuration " +
		"file and the memory directory where you'll store files for analysis and indexing.\n\n" +
		"By default, runs an interactive TUI wizard. Use --unattended for scripted setup.\n\n" +
		"After initialization, start the daemon manually with 'memorizer daemon start' " +
		"or set up as a system service for automatic management (recommended for production).\n\n" +
		"By default, configuration and data files are stored in ~/.memorizer/. " +
		"You can customize this location by setting the MEMORIZER_APP_DIR environment variable " +
		"before running initialize.",
	Example: `  # Interactive initialization (TUI wizard)
  memorizer initialize

  # Unattended initialization with environment variable
  memorizer initialize --unattended \
    --use-env-semantic-api-key \
    --start-falkordb-docker \
    --integrations claude-code-hook,claude-code-mcp

  # Unattended with explicit API key and custom database
  memorizer initialize --unattended \
    --semantic-api-key sk-... \
    --graph-database my-project \
    --integrations claude-code-hook

  # Using Gemini as semantic provider
  memorizer initialize --unattended \
    --semantic-provider gemini \
    --use-env-semantic-api-key \
    --start-falkordb-podman

  # Force overwrite existing config
  memorizer initialize --force`,
	PreRunE: validateInit,
	RunE:    runInit,
}

func init() {
	// Directory options
	InitializeCmd.Flags().StringVar(&initializeMemoryRoot, "memory-root", config.DefaultConfig.Memory.Root, "Memory directory")
	InitializeCmd.Flags().StringVar(&initializeCacheDir, "cache-dir", "", "Cache directory (default: <memory-root>/.cache)")
	InitializeCmd.Flags().BoolVar(&initializeForce, "force", false, "Overwrite existing config")

	// Mode selection
	InitializeCmd.Flags().BoolVar(&initializeUnattended, "unattended", false, "Run in unattended mode without interactive prompts")

	// Semantic provider configuration
	InitializeCmd.Flags().StringVar(&initializeSemanticProvider, "semantic-provider", "", "Semantic analysis provider (claude, openai, gemini)")
	InitializeCmd.Flags().StringVar(&initializeSemanticModel, "semantic-model", "", "Model for semantic analysis (provider-specific)")
	InitializeCmd.Flags().StringVar(&initializeSemanticAPIKey, "semantic-api-key", "", "API key for semantic analysis")
	InitializeCmd.Flags().BoolVar(&initializeUseEnvSemanticAPIKey, "use-env-semantic-api-key", false, "Use environment variable for semantic provider API key")
	InitializeCmd.Flags().BoolVar(&initializeSkipSemantic, "skip-semantic", false, "Disable semantic analysis")

	// HTTP API configuration
	InitializeCmd.Flags().IntVar(&initializeHTTPPort, "http-port", -1, "HTTP API port (0 to disable, -1 for wizard default)")

	// FalkorDB configuration
	InitializeCmd.Flags().StringVar(&initializeGraphHost, "graph-host", config.DefaultConfig.Graph.Host, "FalkorDB host")
	InitializeCmd.Flags().IntVar(&initializeGraphPort, "graph-port", config.DefaultConfig.Graph.Port, "FalkorDB port")
	InitializeCmd.Flags().StringVar(&initializeGraphDatabase, "graph-database", config.DefaultConfig.Graph.Database, "FalkorDB database name")
	InitializeCmd.Flags().StringVar(&initializeGraphPassword, "graph-password", "", "FalkorDB password")
	InitializeCmd.Flags().BoolVar(&initializeStartFalkorDBDocker, "start-falkordb-docker", false, "Start FalkorDB using Docker")
	InitializeCmd.Flags().BoolVar(&initializeStartFalkorDBPodman, "start-falkordb-podman", false, "Start FalkorDB using Podman")
	InitializeCmd.Flags().BoolVar(&initializeSkipFalkorDBCheck, "skip-falkordb-check", false, "Skip FalkorDB connectivity verification")

	// Embeddings configuration
	InitializeCmd.Flags().BoolVar(&initializeEnableEmbeddings, "enable-embeddings", false, "Enable embeddings")
	InitializeCmd.Flags().BoolVar(&initializeDisableEmbeddings, "disable-embeddings", false, "Disable embeddings (default)")
	InitializeCmd.Flags().StringVar(&initializeOpenAIAPIKey, "openai-api-key", "", "OpenAI API key for embeddings")
	InitializeCmd.Flags().BoolVar(&initializeUseEnvOpenAIAPIKey, "use-env-openai-api-key", false, "Use OPENAI_API_KEY from environment")

	// Integration configuration
	InitializeCmd.Flags().StringSliceVar(&initializeIntegrations, "integrations", []string{}, "Integrations to setup (comma-separated)")

	InitializeCmd.Flags().SortFlags = false
}

func validateInit(cmd *cobra.Command, args []string) error {
	// Check platform support for service manager integration
	if !servicemanager.IsPlatformSupported() {
		return fmt.Errorf("service manager integration only supported on Linux and macOS; current platform: %s", runtime.GOOS)
	}

	// Validate mutually exclusive flags
	if initializeEnableEmbeddings && initializeDisableEmbeddings {
		return fmt.Errorf("--enable-embeddings and --disable-embeddings are mutually exclusive")
	}

	if initializeSemanticAPIKey != "" && initializeUseEnvSemanticAPIKey {
		return fmt.Errorf("--semantic-api-key and --use-env-semantic-api-key are mutually exclusive")
	}

	if initializeOpenAIAPIKey != "" && initializeUseEnvOpenAIAPIKey {
		return fmt.Errorf("--openai-api-key and --use-env-openai-api-key are mutually exclusive")
	}

	// Validate http-port flag if provided
	if initializeHTTPPort < -1 || initializeHTTPPort > 65535 {
		return fmt.Errorf("--http-port must be -1 (default), 0 (disabled), or 1-65535")
	}

	// Validate graph-port range
	if initializeGraphPort < 1 || initializeGraphPort > 65535 {
		return fmt.Errorf("--graph-port must be between 1 and 65535")
	}

	// Validate semantic provider if specified
	if initializeSemanticProvider != "" && initializeSkipSemantic {
		return fmt.Errorf("--semantic-provider and --skip-semantic are mutually exclusive")
	}
	if initializeSemanticProvider != "" && initializeSemanticProvider != "claude" && initializeSemanticProvider != "openai" && initializeSemanticProvider != "gemini" {
		return fmt.Errorf("--semantic-provider must be one of: claude, openai, gemini")
	}

	// Validate FalkorDB container runtime flags
	if initializeStartFalkorDBDocker && initializeStartFalkorDBPodman {
		return fmt.Errorf("--start-falkordb-docker and --start-falkordb-podman are mutually exclusive")
	}
	if initializeStartFalkorDBDocker && !container.IsDockerAvailable() {
		return fmt.Errorf("--start-falkordb-docker specified but Docker is not available")
	}
	if initializeStartFalkorDBPodman && !container.IsPodmanAvailable() {
		return fmt.Errorf("--start-falkordb-podman specified but Podman is not available")
	}

	// Unattended mode validation
	if initializeUnattended {
		// Semantic analysis: require API key unless skipped
		if !initializeSkipSemantic {
			provider := initializeSemanticProvider
			if provider == "" {
				provider = "claude" // default
			}

			// Check for API key based on provider
			hasKey := initializeSemanticAPIKey != ""
			if !hasKey {
				// Check --use-env-semantic-api-key flag or implicit env var presence
				switch provider {
				case "claude":
					hasKey = os.Getenv(config.ClaudeAPIKeyEnv) != ""
				case "openai":
					hasKey = os.Getenv(config.OpenAIAPIKeyEnv) != ""
				case "gemini":
					hasKey = os.Getenv(config.GoogleAPIKeyEnv) != ""
				}
			}

			if !hasKey {
				envVar := config.ClaudeAPIKeyEnv
				switch provider {
				case "openai":
					envVar = config.OpenAIAPIKeyEnv
				case "gemini":
					envVar = config.GoogleAPIKeyEnv
				}
				return fmt.Errorf("unattended mode requires --semantic-api-key, --use-env-semantic-api-key, or %s environment variable (or use --skip-semantic)", envVar)
			}
		}

		// FalkorDB must be addressed
		startingFalkorDB := initializeStartFalkorDBDocker || initializeStartFalkorDBPodman
		if !startingFalkorDB && !initializeSkipFalkorDBCheck {
			// Check if FalkorDB is already running
			if !container.IsFalkorDBRunning(initializeGraphPort) {
				return fmt.Errorf("unattended mode requires FalkorDB to be running, use --start-falkordb-docker or --start-falkordb-podman to auto-start, or --skip-falkordb-check to bypass")
			}
		}

		// Embeddings validation
		if initializeEnableEmbeddings {
			if initializeOpenAIAPIKey == "" && !initializeUseEnvOpenAIAPIKey {
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
	if initializeUnattended {
		return runUnattended(cmd)
	}
	return runInteractive(cmd)
}

func runInteractive(cmd *cobra.Command) error {
	// Get app directory
	appDir, err := config.GetAppDir()
	if err != nil {
		return fmt.Errorf("failed to get app directory; %w", err)
	}
	configPath := filepath.Join(appDir, config.ConfigFile)

	// Check for existing config
	if !initializeForce {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("config file already exists at %s (use --force to overwrite)", configPath)
		}
	}

	// Build initial config from defaults and flags
	cfg := config.DefaultConfig
	if initializeMemoryRoot != "" {
		cfg.Memory.Root = config.ExpandHome(initializeMemoryRoot)
	} else {
		cfg.Memory.Root = config.ExpandHome(cfg.Memory.Root)
	}
	if initializeCacheDir != "" {
		cfg.Semantic.CacheDir = config.ExpandHome(initializeCacheDir)
	} else {
		cfg.Semantic.CacheDir = config.ExpandHome(cfg.Semantic.CacheDir)
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

	// Get selected integrations from wizard
	selectedIntegrations := result.SelectedIntegrations

	// Finalize configuration (skip next steps - will print after startup handling)
	if err := finalizeInit(configPath, result.Config, selectedIntegrations, true); err != nil {
		return err
	}

	// Handle startup step choices and print next steps with context
	return handleStartupChoices(result)
}

func runUnattended(cmd *cobra.Command) error {
	// Get app directory
	appDir, err := config.GetAppDir()
	if err != nil {
		return fmt.Errorf("failed to get app directory; %w", err)
	}
	configPath := filepath.Join(appDir, config.ConfigFile)

	// Check for existing config
	if !initializeForce {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("config file already exists at %s (use --force to overwrite)", configPath)
		}
	}

	// Build config from flags
	cfg := config.DefaultConfig

	// Memory root
	if initializeMemoryRoot != "" {
		cfg.Memory.Root = config.ExpandHome(initializeMemoryRoot)
	} else {
		cfg.Memory.Root = config.ExpandHome(cfg.Memory.Root)
	}

	// Cache dir
	if initializeCacheDir != "" {
		cfg.Semantic.CacheDir = config.ExpandHome(initializeCacheDir)
	} else {
		cfg.Semantic.CacheDir = config.ExpandHome(cfg.Semantic.CacheDir)
	}

	// Semantic provider configuration
	if initializeSkipSemantic {
		cfg.Semantic.Enabled = false
		cfg.Semantic.Provider = ""
		cfg.Semantic.Model = ""
		cfg.Semantic.APIKey = ""
	} else {
		cfg.Semantic.Enabled = true

		// Provider selection
		if initializeSemanticProvider != "" {
			cfg.Semantic.Provider = initializeSemanticProvider
		} else {
			cfg.Semantic.Provider = config.DefaultSemanticProvider
		}

		// Model selection (use provider default if not specified)
		if initializeSemanticModel != "" {
			cfg.Semantic.Model = initializeSemanticModel
		} else {
			switch cfg.Semantic.Provider {
			case "claude":
				cfg.Semantic.Model = config.DefaultClaudeModel
				cfg.Semantic.RateLimitPerMin = config.DefaultClaudeRateLimit
			case "openai":
				cfg.Semantic.Model = config.DefaultOpenAIModel
				cfg.Semantic.RateLimitPerMin = config.DefaultOpenAIRateLimit
			case "gemini":
				cfg.Semantic.Model = config.DefaultGeminiModel
				cfg.Semantic.RateLimitPerMin = config.DefaultGeminiRateLimit
			}
		}

		// API key - check explicit flag, then use-env flag, then implicit env vars
		if initializeSemanticAPIKey != "" {
			cfg.Semantic.APIKey = initializeSemanticAPIKey
		} else if initializeUseEnvSemanticAPIKey {
			// Unified env var flag - read from provider-appropriate env var
			switch cfg.Semantic.Provider {
			case "claude":
				cfg.Semantic.APIKey = os.Getenv(config.ClaudeAPIKeyEnv)
			case "openai":
				cfg.Semantic.APIKey = os.Getenv(config.OpenAIAPIKeyEnv)
			case "gemini":
				cfg.Semantic.APIKey = os.Getenv(config.GoogleAPIKeyEnv)
			}
		} else {
			// Try to get from environment based on provider (implicit)
			switch cfg.Semantic.Provider {
			case "claude":
				cfg.Semantic.APIKey = os.Getenv(config.ClaudeAPIKeyEnv)
			case "openai":
				cfg.Semantic.APIKey = os.Getenv(config.OpenAIAPIKeyEnv)
			case "gemini":
				cfg.Semantic.APIKey = os.Getenv(config.GoogleAPIKeyEnv)
			}
		}
	}

	// HTTP port
	if initializeHTTPPort >= 0 {
		cfg.Daemon.HTTPPort = initializeHTTPPort
		if initializeHTTPPort > 0 {
			cfg.MCP.DaemonPort = initializeHTTPPort
			cfg.MCP.DaemonHost = "localhost"
		}
	}

	// FalkorDB
	cfg.Graph.Host = initializeGraphHost
	cfg.Graph.Port = initializeGraphPort
	cfg.Graph.Database = initializeGraphDatabase
	cfg.Graph.Password = initializeGraphPassword

	// Start FalkorDB if requested
	if initializeStartFalkorDBDocker || initializeStartFalkorDBPodman {
		var runtime container.Runtime
		if initializeStartFalkorDBDocker {
			runtime = container.RuntimeDocker
		} else {
			runtime = container.RuntimePodman
		}

		fmt.Printf("Starting FalkorDB in %s...\n", container.GetRuntime(runtime))
		opts := container.StartOptions{
			Port:    initializeGraphPort,
			DataDir: fmt.Sprintf("%s/falkordb", appDir),
			Detach:  true,
		}
		if err := container.StartFalkorDB(runtime, opts); err != nil {
			return fmt.Errorf("failed to start FalkorDB; %w", err)
		}
		fmt.Println("FalkorDB started successfully.")
		fmt.Printf("  Redis port: %d\n", initializeGraphPort)
		fmt.Printf("  Browser UI: http://localhost:3000\n")
	}

	// Embeddings
	if initializeEnableEmbeddings {
		cfg.Embeddings.Enabled = true
		if initializeOpenAIAPIKey != "" {
			cfg.Embeddings.APIKey = initializeOpenAIAPIKey
		} else if initializeUseEnvOpenAIAPIKey {
			// Detect and write env var to config for service manager compatibility
			envKey := os.Getenv(config.EmbeddingsAPIKeyEnv)
			cfg.Embeddings.APIKey = envKey
		} else if os.Getenv(config.EmbeddingsAPIKeyEnv) != "" {
			// Env var exists but not explicitly requested - still capture it
			cfg.Embeddings.APIKey = os.Getenv(config.EmbeddingsAPIKeyEnv)
		}
	}

	// Integrations (for setup, not config tracking)
	// Finalize configuration (print next steps for unattended mode)
	return finalizeInit(configPath, &cfg, initializeIntegrations, false)
}

func handleStartupChoices(result *tuiinit.WizardResult) error {
	if result.StartupStep == nil {
		// No startup step (shouldn't happen)
		return nil
	}

	step := result.StartupStep
	installChoice := step.GetInstallChoice()
	startChoice := step.GetStartChoice()

	switch installChoice {
	case 0: // InstallUser (imported as steps.InstallUser but we can't access it here)
		installPath := step.GetInstallPath()
		fmt.Printf("\n✓ Service installed: %s\n", installPath)

		switch startChoice {
		case 0: // StartNow
			fmt.Println("\nEnabling and starting daemon via service manager...")

			if runtime.GOOS == "linux" {
				// systemd user service
				if err := runCommand("systemctl", "--user", "daemon-reload"); err != nil {
					return fmt.Errorf("failed to reload systemd daemon; %w", err)
				}
				if err := runCommand("systemctl", "--user", "enable", "memorizer"); err != nil {
					return fmt.Errorf("failed to enable service; %w", err)
				}
				if err := runCommand("systemctl", "--user", "start", "memorizer"); err != nil {
					return fmt.Errorf("failed to start service; %w", err)
				}
				fmt.Println("✓ Daemon enabled and started via systemd")

			} else if runtime.GOOS == "darwin" {
				// launchd user agent
				user := os.Getenv("USER")
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to get home directory; %w", err)
				}

				// Get UID
				uidCmd := exec.Command("id", "-u")
				uidOutput, err := uidCmd.Output()
				if err != nil {
					return fmt.Errorf("failed to get UID; %w", err)
				}
				uid := strings.TrimSpace(string(uidOutput))

				plistPath := fmt.Sprintf("%s/Library/LaunchAgents/com.%s.memorizer.plist", home, user)
				serviceName := fmt.Sprintf("gui/%s/com.%s.memorizer", uid, user)

				// Bootstrap (load) the agent
				if err := runCommand("launchctl", "bootstrap", fmt.Sprintf("gui/%s", uid), plistPath); err != nil {
					// Ignore error if already loaded
					fmt.Printf("Note: Service may already be loaded (ignoring bootstrap error)\n")
				}

				// Enable the agent
				if err := runCommand("launchctl", "enable", serviceName); err != nil {
					return fmt.Errorf("failed to enable service; %w", err)
				}

				// Start the agent
				if err := runCommand("launchctl", "kickstart", "-k", serviceName); err != nil {
					return fmt.Errorf("failed to start service; %w", err)
				}

				fmt.Println("✓ Daemon enabled and started via launchd")
			}

		case 1: // StartLater
			// Service installed but not started - instructions will be shown in printNextSteps()
		}

	case 1: // InstallSystem
		fmt.Println("\n" + step.GetSystemInstructions())

	case 2: // InstallSkip
		fmt.Println("\nAutomatic startup skipped.")
	}

	// Print next steps with startup context
	printNextSteps(result.Config, &StartupInfo{
		InstallChoice: int(installChoice),
		StartChoice:   int(startChoice),
	})

	return nil
}

func finalizeInit(configPath string, cfg *config.Config, integrationNames []string, skipNextSteps bool) error {
	// Create directories
	if err := os.MkdirAll(cfg.Memory.Root, 0755); err != nil {
		return fmt.Errorf("failed to create memory directory; %w", err)
	}

	if err := os.MkdirAll(cfg.Semantic.CacheDir, 0755); err != nil {
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
	fmt.Printf("  Created memory directory: %s\n", cfg.Memory.Root)
	fmt.Printf("  Created cache directory: %s\n", cfg.Semantic.CacheDir)
	fmt.Printf("  FalkorDB: %s:%d (database: %s)\n", cfg.Graph.Host, cfg.Graph.Port, cfg.Graph.Database)
	if cfg.Embeddings.Enabled {
		fmt.Printf("  Embeddings: %s (%s)\n", cfg.Embeddings.Provider, cfg.Embeddings.Model)
	} else {
		fmt.Printf("  Embeddings: disabled\n")
	}

	// Setup integrations
	if len(integrationNames) > 0 {
		_, err := setupIntegrations(integrationNames)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n\n", err)
		}
	}

	// Print next steps (unless interactive mode will handle it later)
	if !skipNextSteps {
		printNextSteps(cfg, nil)
	}

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

		// Skip if already enabled (prevents duplicate hooks)
		if enabled, _ := integration.IsEnabled(); enabled {
			fmt.Printf("  Integration %s already configured (skipped)\n", name)
			enabledIntegrations = append(enabledIntegrations, name)
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

// StartupInfo contains information about service-manager setup choices
type StartupInfo struct {
	InstallChoice int // 0=InstallUser, 1=InstallSystem, 2=InstallSkip
	StartChoice   int // 0=StartNow, 1=StartLater (only relevant for InstallUser)
}

func printNextSteps(cfg *config.Config, startup *StartupInfo) {
	apiKeyConfigured := cfg.Semantic.APIKey != "" || os.Getenv(config.ClaudeAPIKeyEnv) != ""
	falkorDBRunning := container.IsFalkorDBRunning(cfg.Graph.Port)

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
		dockerAvailable := container.IsDockerAvailable()
		podmanAvailable := container.IsPodmanAvailable()

		fmt.Printf("%d. Start FalkorDB (required):\n", stepNum)

		if dockerAvailable && podmanAvailable {
			// Show both options
			fmt.Printf("   # Using Docker:\n")
			fmt.Printf("   docker run -d --name memorizer-falkordb -p %d:6379 -p 3000:3000 falkordb/falkordb\n\n", cfg.Graph.Port)
			fmt.Printf("   # Using Podman:\n")
			fmt.Printf("   podman run -d --name memorizer-falkordb --network=host falkordb/falkordb\n")
		} else if dockerAvailable {
			fmt.Printf("   docker run -d --name memorizer-falkordb -p %d:6379 -p 3000:3000 falkordb/falkordb\n", cfg.Graph.Port)
		} else if podmanAvailable {
			fmt.Printf("   podman run -d --name memorizer-falkordb --network=host falkordb/falkordb\n")
		} else {
			// Neither available - show generic message
			fmt.Printf("   Install Docker or Podman, then run:\n")
			fmt.Printf("   docker run -d --name memorizer-falkordb -p %d:6379 -p 3000:3000 falkordb/falkordb\n", cfg.Graph.Port)
		}
		stepNum++
	}

	fmt.Printf("%d. Add files to %s\n", stepNum, cfg.Memory.Root)
	stepNum++

	// Only show daemon startup instructions if service-manager was NOT set up,
	// or if it was set up but the daemon is not yet started
	showDaemonInstructions := true
	if startup != nil {
		if startup.InstallChoice == 0 && startup.StartChoice == 0 {
			// InstallUser + StartNow: daemon already started via service-manager
			showDaemonInstructions = false
		} else if startup.InstallChoice == 1 {
			// InstallSystem: system-level instructions already shown
			showDaemonInstructions = false
		}
	}

	if showDaemonInstructions {
		fmt.Printf("%d. Start the daemon:\n", stepNum)
		if startup != nil && startup.InstallChoice == 0 && startup.StartChoice == 1 {
			// InstallUser + StartLater: service is installed but not started
			fmt.Printf("   # Service installed - enable and start it:\n")
			if runtime.GOOS == "linux" {
				fmt.Printf("   systemctl --user enable memorizer\n")
				fmt.Printf("   systemctl --user start memorizer\n\n")
			} else if runtime.GOOS == "darwin" {
				user := os.Getenv("USER")
				fmt.Printf("   launchctl enable gui/$(id -u)/com.%s.memorizer\n", user)
				fmt.Printf("   launchctl kickstart -k gui/$(id -u)/com.%s.memorizer\n\n", user)
			}
			fmt.Printf("   # Or manually (foreground):\n")
			fmt.Printf("   memorizer daemon start\n")
		} else {
			// InstallSkip or unattended mode: show all options
			fmt.Printf("   # Option A: Manual (foreground)\n")
			fmt.Printf("   memorizer daemon start\n\n")
			fmt.Printf("   # Option B: Manual (background)\n")
			fmt.Printf("   nohup memorizer daemon start &\n\n")
			fmt.Printf("   # Option C: System service (background, recommended)\n")
			fmt.Printf("   See 'Running as a Service' in README.md\n")
		}
	}
}

// Helper functions

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s; output: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func findBinaryPath() (string, error) {
	// Try to get the current executable path
	execPath, err := os.Executable()
	if err == nil {
		if filepath.Base(execPath) == "memorizer" {
			return execPath, nil
		}
	}

	// Try common installation paths
	home, err := os.UserHomeDir()
	if err == nil {
		commonPath := filepath.Join(home, ".local", "bin", "memorizer")
		if _, err := os.Stat(commonPath); err == nil {
			return commonPath, nil
		}
	}

	// Try PATH
	pathBinary, err := exec.LookPath("memorizer")
	if err == nil {
		return pathBinary, nil
	}

	return "", fmt.Errorf("could not locate memorizer binary")
}
