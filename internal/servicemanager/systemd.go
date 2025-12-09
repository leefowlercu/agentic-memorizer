package servicemanager

import (
	"fmt"
	"os"
	"path/filepath"
)

// SystemdConfig holds configuration for generating systemd unit files
type SystemdConfig struct {
	BinaryPath string
	User       string
	Home       string
	LogFile    string
}

// GenerateUserUnit generates a systemd user-level unit file
func GenerateUserUnit(cfg SystemdConfig) string {
	return fmt.Sprintf(`[Unit]
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

[Install]
WantedBy=default.target
`, cfg.User, cfg.User, cfg.Home, cfg.BinaryPath, cfg.Home)
}

// GenerateSystemUnit generates a systemd system-level unit file
func GenerateSystemUnit(cfg SystemdConfig) string {
	return fmt.Sprintf(`[Unit]
Description=Agentic Memorizer Daemon
Documentation=https://github.com/leefowlercu/agentic-memorizer
After=network.target

[Service]
Type=notify
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

[Install]
WantedBy=multi-user.target
`, cfg.Home, cfg.BinaryPath, cfg.Home)
}

// GetUserUnitPath returns the path for a user-level systemd unit file
func GetUserUnitPath(home string) (string, error) {
	path := filepath.Join(home, ".config", "systemd", "user", "memorizer.service")
	return path, nil
}

// GetSystemUnitPath returns the path for a system-level systemd unit file
func GetSystemUnitPath() string {
	return "/etc/systemd/system/memorizer.service"
}

// InstallUserUnit writes a user-level systemd unit file
func InstallUserUnit(unitContent string, home string) error {
	unitPath, err := GetUserUnitPath(home)
	if err != nil {
		return fmt.Errorf("failed to get unit path; %w", err)
	}

	// Create parent directory
	unitDir := filepath.Dir(unitPath)
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		return fmt.Errorf("failed to create unit directory; %w", err)
	}

	// Write unit file
	if err := os.WriteFile(unitPath, []byte(unitContent), 0644); err != nil {
		return fmt.Errorf("failed to write unit file; %w", err)
	}

	return nil
}

// GetSystemdUserInstructions returns formatted instructions for user-level systemd setup
func GetSystemdUserInstructions(unitPath string) string {
	return fmt.Sprintf(`User-level systemd service installed at:
  %s

To enable and start the service:
  systemctl --user daemon-reload
  systemctl --user enable memorizer
  systemctl --user start memorizer

To check status:
  systemctl --user status memorizer

To view logs:
  journalctl --user -u memorizer -f`, unitPath)
}

// GetSystemdSystemInstructions returns formatted instructions for system-level systemd setup
func GetSystemdSystemInstructions(unitContent string) string {
	systemPath := GetSystemUnitPath()
	return fmt.Sprintf(`System-level installation requires root privileges.

1. Save the unit file:
   sudo tee %s <<'EOF'
%sEOF

2. Reload systemd and enable the service:
   sudo systemctl daemon-reload
   sudo systemctl enable memorizer
   sudo systemctl start memorizer

3. Check status:
   sudo systemctl status memorizer

4. View logs:
   journalctl -u memorizer -f`, systemPath, unitContent)
}
