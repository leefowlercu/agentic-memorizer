package container

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Runtime represents a container runtime
type Runtime string

const (
	// RuntimeDocker represents the Docker container runtime
	RuntimeDocker Runtime = "docker"
	// RuntimePodman represents the Podman container runtime
	RuntimePodman Runtime = "podman"
	// RuntimeNone represents no available container runtime
	RuntimeNone Runtime = ""
)

// ContainerName is the name used for the FalkorDB container
const ContainerName = "memorizer-falkordb"

// StartOptions configures FalkorDB container startup
type StartOptions struct {
	Port    int    // Redis port (default: 6379)
	DataDir string // Host directory for persistent data (optional)
	Detach  bool   // Run in background (default: true)
}

// IsDockerAvailable checks if Docker is installed and running
func IsDockerAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "info")
	return cmd.Run() == nil
}

// IsPodmanAvailable checks if Podman is installed and running
func IsPodmanAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "podman", "info")
	return cmd.Run() == nil
}

// GetRuntime returns the display name for a runtime
func GetRuntime(r Runtime) string {
	switch r {
	case RuntimeDocker:
		return "Docker"
	case RuntimePodman:
		return "Podman"
	default:
		return ""
	}
}

// IsFalkorDBRunning checks if the FalkorDB container is running in any runtime.
// Checks Docker first, then Podman.
func IsFalkorDBRunning(port int) bool {
	if IsFalkorDBRunningIn(RuntimeDocker, port) {
		return true
	}
	return IsFalkorDBRunningIn(RuntimePodman, port)
}

// IsFalkorDBRunningIn checks if the FalkorDB container is running in a specific runtime
func IsFalkorDBRunningIn(runtime Runtime, port int) bool {
	if runtime == RuntimeNone {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, string(runtime), "inspect", "-f", "{{.State.Running}}", ContainerName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}

// containerExistsIn checks if the FalkorDB container exists in a specific runtime (running or stopped)
func containerExistsIn(runtime Runtime) bool {
	if runtime == RuntimeNone {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, string(runtime), "inspect", ContainerName)
	return cmd.Run() == nil
}

// StartFalkorDB starts the FalkorDB container using the specified runtime.
// If a stopped container exists, it will be started.
// Otherwise, a new container will be created with the specified options.
func StartFalkorDB(runtime Runtime, opts StartOptions) error {
	if runtime == RuntimeNone {
		return fmt.Errorf("no container runtime specified")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Apply defaults
	if opts.Port == 0 {
		opts.Port = 6379
	}

	runtimeBin := string(runtime)

	// Check if container already exists in this runtime
	if containerExistsIn(runtime) {
		// Check if already running
		if IsFalkorDBRunningIn(runtime, opts.Port) {
			return nil // Already running
		}

		// Try to start existing container
		startCmd := exec.CommandContext(ctx, runtimeBin, "start", ContainerName)
		if err := startCmd.Run(); err != nil {
			// Container exists but won't start - remove it and create fresh
			removeCmd := exec.CommandContext(ctx, runtimeBin, "rm", "-f", ContainerName)
			if rmErr := removeCmd.Run(); rmErr != nil {
				return fmt.Errorf("failed to start existing container and failed to remove it; start: %w, remove: %v", err, rmErr)
			}
		} else {
			// Started successfully, wait for ready
			return waitForReady(ctx, runtime)
		}
	}

	// Build runtime-specific arguments
	var args []string
	switch runtime {
	case RuntimeDocker:
		args = buildDockerArgs(opts)
	case RuntimePodman:
		args = buildPodmanArgs(opts)
	default:
		return fmt.Errorf("unsupported runtime: %s", runtime)
	}

	cmd := exec.CommandContext(ctx, runtimeBin, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create container; %w; output: %s", err, strings.TrimSpace(string(output)))
	}

	// Wait for FalkorDB to be ready
	return waitForReady(ctx, runtime)
}

// buildDockerArgs builds the docker run arguments
func buildDockerArgs(opts StartOptions) []string {
	args := []string{
		"run",
		"--name", ContainerName,
		"-p", fmt.Sprintf("%d:6379", opts.Port),
		"-p", "3000:3000", // Browser UI
		"--restart", "unless-stopped",
	}

	if opts.DataDir != "" {
		args = append(args, "-v", fmt.Sprintf("%s:/data", opts.DataDir))
		args = append(args, "-e", "FALKORDB_DATA_PATH=/data")
	}

	if opts.Detach {
		args = append(args, "-d")
	}

	args = append(args, "falkordb/falkordb:latest")
	return args
}

// buildPodmanArgs builds the podman run arguments
// Uses --network=host for localhost connectivity (no port mapping needed)
func buildPodmanArgs(opts StartOptions) []string {
	args := []string{
		"run",
		"--name", ContainerName,
		"--network=host",
		"--restart", "unless-stopped",
	}

	if opts.DataDir != "" {
		args = append(args, "-v", fmt.Sprintf("%s:/data", opts.DataDir))
		args = append(args, "-e", "FALKORDB_DATA_PATH=/data")
	}

	if opts.Detach {
		args = append(args, "-d")
	}

	args = append(args, "falkordb/falkordb:latest")
	return args
}

// waitForReady waits for FalkorDB to respond to ping
func waitForReady(ctx context.Context, runtime Runtime) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	runtimeBin := string(runtime)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for FalkorDB to be ready")
		case <-ticker.C:
			// Try to ping FalkorDB via redis-cli inside container
			pingCmd := exec.CommandContext(ctx, runtimeBin, "exec", ContainerName, "redis-cli", "ping")
			output, err := pingCmd.Output()
			if err == nil && strings.TrimSpace(string(output)) == "PONG" {
				return nil
			}
		}
	}
}
