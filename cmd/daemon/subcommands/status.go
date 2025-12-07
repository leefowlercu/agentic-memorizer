package subcommands

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"syscall"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/daemon"
	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/spf13/cobra"
)

var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	Long: "\nShow the current status of the background indexing daemon.\n\n" +
		"Displays whether the daemon is running, graph database statistics (queried directly from FalkorDB), " +
		"and configuration details.",
	PreRunE: validateStatus,
	RunE:    runStatus,
}

func validateStatus(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config; %w", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config; %w", err)
	}

	pidFile, err := config.GetPIDPath()
	if err != nil {
		return fmt.Errorf("failed to get PID path; %w", err)
	}

	// Build main section
	section := format.NewSection("Daemon Status").AddDivider()

	// Check if daemon is running
	var pid int
	var running bool

	data, err := os.ReadFile(pidFile)
	if err == nil {
		fmt.Sscanf(string(data), "%d", &pid)

		// Check if process exists
		process, err := os.FindProcess(pid)
		if err == nil {
			err = process.Signal(syscall.Signal(0))
			running = err == nil
		}
	}

	if running {
		managed := daemon.IsServiceManaged()
		if managed {
			section.AddKeyValue("Status", fmt.Sprintf("Running (PID %d, service-managed)", pid))
		} else {
			section.AddKeyValue("Status", fmt.Sprintf("Running (PID %d, foreground)", pid))
		}
	} else {
		section.AddKeyValue("Status", "Not running")
	}

	// Add Graph Database subsection
	graphSection := format.NewSection("Graph Database").SetLevel(1).AddDivider()
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	graphConfig := graph.ManagerConfig{
		Client: graph.ClientConfig{
			Host:     cfg.Graph.Host,
			Port:     cfg.Graph.Port,
			Database: cfg.Graph.Database,
			Password: cfg.Graph.Password,
		},
		Schema:     graph.DefaultSchemaConfig(),
		MemoryRoot: cfg.MemoryRoot,
	}

	graphManager := graph.NewManager(graphConfig, logger)
	var graphNote string
	if err := graphManager.Initialize(ctx); err != nil {
		graphSection.AddKeyValue("Status", fmt.Sprintf("Not connected (%s:%d)", cfg.Graph.Host, cfg.Graph.Port))
		graphSection.AddKeyValue("Error", fmt.Sprintf("%v", err))
		graphNote = "To start FalkorDB: agentic-memorizer graph start"
	} else {
		defer graphManager.Close()

		stats, err := graphManager.GetStats(ctx)
		if err != nil {
			graphSection.AddKeyValue("Status", "Connected (failed to get stats)")
		} else {
			graphSection.AddKeyValue("Status", fmt.Sprintf("Connected (%s:%d)", cfg.Graph.Host, cfg.Graph.Port))
			graphSection.AddKeyValue("Database", cfg.Graph.Database)
			graphSection.AddKeyValuef("Files", "%d", stats.TotalFiles)
			graphSection.AddKeyValuef("Tags", "%d", stats.TotalTags)
			graphSection.AddKeyValuef("Topics", "%d", stats.TotalTopics)
			graphSection.AddKeyValuef("Entities", "%d", stats.TotalEntities)
			graphSection.AddKeyValuef("Edges", "%d", stats.TotalEdges)
		}
	}
	section.AddSubsection(graphSection)

	// Add Configuration subsection
	configSection := format.NewSection("Configuration").SetLevel(1).AddDivider()
	configSection.AddKeyValue("Memory Root", cfg.MemoryRoot)
	configSection.AddKeyValue("Cache Dir", cfg.Analysis.CacheDir)
	configSection.AddKeyValuef("Rebuild Interval", "%d minutes", cfg.Daemon.FullRebuildIntervalMinutes)
	configSection.AddKeyValuef("Workers", "%d", cfg.Daemon.Workers)
	configSection.AddKeyValuef("Rate Limit", "%d/min", cfg.Daemon.RateLimitPerMin)
	if cfg.Daemon.HTTPPort > 0 {
		configSection.AddKeyValuef("HTTP Server", "http://localhost:%d", cfg.Daemon.HTTPPort)
	}
	section.AddSubsection(configSection)

	// Add Service Management subsection
	serviceSection := format.NewSection("Service Management").SetLevel(1).AddDivider()
	sm := daemon.DetectServiceManager()
	switch sm {
	case "systemd":
		serviceSection.AddKeyValue("Platform", "Linux (systemd available)")
		serviceSection.AddKeyValue("Setup", "agentic-memorizer daemon systemctl")
	case "launchd":
		serviceSection.AddKeyValue("Platform", "macOS (launchd available)")
		serviceSection.AddKeyValue("Setup", "agentic-memorizer daemon launchctl")
	default:
		serviceSection.AddKeyValue("Platform", runtime.GOOS)
		serviceSection.AddKeyValue("Management", "Manual management required")
	}
	section.AddSubsection(serviceSection)

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

	// Add notes
	if !running {
		sm := daemon.DetectServiceManager()
		switch sm {
		case "systemd":
			fmt.Println("\nNote: Check if managed by systemd:")
			fmt.Println("  systemctl --user status agentic-memorizer")
		case "launchd":
			fmt.Println("\nNote: Check if managed by launchd:")
			fmt.Println("  launchctl list | grep agentic-memorizer")
		}
	}

	if graphNote != "" {
		fmt.Printf("\n%s\n", graphNote)
	}

	return nil
}
