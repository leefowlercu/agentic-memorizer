package subcommands

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/spf13/cobra"
)

var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check FalkorDB status",
	Long: "\nCheck the health and status of the FalkorDB knowledge graph.\n\n" +
		"This command checks if the FalkorDB container is running and connects to it " +
		"to retrieve statistics about the knowledge graph including node counts, " +
		"relationship counts, and category distribution.",
	PreRunE: validateStatus,
	RunE:    runStatus,
}

func validateStatus(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	containerName := "memorizer-falkordb"

	// Check Docker availability
	dockerAvailable := true
	if _, err := exec.LookPath("docker"); err != nil {
		dockerAvailable = false
	}

	// Check container status
	containerRunning := false
	if dockerAvailable {
		checkCmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", containerName)
		output, err := checkCmd.Output()
		if err == nil && strings.TrimSpace(string(output)) == "true" {
			containerRunning = true
		}
	}

	fmt.Printf("FalkorDB Status\n")
	fmt.Printf("===============\n\n")

	if !dockerAvailable {
		fmt.Printf("Docker:     not installed\n")
		fmt.Printf("Container:  N/A\n")
		fmt.Printf("\nInstall Docker to use the FalkorDB knowledge graph.\n")
		return nil
	}

	fmt.Printf("Docker:     available\n")
	if containerRunning {
		fmt.Printf("Container:  running\n")
	} else {
		// Check if container exists but stopped
		checkCmd := exec.Command("docker", "inspect", containerName)
		if err := checkCmd.Run(); err == nil {
			fmt.Printf("Container:  stopped\n")
			fmt.Printf("\nRun 'agentic-memorizer graph start' to start the container.\n")
		} else {
			fmt.Printf("Container:  not created\n")
			fmt.Printf("\nRun 'agentic-memorizer graph start' to create and start the container.\n")
		}
		return nil
	}

	// Connect to FalkorDB and get stats
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config; %w", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config; %w", err)
	}

	managerConfig := graph.ManagerConfig{
		Client: graph.ClientConfig{
			Host:     cfg.Graph.Host,
			Port:     cfg.Graph.Port,
			Database: config.GraphDatabase, // Hardcoded convention
		},
		MemoryRoot: cfg.MemoryRoot,
	}

	manager := graph.NewManager(managerConfig, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := manager.Initialize(ctx); err != nil {
		fmt.Printf("Connection: failed (%s)\n", err)
		return nil
	}
	defer manager.Close()

	fmt.Printf("Connection: connected\n")

	// Get health status
	health, err := manager.Health(ctx)
	if err != nil {
		fmt.Printf("Health:     error (%s)\n", err)
	} else if health.Stats != nil {
		fmt.Printf("Database:   %s\n", health.Database)
		fmt.Printf("\nGraph Statistics\n")
		fmt.Printf("----------------\n")
		fmt.Printf("Nodes:         %d\n", health.Stats.NodeCount)
		fmt.Printf("Relationships: %d\n", health.Stats.RelationshipCount)
	}

	// Get detailed stats
	stats, err := manager.GetStats(ctx)
	if err == nil && stats != nil {
		fmt.Printf("\nDetailed Statistics\n")
		fmt.Printf("-------------------\n")
		fmt.Printf("Files:    %d\n", stats.TotalFiles)
		fmt.Printf("Tags:     %d\n", stats.TotalTags)
		fmt.Printf("Topics:   %d\n", stats.TotalTopics)
		fmt.Printf("Entities: %d\n", stats.TotalEntities)
		fmt.Printf("Edges:    %d\n", stats.TotalEdges)

		if len(stats.Categories) > 0 {
			fmt.Printf("\nCategories\n")
			fmt.Printf("----------\n")
			for category, count := range stats.Categories {
				fmt.Printf("  %s: %d\n", category, count)
			}
		}
	}

	fmt.Printf("\nBrowser UI: http://localhost:3000\n")

	return nil
}
