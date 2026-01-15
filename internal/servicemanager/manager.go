// Package servicemanager provides platform detection and service management utilities.
package servicemanager

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// ServiceState represents the installation state of the service.
type ServiceState string

const (
	// ServiceStateEnabled indicates the service is installed and enabled for auto-start.
	ServiceStateEnabled ServiceState = "enabled"

	// ServiceStateDisabled indicates the service is installed but not enabled for auto-start.
	ServiceStateDisabled ServiceState = "disabled"

	// ServiceStateNotInstalled indicates the service is not installed.
	ServiceStateNotInstalled ServiceState = "not-installed"
)

// String returns the service state as a string.
func (s ServiceState) String() string {
	return string(s)
}

// DaemonHealth represents the health status from the daemon's /readyz endpoint.
type DaemonHealth struct {
	// Status is the overall daemon health: "healthy", "degraded", or "unhealthy".
	Status string `json:"status"`

	// Ready indicates whether the daemon can serve requests.
	Ready bool `json:"ready"`

	// Uptime is how long the daemon has been running.
	Uptime time.Duration `json:"uptime"`

	// Components contains per-component health status.
	Components map[string]ComponentHealthInfo `json:"components,omitempty"`
}

// ComponentHealthInfo represents the health status of a single daemon component.
type ComponentHealthInfo struct {
	// Status is the component health state: "running", "failed", or "stopped".
	Status string `json:"status"`

	// Error contains the error message if status is "failed".
	Error string `json:"error,omitempty"`

	// LastChecked is when the health was last evaluated.
	LastChecked time.Time `json:"last_checked"`
}

// DaemonStatus represents the current status of the daemon service.
type DaemonStatus struct {
	// IsRunning indicates whether the daemon process is running.
	IsRunning bool

	// PID is the process ID of the daemon (0 if not running).
	PID int

	// ServiceState indicates the installation state of the service.
	ServiceState ServiceState

	// Health contains the daemon health from /readyz endpoint (nil if not running or unreachable).
	Health *DaemonHealth

	// Error contains any error encountered while getting status.
	Error error
}

// DaemonManager provides platform-agnostic daemon service management.
type DaemonManager interface {
	// Install writes the service file and enables auto-start.
	Install(ctx context.Context) error

	// Uninstall stops the service, disables auto-start, and removes the service file.
	Uninstall(ctx context.Context) error

	// StartDaemon starts the daemon via the system service manager.
	StartDaemon(ctx context.Context) error

	// StopDaemon stops the daemon via the system service manager.
	StopDaemon(ctx context.Context) error

	// Restart stops and starts the daemon.
	Restart(ctx context.Context) error

	// Status returns detailed daemon status including health.
	Status(ctx context.Context) (DaemonStatus, error)

	// IsInstalled checks if the service file exists.
	IsInstalled() (bool, error)
}

// CommandExecutor abstracts command execution for testability.
type CommandExecutor interface {
	// Run executes a command and returns its combined output.
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// defaultExecutor implements CommandExecutor using os/exec.
type defaultExecutor struct{}

// Run executes a command using os/exec.
func (e *defaultExecutor) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

// NewCommandExecutor returns the default command executor.
func NewCommandExecutor() CommandExecutor {
	return &defaultExecutor{}
}

// NewDaemonManager returns the appropriate platform-specific DaemonManager implementation.
// Returns an error if the platform is not supported.
func NewDaemonManager() (DaemonManager, error) {
	return NewDaemonManagerWithExecutor(NewCommandExecutor())
}

// NewDaemonManagerWithExecutor returns a DaemonManager with a custom command executor.
// This is primarily used for testing.
func NewDaemonManagerWithExecutor(executor CommandExecutor) (DaemonManager, error) {
	platform := DetectPlatform()
	if !IsPlatformSupported(platform) {
		return nil, fmt.Errorf("platform %s is not supported", platform)
	}

	switch platform {
	case PlatformMacOS:
		return newLaunchdManager(executor), nil
	case PlatformLinux:
		return newSystemdManager(executor), nil
	default:
		return nil, fmt.Errorf("unexpected platform: %s", platform)
	}
}
