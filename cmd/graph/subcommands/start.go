package subcommands

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/spf13/cobra"
)

var detached bool

var StartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the FalkorDB container",
	Long: "\nStart the FalkorDB Docker container for knowledge graph storage.\n\n" +
		"FalkorDB is required for the daemon to store file metadata, semantic analysis results, " +
		"and embeddings. The container is started using Docker and persists data in ~/.agentic-memorizer/falkordb/.\n\n" +
		"Use --detach to run the container in the background (default). Without --detach, " +
		"the container runs in foreground mode and can be stopped with Ctrl+C.",
	PreRunE: validateStart,
	RunE:    runStart,
}

func init() {
	StartCmd.Flags().BoolVarP(&detached, "detach", "d", true, "Run container in background")
}

func validateStart(cmd *cobra.Command, args []string) error {
	// Check Docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not found; please install Docker to use the FalkorDB knowledge graph")
	}

	cmd.SilenceUsage = true
	return nil
}

func runStart(cmd *cobra.Command, args []string) error {
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config; %w", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config; %w", err)
	}

	appDir, err := config.GetAppDir()
	if err != nil {
		return fmt.Errorf("failed to get app directory; %w", err)
	}

	containerName := "memorizer-falkordb"
	dataDir := fmt.Sprintf("%s/falkordb", appDir)

	// Check if container already exists
	checkCmd := exec.Command("docker", "inspect", containerName)
	if err := checkCmd.Run(); err == nil {
		// Container exists, check if running
		statusCmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", containerName)
		output, err := statusCmd.Output()
		if err == nil && strings.TrimSpace(string(output)) == "true" {
			fmt.Printf("FalkorDB container is already running\n")
			return nil
		}

		// Container exists but not running, start it
		fmt.Printf("Starting existing FalkorDB container...\n")
		startCmd := exec.Command("docker", "start", containerName)
		if err := startCmd.Run(); err != nil {
			return fmt.Errorf("failed to start container; %w", err)
		}
	} else {
		// Container doesn't exist, create and start it
		fmt.Printf("Creating FalkorDB container...\n")
		fmt.Printf("Data directory: %s\n", dataDir)

		dockerArgs := []string{
			"run",
			"--name", containerName,
			"-p", fmt.Sprintf("%d:6379", cfg.Graph.Port),
			"-p", "3000:3000", // Browser UI
			"-v", fmt.Sprintf("%s:/data", dataDir),
			"--restart", "unless-stopped",
		}

		if detached {
			dockerArgs = append(dockerArgs, "-d")
		}

		dockerArgs = append(dockerArgs, "falkordb/falkordb:latest")

		createCmd := exec.Command("docker", dockerArgs...)
		createCmd.Stdout = cmd.OutOrStdout()
		createCmd.Stderr = cmd.ErrOrStderr()

		if err := createCmd.Run(); err != nil {
			return fmt.Errorf("failed to create container; %w", err)
		}
	}

	// Wait for container to be healthy
	fmt.Printf("Waiting for FalkorDB to be ready...\n")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for FalkorDB to be ready")
		case <-ticker.C:
			// Try to ping FalkorDB via redis-cli inside container
			pingCmd := exec.Command("docker", "exec", containerName, "redis-cli", "ping")
			output, err := pingCmd.Output()
			if err == nil && strings.TrimSpace(string(output)) == "PONG" {
				fmt.Printf("FalkorDB is ready\n")
				fmt.Printf("  Redis port: %d\n", cfg.Graph.Port)
				fmt.Printf("  Browser UI: http://localhost:3000\n")
				return nil
			}
		}
	}
}
