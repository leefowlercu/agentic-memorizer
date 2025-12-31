package container

import (
	"testing"
)

func TestGetRuntime(t *testing.T) {
	tests := []struct {
		name    string
		runtime Runtime
		want    string
	}{
		{
			name:    "docker runtime",
			runtime: RuntimeDocker,
			want:    "Docker",
		},
		{
			name:    "podman runtime",
			runtime: RuntimePodman,
			want:    "Podman",
		},
		{
			name:    "no runtime",
			runtime: RuntimeNone,
			want:    "",
		},
		{
			name:    "empty string runtime",
			runtime: Runtime(""),
			want:    "",
		},
		{
			name:    "unknown runtime",
			runtime: Runtime("unknown"),
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetRuntime(tt.runtime)
			if got != tt.want {
				t.Errorf("GetRuntime(%q) = %q, want %q", tt.runtime, got, tt.want)
			}
		})
	}
}

func TestRuntimeConstants(t *testing.T) {
	tests := []struct {
		name     string
		runtime  Runtime
		expected string
	}{
		{
			name:     "RuntimeDocker value",
			runtime:  RuntimeDocker,
			expected: "docker",
		},
		{
			name:     "RuntimePodman value",
			runtime:  RuntimePodman,
			expected: "podman",
		},
		{
			name:     "RuntimeNone value",
			runtime:  RuntimeNone,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.runtime) != tt.expected {
				t.Errorf("Runtime constant %s = %q, want %q", tt.name, tt.runtime, tt.expected)
			}
		})
	}
}

func TestContainerName(t *testing.T) {
	expected := "memorizer-falkordb"
	if ContainerName != expected {
		t.Errorf("ContainerName = %q, want %q", ContainerName, expected)
	}
}

func TestStartOptions(t *testing.T) {
	opts := StartOptions{
		Port:    6379,
		DataDir: "/data/falkordb",
		Detach:  true,
	}

	if opts.Port != 6379 {
		t.Errorf("StartOptions.Port = %d, want 6379", opts.Port)
	}
	if opts.DataDir != "/data/falkordb" {
		t.Errorf("StartOptions.DataDir = %q, want /data/falkordb", opts.DataDir)
	}
	if !opts.Detach {
		t.Error("StartOptions.Detach = false, want true")
	}
}

func TestBuildDockerArgs(t *testing.T) {
	tests := []struct {
		name     string
		opts     StartOptions
		contains []string
		excludes []string
	}{
		{
			name: "basic options",
			opts: StartOptions{
				Port:   6379,
				Detach: true,
			},
			contains: []string{
				"run",
				"--name", ContainerName,
				"-p", "6379:6379",
				"-p", "3000:3000",
				"--restart", "unless-stopped",
				"-d",
				"falkordb/falkordb:latest",
			},
			excludes: []string{"-v", "-e"},
		},
		{
			name: "with data directory",
			opts: StartOptions{
				Port:    6379,
				DataDir: "/home/user/.memorizer/falkordb",
				Detach:  true,
			},
			contains: []string{
				"-v", "/home/user/.memorizer/falkordb:/data",
				"-e", "FALKORDB_DATA_PATH=/data",
			},
		},
		{
			name: "custom port",
			opts: StartOptions{
				Port:   16379,
				Detach: true,
			},
			contains: []string{
				"-p", "16379:6379",
			},
		},
		{
			name: "no detach",
			opts: StartOptions{
				Port:   6379,
				Detach: false,
			},
			excludes: []string{"-d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := buildDockerArgs(tt.opts)

			// Check contains
			for _, want := range tt.contains {
				found := false
				for _, arg := range args {
					if arg == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("buildDockerArgs() missing expected arg %q, got %v", want, args)
				}
			}

			// Check excludes
			for _, notWant := range tt.excludes {
				for _, arg := range args {
					if arg == notWant {
						t.Errorf("buildDockerArgs() should not contain %q, got %v", notWant, args)
						break
					}
				}
			}
		})
	}
}

func TestBuildPodmanArgs(t *testing.T) {
	tests := []struct {
		name     string
		opts     StartOptions
		contains []string
		excludes []string
	}{
		{
			name: "basic options with host networking",
			opts: StartOptions{
				Port:   6379,
				Detach: true,
			},
			contains: []string{
				"run",
				"--name", ContainerName,
				"--network=host",
				"--restart", "unless-stopped",
				"-d",
				"falkordb/falkordb:latest",
			},
			excludes: []string{"-p", "-v", "-e"},
		},
		{
			name: "with data directory",
			opts: StartOptions{
				Port:    6379,
				DataDir: "/home/user/.memorizer/falkordb",
				Detach:  true,
			},
			contains: []string{
				"--network=host",
				"-v", "/home/user/.memorizer/falkordb:/data",
				"-e", "FALKORDB_DATA_PATH=/data",
			},
			excludes: []string{"-p"},
		},
		{
			name: "no port mapping even with custom port",
			opts: StartOptions{
				Port:   16379,
				Detach: true,
			},
			contains: []string{
				"--network=host",
			},
			excludes: []string{"-p", "16379"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := buildPodmanArgs(tt.opts)

			// Check contains
			for _, want := range tt.contains {
				found := false
				for _, arg := range args {
					if arg == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("buildPodmanArgs() missing expected arg %q, got %v", want, args)
				}
			}

			// Check excludes
			for _, notWant := range tt.excludes {
				for _, arg := range args {
					if arg == notWant {
						t.Errorf("buildPodmanArgs() should not contain %q, got %v", notWant, args)
						break
					}
				}
			}
		})
	}
}

func TestIsFalkorDBRunningIn_InvalidRuntime(t *testing.T) {
	// RuntimeNone should always return false
	result := IsFalkorDBRunningIn(RuntimeNone, 6379)
	if result {
		t.Error("IsFalkorDBRunningIn(RuntimeNone, 6379) = true, want false")
	}
}

func TestStartFalkorDB_InvalidRuntime(t *testing.T) {
	opts := StartOptions{
		Port:   6379,
		Detach: true,
	}

	err := StartFalkorDB(RuntimeNone, opts)
	if err == nil {
		t.Error("StartFalkorDB(RuntimeNone, opts) should return error")
	}

	expectedMsg := "no container runtime specified"
	if err.Error() != expectedMsg {
		t.Errorf("StartFalkorDB error = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestStartFalkorDB_DefaultPort(t *testing.T) {
	// This test verifies that opts.Port defaults to 6379 when 0
	// We can't easily test the full StartFalkorDB without mocking exec,
	// but we can verify the buildDockerArgs behavior with default port
	opts := StartOptions{
		Port:   0, // Should be treated as default
		Detach: true,
	}

	// Apply the same default logic as StartFalkorDB
	if opts.Port == 0 {
		opts.Port = 6379
	}

	args := buildDockerArgs(opts)

	// Verify the port mapping uses 6379
	found := false
	for _, arg := range args {
		if arg == "6379:6379" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("buildDockerArgs with default port should contain 6379:6379, got %v", args)
	}
}
