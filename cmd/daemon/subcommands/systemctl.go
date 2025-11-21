package subcommands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
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

	// Get current binary path
	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path; %w", err)
	}

	// Resolve symlinks
	binaryPath, err = filepath.EvalSymlinks(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to resolve binary path; %w", err)
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

	// Generate systemd unit file
	unitFile := fmt.Sprintf(`[Unit]
Description=Agentic Memorizer Daemon
Documentation=https://github.com/leefowlercu/agentic-memorizer
After=network.target

[Service]
Type=notify
User=%s
Group=%s
WorkingDirectory=%s
ExecStart=%s daemon start
Restart=on-failure
RestartSec=5s
TimeoutStartSec=60s
TimeoutStopSec=30s

# Security settings
NoNewPrivileges=true
PrivateTmp=true

# Environment
Environment="HOME=%s"

# Logging (optional - remove if using journald)
# StandardOutput=append:/var/log/agentic-memorizer/daemon.log
# StandardError=append:/var/log/agentic-memorizer/daemon.log

[Install]
WantedBy=multi-user.target
`, user, user, home, binaryPath, home)

	// Print the unit file
	fmt.Println(unitFile)

	// Print installation instructions
	fmt.Println("\n# Installation Instructions:")
	fmt.Println("#")
	fmt.Println("# For system-wide installation (requires root):")
	fmt.Printf("#   sudo tee /etc/systemd/system/agentic-memorizer.service <<'EOF'\n%sEOF\n", unitFile)
	fmt.Println("#   sudo systemctl daemon-reload")
	fmt.Println("#   sudo systemctl enable agentic-memorizer")
	fmt.Println("#   sudo systemctl start agentic-memorizer")
	fmt.Println("#")
	fmt.Println("# For user-level installation (no root required):")
	fmt.Println("#   mkdir -p ~/.config/systemd/user")
	fmt.Printf("#   tee ~/.config/systemd/user/agentic-memorizer.service <<'EOF'\n%sEOF\n", unitFile)
	fmt.Println("#   systemctl --user daemon-reload")
	fmt.Println("#   systemctl --user enable agentic-memorizer")
	fmt.Println("#   systemctl --user start agentic-memorizer")
	fmt.Println("#")
	fmt.Println("# Check status:")
	fmt.Println("#   systemctl status agentic-memorizer  # system-wide")
	fmt.Println("#   systemctl --user status agentic-memorizer  # user-level")
	fmt.Println("#")
	fmt.Println("# View logs:")
	fmt.Println("#   journalctl -u agentic-memorizer -f  # system-wide")
	fmt.Println("#   journalctl --user -u agentic-memorizer -f  # user-level")

	return nil
}
