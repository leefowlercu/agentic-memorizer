package subcommands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
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

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config; %w", err)
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
		home = "/Users/" + user
	}

	// Get log file path from config
	logFile := cfg.Daemon.LogFile
	if logFile == "" {
		logFile = filepath.Join(home, ".agentic-memorizer", "daemon.log")
	}

	// Generate label from username
	label := fmt.Sprintf("com.%s.agentic-memorizer", user)

	// Generate launchd plist
	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>

	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>daemon</string>
		<string>start</string>
	</array>

	<key>WorkingDirectory</key>
	<string>%s</string>

	<key>RunAtLoad</key>
	<true/>

	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>

	<key>StandardOutPath</key>
	<string>%s</string>

	<key>StandardErrorPath</key>
	<string>%s</string>

	<key>EnvironmentVariables</key>
	<dict>
		<key>HOME</key>
		<string>%s</string>
	</dict>

	<key>ProcessType</key>
	<string>Background</string>

	<key>ThrottleInterval</key>
	<integer>30</integer>
</dict>
</plist>
`, label, binaryPath, home, logFile, logFile, home)

	// Print the plist
	fmt.Println(plistContent)

	// Print installation instructions
	plistPath := filepath.Join(home, "Library", "LaunchAgents", label+".plist")
	fmt.Println("\n# Installation Instructions:")
	fmt.Println("#")
	fmt.Println("# 1. Create LaunchAgents directory if it doesn't exist:")
	fmt.Printf("#    mkdir -p %s/Library/LaunchAgents\n", home)
	fmt.Println("#")
	fmt.Println("# 2. Save the plist file:")
	fmt.Printf("#    agentic-memorizer daemon launchctl > %s\n", plistPath)
	fmt.Println("#")
	fmt.Println("# 3. Load the service:")
	fmt.Printf("#    launchctl load %s\n", plistPath)
	fmt.Println("#")
	fmt.Println("# 4. Start the service:")
	fmt.Printf("#    launchctl start %s\n", label)
	fmt.Println("#")
	fmt.Println("# Management commands:")
	fmt.Printf("#   launchctl stop %s      # Stop the daemon\n", label)
	fmt.Printf("#   launchctl start %s     # Start the daemon\n", label)
	fmt.Printf("#   launchctl unload %s  # Disable autostart\n", plistPath)
	fmt.Println("#")
	fmt.Println("# View logs:")
	fmt.Printf("#   tail -f %s\n", logFile)
	fmt.Println("#")
	fmt.Println("# Check if running:")
	fmt.Printf("#   launchctl list | grep %s\n", label)

	return nil
}
