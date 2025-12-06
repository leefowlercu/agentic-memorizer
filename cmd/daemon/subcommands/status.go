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

	fmt.Println("Daemon Status")
	fmt.Println("=============")
	if running {
		managed := daemon.IsServiceManaged()
		if managed {
			fmt.Printf("Status: Running (PID %d, service-managed)\n", pid)
		} else {
			fmt.Printf("Status: Running (PID %d, foreground)\n", pid)
		}
	} else {
		fmt.Println("Status: Not running")

		// Add service manager check
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

	// Check graph status (FalkorDB is required)
	fmt.Println("\nGraph Database")
	fmt.Println("--------------")
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
	if err := graphManager.Initialize(ctx); err != nil {
		fmt.Printf("Status: Not connected (%s:%d)\n", cfg.Graph.Host, cfg.Graph.Port)
		fmt.Printf("Error: %v\n", err)
		fmt.Println("\nTo start FalkorDB: agentic-memorizer graph start")
	} else {
		defer graphManager.Close()

		stats, err := graphManager.GetStats(ctx)
		if err != nil {
			fmt.Println("Status: Connected (failed to get stats)")
		} else {
			fmt.Printf("Status: Connected (%s:%d)\n", cfg.Graph.Host, cfg.Graph.Port)
			fmt.Printf("Database: %s\n", cfg.Graph.Database)
			fmt.Printf("Files: %d\n", stats.TotalFiles)
			fmt.Printf("Tags: %d\n", stats.TotalTags)
			fmt.Printf("Topics: %d\n", stats.TotalTopics)
			fmt.Printf("Entities: %d\n", stats.TotalEntities)
			fmt.Printf("Edges: %d\n", stats.TotalEdges)
		}
	}

	// Configuration
	fmt.Println("\nConfiguration")
	fmt.Println("-------------")
	fmt.Printf("Memory Root: %s\n", cfg.MemoryRoot)
	fmt.Printf("Cache Dir: %s\n", cfg.Analysis.CacheDir)
	fmt.Printf("Rebuild Interval: %d minutes\n", cfg.Daemon.FullRebuildIntervalMinutes)
	fmt.Printf("Workers: %d\n", cfg.Daemon.Workers)
	fmt.Printf("Rate Limit: %d/min\n", cfg.Daemon.RateLimitPerMin)
	if cfg.Daemon.HTTPPort > 0 {
		fmt.Printf("HTTP Server: http://localhost:%d\n", cfg.Daemon.HTTPPort)
	}

	// Service Management
	fmt.Println("\nService Management")
	fmt.Println("------------------")
	sm := daemon.DetectServiceManager()
	switch sm {
	case "systemd":
		fmt.Println("Platform: Linux (systemd available)")
		fmt.Println("Setup: agentic-memorizer daemon systemctl")
	case "launchd":
		fmt.Println("Platform: macOS (launchd available)")
		fmt.Println("Setup: agentic-memorizer daemon launchctl")
	default:
		fmt.Printf("Platform: %s\n", runtime.GOOS)
		fmt.Println("Manual management required")
	}

	return nil
}
