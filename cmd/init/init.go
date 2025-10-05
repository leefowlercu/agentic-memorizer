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
		"The init command sets up the memorizer by creating a default configuration " +
		"file and the memory directory where you'll store files for indexing.",
	Example: `  # Default initialization
  memorizer init

  # Custom memory directory
  memorizer init --memory-root ~/my-memory

  # Custom config location
  memorizer init --config-path ~/.memorizer-config.yaml

  # Force overwrite existing config
  memorizer init --force`,
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

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	execDir := filepath.Dir(execPath)
	configPath := filepath.Join(execDir, "memorizer-config.yaml")

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
	fmt.Printf("3. Run: memorizer\n")

	return nil
}
