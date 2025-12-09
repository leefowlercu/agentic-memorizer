package subcommands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/servicemanager"
	"github.com/spf13/cobra"
)

var LaunchctlCmd = &cobra.Command{
	Use:   "launchctl",
	Short: "Generate launchd plist file",
	Long: "\nGenerate a launchd property list file for managing the daemon on macOS.\n\n" +
		"This command outputs a launchd plist file that can be installed to manage " +
		"the daemon with macOS launchd. The generated file includes automatic start " +
		"on login, automatic restart on failure, and proper log file configuration.",
	PreRunE: validateLaunchctl,
	RunE:    runLaunchctl,
}

func validateLaunchctl(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runLaunchctl(cmd *cobra.Command, args []string) error {
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config; %w", err)
	}

	// Load config with defensive fallback
	cfg, err := config.GetConfig()
	if err != nil || cfg == nil {
		cfg = &config.DefaultConfig
	}

	// Get binary path
	binaryPath, err := servicemanager.GetBinaryPath()
	if err != nil {
		return fmt.Errorf("failed to get binary path; %w", err)
	}

	// Get user and home directory
	user := os.Getenv("USER")
	if user == "" {
		user = "youruser"
	}

	home := os.Getenv("HOME")
	if home == "" {
		home = "/Users/" + user
	}

	// Get log file path from config
	logFile := cfg.Daemon.LogFile
	if logFile == "" {
		logFile = filepath.Join(home, ".memorizer", "daemon.log")
	}

	// Build launchd config
	launchdCfg := servicemanager.LaunchdConfig{
		BinaryPath: binaryPath,
		User:       user,
		Home:       home,
		LogFile:    logFile,
	}

	// Generate plist
	plistContent := servicemanager.GeneratePlist(launchdCfg)

	// Print the user plist file (default)
	fmt.Println("# User-level launchd plist file:")
	fmt.Println(plistContent)

	// Get user agent path
	userPlistPath, _ := servicemanager.GetUserAgentPath(home, user)

	// Print user-level installation instructions
	fmt.Println("\n# ========================================")
	fmt.Println("# User-Level Installation (No Root Required)")
	fmt.Println("# ========================================")
	fmt.Println()
	fmt.Printf("# 1. Install the plist file:\n")
	fmt.Printf("   mkdir -p %s/Library/LaunchAgents\n", home)
	fmt.Printf("   tee %s <<'EOF'\n", userPlistPath)
	fmt.Print(plistContent)
	fmt.Println("EOF")
	fmt.Println()
	fmt.Println("# 2. Enable and start:")
	fmt.Println(servicemanager.GetLaunchdUserInstructions(userPlistPath, user))

	// Get system daemon path
	systemPlistPath := servicemanager.GetSystemDaemonPath(user)

	// Print system-level installation instructions
	fmt.Println("\n\n# ========================================")
	fmt.Println("# System-Level Installation (Requires Root)")
	fmt.Println("# ========================================")
	fmt.Println()
	fmt.Println(servicemanager.GetLaunchdSystemInstructions(plistContent, systemPlistPath))

	return nil
}
