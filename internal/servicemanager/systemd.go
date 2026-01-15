package servicemanager

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

const (
	// systemdServiceName is the systemd service name.
	systemdServiceName = "memorizer.service"
)

// systemdUnitTemplate is the template for the systemd unit file.
const systemdUnitTemplate = `[Unit]
Description=Memorizer Daemon - Knowledge Graph File Monitor
After=network.target

[Service]
Type=simple
ExecStart={{.BinaryPath}} daemon start
Restart=on-failure
RestartSec=5
StartLimitBurst=5
StartLimitIntervalSec=60

[Install]
WantedBy=default.target
`

// systemdManager implements DaemonManager for Linux using systemd user units.
type systemdManager struct {
	executor CommandExecutor
}

// newSystemdManager creates a new systemd-based daemon manager.
func newSystemdManager(executor CommandExecutor) *systemdManager {
	return &systemdManager{
		executor: executor,
	}
}

// getUnitPath returns the path to the systemd user unit file.
func getUnitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory; %w", err)
	}
	return filepath.Join(home, ".config", "systemd", "user", systemdServiceName), nil
}

// generateUnitFile generates the systemd unit file content.
func generateUnitFile() (string, error) {
	binaryPath := GetBinaryPath()

	data := struct {
		BinaryPath string
	}{
		BinaryPath: binaryPath,
	}

	tmpl, err := template.New("unit").Parse(systemdUnitTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse unit template; %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute unit template; %w", err)
	}

	return buf.String(), nil
}

// Install writes the systemd unit file and enables auto-start.
func (m *systemdManager) Install(ctx context.Context) error {
	unitPath, err := getUnitPath()
	if err != nil {
		return err
	}

	// Generate unit file content
	content, err := generateUnitFile()
	if err != nil {
		return err
	}

	// Ensure systemd user directory exists
	unitDir := filepath.Dir(unitPath)
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd user directory; %w", err)
	}

	// Write unit file
	if err := os.WriteFile(unitPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write unit file; %w", err)
	}

	// Reload systemd daemon
	_, err = m.executor.Run(ctx, "systemctl", "--user", "daemon-reload")
	if err != nil {
		return fmt.Errorf("failed to reload systemd daemon; %w", err)
	}

	// Enable the service
	_, err = m.executor.Run(ctx, "systemctl", "--user", "enable", systemdServiceName)
	if err != nil {
		return fmt.Errorf("failed to enable service; %w", err)
	}

	return nil
}

// Uninstall stops the service, disables auto-start, and removes the unit file.
func (m *systemdManager) Uninstall(ctx context.Context) error {
	unitPath, err := getUnitPath()
	if err != nil {
		return err
	}

	// Stop the service (ignoring errors if not running)
	_, _ = m.executor.Run(ctx, "systemctl", "--user", "stop", systemdServiceName)

	// Disable the service (ignoring errors if not enabled)
	_, _ = m.executor.Run(ctx, "systemctl", "--user", "disable", systemdServiceName)

	// Remove the unit file
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove unit file; %w", err)
	}

	// Reload systemd daemon
	_, _ = m.executor.Run(ctx, "systemctl", "--user", "daemon-reload")

	return nil
}

// StartDaemon starts the daemon via systemctl.
func (m *systemdManager) StartDaemon(ctx context.Context) error {
	_, err := m.executor.Run(ctx, "systemctl", "--user", "start", systemdServiceName)
	if err != nil {
		return fmt.Errorf("failed to start service; %w", err)
	}
	return nil
}

// StopDaemon stops the daemon via systemctl.
func (m *systemdManager) StopDaemon(ctx context.Context) error {
	_, err := m.executor.Run(ctx, "systemctl", "--user", "stop", systemdServiceName)
	if err != nil {
		return fmt.Errorf("failed to stop service; %w", err)
	}
	return nil
}

// Restart stops and starts the daemon.
func (m *systemdManager) Restart(ctx context.Context) error {
	_, err := m.executor.Run(ctx, "systemctl", "--user", "restart", systemdServiceName)
	if err != nil {
		return fmt.Errorf("failed to restart service; %w", err)
	}
	return nil
}

// Status returns the current daemon status.
func (m *systemdManager) Status(ctx context.Context) (DaemonStatus, error) {
	status := DaemonStatus{
		ServiceState: ServiceStateNotInstalled,
	}

	// Check if installed
	installed, err := m.IsInstalled()
	if err != nil {
		status.Error = err
		return status, nil
	}

	if !installed {
		return status, nil
	}

	// Get service status from systemctl
	output, err := m.executor.Run(ctx, "systemctl", "--user", "show", systemdServiceName,
		"--property=ActiveState,MainPID,UnitFileState")
	if err != nil {
		// Service is installed but systemctl failed
		status.ServiceState = ServiceStateDisabled
		return status, nil
	}

	// Parse systemctl show output
	status.ServiceState, status.PID, status.IsRunning = parseSystemctlOutput(string(output))

	// If running, fetch health
	if status.IsRunning {
		health, err := fetchDaemonHealth(ctx)
		if err == nil {
			status.Health = health
		}
	}

	return status, nil
}

// IsInstalled checks if the unit file exists.
func (m *systemdManager) IsInstalled() (bool, error) {
	unitPath, err := getUnitPath()
	if err != nil {
		return false, err
	}

	_, err = os.Stat(unitPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// parseSystemctlOutput parses the output of systemctl show.
// Returns service state, PID, and whether the service is running.
func parseSystemctlOutput(output string) (ServiceState, int, bool) {
	state := ServiceStateDisabled
	pid := 0
	running := false

	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		switch key {
		case "ActiveState":
			if value == "active" || value == "activating" {
				running = true
			}
		case "MainPID":
			if p, err := strconv.Atoi(value); err == nil && p > 0 {
				pid = p
			}
		case "UnitFileState":
			switch value {
			case "enabled", "enabled-runtime":
				state = ServiceStateEnabled
			case "disabled":
				state = ServiceStateDisabled
			}
		}
	}

	return state, pid, running
}
