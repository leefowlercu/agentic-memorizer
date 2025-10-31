# Init System Integration: Automatic Daemon Startup at Login

**Status**: Design Phase
**Author**: System Design
**Date**: 2025-10-31
**Target**: `agentic-memorizer init` command enhancement

## Executive Summary

This document outlines the implementation plan for integrating automatic daemon startup at login via native OS service managers (macOS `launchd` and Linux `systemd`). This functionality will be integrated into the `agentic-memorizer init` command, providing users with a seamless setup experience where the daemon automatically starts on system boot/login and maintains continuous operation.

### Goals

1. Provide one-command setup for automatic daemon startup via `agentic-memorizer init`
2. Support both macOS (launchd) and Linux (systemd) platforms
3. Maintain clean uninstall/disable capabilities
4. Follow platform best practices for service management
5. Provide clear user feedback and error handling

### Non-Goals

- Windows support (out of scope for initial implementation)
- Custom init systems (e.g., OpenRC, runit)
- System-wide daemons (user-level services only)

## Architecture Overview

### Current State

The agentic-memorizer currently has:
- Daemon implementation with lifecycle management (`daemon start/stop/status/restart/rebuild/logs`)
- PID file management at `~/.agentic-memorizer/daemon.pid`
- Signal handling (SIGTERM, SIGINT, SIGUSR1)
- Manual service file templates in `examples/` directory
- Basic daemon start capability in `init` command (foreground only)

### Proposed Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                   agentic-memorizer init                        │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  1. Detect Platform (macOS/Linux)                        │  │
│  │  2. Create Config & Memory Directory                     │  │
│  │  3. [Optional] Setup Claude Code Hooks                   │  │
│  │  4. [NEW] Install Service Definition                     │  │
│  │  5. [NEW] Enable & Start Service                         │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                                       │
┌───────▼────────┐                  ┌──────────▼──────────┐
│  macOS launchd │                  │   Linux systemd     │
├────────────────┤                  ├─────────────────────┤
│ Location:      │                  │ Location:           │
│  ~/Library/    │                  │  ~/.config/         │
│  LaunchAgents/ │                  │  systemd/user/      │
│                │                  │                     │
│ File:          │                  │ File:               │
│  com.agentic-  │                  │  agentic-memorizer  │
│  memorizer.    │                  │  .service           │
│  daemon.plist  │                  │                     │
│                │                  │                     │
│ Management:    │                  │ Management:         │
│  launchctl     │                  │  systemctl --user   │
└────────────────┘                  └─────────────────────┘
        │                                       │
        └───────────────────┬───────────────────┘
                            │
                ┌───────────▼──────────┐
                │  Daemon Process      │
                │  (Background)        │
                │                      │
                │  - File watching     │
                │  - Index updates     │
                │  - Auto-restart      │
                └──────────────────────┘
```

### Design Principles

1. **Platform Idiomatic**: Follow each platform's conventions and best practices
2. **User-Level Services**: Run as user services (not system-wide) for security and simplicity
3. **Non-Destructive**: Preserve existing configurations, allow clean uninstall
4. **Fail-Safe**: Graceful degradation if service installation fails
5. **Observable**: Clear feedback on success/failure, actionable error messages

## Technical Design

### Component Structure

```
internal/
└── service/
    ├── manager.go           # Platform-agnostic service management interface
    ├── launchd.go          # macOS launchd implementation
    ├── systemd.go          # Linux systemd implementation
    ├── detector.go         # Platform detection logic
    └── templates.go        # Service file templates (embedded)
```

### Core Interfaces

```go
// Package: internal/service

// ServiceManager defines platform-agnostic service operations
type ServiceManager interface {
    // Install creates and installs the service definition file
    Install(config ServiceConfig) error

    // Enable configures the service to start at login/boot
    Enable() error

    // Start immediately starts the service
    Start() error

    // Stop stops the running service
    Stop() error

    // Disable prevents the service from starting at login/boot
    Disable() error

    // Uninstall removes the service definition file
    Uninstall() error

    // IsInstalled checks if service definition exists
    IsInstalled() (bool, error)

    // IsRunning checks if service is currently active
    IsRunning() (bool, error)

    // Status returns detailed service status information
    Status() (*ServiceStatus, error)
}

// ServiceConfig holds configuration for service installation
type ServiceConfig struct {
    BinaryPath      string   // Full path to agentic-memorizer binary
    WorkingDir      string   // Working directory for daemon
    ConfigPath      string   // Path to config.yaml
    LogFile         string   // Path to daemon log file
    EnvVars         []string // Environment variables (e.g., ANTHROPIC_API_KEY)
    RunAtLoad       bool     // Start immediately after installation
    KeepAlive       bool     // Auto-restart on crash
    StandardOutPath string   // Optional stdout redirect
    StandardErrPath string   // Optional stderr redirect
}

// ServiceStatus represents the current state of the service
type ServiceStatus struct {
    Installed bool
    Running   bool
    Enabled   bool
    PID       int
    Uptime    time.Duration
    LastError string
}
```

### Platform Detection

```go
// Package: internal/service

import "runtime"

// DetectPlatform identifies the current operating system
func DetectPlatform() Platform {
    switch runtime.GOOS {
    case "darwin":
        return PlatformMacOS
    case "linux":
        return PlatformLinux
    default:
        return PlatformUnsupported
    }
}

// GetServiceManager returns the appropriate ServiceManager for the platform
func GetServiceManager() (ServiceManager, error) {
    switch DetectPlatform() {
    case PlatformMacOS:
        return NewLaunchdManager(), nil
    case PlatformLinux:
        return NewSystemdManager(), nil
    default:
        return nil, ErrUnsupportedPlatform
    }
}
```

### macOS launchd Implementation

#### Service File Location

```
~/Library/LaunchAgents/com.agentic-memorizer.daemon.plist
```

#### Template Structure

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <!-- Service Identification -->
    <key>Label</key>
    <string>com.agentic-memorizer.daemon</string>

    <!-- Binary and Arguments -->
    <key>ProgramArguments</key>
    <array>
        <string>{{.BinaryPath}}</string>
        <string>daemon</string>
        <string>start</string>
    </array>

    <!-- Working Directory -->
    <key>WorkingDirectory</key>
    <string>{{.WorkingDir}}</string>

    <!-- Environment Variables -->
    {{if .EnvVars}}
    <key>EnvironmentVariables</key>
    <dict>
        {{range .EnvVars}}
        <key>{{.Key}}</key>
        <string>{{.Value}}</string>
        {{end}}
    </dict>
    {{end}}

    <!-- Auto-Start Configuration -->
    <key>RunAtLoad</key>
    <true/>

    <!-- Auto-Restart Configuration -->
    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
        <key>Crashed</key>
        <true/>
    </dict>

    <!-- Restart Throttling -->
    <key>ThrottleInterval</key>
    <integer>60</integer>

    <!-- Logging -->
    <key>StandardOutPath</key>
    <string>{{.LogFile}}</string>
    <key>StandardErrorPath</key>
    <string>{{.LogFile}}</string>

    <!-- Process Type -->
    <key>ProcessType</key>
    <string>Background</string>

    <!-- Nice Level (lower priority) -->
    <key>Nice</key>
    <integer>5</integer>
</dict>
</plist>
```

#### Implementation Details

```go
// Package: internal/service

type LaunchdManager struct {
    label       string
    plistPath   string
}

func NewLaunchdManager() *LaunchdManager {
    homeDir, _ := os.UserHomeDir()
    return &LaunchdManager{
        label:     "com.agentic-memorizer.daemon",
        plistPath: filepath.Join(homeDir, "Library", "LaunchAgents", "com.agentic-memorizer.daemon.plist"),
    }
}

func (m *LaunchdManager) Install(config ServiceConfig) error {
    // 1. Ensure LaunchAgents directory exists
    dir := filepath.Dir(m.plistPath)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
    }

    // 2. Parse template with config
    plistContent, err := m.generatePlist(config)
    if err != nil {
        return fmt.Errorf("failed to generate plist: %w", err)
    }

    // 3. Write plist file atomically
    tmpPath := m.plistPath + ".tmp"
    if err := os.WriteFile(tmpPath, plistContent, 0644); err != nil {
        return fmt.Errorf("failed to write plist: %w", err)
    }

    if err := os.Rename(tmpPath, m.plistPath); err != nil {
        return fmt.Errorf("failed to install plist: %w", err)
    }

    return nil
}

func (m *LaunchdManager) Enable() error {
    // launchd services are enabled by loading them
    return m.Start()
}

func (m *LaunchdManager) Start() error {
    // Load the service with launchctl
    cmd := exec.Command("launchctl", "load", "-w", m.plistPath)
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("failed to load service: %w (output: %s)", err, output)
    }
    return nil
}

func (m *LaunchdManager) Stop() error {
    cmd := exec.Command("launchctl", "unload", "-w", m.plistPath)
    if output, err := cmd.CombinedOutput(); err != nil {
        // Ignore error if service not loaded
        if !strings.Contains(string(output), "Could not find specified service") {
            return fmt.Errorf("failed to unload service: %w (output: %s)", err, output)
        }
    }
    return nil
}

func (m *LaunchdManager) IsRunning() (bool, error) {
    cmd := exec.Command("launchctl", "list", m.label)
    output, err := cmd.CombinedOutput()
    if err != nil {
        // Service not running
        return false, nil
    }

    // Parse output to check PID
    lines := strings.Split(string(output), "\n")
    for _, line := range lines {
        if strings.Contains(line, "\"PID\" = ") {
            return !strings.Contains(line, "\"PID\" = 0"), nil
        }
    }
    return false, nil
}
```

#### Command Execution

```bash
# Install and enable
launchctl load -w ~/Library/LaunchAgents/com.agentic-memorizer.daemon.plist

# Disable and unload
launchctl unload -w ~/Library/LaunchAgents/com.agentic-memorizer.daemon.plist

# Check status
launchctl list com.agentic-memorizer.daemon

# Force stop (if unresponsive)
launchctl kill SIGTERM system/com.agentic-memorizer.daemon
```

### Linux systemd Implementation

#### Service File Location

```
~/.config/systemd/user/agentic-memorizer.service
```

#### Template Structure

```ini
[Unit]
Description=Agentic Memorizer Daemon
Documentation=https://github.com/leefowlercu/agentic-memorizer
After=network.target

[Service]
Type=simple
ExecStart={{.BinaryPath}} daemon start
WorkingDirectory={{.WorkingDir}}
Restart=on-failure
RestartSec=60s

# Environment variables
{{range .EnvVars}}
Environment="{{.Key}}={{.Value}}"
{{end}}

# Logging
StandardOutput=append:{{.LogFile}}
StandardError=append:{{.LogFile}}

# Security hardening
NoNewPrivileges=true
PrivateTmp=true

# Resource limits
LimitNOFILE=65536

[Install]
WantedBy=default.target
```

#### Implementation Details

```go
// Package: internal/service

type SystemdManager struct {
    serviceName string
    servicePath string
}

func NewSystemdManager() *SystemdManager {
    homeDir, _ := os.UserHomeDir()
    return &SystemdManager{
        serviceName: "agentic-memorizer",
        servicePath: filepath.Join(homeDir, ".config", "systemd", "user", "agentic-memorizer.service"),
    }
}

func (m *SystemdManager) Install(config ServiceConfig) error {
    // 1. Ensure systemd/user directory exists
    dir := filepath.Dir(m.servicePath)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("failed to create systemd user directory: %w", err)
    }

    // 2. Parse template with config
    serviceContent, err := m.generateService(config)
    if err != nil {
        return fmt.Errorf("failed to generate service file: %w", err)
    }

    // 3. Write service file atomically
    tmpPath := m.servicePath + ".tmp"
    if err := os.WriteFile(tmpPath, serviceContent, 0644); err != nil {
        return fmt.Errorf("failed to write service file: %w", err)
    }

    if err := os.Rename(tmpPath, m.servicePath); err != nil {
        return fmt.Errorf("failed to install service file: %w", err)
    }

    // 4. Reload systemd daemon
    cmd := exec.Command("systemctl", "--user", "daemon-reload")
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("failed to reload systemd: %w (output: %s)", err, output)
    }

    return nil
}

func (m *SystemdManager) Enable() error {
    cmd := exec.Command("systemctl", "--user", "enable", m.serviceName)
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("failed to enable service: %w (output: %s)", err, output)
    }
    return nil
}

func (m *SystemdManager) Start() error {
    cmd := exec.Command("systemctl", "--user", "start", m.serviceName)
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("failed to start service: %w (output: %s)", err, output)
    }
    return nil
}

func (m *SystemdManager) Stop() error {
    cmd := exec.Command("systemctl", "--user", "stop", m.serviceName)
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("failed to stop service: %w (output: %s)", err, output)
    }
    return nil
}

func (m *SystemdManager) Disable() error {
    cmd := exec.Command("systemctl", "--user", "disable", m.serviceName)
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("failed to disable service: %w (output: %s)", err, output)
    }
    return nil
}

func (m *SystemdManager) IsRunning() (bool, error) {
    cmd := exec.Command("systemctl", "--user", "is-active", m.serviceName)
    output, err := cmd.Output()
    if err != nil {
        // Exit code 3 means inactive
        return false, nil
    }
    return strings.TrimSpace(string(output)) == "active", nil
}

func (m *SystemdManager) Status() (*ServiceStatus, error) {
    status := &ServiceStatus{
        Installed: m.IsInstalled(),
    }

    // Get detailed status
    cmd := exec.Command("systemctl", "--user", "status", m.serviceName, "--no-pager")
    output, _ := cmd.CombinedOutput()

    // Parse output for status information
    lines := strings.Split(string(output), "\n")
    for _, line := range lines {
        if strings.Contains(line, "Active:") {
            status.Running = strings.Contains(line, "active (running)")
        }
        if strings.Contains(line, "Main PID:") {
            // Parse PID
            parts := strings.Fields(line)
            if len(parts) >= 3 {
                if pid, err := strconv.Atoi(parts[2]); err == nil {
                    status.PID = pid
                }
            }
        }
    }

    return status, nil
}
```

#### Command Execution

```bash
# Install service
systemctl --user daemon-reload

# Enable (auto-start at login)
systemctl --user enable agentic-memorizer.service

# Start immediately
systemctl --user start agentic-memorizer.service

# Check status
systemctl --user status agentic-memorizer.service

# View logs
journalctl --user -u agentic-memorizer.service -f

# Disable auto-start
systemctl --user disable agentic-memorizer.service

# Stop service
systemctl --user stop agentic-memorizer.service
```

### Init Command Integration

#### Updated Command Flow

```go
// Package: cmd/init

func runInit(cmd *cobra.Command, args []string) error {
    // ... existing initialization code ...

    // New: Service installation prompt/flag
    installService, _ := cmd.Flags().GetBool("install-service")
    enableService, _ := cmd.Flags().GetBool("enable-service")

    if !installService && !cmd.Flags().Changed("install-service") {
        // Interactive prompt
        installService = promptYesNo("Install system service for automatic daemon startup?")
    }

    if installService {
        if err := installDaemonService(cfg, enableService); err != nil {
            fmt.Fprintf(os.Stderr, "Warning: Failed to install service: %v\n", err)
            fmt.Fprintf(os.Stderr, "You can manually setup the service later using the examples/ directory.\n")
        } else {
            fmt.Println("✓ Daemon service installed successfully")
            if enableService {
                fmt.Println("✓ Daemon will start automatically at login")
            }
        }
    }

    return nil
}

func installDaemonService(cfg *config.Config, enable bool) error {
    // 1. Get service manager for platform
    mgr, err := service.GetServiceManager()
    if err != nil {
        return fmt.Errorf("unsupported platform: %w", err)
    }

    // 2. Check if already installed
    installed, err := mgr.IsInstalled()
    if err != nil {
        return fmt.Errorf("failed to check installation status: %w", err)
    }

    if installed {
        fmt.Println("Service already installed, skipping...")
        return nil
    }

    // 3. Find binary path
    binaryPath, err := hooks.FindBinaryPath()
    if err != nil {
        return fmt.Errorf("failed to find binary: %w", err)
    }

    // 4. Prepare service configuration
    homeDir, _ := os.UserHomeDir()
    serviceConfig := service.ServiceConfig{
        BinaryPath:      binaryPath,
        WorkingDir:      homeDir,
        ConfigPath:      cfg.GetConfigPath(),
        LogFile:         util.ExpandPath(cfg.Daemon.LogFile),
        RunAtLoad:       enable,
        KeepAlive:       true,
        EnvVars:         getEnvVars(cfg),
    }

    // 5. Install service
    if err := mgr.Install(serviceConfig); err != nil {
        return fmt.Errorf("failed to install service: %w", err)
    }

    // 6. Enable if requested
    if enable {
        if err := mgr.Enable(); err != nil {
            return fmt.Errorf("failed to enable service: %w", err)
        }

        // 7. Start immediately
        if err := mgr.Start(); err != nil {
            return fmt.Errorf("failed to start service: %w", err)
        }
    }

    return nil
}

func getEnvVars(cfg *config.Config) []string {
    var envVars []string

    // Pass through ANTHROPIC_API_KEY if set
    if apiKey := os.Getenv(cfg.Claude.APIKeyEnv); apiKey != "" {
        envVars = append(envVars, fmt.Sprintf("%s=%s", cfg.Claude.APIKeyEnv, apiKey))
    }

    // Pass through any other required env vars
    // (could be expanded for custom configurations)

    return envVars
}
```

#### New CLI Flags

```go
func init() {
    InitCmd.Flags().Bool("install-service", false, "Install system service for automatic daemon startup")
    InitCmd.Flags().Bool("enable-service", true, "Enable and start the service immediately (requires --install-service)")
    InitCmd.Flags().Bool("skip-service", false, "Skip service installation (override interactive prompt)")
}
```

#### Usage Examples

```bash
# Interactive (prompts for service installation)
agentic-memorizer init

# Install and enable service automatically
agentic-memorizer init --install-service --enable-service

# Install service but don't start yet
agentic-memorizer init --install-service --enable-service=false

# Skip service installation entirely
agentic-memorizer init --skip-service
```

### Uninstall Capabilities

#### New `daemon uninstall` Command

```go
// Package: cmd/daemon

var UninstallCmd = &cobra.Command{
    Use:   "uninstall",
    Short: "Uninstall the daemon service",
    Long: `Stop, disable, and remove the daemon service configuration.

This will:
1. Stop the daemon if running
2. Disable automatic startup
3. Remove the service configuration file

Your config, index, and memory files will NOT be deleted.`,
    RunE: runUninstall,
}

func runUninstall(cmd *cobra.Command, args []string) error {
    // 1. Get service manager
    mgr, err := service.GetServiceManager()
    if err != nil {
        return fmt.Errorf("unsupported platform: %w", err)
    }

    // 2. Check if installed
    installed, err := mgr.IsInstalled()
    if err != nil {
        return fmt.Errorf("failed to check installation: %w", err)
    }

    if !installed {
        fmt.Println("Service not installed, nothing to do")
        return nil
    }

    // 3. Stop if running
    running, _ := mgr.IsRunning()
    if running {
        fmt.Println("Stopping daemon...")
        if err := mgr.Stop(); err != nil {
            return fmt.Errorf("failed to stop daemon: %w", err)
        }
    }

    // 4. Disable
    fmt.Println("Disabling service...")
    if err := mgr.Disable(); err != nil {
        fmt.Fprintf(os.Stderr, "Warning: Failed to disable: %v\n", err)
    }

    // 5. Uninstall
    fmt.Println("Removing service configuration...")
    if err := mgr.Uninstall(); err != nil {
        return fmt.Errorf("failed to uninstall: %w", err)
    }

    fmt.Println("✓ Service uninstalled successfully")
    fmt.Println("\nNote: Your config, memory, and index files have not been deleted.")
    fmt.Println("To completely remove agentic-memorizer, run: rm -rf ~/.agentic-memorizer")

    return nil
}
```

#### Usage

```bash
# Uninstall the service
agentic-memorizer daemon uninstall

# Complete removal (service + data)
agentic-memorizer daemon uninstall
rm -rf ~/.agentic-memorizer
```

## Edge Cases and Error Handling

### Platform Detection Failures

**Scenario**: Running on unsupported OS (e.g., Windows, BSD)

**Handling**:
```go
if platform == PlatformUnsupported {
    fmt.Fprintf(os.Stderr, "Warning: Automatic service installation not supported on %s\n", runtime.GOOS)
    fmt.Fprintf(os.Stderr, "You can still run the daemon manually: agentic-memorizer daemon start\n")
    return nil // Non-fatal, continue with init
}
```

### Permission Denied Errors

**Scenario**: User lacks permissions to write service files

**Handling**:
```go
if os.IsPermission(err) {
    return fmt.Errorf("permission denied writing service file: %w\n\nTry:\n  sudo chown -R $USER ~/.config/systemd/user/", err)
}
```

### Service Already Running

**Scenario**: Service is already installed and running

**Handling**:
```go
if installed {
    running, _ := mgr.IsRunning()
    if running {
        fmt.Println("Service already installed and running")
        return nil
    }
    fmt.Println("Service installed but not running, starting...")
    return mgr.Start()
}
```

### Binary Not in PATH

**Scenario**: Binary installed in non-standard location

**Handling**:
```go
binaryPath, err := hooks.FindBinaryPath()
if err != nil {
    fmt.Fprintf(os.Stderr, "Warning: Could not auto-detect binary path\n")
    fmt.Fprintf(os.Stderr, "Please ensure agentic-memorizer is in your PATH or specify manually\n")
    return err
}
```

### API Key Not Available

**Scenario**: ANTHROPIC_API_KEY not set in environment

**Handling**:
```go
if apiKey := os.Getenv(cfg.Claude.APIKeyEnv); apiKey == "" {
    fmt.Fprintf(os.Stderr, "Warning: %s not set in environment\n", cfg.Claude.APIKeyEnv)
    fmt.Fprintf(os.Stderr, "The daemon may fail to start. Set this variable before enabling the service:\n")
    fmt.Fprintf(os.Stderr, "  export %s='your-api-key'\n", cfg.Claude.APIKeyEnv)
}
```

### systemd Not Available

**Scenario**: Linux system without systemd (rare)

**Handling**:
```go
cmd := exec.Command("systemctl", "--version")
if err := cmd.Run(); err != nil {
    return fmt.Errorf("systemctl not found: systemd may not be installed on this system")
}
```

### launchctl Failures

**Scenario**: macOS launchctl command fails unexpectedly

**Handling**:
```go
if err := cmd.Run(); err != nil {
    exitErr, ok := err.(*exec.ExitError)
    if ok {
        return fmt.Errorf("launchctl failed (exit %d): %s\n\nOutput: %s",
            exitErr.ExitCode(),
            err,
            exitErr.Stderr)
    }
    return fmt.Errorf("launchctl failed: %w", err)
}
```

## Testing Strategy

### Unit Tests

```go
// internal/service/manager_test.go

func TestLaunchdManager_Install(t *testing.T) {
    // Test plist generation
    // Test file writing
    // Test error conditions
}

func TestSystemdManager_Install(t *testing.T) {
    // Test service file generation
    // Test file writing
    // Test daemon-reload
}

func TestServiceConfig_Validation(t *testing.T) {
    // Test invalid paths
    // Test missing binary
    // Test environment variable parsing
}
```

### Integration Tests

```bash
# test/integration/service_test.sh

test_install_service() {
    # Run init with service installation
    ./agentic-memorizer init --install-service --enable-service

    # Verify service file exists
    # Verify service is running
    # Verify index is being updated
}

test_uninstall_service() {
    # Install first
    ./agentic-memorizer init --install-service

    # Uninstall
    ./agentic-memorizer daemon uninstall

    # Verify service removed
    # Verify daemon stopped
}

test_service_restart() {
    # Install service
    # Kill daemon process
    # Wait for auto-restart
    # Verify daemon running again
}
```

### Manual Testing Checklist

- [ ] macOS: Install service via init command
- [ ] macOS: Verify service starts at login (logout/login)
- [ ] macOS: Verify daemon auto-restarts after crash
- [ ] macOS: Verify logs are written correctly
- [ ] macOS: Uninstall service cleanly
- [ ] Linux: Install service via init command
- [ ] Linux: Verify service starts at login (logout/login)
- [ ] Linux: Verify daemon auto-restarts after crash
- [ ] Linux: Verify logs are written via journald
- [ ] Linux: Uninstall service cleanly
- [ ] Both: Verify existing files are not overwritten
- [ ] Both: Verify API key is passed to daemon
- [ ] Both: Verify graceful failure on unsupported platforms

## Documentation Updates

### README.md Updates

**Section: Installation**

```markdown
## Installation

### Quick Start (with Automatic Daemon)

The fastest way to get started is to use the init command with automatic service installation:

```bash
# Download and install binary (example)
curl -sSL https://github.com/leefowlercu/agentic-memorizer/releases/latest/download/agentic-memorizer-$(uname -s)-$(uname -m) -o agentic-memorizer
chmod +x agentic-memorizer
mv agentic-memorizer ~/.local/bin/

# Set your Anthropic API key
export ANTHROPIC_API_KEY='your-api-key-here'

# Initialize with automatic daemon setup
agentic-memorizer init --install-service --setup-hooks
```

This will:
1. Create configuration and memory directories
2. Install the daemon as a system service (auto-starts at login)
3. Setup Claude Code integration hooks
4. Start the daemon immediately

### Manual Installation

If you prefer to manage the daemon manually or are on an unsupported platform:

```bash
# Initialize without service
agentic-memorizer init --skip-service

# Start daemon manually
agentic-memorizer daemon start
```

## Daemon Management

### Automatic Startup (Recommended)

The daemon can be configured to start automatically at login:

```bash
# Install as a service during init
agentic-memorizer init --install-service

# Or install service after initialization
# (Note: This command will be added in implementation)
agentic-memorizer daemon install
```

### Manual Management

```bash
# Start daemon (foreground)
agentic-memorizer daemon start

# Stop daemon
agentic-memorizer daemon stop

# Check status
agentic-memorizer daemon status

# View logs
agentic-memorizer daemon logs -f
```

### Uninstalling the Service

```bash
# Stop and remove the service
agentic-memorizer daemon uninstall

# Complete removal (including all data)
agentic-memorizer daemon uninstall
rm -rf ~/.agentic-memorizer
```
```

### config.yaml.example Updates

```yaml
# Daemon Configuration
daemon:
  enabled: true  # Change to true by default (service installation implies enabled)

  # ... existing settings ...

  # Service Configuration (applied during 'init --install-service')
  service:
    # Auto-restart on failure
    keep_alive: true

    # Restart delay after crash (seconds)
    restart_delay: 60

    # Run at load (start immediately after installation)
    run_at_load: true
```

## Implementation Phases

### Phase 1: Foundation (Week 1)

**Goal**: Create service management abstraction and platform detection

**Tasks**:
1. Create `internal/service/` package structure
2. Define `ServiceManager` interface
3. Implement platform detection (`detector.go`)
4. Create service configuration structures
5. Write unit tests for platform detection
6. Document interfaces and design decisions

**Deliverables**:
- [ ] `internal/service/manager.go` - Interface definitions
- [ ] `internal/service/detector.go` - Platform detection
- [ ] `internal/service/manager_test.go` - Unit tests
- [ ] Design documentation

**Success Criteria**:
- Platform detection works on macOS, Linux, and unsupported systems
- Interface is clear and well-documented
- All unit tests pass

### Phase 2: macOS Implementation (Week 1-2)

**Goal**: Implement launchd service management

**Tasks**:
1. Create `internal/service/launchd.go`
2. Implement plist template (embedded)
3. Implement `LaunchdManager` methods:
   - Install
   - Enable
   - Start
   - Stop
   - Disable
   - Uninstall
   - IsInstalled
   - IsRunning
   - Status
4. Add launchctl command execution with error handling
5. Write unit tests for plist generation
6. Write integration tests for service lifecycle

**Deliverables**:
- [ ] `internal/service/launchd.go` - Full implementation
- [ ] `internal/service/launchd_test.go` - Unit tests
- [ ] `test/integration/launchd_test.sh` - Integration tests
- [ ] Example plist updated in `examples/`

**Success Criteria**:
- Service installs correctly on macOS
- Service starts at login
- Service auto-restarts on crash
- All tests pass

### Phase 3: Linux Implementation (Week 2)

**Goal**: Implement systemd service management

**Tasks**:
1. Create `internal/service/systemd.go`
2. Implement systemd unit template (embedded)
3. Implement `SystemdManager` methods:
   - Install
   - Enable
   - Start
   - Stop
   - Disable
   - Uninstall
   - IsInstalled
   - IsRunning
   - Status
4. Add systemctl command execution with error handling
5. Implement daemon-reload logic
6. Write unit tests for unit file generation
7. Write integration tests for service lifecycle

**Deliverables**:
- [ ] `internal/service/systemd.go` - Full implementation
- [ ] `internal/service/systemd_test.go` - Unit tests
- [ ] `test/integration/systemd_test.sh` - Integration tests
- [ ] Example unit file updated in `examples/`

**Success Criteria**:
- Service installs correctly on Linux
- Service starts at login
- Service auto-restarts on crash
- Logs integrate with journald
- All tests pass

### Phase 4: Init Command Integration (Week 2-3)

**Goal**: Integrate service installation into init command

**Tasks**:
1. Update `cmd/init/init.go`:
   - Add `--install-service` flag
   - Add `--enable-service` flag
   - Add `--skip-service` flag
   - Implement interactive prompts
   - Add service installation logic
2. Implement `installDaemonService()` function
3. Add comprehensive error handling
4. Update help text and usage examples
5. Write tests for flag combinations
6. Test interactive prompt flow

**Deliverables**:
- [ ] Updated `cmd/init/init.go`
- [ ] Unit tests for init command
- [ ] Integration tests for full init flow
- [ ] Updated CLI documentation

**Success Criteria**:
- `init --install-service` works on both platforms
- Interactive prompts provide clear guidance
- Errors are handled gracefully with actionable messages
- All tests pass

### Phase 5: Uninstall Command (Week 3)

**Goal**: Implement service uninstall capability

**Tasks**:
1. Create `cmd/daemon/uninstall.go`
2. Implement uninstall logic:
   - Stop running service
   - Disable auto-start
   - Remove service file
   - Preserve user data
3. Add confirmation prompt for safety
4. Add `--force` flag for non-interactive
5. Write tests for uninstall scenarios

**Deliverables**:
- [ ] `cmd/daemon/uninstall.go` - Uninstall command
- [ ] Unit tests for uninstall logic
- [ ] Integration tests for uninstall flow
- [ ] Updated help text

**Success Criteria**:
- Service uninstalls cleanly on both platforms
- User data is preserved
- No orphaned processes or files remain
- All tests pass

### Phase 6: Documentation and Polish (Week 3-4)

**Goal**: Complete documentation and user experience polish

**Tasks**:
1. Update README.md:
   - Installation instructions
   - Daemon management section
   - Troubleshooting guide
   - Service management examples
2. Update config.yaml.example with service options
3. Create troubleshooting guide in docs/
4. Update CHANGELOG.md
5. Create migration guide for existing users
6. Review all error messages for clarity
7. Add verbose logging for debugging

**Deliverables**:
- [ ] Updated README.md
- [ ] `docs/service-management.md` - Detailed guide
- [ ] `docs/troubleshooting.md` - Common issues
- [ ] `docs/migration.md` - Upgrade guide
- [ ] Updated CHANGELOG.md
- [ ] All examples updated

**Success Criteria**:
- Documentation is comprehensive and clear
- Common issues are documented with solutions
- Migration path for existing users is clear
- All examples work as documented

### Phase 7: Testing and Release (Week 4)

**Goal**: Comprehensive testing and release preparation

**Tasks**:
1. Run full test suite on macOS
2. Run full test suite on Linux (multiple distros)
3. Test upgrade scenarios (existing install → new version)
4. Test edge cases:
   - Missing dependencies
   - Permission issues
   - Corrupted service files
   - Concurrent installations
5. Performance testing:
   - Service startup time
   - Daemon restart time
   - Resource usage
6. Security review:
   - File permissions
   - API key handling
   - Service isolation
7. Create release notes
8. Tag release

**Deliverables**:
- [ ] Test report (all platforms)
- [ ] Performance benchmarks
- [ ] Security review report
- [ ] Release notes
- [ ] Tagged release

**Success Criteria**:
- All tests pass on both platforms
- No security vulnerabilities identified
- Performance meets targets (<2s startup)
- Release is ready for production use

## Implementation Checklist

### Pre-Implementation

- [ ] Review and approve design document
- [ ] Verify platform testing environments available (macOS, Linux)
- [ ] Set up test accounts for integration testing
- [ ] Create feature branch: `feature/service-integration`

### Phase 1: Foundation

- [ ] Create `internal/service/` directory
- [ ] Implement `ServiceManager` interface in `manager.go`
- [ ] Implement platform detection in `detector.go`
- [ ] Create `ServiceConfig` and `ServiceStatus` structures
- [ ] Write unit tests for platform detection
- [ ] Write unit tests for service configuration validation
- [ ] Document interface contracts and design decisions
- [ ] Code review: Foundation implementation
- [ ] Merge Phase 1 to feature branch

### Phase 2: macOS Implementation

- [ ] Create `internal/service/launchd.go`
- [ ] Embed plist template in code
- [ ] Implement `LaunchdManager.Install()`
- [ ] Implement `LaunchdManager.Enable()`
- [ ] Implement `LaunchdManager.Start()`
- [ ] Implement `LaunchdManager.Stop()`
- [ ] Implement `LaunchdManager.Disable()`
- [ ] Implement `LaunchdManager.Uninstall()`
- [ ] Implement `LaunchdManager.IsInstalled()`
- [ ] Implement `LaunchdManager.IsRunning()`
- [ ] Implement `LaunchdManager.Status()`
- [ ] Add comprehensive error handling for launchctl commands
- [ ] Write unit tests for plist template generation
- [ ] Write unit tests for all LaunchdManager methods
- [ ] Create integration test script: `test/integration/launchd_test.sh`
- [ ] Test service installation on macOS
- [ ] Test service enable/start on macOS
- [ ] Test service stop/disable on macOS
- [ ] Test service uninstall on macOS
- [ ] Test auto-restart after crash on macOS
- [ ] Test login persistence on macOS
- [ ] Update example plist in `examples/`
- [ ] Code review: macOS implementation
- [ ] Merge Phase 2 to feature branch

### Phase 3: Linux Implementation

- [ ] Create `internal/service/systemd.go`
- [ ] Embed systemd unit template in code
- [ ] Implement `SystemdManager.Install()`
- [ ] Implement `SystemdManager.Enable()`
- [ ] Implement `SystemdManager.Start()`
- [ ] Implement `SystemdManager.Stop()`
- [ ] Implement `SystemdManager.Disable()`
- [ ] Implement `SystemdManager.Uninstall()`
- [ ] Implement `SystemdManager.IsInstalled()`
- [ ] Implement `SystemdManager.IsRunning()`
- [ ] Implement `SystemdManager.Status()`
- [ ] Implement daemon-reload logic
- [ ] Add comprehensive error handling for systemctl commands
- [ ] Write unit tests for unit file template generation
- [ ] Write unit tests for all SystemdManager methods
- [ ] Create integration test script: `test/integration/systemd_test.sh`
- [ ] Test service installation on Linux (Ubuntu)
- [ ] Test service installation on Linux (Fedora)
- [ ] Test service enable/start on Linux
- [ ] Test service stop/disable on Linux
- [ ] Test service uninstall on Linux
- [ ] Test auto-restart after crash on Linux
- [ ] Test login persistence on Linux
- [ ] Test journald log integration on Linux
- [ ] Update example unit file in `examples/`
- [ ] Code review: Linux implementation
- [ ] Merge Phase 3 to feature branch

### Phase 4: Init Command Integration

- [ ] Add `--install-service` flag to `cmd/init/init.go`
- [ ] Add `--enable-service` flag to `cmd/init/init.go`
- [ ] Add `--skip-service` flag to `cmd/init/init.go`
- [ ] Implement interactive prompt for service installation
- [ ] Implement `installDaemonService()` function
- [ ] Implement `getEnvVars()` helper function
- [ ] Add platform detection check in init flow
- [ ] Add service already-installed check
- [ ] Add comprehensive error handling with user-friendly messages
- [ ] Add warning for missing API key
- [ ] Update help text for init command
- [ ] Update usage examples in init command
- [ ] Write unit tests for service installation logic
- [ ] Write unit tests for flag parsing
- [ ] Write integration test for `init --install-service`
- [ ] Write integration test for `init --skip-service`
- [ ] Test interactive prompt flow
- [ ] Test service installation via init on macOS
- [ ] Test service installation via init on Linux
- [ ] Test error handling (permission denied, missing binary, etc.)
- [ ] Code review: Init integration
- [ ] Merge Phase 4 to feature branch

### Phase 5: Uninstall Command

- [ ] Create `cmd/daemon/uninstall.go`
- [ ] Implement `runUninstall()` function
- [ ] Add service installed check
- [ ] Add service running check and stop logic
- [ ] Implement disable logic
- [ ] Implement service file removal logic
- [ ] Add confirmation prompt (optional)
- [ ] Add `--force` flag for non-interactive uninstall
- [ ] Add clear messaging about data preservation
- [ ] Update help text for uninstall command
- [ ] Write unit tests for uninstall logic
- [ ] Write integration test for uninstall command
- [ ] Test uninstall on macOS (service running)
- [ ] Test uninstall on macOS (service stopped)
- [ ] Test uninstall on Linux (service running)
- [ ] Test uninstall on Linux (service stopped)
- [ ] Test uninstall when service not installed
- [ ] Verify data preservation after uninstall
- [ ] Verify no orphaned processes after uninstall
- [ ] Code review: Uninstall command
- [ ] Merge Phase 5 to feature branch

### Phase 6: Documentation and Polish

#### README.md Updates
- [ ] Add "Installation" section with service setup
- [ ] Add "Quick Start" example with `--install-service`
- [ ] Add "Daemon Management" section
- [ ] Add "Service Management" subsection
- [ ] Add "Troubleshooting" section reference
- [ ] Update all CLI examples to mention service option
- [ ] Add platform-specific notes (macOS/Linux)

#### Additional Documentation
- [ ] Create `docs/service-management.md`:
  - [ ] Detailed service management guide
  - [ ] Platform-specific instructions
  - [ ] Configuration options
  - [ ] Advanced usage scenarios
- [ ] Create `docs/troubleshooting.md`:
  - [ ] Common installation issues
  - [ ] Permission problems
  - [ ] Service not starting
  - [ ] Auto-restart issues
  - [ ] Log access instructions
- [ ] Create `docs/migration.md`:
  - [ ] Upgrade guide for existing users
  - [ ] Manual → service migration steps
  - [ ] Breaking changes (if any)

#### Configuration Updates
- [ ] Update `config.yaml.example` with service configuration
- [ ] Add comments explaining service options
- [ ] Set `daemon.enabled: true` as default
- [ ] Add service-specific settings section

#### Examples Directory
- [ ] Update `examples/com.agentic-memorizer.daemon.plist`
- [ ] Update `examples/agentic-memorizer.service`
- [ ] Add `examples/README.md` with usage instructions
- [ ] Add manual installation instructions

#### Error Messages and UX
- [ ] Review all error messages for clarity
- [ ] Ensure actionable suggestions in all errors
- [ ] Add verbose logging for debugging service issues
- [ ] Test error message quality with fresh users

#### CHANGELOG
- [ ] Add new features to CHANGELOG.md
- [ ] Document breaking changes (if any)
- [ ] Credit contributors
- [ ] Add migration notes

#### Code Review
- [ ] Review all documentation for accuracy
- [ ] Review all examples for correctness
- [ ] Check for typos and grammar
- [ ] Verify all links work
- [ ] Ensure consistency across docs

### Phase 7: Testing and Release

#### Functional Testing - macOS
- [ ] Fresh install with `init --install-service` on macOS
- [ ] Verify service file created correctly
- [ ] Verify daemon starts automatically
- [ ] Logout and login, verify daemon auto-starts
- [ ] Kill daemon process, verify auto-restart
- [ ] Run `daemon status`, verify correct status
- [ ] Run `daemon logs`, verify log output
- [ ] Run `daemon uninstall`, verify clean removal
- [ ] Repeat on multiple macOS versions (if possible)

#### Functional Testing - Linux
- [ ] Fresh install with `init --install-service` on Ubuntu
- [ ] Fresh install with `init --install-service` on Fedora
- [ ] Fresh install with `init --install-service` on Arch (optional)
- [ ] Verify service file created correctly
- [ ] Verify daemon starts automatically
- [ ] Logout and login, verify daemon auto-starts
- [ ] Kill daemon process, verify auto-restart
- [ ] Run `daemon status`, verify correct status
- [ ] Run `daemon logs`, verify journald integration
- [ ] Run `daemon uninstall`, verify clean removal

#### Edge Case Testing
- [ ] Test installation without ANTHROPIC_API_KEY set
- [ ] Test installation in directory without write permissions
- [ ] Test concurrent installations (multiple terminals)
- [ ] Test installation with corrupted config file
- [ ] Test installation when service already exists
- [ ] Test installation when daemon already running (manual)
- [ ] Test uninstall when service file is corrupted
- [ ] Test uninstall when daemon process is hung
- [ ] Test platform detection on unsupported OS

#### Upgrade Testing
- [ ] Install previous version manually
- [ ] Upgrade to new version with service
- [ ] Verify smooth migration
- [ ] Verify existing config preserved
- [ ] Verify existing index preserved
- [ ] Test upgrade on macOS
- [ ] Test upgrade on Linux

#### Performance Testing
- [ ] Measure service startup time (target: <2s)
- [ ] Measure daemon restart time after crash
- [ ] Measure init command time with service installation
- [ ] Monitor memory usage of daemon under service
- [ ] Monitor CPU usage of daemon under service
- [ ] Test with large memory directories (1000+ files)

#### Security Review
- [ ] Verify service file permissions (644)
- [ ] Verify API key not leaked in logs
- [ ] Verify API key passed securely to daemon
- [ ] Verify daemon runs as user (not root)
- [ ] Verify no privilege escalation possible
- [ ] Verify log file permissions appropriate
- [ ] Review all exec.Command calls for injection risks
- [ ] Review all file operations for race conditions

#### Regression Testing
- [ ] Run full existing test suite
- [ ] Verify existing daemon commands still work
- [ ] Verify manual daemon start/stop still works
- [ ] Verify init without service still works
- [ ] Verify read command still works
- [ ] Verify hook integration still works
- [ ] Test on systems without systemd/launchd

#### Documentation Verification
- [ ] Follow README installation guide step-by-step (macOS)
- [ ] Follow README installation guide step-by-step (Linux)
- [ ] Verify all CLI examples work as documented
- [ ] Verify all troubleshooting steps work
- [ ] Verify migration guide is accurate
- [ ] Check for broken links in documentation
- [ ] Verify example files are up-to-date

#### Release Preparation
- [ ] Create comprehensive test report
- [ ] Document any known issues
- [ ] Create performance benchmark report
- [ ] Create security review summary
- [ ] Write release notes highlighting:
  - [ ] New service installation feature
  - [ ] Breaking changes (if any)
  - [ ] Migration instructions
  - [ ] Platform support
- [ ] Update version number in code
- [ ] Update version in CHANGELOG.md
- [ ] Create git tag for release
- [ ] Build binaries for release
- [ ] Test release binaries on fresh systems
- [ ] Write release announcement

#### Post-Release
- [ ] Merge feature branch to main
- [ ] Push release tag
- [ ] Publish release on GitHub
- [ ] Update documentation site (if applicable)
- [ ] Announce release to users
- [ ] Monitor for bug reports
- [ ] Create hotfix plan if needed

## Success Metrics

### User Experience
- **Target**: 95%+ of users can install service without issues
- **Target**: Service installation adds <5s to init time
- **Target**: Clear error messages for all failure modes

### Reliability
- **Target**: Daemon auto-restarts within 60s of crash
- **Target**: 99.9%+ service uptime under normal conditions
- **Target**: Zero data corruption from service crashes

### Performance
- **Target**: Service startup <2s on average hardware
- **Target**: Daemon restart <3s on average hardware
- **Target**: No measurable performance degradation vs manual start

### Documentation
- **Target**: Zero ambiguous or incomplete instructions
- **Target**: All common issues documented with solutions
- **Target**: 100% of examples work as documented

## Risks and Mitigations

### Risk: Platform-Specific Bugs

**Likelihood**: Medium
**Impact**: High

**Mitigation**:
- Comprehensive testing on multiple OS versions
- Platform-specific unit tests
- Fallback to manual installation on failure
- Clear error messages pointing to manual instructions

### Risk: Permission Issues

**Likelihood**: Medium
**Impact**: Medium

**Mitigation**:
- Check permissions before file operations
- Provide clear error messages with fix instructions
- Document common permission issues
- Provide manual fix commands in error output

### Risk: API Key Leakage

**Likelihood**: Low
**Impact**: Critical

**Mitigation**:
- Never log API keys
- Pass via environment variables only
- Warn if API key in config file
- Security review all logging code
- Test with API key monitoring

### Risk: Service Interference

**Likelihood**: Low
**Impact**: Medium

**Mitigation**:
- Check if daemon already running before service start
- Use unique service names
- Clean PID file management
- Proper signal handling

### Risk: Upgrade Failures

**Likelihood**: Medium
**Impact**: Medium

**Mitigation**:
- Preserve existing configs during upgrade
- Detect and migrate old service installations
- Provide rollback instructions
- Test upgrade path extensively

## Open Questions

1. **Should we support system-wide installation?**
   - Current design: User-level services only
   - Trade-off: System-wide would require root, but might be useful for shared machines
   - Decision: Start with user-level, add system-wide if requested

2. **Should we automatically migrate existing manual setups?**
   - Current design: Fresh installs only
   - Trade-off: Auto-migration is convenient but risky
   - Decision: Provide migration instructions, manual migration for now

3. **Should we add `daemon install` and `daemon enable` commands?**
   - Current design: Service installation only via `init --install-service`
   - Trade-off: Separate commands are more flexible but add complexity
   - Decision: Start with init integration, add separate commands if needed

4. **How should we handle multiple API keys (personal vs work)?**
   - Current design: Single API key from environment
   - Trade-off: Multiple profiles would be powerful but complex
   - Decision: Single key for now, document manual multi-profile setup

5. **Should logs go to service manager (journald/syslog) or daemon.log?**
   - Current design: daemon.log file
   - Trade-off: Service logs are more standard but harder to access
   - Decision: Keep daemon.log, optionally duplicate to system logs

## Appendix

### Platform Support Matrix

| Platform | Init System | Supported | Priority | Notes |
|----------|-------------|-----------|----------|-------|
| macOS 10.15+ | launchd | Yes | High | Primary development platform |
| macOS 10.14- | launchd | Probably | Low | May work, not tested |
| Ubuntu 20.04+ | systemd | Yes | High | Popular Linux distro |
| Fedora 35+ | systemd | Yes | Medium | RPM-based testing |
| Debian 11+ | systemd | Yes | Medium | Similar to Ubuntu |
| Arch Linux | systemd | Probably | Low | Community contributed |
| CentOS/RHEL 8+ | systemd | Probably | Low | Enterprise Linux |
| Alpine Linux | OpenRC | No | - | Different init system |
| Windows 10/11 | Task Scheduler | No | - | Out of scope for v1 |

### Reference Links

#### macOS launchd
- [launchd.plist man page](https://www.manpagez.com/man/5/launchd.plist/)
- [Creating Launch Daemons and Agents](https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/CreatingLaunchdJobs.html)
- [launchctl man page](https://www.manpagez.com/man/1/launchctl/)

#### Linux systemd
- [systemd.service man page](https://www.freedesktop.org/software/systemd/man/systemd.service.html)
- [systemd.unit man page](https://www.freedesktop.org/software/systemd/man/systemd.unit.html)
- [systemctl man page](https://www.freedesktop.org/software/systemd/man/systemctl.html)

#### Go Libraries
- [os/exec package](https://pkg.go.dev/os/exec)
- [text/template package](https://pkg.go.dev/text/template)

### Related Issues/PRs

- Initial daemon implementation: (reference PR number)
- Background indexing feature: (reference PR number)

### Contributors

- (List contributors as they work on this feature)

---

**Document Status**: Draft
**Last Updated**: 2025-10-31
**Next Review**: Before Phase 1 implementation
