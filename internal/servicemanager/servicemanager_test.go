package servicemanager

import (
	"runtime"
	"testing"
)

func TestPlatform_String(t *testing.T) {
	tests := []struct {
		platform Platform
		want     string
	}{
		{PlatformLinux, "linux"},
		{PlatformMacOS, "darwin"},
		{PlatformWindows, "windows"},
		{PlatformUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.platform.String(); got != tt.want {
				t.Errorf("Platform.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectPlatform(t *testing.T) {
	platform := DetectPlatform()

	// Should match the current runtime
	switch runtime.GOOS {
	case "linux":
		if platform != PlatformLinux {
			t.Errorf("expected PlatformLinux on linux, got %v", platform)
		}
	case "darwin":
		if platform != PlatformMacOS {
			t.Errorf("expected PlatformMacOS on darwin, got %v", platform)
		}
	case "windows":
		if platform != PlatformWindows {
			t.Errorf("expected PlatformWindows on windows, got %v", platform)
		}
	}
}

func TestIsPlatformSupported(t *testing.T) {
	tests := []struct {
		platform Platform
		want     bool
	}{
		{PlatformLinux, true},
		{PlatformMacOS, true},
		{PlatformWindows, false},
		{PlatformUnknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.platform.String(), func(t *testing.T) {
			if got := IsPlatformSupported(tt.platform); got != tt.want {
				t.Errorf("IsPlatformSupported(%v) = %v, want %v", tt.platform, got, tt.want)
			}
		})
	}
}

func TestGetBinaryPath(t *testing.T) {
	// GetBinaryPath should return a non-empty string
	path := GetBinaryPath()

	if path == "" {
		t.Error("GetBinaryPath() returned empty string")
	}
}
