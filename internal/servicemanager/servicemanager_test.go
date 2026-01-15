package servicemanager

import (
	"context"
	"runtime"
	"strings"
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

func TestGetConfigDir(t *testing.T) {
	configDir, err := GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir() error = %v", err)
	}

	if configDir == "" {
		t.Error("GetConfigDir() returned empty string")
	}

	// Should end with .config/memorizer
	if !strings.HasSuffix(configDir, ".config/memorizer") {
		t.Errorf("GetConfigDir() = %v, want suffix .config/memorizer", configDir)
	}
}

func TestGetDataDir(t *testing.T) {
	dataDir, err := GetDataDir()
	if err != nil {
		t.Fatalf("GetDataDir() error = %v", err)
	}

	if dataDir == "" {
		t.Error("GetDataDir() returned empty string")
	}

	// Should end with .config/memorizer/falkordb
	if !strings.HasSuffix(dataDir, ".config/memorizer/falkordb") {
		t.Errorf("GetDataDir() = %v, want suffix .config/memorizer/falkordb", dataDir)
	}
}

func TestServiceState_String(t *testing.T) {
	tests := []struct {
		state ServiceState
		want  string
	}{
		{ServiceStateEnabled, "enabled"},
		{ServiceStateDisabled, "disabled"},
		{ServiceStateNotInstalled, "not-installed"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("ServiceState.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewDaemonManager(t *testing.T) {
	// This test verifies factory function returns correct type for current platform
	manager, err := NewDaemonManager()

	switch runtime.GOOS {
	case "linux":
		if err != nil {
			t.Fatalf("NewDaemonManager() error = %v on linux", err)
		}
		if _, ok := manager.(*systemdManager); !ok {
			t.Errorf("NewDaemonManager() returned %T, want *systemdManager on linux", manager)
		}
	case "darwin":
		if err != nil {
			t.Fatalf("NewDaemonManager() error = %v on darwin", err)
		}
		if _, ok := manager.(*launchdManager); !ok {
			t.Errorf("NewDaemonManager() returned %T, want *launchdManager on darwin", manager)
		}
	case "windows":
		if err == nil {
			t.Error("NewDaemonManager() should return error on windows")
		}
	}
}

func TestNewDaemonManagerWithExecutor(t *testing.T) {
	executor := &mockExecutor{}

	manager, err := NewDaemonManagerWithExecutor(executor)

	switch runtime.GOOS {
	case "linux":
		if err != nil {
			t.Fatalf("NewDaemonManagerWithExecutor() error = %v on linux", err)
		}
		if _, ok := manager.(*systemdManager); !ok {
			t.Errorf("NewDaemonManagerWithExecutor() returned %T, want *systemdManager on linux", manager)
		}
	case "darwin":
		if err != nil {
			t.Fatalf("NewDaemonManagerWithExecutor() error = %v on darwin", err)
		}
		if _, ok := manager.(*launchdManager); !ok {
			t.Errorf("NewDaemonManagerWithExecutor() returned %T, want *launchdManager on darwin", manager)
		}
	case "windows":
		if err == nil {
			t.Error("NewDaemonManagerWithExecutor() should return error on windows")
		}
	}
}

func TestNewCommandExecutor(t *testing.T) {
	executor := NewCommandExecutor()

	if executor == nil {
		t.Error("NewCommandExecutor() returned nil")
	}

	if _, ok := executor.(*defaultExecutor); !ok {
		t.Errorf("NewCommandExecutor() returned %T, want *defaultExecutor", executor)
	}
}

// mockExecutor is a test double for CommandExecutor.
type mockExecutor struct {
	commands []executedCommand
	outputs  map[string]string
	errors   map[string]error
}

type executedCommand struct {
	name string
	args []string
}

func (m *mockExecutor) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	m.commands = append(m.commands, executedCommand{name: name, args: args})

	key := name
	if len(args) > 0 {
		key = name + " " + strings.Join(args, " ")
	}

	if err, ok := m.errors[key]; ok {
		return nil, err
	}

	if output, ok := m.outputs[key]; ok {
		return []byte(output), nil
	}

	return nil, nil
}
