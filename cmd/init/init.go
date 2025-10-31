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
		"(use --setup-hooks flag or respond to the interactive prompt).\n\n" +
		"For optimal performance, the background daemon mode is recommended. The daemon " +
		"maintains a precomputed index for <50ms startup times (use --with-daemon flag or " +
		"respond to the interactive prompt).",
	Example: `  # Default initialization (prompts for hooks and daemon)
  agentic-memorizer init

  # Initialize with hooks and daemon
  agentic-memorizer init --setup-hooks --with-daemon

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
	InitCmd.Flags().String("cache-dir", config.DefaultConfig.Analysis.CacheDir, "Cache directory")
	InitCmd.Flags().Bool("force", false, "Overwrite existing config")
	InitCmd.Flags().Bool("setup-hooks", false, "Configure Claude Code SessionStart hooks")
	InitCmd.Flags().Bool("skip-hooks", false, "Skip Claude Code hook setup prompt")
	InitCmd.Flags().Bool("with-daemon", false, "Start background daemon after initialization")
	InitCmd.Flags().Bool("skip-daemon", false, "Skip daemon start prompt")

	InitCmd.Flags().SortFlags = false
}

func runInit(cmd *cobra.Command, args []string) error {
	memoryRoot, _ := cmd.Flags().GetString("memory-root")
	cacheDir, _ := cmd.Flags().GetString("cache-dir")
	force, _ := cmd.Flags().GetBool("force")
	setupHooks, _ := cmd.Flags().GetBool("setup-hooks")
	skipHooks, _ := cmd.Flags().GetBool("skip-hooks")
	withDaemon, _ := cmd.Flags().GetBool("with-daemon")
	skipDaemon, _ := cmd.Flags().GetBool("skip-daemon")

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory; %w", err)
	}
	configPath := filepath.Join(home, config.AppDirName, config.ConfigFile)

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

	daemonStarted := false
	if err := handleDaemonSetup(withDaemon, skipDaemon, &daemonStarted); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n\n", err)
	}

	fmt.Printf("\nNext steps:\n")
	fmt.Printf("1. Set your Claude API key: export ANTHROPIC_API_KEY=\"your-key-here\"\n")
	fmt.Printf("2. Add files to %s\n", memoryRoot)
	if daemonStarted {
		fmt.Printf("3. Daemon is running in background (check status: agentic-memorizer daemon status)\n")
		fmt.Printf("4. Start using Claude Code with <50ms startup times!\n")
	} else {
		fmt.Printf("3. Recommended: Start the background daemon for optimal performance:\n")
		fmt.Printf("   agentic-memorizer daemon start\n")
		fmt.Printf("4. Or use on-demand indexing: agentic-memorizer\n")
	}

	return nil
}

func handleHookSetup(setupHooks, skipHooks bool) error {
	if skipHooks {
		return nil
	}

	if !setupHooks {
		fmt.Printf("\nConfigure Claude Code SessionStart hooks?\n")
		fmt.Printf("(Hooks enable automatic indexing when Claude Code starts)\n")
		fmt.Printf("Note: For best performance, use daemon mode instead of hooks.\n")
		fmt.Printf("Configure hooks? [y/N]: ")
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

func handleDaemonSetup(withDaemon, skipDaemon bool, daemonStarted *bool) error {
	*daemonStarted = false

	if skipDaemon {
		return nil
	}

	if !withDaemon {
		fmt.Printf("\nStart background daemon for optimal performance (<50ms startup)? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input; %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Printf("\nYou can start the daemon later with: agentic-memorizer daemon start\n")
			return nil
		}
		withDaemon = true
	}

	if withDaemon {
		fmt.Printf("\nStarting background daemon...\n")

		binaryPath, err := hooks.FindBinaryPath()
		if err != nil {
			return fmt.Errorf("could not find agentic-memorizer binary; %w\nStart daemon manually: agentic-memorizer daemon start", err)
		}

		// Check if daemon is already running
		pidFile, err := config.GetPIDPath()
		if err != nil {
			return fmt.Errorf("failed to get PID path; %w", err)
		}

		if _, err := os.Stat(pidFile); err == nil {
			// PID file exists, check if daemon is actually running
			data, err := os.ReadFile(pidFile)
			if err == nil {
				var pid int
				fmt.Sscanf(string(data), "%d", &pid)
				process, err := os.FindProcess(pid)
				if err == nil && process.Signal(os.Signal(nil)) == nil {
					fmt.Printf("✓ Daemon is already running (PID %d)\n", pid)
					*daemonStarted = true
					return nil
				}
			}
		}

		// Start daemon using timeout to prevent hanging
		cmd := fmt.Sprintf("timeout 5 %s daemon start > /dev/null 2>&1 &", binaryPath)
		if err := os.WriteFile("/tmp/start-daemon.sh", []byte("#!/bin/bash\n"+cmd), 0755); err != nil {
			return fmt.Errorf("failed to create daemon start script; %w", err)
		}

		// Execute the script
		execCmd := "/bin/bash"
		args := []string{"/tmp/start-daemon.sh"}
		process, err := os.StartProcess(execCmd, args, &os.ProcAttr{
			Files: []*os.File{nil, nil, nil},
		})
		if err != nil {
			return fmt.Errorf("failed to start daemon; %w\nStart daemon manually: agentic-memorizer daemon start", err)
		}

		// Don't wait for the daemon to finish
		process.Release()

		fmt.Printf("✓ Background daemon started\n")
		*daemonStarted = true
	}

	return nil
}
