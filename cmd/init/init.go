package cmdinit

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/spf13/cobra"
)

var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration and memory directory",
	Long: "\nCreates default configuration file and memory directory.\n\n" +
		"The init command sets up the Agentic Memorizer by creating a default configuration " +
		"file and the memory directory where you'll store files for analysis and indexing.",
	Example: `  # Default initialization
  agentic-memorizer init

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

	InitCmd.Flags().SortFlags = false
}

func runInit(cmd *cobra.Command, args []string) error {
	memoryRoot, _ := cmd.Flags().GetString("memory-root")
	cacheDir, _ := cmd.Flags().GetString("cache-dir")
	force, _ := cmd.Flags().GetBool("force")

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
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

	fmt.Printf("✓ Created configuration file: %s\n\n", configPath)
	fmt.Printf("✓ Created memory directory: %s\n", memoryRoot)
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Memory Root: %s\n", memoryRoot)
	fmt.Printf("  Cache Dir: %s\n\n", cacheDir)
	fmt.Printf("Next steps:\n")
	fmt.Printf("1. Set your Claude API key: export ANTHROPIC_API_KEY=\"your-key-here\"\n")
	fmt.Printf("2. Add files to %s\n", memoryRoot)
	fmt.Printf("3. Run: agentic-memorizer\n")

	return nil
}
