package container

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuntime_String(t *testing.T) {
	tests := []struct {
		runtime Runtime
		want    string
	}{
		{RuntimeDocker, "docker"},
		{RuntimePodman, "podman"},
		{RuntimeNone, ""},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.runtime.String(); got != tt.want {
				t.Errorf("Runtime.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRuntime_DisplayName(t *testing.T) {
	tests := []struct {
		runtime Runtime
		want    string
	}{
		{RuntimeDocker, "Docker"},
		{RuntimePodman, "Podman"},
		{RuntimeNone, "None"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.runtime.DisplayName(); got != tt.want {
				t.Errorf("Runtime.DisplayName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectRuntime(t *testing.T) {
	// This test verifies the function runs without panicking
	// Actual availability depends on the environment
	runtime := DetectRuntime()

	// Should be one of the valid values
	if runtime != RuntimeDocker && runtime != RuntimePodman && runtime != RuntimeNone {
		t.Errorf("DetectRuntime() returned invalid runtime: %v", runtime)
	}
}

func TestIsDockerAvailable(t *testing.T) {
	// This test just verifies the function runs without panicking
	// The actual result depends on whether Docker is installed
	_ = IsDockerAvailable()
}

func TestIsPodmanAvailable(t *testing.T) {
	// This test just verifies the function runs without panicking
	// The actual result depends on whether Podman is installed
	_ = IsPodmanAvailable()
}

func TestStartOptions_Defaults(t *testing.T) {
	opts := StartOptions{
		Port: 6379,
	}

	if opts.Port != 6379 {
		t.Errorf("expected port 6379, got %d", opts.Port)
	}

	if opts.DataDir != "" {
		t.Errorf("expected empty DataDir, got %s", opts.DataDir)
	}

	if opts.Detach != false {
		t.Error("expected Detach to be false by default")
	}
}

func TestEnsureDataDir_DefaultsToConfigDir(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	opts, err := ensureDataDir(StartOptions{})
	if err != nil {
		t.Fatalf("ensureDataDir() error = %v", err)
	}

	expected := filepath.Join(tempHome, ".config", "memorizer", "falkordb")
	if opts.DataDir != expected {
		t.Errorf("ensureDataDir() DataDir = %q, want %q", opts.DataDir, expected)
	}

	info, err := os.Stat(expected)
	if err != nil {
		t.Fatalf("expected data dir to exist; %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected data dir to be a directory")
	}
}

func TestEnsureDataDir_UsesProvidedDir(t *testing.T) {
	tempDir := t.TempDir()
	custom := filepath.Join(tempDir, "custom-data")

	opts, err := ensureDataDir(StartOptions{DataDir: custom})
	if err != nil {
		t.Fatalf("ensureDataDir() error = %v", err)
	}

	if opts.DataDir != custom {
		t.Errorf("ensureDataDir() DataDir = %q, want %q", opts.DataDir, custom)
	}

	info, err := os.Stat(custom)
	if err != nil {
		t.Fatalf("expected custom data dir to exist; %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected custom data dir to be a directory")
	}
}

func TestContainerName(t *testing.T) {
	if ContainerName != "memorizer-falkordb" {
		t.Errorf("expected container name 'memorizer-falkordb', got '%s'", ContainerName)
	}
}

func TestFalkorDBImage(t *testing.T) {
	if FalkorDBImage != "falkordb/falkordb:latest" {
		t.Errorf("expected image 'falkordb/falkordb:latest', got '%s'", FalkorDBImage)
	}
}
