package subcommands

import (
	"fmt"
	"os"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/servicemanager"
	"github.com/spf13/cobra"
)

var SystemctlCmd = &cobra.Command{
	Use:   "systemctl",
	Short: "Generate systemd unit file",
	Long: "\nGenerate a systemd unit file for managing the daemon as a system service.\n\n" +
		"This command outputs a systemd unit file that can be installed to manage " +
		"the daemon with systemd. The generated file uses Type=notify for precise " +
		"readiness control and includes recommended settings for automatic restarts " +
		"and health monitoring.",
	PreRunE: validateSystemctl,
	RunE:    runSystemctl,
}

func validateSystemctl(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runSystemctl(cmd *cobra.Command, args []string) error {
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
		home = "/home/" + user
	}

	// Build systemd config
	systemdCfg := servicemanager.SystemdConfig{
		BinaryPath: binaryPath,
		User:       user,
		Home:       home,
		LogFile:    cfg.Daemon.LogFile,
	}

	// Generate user unit file
	userUnit := servicemanager.GenerateUserUnit(systemdCfg)

	// Generate system unit file
	systemUnit := servicemanager.GenerateSystemUnit(systemdCfg)

	// Print the user unit file (default)
	fmt.Println("# User-level systemd unit file:")
	fmt.Println(userUnit)

	// Get user unit path for instructions
	userUnitPath, _ := servicemanager.GetUserUnitPath(home)

	// Print user-level installation instructions
	fmt.Println("\n# ========================================")
	fmt.Println("# User-Level Installation (No Root Required)")
	fmt.Println("# ========================================")
	fmt.Println()
	fmt.Printf("# 1. Install the unit file:\n")
	fmt.Printf("   mkdir -p %s\n", home+"/.config/systemd/user")
	fmt.Printf("   tee %s <<'EOF'\n", userUnitPath)
	fmt.Print(userUnit)
	fmt.Println("EOF")
	fmt.Println()
	fmt.Println("# 2. Enable and start:")
	fmt.Println(servicemanager.GetSystemdUserInstructions(userUnitPath))

	// Print system-level installation instructions
	fmt.Println("\n\n# ========================================")
	fmt.Println("# System-Level Installation (Requires Root)")
	fmt.Println("# ========================================")
	fmt.Println()
	fmt.Println(servicemanager.GetSystemdSystemInstructions(systemUnit))

	return nil
}
