// Package servicemanager provides platform detection and service management utilities.
package servicemanager

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Platform represents an operating system platform.
type Platform string

const (
	// PlatformLinux represents Linux.
	PlatformLinux Platform = "linux"
	// PlatformMacOS represents macOS.
	PlatformMacOS Platform = "darwin"
	// PlatformWindows represents Windows.
	PlatformWindows Platform = "windows"
	// PlatformUnknown represents an unknown platform.
	PlatformUnknown Platform = "unknown"
)

// String returns the platform as a string.
func (p Platform) String() string {
	return string(p)
}

// DetectPlatform returns the current platform.
func DetectPlatform() Platform {
	switch runtime.GOOS {
	case "linux":
		return PlatformLinux
	case "darwin":
		return PlatformMacOS
	case "windows":
		return PlatformWindows
	default:
		return PlatformUnknown
	}
}

// IsPlatformSupported returns true if the platform is supported.
// Currently only Linux and macOS are supported.
func IsPlatformSupported(p Platform) bool {
	return p == PlatformLinux || p == PlatformMacOS
}

// GetBinaryPath returns the path to the memorizer binary.
// It checks in order:
// 1. The current executable path
// 2. ~/.local/bin/memorizer
// 3. PATH lookup
func GetBinaryPath() string {
	// Try current executable
	if exe, err := os.Executable(); err == nil {
		return exe
	}

	// Try ~/.local/bin
	if home, err := os.UserHomeDir(); err == nil {
		localBin := filepath.Join(home, ".local", "bin", "memorizer")
		if _, err := os.Stat(localBin); err == nil {
			return localBin
		}
	}

	// Try PATH lookup
	if path, err := exec.LookPath("memorizer"); err == nil {
		return path
	}

	// Fallback to just "memorizer" (will use PATH at runtime)
	return "memorizer"
}

// GetConfigDir returns the configuration directory for the current platform.
func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "memorizer"), nil
}

// GetDataDir returns the data directory for FalkorDB persistence.
func GetDataDir() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "falkordb"), nil
}
