package subcommands

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/spf13/cobra"
)

var statusTimeout int

var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check FalkorDB connection and graph statistics",
	Long: "\nCheck the connection status and statistics of the FalkorDB knowledge graph.\n\n" +
		"Attempts to connect to the configured FalkorDB instance and displays " +
		"configuration details, graph statistics, and category breakdown if connected.",
	Example: `  # Check FalkorDB connection and graph status
  memorizer graph status

  # Check with custom timeout
  memorizer graph status --timeout 30`,
	PreRunE: validateStatus,
	RunE:    runStatus,
}

func init() {
	StatusCmd.Flags().IntVar(&statusTimeout, "timeout", 10, "Connection timeout in seconds")
}

func validateStatus(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Load config
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config; %w", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config; %w", err)
	}

	section := format.NewSection("Graph Status").AddDivider()

	// Build graph manager config
	managerConfig := graph.ManagerConfig{
		Client: graph.ClientConfig{
			Host:     cfg.Graph.Host,
			Port:     cfg.Graph.Port,
			Database: cfg.Graph.Database,
			Password: cfg.Graph.Password,
		},
		MemoryRoot: cfg.Memory.Root,
	}

	// Use discard logger to suppress graph initialization logs
	discardLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	manager := graph.NewManager(managerConfig, discardLogger)

	// Attempt connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(statusTimeout)*time.Second)
	defer cancel()

	if err := manager.Initialize(ctx); err != nil {
		// Connection failed - minimal output
		section.AddKeyValue("Connection", fmt.Sprintf("failed (%s)", err))
		section.AddKeyValue("Host", cfg.Graph.Host)
		section.AddKeyValuef("Port", "%d", cfg.Graph.Port)
	} else {
		// Connection succeeded - full output
		defer manager.Close()

		section.AddKeyValue("Connection", "connected")

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

		// Get health status for graph statistics
		health, err := manager.Health(ctx)
		if err == nil && health.Stats != nil {
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
	}

	// Single output path
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
