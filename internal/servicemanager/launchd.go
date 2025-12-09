package servicemanager

import (
	"fmt"
	"os"
	"path/filepath"
)

// LaunchdConfig holds configuration for generating launchd plist files
type LaunchdConfig struct {
	BinaryPath string
	User       string
	Home       string
	LogFile    string
}

// GeneratePlist generates a launchd property list (plist) file
func GeneratePlist(cfg LaunchdConfig) string {
	label := fmt.Sprintf("com.%s.memorizer", cfg.User)

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
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
`, label, cfg.BinaryPath, cfg.Home, cfg.LogFile, cfg.LogFile, cfg.Home)
}

// GetUserAgentPath returns the path for a user-level launchd agent plist
func GetUserAgentPath(home, user string) (string, error) {
	path := filepath.Join(home, "Library", "LaunchAgents", fmt.Sprintf("com.%s.memorizer.plist", user))
	return path, nil
}

// GetSystemDaemonPath returns the path for a system-level launchd daemon plist
func GetSystemDaemonPath(user string) string {
	return fmt.Sprintf("/Library/LaunchDaemons/com.%s.memorizer.plist", user)
}

// InstallUserAgent writes a user-level launchd agent plist file
func InstallUserAgent(plistContent, plistPath string) error {
	// Create parent directory
	plistDir := filepath.Dir(plistPath)
	if err := os.MkdirAll(plistDir, 0755); err != nil {
		return fmt.Errorf("failed to create plist directory; %w", err)
	}

	// Write plist file
	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("failed to write plist file; %w", err)
	}

	return nil
}

// GetLaunchdUserInstructions returns formatted instructions for user-level launchd setup
func GetLaunchdUserInstructions(plistPath, user string) string {
	label := fmt.Sprintf("com.%s.memorizer", user)

	return fmt.Sprintf(`User-level launchd agent installed at:
  %s

To load and start the service:
  launchctl bootstrap gui/$(id -u) %s
  launchctl enable gui/$(id -u)/%s
  launchctl kickstart -k gui/$(id -u)/%s

To check if running:
  launchctl list | grep %s

To stop and unload:
  launchctl bootout gui/$(id -u)/%s`, plistPath, plistPath, label, label, label, plistPath)
}

// GetLaunchdSystemInstructions returns formatted instructions for system-level launchd setup
func GetLaunchdSystemInstructions(plistContent, plistPath string) string {
	return fmt.Sprintf(`System-level installation requires root privileges.

1. Save the plist file:
   sudo tee %s <<'EOF'
%sEOF

2. Set permissions:
   sudo chmod 644 %s
   sudo chown root:wheel %s

3. Load and start the service:
   sudo launchctl bootstrap system %s
   sudo launchctl enable system/%s
   sudo launchctl kickstart -k system/%s

4. Check if running:
   sudo launchctl list | grep memorizer

5. To stop and unload:
   sudo launchctl bootout system/%s`, plistPath, plistContent, plistPath, plistPath, plistPath,
		filepath.Base(plistPath), filepath.Base(plistPath), plistPath)
}
