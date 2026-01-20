package servicemanager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
)

const (
	// launchdServiceLabel is the launchd service identifier.
	launchdServiceLabel = "com.leefowlercu.memorizer"

	// launchdPlistName is the plist filename.
	launchdPlistName = "com.leefowlercu.memorizer.plist"
)

// launchdPlistTemplate is the template for the launchd plist file.
const launchdPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.Label}}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.BinaryPath}}</string>
        <string>daemon</string>
        <string>start</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
    </dict>
    <key>ThrottleInterval</key>
    <integer>10</integer>
</dict>
</plist>
`

// launchdManager implements DaemonManager for macOS using launchd.
type launchdManager struct {
	executor CommandExecutor
}

// newLaunchdManager creates a new launchd-based daemon manager.
func newLaunchdManager(executor CommandExecutor) *launchdManager {
	return &launchdManager{
		executor: executor,
	}
}

// getPlistPath returns the path to the launchd plist file.
func getPlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory; %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchdPlistName), nil
}

// generatePlist generates the launchd plist content.
func generatePlist() (string, error) {
	binaryPath := GetBinaryPath()

	data := struct {
		Label      string
		BinaryPath string
	}{
		Label:      launchdServiceLabel,
		BinaryPath: binaryPath,
	}

	tmpl, err := template.New("plist").Parse(launchdPlistTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse plist template; %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute plist template; %w", err)
	}

	return buf.String(), nil
}

// Install writes the launchd plist and enables auto-start.
func (m *launchdManager) Install(ctx context.Context) error {
	plistPath, err := getPlistPath()
	if err != nil {
		return err
	}

	// Generate plist content
	content, err := generatePlist()
	if err != nil {
		return err
	}

	// Ensure LaunchAgents directory exists
	launchAgentsDir := filepath.Dir(plistPath)
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory; %w", err)
	}

	// Write plist file
	if err := os.WriteFile(plistPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write plist file; %w", err)
	}

	// Load the service with launchctl (also enables it)
	_, err = m.executor.Run(ctx, "launchctl", "load", "-w", plistPath)
	if err != nil {
		return fmt.Errorf("failed to load service with launchctl; %w", err)
	}

	return nil
}

// Uninstall stops the service, disables auto-start, and removes the plist.
func (m *launchdManager) Uninstall(ctx context.Context) error {
	plistPath, err := getPlistPath()
	if err != nil {
		return err
	}

	// Unload the service (ignoring errors if not loaded)
	_, _ = m.executor.Run(ctx, "launchctl", "unload", plistPath)

	// Remove the plist file
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist file; %w", err)
	}

	return nil
}

// StartDaemon starts the daemon via launchctl.
func (m *launchdManager) StartDaemon(ctx context.Context) error {
	_, err := m.executor.Run(ctx, "launchctl", "start", launchdServiceLabel)
	if err != nil {
		return fmt.Errorf("failed to start service; %w", err)
	}
	return nil
}

// StopDaemon stops the daemon via launchctl.
func (m *launchdManager) StopDaemon(ctx context.Context) error {
	_, err := m.executor.Run(ctx, "launchctl", "stop", launchdServiceLabel)
	if err != nil {
		return fmt.Errorf("failed to stop service; %w", err)
	}
	return nil
}

// Restart stops and starts the daemon.
func (m *launchdManager) Restart(ctx context.Context) error {
	// Stop first (ignore errors if not running)
	_ = m.StopDaemon(ctx)

	// Give it a moment to stop
	time.Sleep(100 * time.Millisecond)

	// Start
	return m.StartDaemon(ctx)
}

// Status returns the current daemon status.
func (m *launchdManager) Status(ctx context.Context) (DaemonStatus, error) {
	status := DaemonStatus{
		ServiceState: ServiceStateNotInstalled,
	}

	// Check if installed
	installed, err := m.IsInstalled()
	if err != nil {
		status.Error = err
		return status, nil //nolint:nilerr // Return status with error field populated
	}

	if !installed {
		return status, nil
	}

	// Get service status from launchctl
	output, err := m.executor.Run(ctx, "launchctl", "list", launchdServiceLabel)
	if err != nil {
		// Service is installed but not loaded
		status.ServiceState = ServiceStateDisabled
		return status, nil //nolint:nilerr // Not running is a valid state, not an error
	}

	// Parse launchctl list output
	// Format: "PID\tStatus\tLabel" or just status info
	status.ServiceState = ServiceStateEnabled
	status.PID, status.IsRunning = parseLaunchctlOutput(string(output))

	// If running, fetch health
	if status.IsRunning {
		health, err := fetchDaemonHealth(ctx)
		if err == nil {
			status.Health = health
		}
	}

	return status, nil
}

// IsInstalled checks if the plist file exists.
func (m *launchdManager) IsInstalled() (bool, error) {
	plistPath, err := getPlistPath()
	if err != nil {
		return false, err
	}

	_, err = os.Stat(plistPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// parseLaunchctlOutput parses the output of launchctl list <label>.
// Returns PID and whether the service is running.
func parseLaunchctlOutput(output string) (int, bool) {
	// launchctl list output format varies, but typically includes:
	// {
	//   "PID" = 12345;
	//   ...
	// }
	// Or for a tabular format: PID\tStatus\tLabel

	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for "PID" = 12345; format
		if strings.HasPrefix(line, `"PID"`) {
			parts := strings.Split(line, "=")
			if len(parts) >= 2 {
				pidStr := strings.TrimSpace(parts[1])
				pidStr = strings.TrimSuffix(pidStr, ";")
				pidStr = strings.TrimSpace(pidStr)
				if pid, err := strconv.Atoi(pidStr); err == nil && pid > 0 {
					return pid, true
				}
			}
		}

		// Check for tabular format where first field is PID
		fields := strings.Fields(line)
		if len(fields) >= 1 {
			if pid, err := strconv.Atoi(fields[0]); err == nil && pid > 0 {
				return pid, true
			}
		}
	}

	return 0, false
}

// fetchDaemonHealth fetches health status from the daemon's /readyz endpoint.
func fetchDaemonHealth(ctx context.Context) (*DaemonHealth, error) {
	cfg := config.Get()
	if cfg == nil {
		return nil, fmt.Errorf("config not initialized")
	}

	url := fmt.Sprintf("http://%s:%d/readyz", cfg.Daemon.HTTPBind, cfg.Daemon.HTTPPort)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("health endpoint returned status %d", resp.StatusCode)
	}

	var health DaemonHealth
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, err
	}

	return &health, nil
}
