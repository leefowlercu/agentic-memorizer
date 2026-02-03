// Package container provides container runtime detection and management for FalkorDB.
package container

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/servicemanager"
)

// ContainerName is the name used for the FalkorDB container.
const ContainerName = "memorizer-falkordb"

// FalkorDBImage is the Docker image for FalkorDB.
const FalkorDBImage = "falkordb/falkordb:latest"

// Runtime represents a container runtime.
type Runtime string

const (
	// RuntimeDocker represents the Docker runtime.
	RuntimeDocker Runtime = "docker"
	// RuntimePodman represents the Podman runtime.
	RuntimePodman Runtime = "podman"
	// RuntimeNone represents no available runtime.
	RuntimeNone Runtime = ""
)

// String returns the runtime as a string.
func (r Runtime) String() string {
	return string(r)
}

// DisplayName returns a human-readable name for the runtime.
func (r Runtime) DisplayName() string {
	switch r {
	case RuntimeDocker:
		return "Docker"
	case RuntimePodman:
		return "Podman"
	default:
		return "None"
	}
}

// StartOptions configures how to start a FalkorDB container.
type StartOptions struct {
	// Port is the host port to map to the container's Redis port (6379).
	Port int
	// DataDir is the host directory for persistent data.
	// If empty, it defaults to the application config directory (e.g. ~/.config/memorizer/falkordb).
	DataDir string
	// Detach runs the container in the background.
	Detach bool
}

func ensureDataDir(opts StartOptions) (StartOptions, error) {
	if opts.DataDir == "" {
		dataDir, err := servicemanager.GetDataDir()
		if err != nil {
			return opts, fmt.Errorf("failed to resolve FalkorDB data dir; %w", err)
		}
		opts.DataDir = dataDir
	}

	if err := os.MkdirAll(opts.DataDir, 0o755); err != nil {
		return opts, fmt.Errorf("failed to create FalkorDB data dir; %w", err)
	}

	return opts, nil
}

// StartPhase represents the current phase of container startup.
type StartPhase int

const (
	// PhaseCheckingContainer indicates checking if container exists.
	PhaseCheckingContainer StartPhase = iota
	// PhaseContainerExists indicates container already exists.
	PhaseContainerExists
	// PhaseStartingExisting indicates starting an existing container.
	PhaseStartingExisting
	// PhasePullingImage indicates pulling the container image.
	PhasePullingImage
	// PhaseCreatingContainer indicates creating a new container.
	PhaseCreatingContainer
	// PhaseWaitingReady indicates waiting for FalkorDB to be ready.
	PhaseWaitingReady
	// PhaseComplete indicates startup is complete.
	PhaseComplete
	// PhaseFailed indicates startup failed.
	PhaseFailed
)

// StartProgress represents a progress update during container startup.
type StartProgress struct {
	Phase   StartPhase
	Message string
	Err     error
}

// IsDockerAvailable checks if Docker is available on the system.
func IsDockerAvailable() bool {
	return isRuntimeAvailable("docker")
}

// IsPodmanAvailable checks if Podman is available on the system.
func IsPodmanAvailable() bool {
	return isRuntimeAvailable("podman")
}

// isRuntimeAvailable checks if a container runtime is available by running its info command.
func isRuntimeAvailable(runtime string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, runtime, "info")
	err := cmd.Run()
	available := err == nil
	slog.Debug("checking runtime availability", "runtime", runtime, "available", available)
	return available
}

// DetectRuntime returns the first available container runtime.
// It prefers Docker over Podman.
func DetectRuntime() Runtime {
	if IsDockerAvailable() {
		return RuntimeDocker
	}
	if IsPodmanAvailable() {
		return RuntimePodman
	}
	return RuntimeNone
}

// AvailableRuntimes returns all available container runtimes.
func AvailableRuntimes() []Runtime {
	slog.Debug("detecting available container runtimes")
	var runtimes []Runtime
	if IsDockerAvailable() {
		runtimes = append(runtimes, RuntimeDocker)
	}
	if IsPodmanAvailable() {
		runtimes = append(runtimes, RuntimePodman)
	}
	slog.Debug("available runtimes detected", "count", len(runtimes), "runtimes", runtimes)
	return runtimes
}

// containerExists checks if the FalkorDB container exists in the given runtime.
func containerExists(runtime Runtime) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, runtime.String(), "container", "inspect", ContainerName)
	return cmd.Run() == nil
}

// containerIsRunning checks if the FalkorDB container is running.
func containerIsRunning(runtime Runtime) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, runtime.String(), "container", "inspect", "-f", "{{.State.Running}}", ContainerName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}

// IsFalkorDBRunning checks if FalkorDB is running on the default port.
func IsFalkorDBRunning() bool {
	return IsFalkorDBRunningIn(RuntimeDocker) || IsFalkorDBRunningIn(RuntimePodman)
}

// IsFalkorDBRunningIn checks if FalkorDB is running in the specified runtime.
func IsFalkorDBRunningIn(runtime Runtime) bool {
	if runtime == RuntimeNone {
		return false
	}
	return containerIsRunning(runtime)
}

// buildDockerArgs builds the arguments for starting a FalkorDB container with Docker.
func buildDockerArgs(opts StartOptions) []string {
	args := []string{"run", "--name", ContainerName}

	if opts.Detach {
		args = append(args, "-d")
	}

	// Port mapping
	args = append(args, "-p", fmt.Sprintf("%d:6379", opts.Port))
	// FalkorDB browser UI port
	args = append(args, "-p", "3000:3000")

	// Data volume if specified
	if opts.DataDir != "" {
		args = append(args, "-v", fmt.Sprintf("%s:/data", opts.DataDir))
	}

	args = append(args, FalkorDBImage)
	return args
}

// buildPodmanArgs builds the arguments for starting a FalkorDB container with Podman.
func buildPodmanArgs(opts StartOptions) []string {
	args := []string{"run", "--name", ContainerName}

	if opts.Detach {
		args = append(args, "-d")
	}

	// Podman uses host network for simpler localhost access
	args = append(args, "--network=host")

	// Data volume if specified
	if opts.DataDir != "" {
		args = append(args, "-v", fmt.Sprintf("%s:/data", opts.DataDir))
	}

	args = append(args, FalkorDBImage)
	return args
}

// waitForReady waits for FalkorDB to be ready by pinging it using redis-cli inside the container.
func waitForReady(runtime Runtime, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		// Use redis-cli inside the container to avoid requiring it locally
		cmd := exec.CommandContext(ctx, runtime.String(), "exec", ContainerName, "redis-cli", "ping")
		output, err := cmd.Output()
		cancel()

		if err == nil && strings.TrimSpace(string(output)) == "PONG" {
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for FalkorDB to be ready")
}

// StartFalkorDB starts the FalkorDB container using the specified runtime.
func StartFalkorDB(runtime Runtime, opts StartOptions) error {
	slog.Info("starting FalkorDB container", "runtime", runtime, "port", opts.Port)

	if runtime == RuntimeNone {
		slog.Error("no container runtime available")
		return fmt.Errorf("no container runtime available")
	}

	var err error
	opts, err = ensureDataDir(opts)
	if err != nil {
		slog.Error("failed to prepare FalkorDB data directory", "error", err)
		return err
	}
	slog.Debug("using FalkorDB data directory", "path", opts.DataDir)

	// Check if container already exists
	if containerExists(runtime) {
		slog.Debug("container exists", "name", ContainerName)

		// If it's running, we're done
		if containerIsRunning(runtime) {
			slog.Info("FalkorDB container already running")
			return nil
		}

		// Start the existing container
		slog.Debug("starting existing container", "name", ContainerName)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, runtime.String(), "start", ContainerName)
		if err := cmd.Run(); err != nil {
			slog.Error("failed to start existing container", "error", err)
			return fmt.Errorf("failed to start existing container; %w", err)
		}
	} else {
		slog.Debug("container does not exist, creating new container", "name", ContainerName)

		// Create and start new container
		var args []string
		if runtime == RuntimeDocker {
			args = buildDockerArgs(opts)
		} else {
			args = buildPodmanArgs(opts)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		slog.Debug("running container command", "runtime", runtime, "args", args)
		cmd := exec.CommandContext(ctx, runtime.String(), args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			slog.Error("failed to start container", "error", err, "output", string(output))
			return fmt.Errorf("failed to start container; %s; %w", string(output), err)
		}
	}

	// Wait for FalkorDB to be ready
	slog.Debug("waiting for FalkorDB to be ready")
	if err := waitForReady(runtime, 30*time.Second); err != nil {
		slog.Error("FalkorDB failed to become ready", "error", err)
		return err
	}

	slog.Info("FalkorDB container started successfully", "port", opts.Port)
	return nil
}

// StopFalkorDB stops the FalkorDB container.
func StopFalkorDB(runtime Runtime) error {
	slog.Info("stopping FalkorDB container", "runtime", runtime)

	if runtime == RuntimeNone {
		slog.Debug("no runtime specified, nothing to stop")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, runtime.String(), "stop", ContainerName)
	if err := cmd.Run(); err != nil {
		slog.Error("failed to stop container", "error", err)
		return err
	}

	slog.Info("FalkorDB container stopped")
	return nil
}

// imageExists checks if the FalkorDB image exists locally.
func imageExists(runtime Runtime) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, runtime.String(), "image", "inspect", FalkorDBImage)
	return cmd.Run() == nil
}

// pullImage pulls the FalkorDB image.
func pullImage(runtime Runtime) error {
	slog.Info("pulling FalkorDB image", "image", FalkorDBImage, "runtime", runtime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, runtime.String(), "pull", FalkorDBImage)
	if err := cmd.Run(); err != nil {
		slog.Error("failed to pull image", "image", FalkorDBImage, "error", err)
		return err
	}

	slog.Info("image pulled successfully", "image", FalkorDBImage)
	return nil
}

// StartFalkorDBWithProgress starts the FalkorDB container and sends progress updates.
func StartFalkorDBWithProgress(runtime Runtime, opts StartOptions, progress chan<- StartProgress) {
	defer close(progress)

	slog.Info("starting FalkorDB container with progress", "runtime", runtime, "port", opts.Port)

	if runtime == RuntimeNone {
		slog.Error("no container runtime available")
		progress <- StartProgress{Phase: PhaseFailed, Err: fmt.Errorf("no container runtime available")}
		return
	}

	var err error
	opts, err = ensureDataDir(opts)
	if err != nil {
		slog.Error("failed to prepare FalkorDB data directory", "error", err)
		progress <- StartProgress{Phase: PhaseFailed, Err: err}
		return
	}
	slog.Debug("using FalkorDB data directory", "path", opts.DataDir)

	// Check if container already exists
	slog.Debug("checking for existing container", "name", ContainerName)
	progress <- StartProgress{Phase: PhaseCheckingContainer, Message: "Checking for existing container..."}

	if containerExists(runtime) {
		slog.Debug("container exists", "name", ContainerName)
		progress <- StartProgress{Phase: PhaseContainerExists, Message: "Found existing container"}

		// If it's running, we're done
		if containerIsRunning(runtime) {
			slog.Info("FalkorDB container already running")
			progress <- StartProgress{Phase: PhaseComplete, Message: "FalkorDB is already running"}
			return
		}

		// Start the existing container
		slog.Debug("starting existing container", "name", ContainerName)
		progress <- StartProgress{Phase: PhaseStartingExisting, Message: "Starting existing container..."}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, runtime.String(), "start", ContainerName)
		if err := cmd.Run(); err != nil {
			slog.Error("failed to start existing container", "error", err)
			progress <- StartProgress{Phase: PhaseFailed, Err: fmt.Errorf("failed to start existing container; %w", err)}
			return
		}
	} else {
		slog.Debug("container does not exist", "name", ContainerName)

		// Check if image exists, pull if needed
		if !imageExists(runtime) {
			slog.Debug("image does not exist, pulling", "image", FalkorDBImage)
			progress <- StartProgress{Phase: PhasePullingImage, Message: "Pulling FalkorDB image (this may take a few minutes)..."}

			if err := pullImage(runtime); err != nil {
				slog.Error("failed to pull image", "error", err)
				progress <- StartProgress{Phase: PhaseFailed, Err: fmt.Errorf("failed to pull image; %w", err)}
				return
			}
		}

		// Create and start new container
		slog.Debug("creating container", "name", ContainerName)
		progress <- StartProgress{Phase: PhaseCreatingContainer, Message: "Creating container..."}

		var args []string
		if runtime == RuntimeDocker {
			args = buildDockerArgs(opts)
		} else {
			args = buildPodmanArgs(opts)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		slog.Debug("running container command", "runtime", runtime, "args", args)
		cmd := exec.CommandContext(ctx, runtime.String(), args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			slog.Error("failed to start container", "error", err, "output", string(output))
			progress <- StartProgress{Phase: PhaseFailed, Err: fmt.Errorf("failed to start container; %s; %w", string(output), err)}
			return
		}
	}

	// Wait for FalkorDB to be ready
	slog.Debug("waiting for FalkorDB to be ready")
	progress <- StartProgress{Phase: PhaseWaitingReady, Message: "Waiting for FalkorDB to be ready..."}

	if err := waitForReady(runtime, 30*time.Second); err != nil {
		slog.Error("FalkorDB failed to become ready", "error", err)
		progress <- StartProgress{Phase: PhaseFailed, Err: err}
		return
	}

	slog.Info("FalkorDB container started successfully", "port", opts.Port)
	progress <- StartProgress{Phase: PhaseComplete, Message: "FalkorDB is ready"}
}
