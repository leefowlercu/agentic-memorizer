package docker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ContainerName is the name used for the FalkorDB container
const ContainerName = "memorizer-falkordb"

// StartOptions configures FalkorDB container startup
type StartOptions struct {
	Port    int    // Redis port (default: 6379)
	DataDir string // Host directory for persistent data (optional)
	Detach  bool   // Run in background (default: true)
}

// IsAvailable checks if Docker is installed and running
func IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "info")
	return cmd.Run() == nil
}

// IsFalkorDBRunning checks if the FalkorDB container is running
func IsFalkorDBRunning(port int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if our container is running
	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Running}}", ContainerName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}

// ContainerExists checks if the FalkorDB container exists (running or stopped)
func ContainerExists() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "inspect", ContainerName)
	return cmd.Run() == nil
}

// StartFalkorDB starts the FalkorDB Docker container.
// If a stopped container exists, it will be started.
// Otherwise, a new container will be created with the specified options.
func StartFalkorDB(opts StartOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Apply defaults
	if opts.Port == 0 {
		opts.Port = 6379
	}

	// Check if container already exists
	if ContainerExists() {
		// Check if already running
		if IsFalkorDBRunning(opts.Port) {
			return nil // Already running
		}

		// Try to start existing container
		startCmd := exec.CommandContext(ctx, "docker", "start", ContainerName)
		if err := startCmd.Run(); err != nil {
			// Container exists but won't start - remove it and create fresh
			removeCmd := exec.CommandContext(ctx, "docker", "rm", "-f", ContainerName)
			if rmErr := removeCmd.Run(); rmErr != nil {
				return fmt.Errorf("failed to start existing container and failed to remove it; start: %w, remove: %v", err, rmErr)
			}
		} else {
			// Started successfully, wait for ready
			return waitForReady(ctx)
		}
	}

	// Create and start new container
	dockerArgs := []string{
		"run",
		"--name", ContainerName,
		"-p", fmt.Sprintf("%d:6379", opts.Port),
		"-p", "3000:3000", // Browser UI
		"--restart", "unless-stopped",
	}

	if opts.DataDir != "" {
		dockerArgs = append(dockerArgs, "-v", fmt.Sprintf("%s:/data", opts.DataDir))
	}

	if opts.Detach {
		dockerArgs = append(dockerArgs, "-d")
	}

	dockerArgs = append(dockerArgs, "falkordb/falkordb:latest")

	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create container; %w; output: %s", err, strings.TrimSpace(string(output)))
	}

	// Wait for FalkorDB to be ready
	return waitForReady(ctx)
}

// StopFalkorDB stops the FalkorDB container
func StopFalkorDB() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "stop", ContainerName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop container; %w", err)
	}
	return nil
}

// RemoveFalkorDB removes the FalkorDB container
func RemoveFalkorDB() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", ContainerName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove container; %w", err)
	}
	return nil
}

// waitForReady waits for FalkorDB to respond to ping
func waitForReady(ctx context.Context) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for FalkorDB to be ready")
		case <-ticker.C:
			// Try to ping FalkorDB via redis-cli inside container
			pingCmd := exec.CommandContext(ctx, "docker", "exec", ContainerName, "redis-cli", "ping")
			output, err := pingCmd.Output()
			if err == nil && strings.TrimSpace(string(output)) == "PONG" {
				return nil
			}
		}
	}
}
