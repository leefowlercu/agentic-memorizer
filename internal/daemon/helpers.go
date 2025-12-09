package daemon

import (
	"fmt"
	"os"
	"runtime"
)

// DetectServiceManager returns the appropriate service manager for the platform
func DetectServiceManager() string {
	switch runtime.GOOS {
	case "linux":
		// Check if systemd is available
		if _, err := os.Stat("/run/systemd/system"); err == nil {
			return "systemd"
		}
		return "linux"
	case "darwin":
		return "launchd"
	default:
		return ""
	}
}

// GetServiceManagerHint returns helpful hint for managing daemon as service
func GetServiceManagerHint(sm string) string {
	switch sm {
	case "systemd":
		return "\nTip: For automatic management, set up as a systemd service:\n" +
			"  agentic-memorizer daemon systemctl\n"
	case "launchd":
		return "\nTip: For automatic management, set up as a launchd service:\n" +
			"  agentic-memorizer daemon launchctl\n"
	case "linux":
		return "\nTip: Consider running in background:\n" +
			"  nohup agentic-memorizer daemon start &\n"
	default:
		return ""
	}
}

// IsServiceManaged attempts to detect if daemon is running under service manager
func IsServiceManaged() bool {
	// Check for systemd
	if os.Getenv("NOTIFY_SOCKET") != "" {
		return true
	}

	// Check for launchd
	if os.Getenv("XPC_SERVICE_NAME") != "" {
		return true
	}

	return false
}

// GetAlreadyRunningHelp returns platform-specific help for "already running" errors
func GetAlreadyRunningHelp() string {
	sm := DetectServiceManager()

	baseHelp := "The daemon appears to be already running.\n\n" +
		"If this is incorrect (stale PID file), try:\n" +
		"  1. Check status: agentic-memorizer daemon status\n" +
		"  2. Stop daemon: agentic-memorizer daemon stop\n" +
		"  3. Remove stale PID file: rm ~/.agentic-memorizer/daemon.pid"

	switch sm {
	case "systemd":
		baseHelp += "\n\nIf managed by systemd, use:\n" +
			"  systemctl --user status agentic-memorizer\n" +
			"  systemctl --user stop agentic-memorizer"
	case "launchd":
		baseHelp += "\n\nIf managed by launchd, use:\n" +
			"  launchctl list | grep agentic-memorizer\n" +
			"  launchctl stop com.$USER.agentic-memorizer"
	}

	return baseHelp
}

// GetNotRunningHelp returns platform-specific help for "not running" errors
func GetNotRunningHelp() string {
	sm := DetectServiceManager()

	switch sm {
	case "systemd":
		return "\n\nIf managed by systemd, check:\n" +
			"  systemctl --user status agentic-memorizer"
	case "launchd":
		return "\n\nIf managed by launchd, check:\n" +
			"  launchctl list | grep agentic-memorizer"
	}

	return ""
}

// GetSignalErrorHelp returns platform-specific help for signal errors
func GetSignalErrorHelp(pid int) string {
	sm := DetectServiceManager()

	msg := fmt.Sprintf("\n\nFailed to signal daemon (PID %d).\n"+
		"This may indicate the daemon is managed by a service manager.\n", pid)

	switch sm {
	case "systemd":
		msg += "\nTry: systemctl --user stop agentic-memorizer"
	case "launchd":
		msg += "\nTry: launchctl stop com.$USER.agentic-memorizer"
	}

	return msg
}
