package initialize

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/docker"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/adapters/claude" // Register Claude adapter
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/adapters/codex"  // Register Codex adapter
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/adapters/gemini" // Register Gemini adapter
	"github.com/leefowlercu/agentic-memorizer/internal/servicemanager"
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
		"After initialization, start the daemon manually with 'memorizer daemon start' " +
		"or set up as a system service for automatic management (recommended for production).\n\n" +
		"By default, configuration and data files are stored in ~/.memorizer/. " +
		"You can customize this location by setting the MEMORIZER_APP_DIR environment variable " +
		"before running initialize.",
	Example: `  # Interactive initialization (TUI wizard)
  memorizer initialize

  # Unattended initialization with required flags
  memorizer initialize --unattended \
    --use-env-anthropic-api-key \
    --start-falkordb \
    --integrations claude-code-hook,claude-code-mcp

  # Unattended with explicit API keys
  memorizer initialize --unattended \
    --anthropic-api-key sk-ant-... \
    --openai-api-key sk-... \
    --enable-embeddings \
    --graph-host localhost \
    --graph-port 6379

  # Force overwrite existing config
  memorizer initialize --force`,
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
	// Check platform support for service manager integration
	if !servicemanager.IsPlatformSupported() {
		return fmt.Errorf("service manager integration only supported on Linux and macOS; current platform: %s", runtime.GOOS)
	}

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

	// Finalize configuration (skip next steps - will print after startup handling)
	if err := finalizeInit(configPath, result.Config, true); err != nil {
		return err
	}

	// Handle startup step choices and print next steps with context
	return handleStartupChoices(result)
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
	} else if useEnvAnthropic {
		// Detect and write env var to config for service manager compatibility
		envKey := os.Getenv(config.ClaudeAPIKeyEnv)
		cfg.Claude.APIKey = envKey
	} else if os.Getenv(config.ClaudeAPIKeyEnv) != "" {
		// Env var exists but not explicitly requested - still capture it
		cfg.Claude.APIKey = os.Getenv(config.ClaudeAPIKeyEnv)
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
		} else if useEnvOpenai {
			// Detect and write env var to config for service manager compatibility
			envKey := os.Getenv(config.EmbeddingsAPIKeyEnv)
			cfg.Embeddings.APIKey = envKey
		} else if os.Getenv(config.EmbeddingsAPIKeyEnv) != "" {
			// Env var exists but not explicitly requested - still capture it
			cfg.Embeddings.APIKey = os.Getenv(config.EmbeddingsAPIKeyEnv)
		}
	}

	// Integrations
	integrationNames, _ := cmd.Flags().GetStringSlice("integrations")
	cfg.Integrations.Enabled = integrationNames

	// Finalize configuration (print next steps for unattended mode)
	return finalizeInit(configPath, &cfg, false)
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

func finalizeInit(configPath string, cfg *config.Config, skipNextSteps bool) error {
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
	fmt.Printf("  FalkorDB: %s:%d (database: %s)\n", cfg.Graph.Host, cfg.Graph.Port, cfg.Graph.Database)
	if cfg.Embeddings.Enabled {
		fmt.Printf("  Embeddings: %s (%s)\n", cfg.Embeddings.Provider, cfg.Embeddings.Model)
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
			fmt.Printf("   memorizer daemon systemctl  # Linux\n")
			fmt.Printf("   memorizer daemon launchctl  # macOS\n")
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
