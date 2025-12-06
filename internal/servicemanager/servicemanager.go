package servicemanager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Platform represents a supported operating system platform
type Platform int

const (
	PlatformUnknown Platform = iota
	PlatformLinux
	PlatformDarwin
)

// String returns the string representation of the platform
func (p Platform) String() string {
	switch p {
	case PlatformLinux:
		return "linux"
	case PlatformDarwin:
		return "darwin"
	default:
		return "unknown"
	}
}

// DetectPlatform returns the current operating system platform
func DetectPlatform() Platform {
	switch runtime.GOOS {
	case "linux":
		return PlatformLinux
	case "darwin":
		return PlatformDarwin
	default:
		return PlatformUnknown
	}
}

// IsPlatformSupported returns true if the current platform supports service manager integration
func IsPlatformSupported() bool {
	return DetectPlatform() != PlatformUnknown
}

// GetBinaryPath attempts to locate the agentic-memorizer binary
// Tries in order: os.Executable(), ~/.local/bin, PATH
func GetBinaryPath() (string, error) {
	// Try to get the current executable path
	execPath, err := os.Executable()
	if err == nil {
		// Resolve symlinks
		resolvedPath, err := filepath.EvalSymlinks(execPath)
		if err == nil && filepath.Base(resolvedPath) == "agentic-memorizer" {
			return resolvedPath, nil
		}
	}

	// Try common installation paths
	home, err := os.UserHomeDir()
	if err == nil {
		commonPath := filepath.Join(home, ".local", "bin", "agentic-memorizer")
		if _, err := os.Stat(commonPath); err == nil {
			return commonPath, nil
		}
	}

	// Try PATH
	pathBinary, err := exec.LookPath("agentic-memorizer")
	if err == nil {
		return pathBinary, nil
	}

	return "", fmt.Errorf("could not locate agentic-memorizer binary")
}
