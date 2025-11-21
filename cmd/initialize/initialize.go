package initialize

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/adapters/claude" // Register Claude adapter
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var InitializeCmd = &cobra.Command{
	Use:   "initialize",
	Short: "Initialize configuration and memory directory",
	Long: "\nCreates default configuration file and memory directory.\n\n" +
		"The initialize command sets up the Agentic Memorizer by creating a default configuration " +
		"file and the memory directory where you'll store files for analysis and indexing.\n\n" +
		"Optionally configures integrations with agent frameworks like Claude Code for " +
		"automatic memory indexing.\n\n" +
		"After initialization, start the daemon manually with 'agentic-memorizer daemon start' " +
		"or set up as a system service for automatic management (recommended for production).\n\n" +
		"By default, configuration and data files are stored in ~/.agentic-memorizer/. " +
		"You can customize this location by setting the MEMORIZER_APP_DIR environment variable " +
		"before running initialize.",
	Example: `  # Default initialization
  agentic-memorizer initialize

  # Initialize with integrations
  agentic-memorizer initialize --setup-integrations

  # Custom memory directory
  agentic-memorizer initialize --memory-root ~/my-memory

  # Custom cache directory
  agentic-memorizer initialize --cache-dir ~/my-memory/.cache

  # Force overwrite existing config
  agentic-memorizer initialize --force

  # After initialization, start the daemon:
  agentic-memorizer daemon start              # Manual start
  agentic-memorizer daemon systemctl          # Generate systemd unit
  agentic-memorizer daemon launchctl          # Generate launchd plist`,
	PreRunE: validateInit,
	RunE:    runInit,
}

func init() {
	InitializeCmd.Flags().String("memory-root", config.DefaultConfig.MemoryRoot, "Memory directory")
	InitializeCmd.Flags().String("cache-dir", config.DefaultConfig.Analysis.CacheDir, "Cache directory")
	InitializeCmd.Flags().Bool("force", false, "Overwrite existing config")
	InitializeCmd.Flags().Bool("setup-integrations", false, "Configure agent framework integrations")
	InitializeCmd.Flags().Bool("skip-integrations", false, "Skip integration setup prompt")

	InitializeCmd.Flags().SortFlags = false
}

func validateInit(cmd *cobra.Command, args []string) error {
	// Validate mutually exclusive flags
	setupIntegrations, _ := cmd.Flags().GetBool("setup-integrations")
	skipIntegrations, _ := cmd.Flags().GetBool("skip-integrations")
	if setupIntegrations && skipIntegrations {
		return fmt.Errorf("--setup-integrations and --skip-integrations are mutually exclusive")
	}

	// All validation passed - errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runInit(cmd *cobra.Command, args []string) error {
	memoryRoot, _ := cmd.Flags().GetString("memory-root")
	cacheDir, _ := cmd.Flags().GetString("cache-dir")
	force, _ := cmd.Flags().GetBool("force")
	setupIntegrations, _ := cmd.Flags().GetBool("setup-integrations")
	skipIntegrations, _ := cmd.Flags().GetBool("skip-integrations")

	// Get app directory (respects MEMORIZER_APP_DIR environment variable)
	appDir, err := config.GetAppDir()
	if err != nil {
		return fmt.Errorf("failed to get app directory; %w", err)
	}
	configPath := filepath.Join(appDir, config.ConfigFile)

	if memoryRoot == "" {
		memoryRoot = config.DefaultConfig.MemoryRoot
	}

	if cacheDir == "" {
		cacheDir = filepath.Join(memoryRoot, ".cache")
	}

	memoryRoot = config.ExpandHome(memoryRoot)
	cacheDir = config.ExpandHome(cacheDir)

	if !force {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("config file already exists at %s (use --force to overwrite)", configPath)
		}
	}

	if err := os.MkdirAll(memoryRoot, 0755); err != nil {
		return fmt.Errorf("failed to create memory directory; %w", err)
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory; %w", err)
	}

	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory; %w", err)
	}

	cfg := config.DefaultConfig
	cfg.MemoryRoot = memoryRoot
	cfg.Analysis.CacheDir = cacheDir

	// Prompt for API key configuration
	apiKey, err := promptForAPIKey()
	if err != nil {
		return fmt.Errorf("failed to prompt for API key; %w", err)
	}

	// Track whether API key was configured (for Next Steps display)
	apiKeyConfigured := apiKey != "" || os.Getenv("ANTHROPIC_API_KEY") != ""

	// Set API key in config if provided (empty means using env var)
	if apiKey != "" {
		cfg.Claude.APIKey = apiKey
	}

	if err := config.WriteConfig(configPath, &cfg); err != nil {
		return fmt.Errorf("failed to write config; %w", err)
	}

	// Initialize config system to load the freshly written config
	// This is needed so integration setup can read config values (e.g., memory_root)
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config; %w", err)
	}

	fmt.Printf("Configuration:\n")
	fmt.Printf("  ✓ Created configuration file: %s\n", configPath)
	fmt.Printf("  ✓ Created memory directory: %s\n", memoryRoot)
	fmt.Printf("  ✓ Created cache directory: %s\n", cacheDir)

	enabledIntegrations, err := handleIntegrationSetup(setupIntegrations, skipIntegrations)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n\n", err)
	}

	// Update config with enabled integrations list if any were set up
	if len(enabledIntegrations) > 0 {
		cfg.Integrations.Enabled = enabledIntegrations
		if err := config.WriteConfig(configPath, &cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update config with enabled integrations; %v\n", err)
		}
		// Reload config after writing
		if err := config.InitConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to reload config; %v\n", err)
		}
	}

	fmt.Printf("\nNext steps:\n")
	stepNum := 1
	if !apiKeyConfigured {
		fmt.Printf("%d. Set your Claude API key: export ANTHROPIC_API_KEY=\"your-key-here\"\n", stepNum)
		stepNum++
	}
	fmt.Printf("%d. Add files to %s\n", stepNum, memoryRoot)
	stepNum++
	fmt.Printf("%d. Start the daemon:\n", stepNum)
	fmt.Printf("   # Option A: Manual (foreground)\n")
	fmt.Printf("   agentic-memorizer daemon start\n\n")
	fmt.Printf("   # Option B: Manual (background)\n")
	fmt.Printf("   nohup agentic-memorizer daemon start &\n\n")
	fmt.Printf("   # Option C: System service (background, recommended)\n")
	fmt.Printf("   agentic-memorizer daemon systemctl  # Linux\n")
	fmt.Printf("   agentic-memorizer daemon launchctl  # macOS\n")

	return nil
}

// promptForAPIKey prompts the user for Claude API key configuration
// Returns the API key to be written to config (empty string if using env var)
func promptForAPIKey() (string, error) {
	reader := bufio.NewReader(os.Stdin)

	// Check if ANTHROPIC_API_KEY is already set
	existingKey := os.Getenv("ANTHROPIC_API_KEY")
	if existingKey != "" {
		fmt.Printf("\nClaude API key detected in ANTHROPIC_API_KEY environment variable.\n")
		fmt.Printf("Use existing API key? [Y/n]: ")
		response, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read input; %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "n" && response != "no" {
			// User wants to use the existing env var
			fmt.Printf("\nUsing ANTHROPIC_API_KEY from environment.\n\n")
			return existingKey, nil
		}
	}

	// Prompt for configuration method
	fmt.Printf("\nHow would you like to configure your Claude API key?\n")
	fmt.Printf("1. Use environment variable (ANTHROPIC_API_KEY)\n")
	fmt.Printf("2. Enter API key directly (will be stored in config file)\n")
	fmt.Printf("3. Skip (configure later)\n")
	fmt.Printf("\nEnter your choice [1/2/3]: ")

	response, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input; %w", err)
	}

	response = strings.TrimSpace(response)

	switch response {
	case "1", "":
		// Use environment variable
		if existingKey == "" {
			fmt.Printf("\nUsing environment variable reference.\n")
			fmt.Printf("Remember to set: export ANTHROPIC_API_KEY=\"your-key-here\"\n\n")
		} else {
			fmt.Printf("\nUsing ANTHROPIC_API_KEY environment variable reference.\n\n")
		}
		return "", nil

	case "2":
		// Direct API key entry with masked input
		fmt.Printf("\nEnter your Claude API key (input will be hidden): ")

		// Read password (masked input)
		bytepw, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println() // Print newline after password input
		if err != nil {
			return "", fmt.Errorf("failed to read API key; %w", err)
		}

		apiKey := strings.TrimSpace(string(bytepw))
		if apiKey == "" {
			fmt.Printf("No API key entered. Falling back to environment variable reference.\n")
			return "", nil
		}

		fmt.Printf("API key configured and will be stored in config file.\n\n")
		return apiKey, nil

	case "3":
		// Skip
		fmt.Printf("\nSkipping API key configuration.\n")
		fmt.Printf("You can configure it later by:\n")
		fmt.Printf("  - Setting environment variable: export ANTHROPIC_API_KEY=\"your-key\"\n")
		fmt.Printf("  - Editing config file and adding claude.api_key\n\n")
		return "", nil

	default:
		fmt.Printf("Invalid choice. Skipping API key configuration.\n\n")
		return "", nil
	}
}

func handleIntegrationSetup(setupIntegrations, skipIntegrations bool) ([]string, error) {
	if skipIntegrations {
		return nil, nil
	}

	// Detect available integrations
	registry := integrations.GlobalRegistry()
	available := registry.DetectAvailable()

	if len(available) == 0 {
		fmt.Printf("\nNo agent frameworks detected on this system.\n")
		fmt.Printf("Supported integrations: Claude Code, Continue.dev, Cline\n")
		fmt.Printf("Install an agent framework and run 'agentic-memorizer integrations setup <name>' to configure.\n\n")
		return nil, nil
	}

	if !setupIntegrations {
		fmt.Printf("\nDetected agent frameworks:\n")
		for _, integration := range available {
			fmt.Printf("  - %s: %s\n", integration.GetName(), integration.GetDescription())
		}
		fmt.Printf("\nConfigure integrations with detected frameworks? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read input; %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Printf("\nTo set up integrations manually, run: agentic-memorizer integrations setup <integration-name>\n")
			return nil, nil
		}
		setupIntegrations = true
	}

	if setupIntegrations {
		binaryPath, err := findBinaryPath()
		if err != nil {
			return nil, fmt.Errorf("could not auto-detect binary path; %w\nPlease manually configure integrations", err)
		}

		fmt.Printf("\nConfigured integrations:\n")
		setupCount := 0
		enabledIntegrations := []string{}

		for _, integration := range available {
			err := integration.Setup(binaryPath)
			if err != nil {
				fmt.Printf("  Warning: Failed to setup %s: %v\n", integration.GetName(), err)
				continue
			}
			fmt.Printf("  ✓ Integration %s configured\n", integration.GetName())
			setupCount++
			enabledIntegrations = append(enabledIntegrations, integration.GetName())
		}

		if setupCount > 0 {
			fmt.Printf("\nBinary path: %s\n", binaryPath)
		} else {
			fmt.Printf("\nNo integrations were configured successfully.\n\n")
		}

		return enabledIntegrations, nil
	}

	return nil, nil
}

// findBinaryPath attempts to locate the agentic-memorizer binary
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
