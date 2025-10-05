package cmdinit

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/hooks"
	"github.com/spf13/cobra"
)

var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration, memory directory, and optionally Claude Code hooks",
	Long: "\nCreates default configuration file and memory directory.\n\n" +
		"The init command sets up the Agentic Memorizer by creating a default configuration " +
		"file and the memory directory where you'll store files for analysis and indexing. " +
		"Optionally configures Claude Code SessionStart hooks for automatic file indexing " +
		"(use --setup-hooks flag or respond to the interactive prompt).",
	Example: `  # Default initialization
  agentic-memorizer init

  # Initialize with automatic hook setup
  agentic-memorizer init --setup-hooks

  # Custom memory directory
  agentic-memorizer init --memory-root ~/my-memory

  # Custom cache directory
  agentic-memorizer init --cache-dir ~/my-memory/.cache

  # Force overwrite existing config
  agentic-memorizer init --force`,
	RunE: runInit,
}

func init() {
	InitCmd.Flags().String("memory-root", config.DefaultConfig.MemoryRoot, "Memory directory")
	InitCmd.Flags().String("cache-dir", config.DefaultConfig.CacheDir, "Cache directory")
	InitCmd.Flags().Bool("force", false, "Overwrite existing config")
	InitCmd.Flags().Bool("setup-hooks", false, "Configure Claude Code SessionStart hooks")
	InitCmd.Flags().Bool("skip-hooks", false, "Skip Claude Code hook setup prompt")

	InitCmd.Flags().SortFlags = false
}

func runInit(cmd *cobra.Command, args []string) error {
	memoryRoot, _ := cmd.Flags().GetString("memory-root")
	cacheDir, _ := cmd.Flags().GetString("cache-dir")
	force, _ := cmd.Flags().GetBool("force")
	setupHooks, _ := cmd.Flags().GetBool("setup-hooks")
	skipHooks, _ := cmd.Flags().GetBool("skip-hooks")

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory; %w", err)
	}
	configPath := filepath.Join(home, ".agentic-memorizer", "config.yaml")

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
	cfg.CacheDir = cacheDir

	if err := config.WriteConfig(configPath, &cfg); err != nil {
		return fmt.Errorf("failed to write config; %w", err)
	}

	fmt.Printf("Configuration:\n")
	fmt.Printf("✓ Created configuration file: %s\n", configPath)
	fmt.Printf("✓ Created memory directory: %s\n", memoryRoot)
	fmt.Printf("✓ Created cache directory: %s\n", cacheDir)

	if err := handleHookSetup(setupHooks, skipHooks); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n\n", err)
	}

	fmt.Printf("Next steps:\n")
	fmt.Printf("1. Set your Claude API key: export ANTHROPIC_API_KEY=\"your-key-here\"\n")
	fmt.Printf("2. Add files to %s\n", memoryRoot)
	fmt.Printf("3. Run: agentic-memorizer\n")

	return nil
}

func handleHookSetup(setupHooks, skipHooks bool) error {
	if skipHooks {
		return nil
	}

	if !setupHooks {
		fmt.Printf("\nConfigure Claude Code SessionStart hooks? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input; %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			settingsPath, _ := hooks.GetClaudeSettingsPath()
			fmt.Printf("\nTo set up hooks manually, add SessionStart hooks to: %s\n", settingsPath)
			fmt.Printf("See README.md for configuration details.\n\n")
			return nil
		}
		setupHooks = true
	}

	if setupHooks {
		binaryPath, err := hooks.FindBinaryPath()
		if err != nil {
			settingsPath, _ := hooks.GetClaudeSettingsPath()
			return fmt.Errorf("could not auto-detect binary path; %w\nPlease manually configure hooks in: %s", err, settingsPath)
		}

		_, updated, err := hooks.SetupSessionStartHooks(binaryPath)
		if err != nil {
			return fmt.Errorf("failed to set up hooks; %w", err)
		}

		settingsPath, _ := hooks.GetClaudeSettingsPath()
		if len(updated) > 0 {
			fmt.Printf("✓ Configured Claude Code SessionStart hooks: %s\n", settingsPath)
			fmt.Printf("  Updated matchers: %s\n", strings.Join(updated, ", "))
			fmt.Printf("  Binary path: %s\n\n", binaryPath)
		} else {
			fmt.Printf("✓ Claude Code SessionStart hooks already configured\n\n")
		}
	}

	return nil
}
