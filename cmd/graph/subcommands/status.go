package subcommands

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/docker"
	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/spf13/cobra"
)

var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check FalkorDB status",
	Long: "\nCheck the health and status of the FalkorDB knowledge graph.\n\n" +
		"Displays whether the FalkorDB container is running and configuration details.",
	PreRunE: validateStatus,
	RunE:    runStatus,
}

func validateStatus(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Load config first
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config; %w", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config; %w", err)
	}

	section := format.NewSection("Graph Status").AddDivider()

	// Check Docker availability
	if !docker.IsAvailable() {
		section.AddKeyValue("Docker", "not installed or not running")
		section.AddKeyValue("Container", "N/A")

		// Add Configuration subsection
		configSection := format.NewSection("Configuration").SetLevel(1).AddDivider()
		configSection.AddKeyValue("Host", cfg.Graph.Host)
		configSection.AddKeyValuef("Port", "%d", cfg.Graph.Port)
		configSection.AddKeyValue("Database", cfg.Graph.Database)

		// Mask password
		passwordValue := "(not set)"
		if cfg.Graph.Password != "" {
			passwordValue = "********"
		}
		configSection.AddKeyValue("Password", passwordValue)

		configSection.AddKeyValuef("Similarity Threshold", "%.1f", cfg.Graph.SimilarityThreshold)
		configSection.AddKeyValuef("Max Similar Files", "%d", cfg.Graph.MaxSimilarFiles)
		section.AddSubsection(configSection)

		formatter, err := format.GetFormatter("text")
		if err != nil {
			return fmt.Errorf("failed to get formatter; %w", err)
		}
		output, err := formatter.Format(section)
		if err != nil {
			return fmt.Errorf("failed to format output; %w", err)
		}
		fmt.Println(output)
		fmt.Printf("\nInstall Docker to use the FalkorDB knowledge graph.\n")
		return nil
	}

	section.AddKeyValue("Docker", "available")

	// Check container status
	containerRunning := docker.IsFalkorDBRunning(0)
	if !containerRunning {
		if docker.ContainerExists() {
			section.AddKeyValue("Container", "stopped")
		} else {
			section.AddKeyValue("Container", "not created")
		}

		// Add Configuration subsection
		configSection := format.NewSection("Configuration").SetLevel(1).AddDivider()
		configSection.AddKeyValue("Host", cfg.Graph.Host)
		configSection.AddKeyValuef("Port", "%d", cfg.Graph.Port)
		configSection.AddKeyValue("Database", cfg.Graph.Database)

		// Mask password
		passwordValue := "(not set)"
		if cfg.Graph.Password != "" {
			passwordValue = "********"
		}
		configSection.AddKeyValue("Password", passwordValue)

		configSection.AddKeyValuef("Similarity Threshold", "%.1f", cfg.Graph.SimilarityThreshold)
		configSection.AddKeyValuef("Max Similar Files", "%d", cfg.Graph.MaxSimilarFiles)
		section.AddSubsection(configSection)

		formatter, err := format.GetFormatter("text")
		if err != nil {
			return fmt.Errorf("failed to get formatter; %w", err)
		}
		output, err := formatter.Format(section)
		if err != nil {
			return fmt.Errorf("failed to format output; %w", err)
		}
		fmt.Println(output)

		if docker.ContainerExists() {
			fmt.Printf("\nTo start the graph, run: agentic-memorizer graph start\n")
		} else {
			fmt.Printf("\nTo create and start the graph, run: agentic-memorizer graph start\n")
		}
		return nil
	}

	section.AddKeyValue("Container", "running")

	// Add Configuration subsection
	configSection := format.NewSection("Configuration").SetLevel(1).AddDivider()
	configSection.AddKeyValue("Host", cfg.Graph.Host)
	configSection.AddKeyValuef("Port", "%d", cfg.Graph.Port)
	configSection.AddKeyValue("Database", cfg.Graph.Database)

	// Mask password
	passwordValue := "(not set)"
	if cfg.Graph.Password != "" {
		passwordValue = "********"
	}
	configSection.AddKeyValue("Password", passwordValue)

	configSection.AddKeyValuef("Similarity Threshold", "%.1f", cfg.Graph.SimilarityThreshold)
	configSection.AddKeyValuef("Max Similar Files", "%d", cfg.Graph.MaxSimilarFiles)
	section.AddSubsection(configSection)

	// Connect to FalkorDB and get stats
	managerConfig := graph.ManagerConfig{
		Client: graph.ClientConfig{
			Host:     cfg.Graph.Host,
			Port:     cfg.Graph.Port,
			Database: cfg.Graph.Database,
		},
		MemoryRoot: cfg.MemoryRoot,
	}

	// Use discard logger to suppress graph initialization logs
	discardLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	manager := graph.NewManager(managerConfig, discardLogger)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := manager.Initialize(ctx); err != nil {
		section.AddKeyValue("Connection", fmt.Sprintf("failed (%s)", err))

		formatter, err := format.GetFormatter("text")
		if err != nil {
			return fmt.Errorf("failed to get formatter; %w", err)
		}
		output, err := formatter.Format(section)
		if err != nil {
			return fmt.Errorf("failed to format output; %w", err)
		}
		fmt.Println(output)
		return nil
	}
	defer manager.Close()

	section.AddKeyValue("Connection", "connected")

	// Get health status
	health, err := manager.Health(ctx)
	if err != nil {
		section.AddKeyValue("Health", fmt.Sprintf("error (%s)", err))
	} else if health.Stats != nil {
		section.AddKeyValue("Database", health.Database)

		// Add graph statistics subsection
		graphStats := format.NewSection("Graph Statistics").SetLevel(1).AddDivider()
		graphStats.AddKeyValuef("Nodes", "%d", health.Stats.NodeCount)
		graphStats.AddKeyValuef("Relationships", "%d", health.Stats.RelationshipCount)
		section.AddSubsection(graphStats)
	}

	// Get detailed stats
	stats, err := manager.GetStats(ctx)
	if err == nil && stats != nil {
		detailedStats := format.NewSection("Detailed Statistics").SetLevel(1).AddDivider()
		detailedStats.AddKeyValuef("Files", "%d", stats.TotalFiles)
		detailedStats.AddKeyValuef("Tags", "%d", stats.TotalTags)
		detailedStats.AddKeyValuef("Topics", "%d", stats.TotalTopics)
		detailedStats.AddKeyValuef("Entities", "%d", stats.TotalEntities)
		detailedStats.AddKeyValuef("Edges", "%d", stats.TotalEdges)
		section.AddSubsection(detailedStats)

		if len(stats.Categories) > 0 {
			categories := format.NewSection("Categories").SetLevel(1).AddDivider()
			for category, count := range stats.Categories {
				categories.AddKeyValuef(category, "%d", count)
			}
			section.AddSubsection(categories)
		}
	}

	// Format and write output
	formatter, err := format.GetFormatter("text")
	if err != nil {
		return fmt.Errorf("failed to get formatter; %w", err)
	}
	output, err := formatter.Format(section)
	if err != nil {
		return fmt.Errorf("failed to format output; %w", err)
	}
	fmt.Println(output)

	fmt.Printf("\nBrowser UI: http://%s:3000\n", cfg.Graph.Host)

	return nil
}
