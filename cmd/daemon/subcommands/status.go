package subcommands

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/index"
	"github.com/spf13/cobra"
)

var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	Long: "\nShow the current status of the background indexing daemon.\n\n" +
		"Displays whether the daemon is running, index statistics, and configuration details.",
	RunE: runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	pidFile, err := config.GetPIDPath()
	if err != nil {
		return fmt.Errorf("failed to get PID path: %w", err)
	}

	indexPath, err := config.GetIndexPath()
	if err != nil {
		return fmt.Errorf("failed to get index path: %w", err)
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
		fmt.Printf("Status: Running (PID %d)\n", pid)
	} else {
		fmt.Println("Status: Not running")
	}

	// Check index status
	indexManager := index.NewManager(indexPath)
	computed, err := indexManager.LoadComputed()
	if err != nil {
		fmt.Println("\nIndex: Not found")
	} else {
		fmt.Println("\nIndex Information")
		fmt.Println("-----------------")
		fmt.Printf("Version: %s\n", computed.Version)
		fmt.Printf("Generated: %s\n", computed.GeneratedAt.Format(time.RFC3339))
		fmt.Printf("Daemon Version: %s\n", computed.DaemonVersion)
		fmt.Printf("Files: %d\n", computed.Index.Stats.TotalFiles)
		fmt.Printf("Analyzed: %d\n", computed.Index.Stats.AnalyzedFiles)
		fmt.Printf("Cached: %d\n", computed.Index.Stats.CachedFiles)
		fmt.Printf("Errors: %d\n", computed.Index.Stats.ErrorFiles)
		fmt.Printf("Build Duration: %dms\n", computed.Metadata.BuildDurationMs)
	}

	// Configuration
	fmt.Println("\nConfiguration")
	fmt.Println("-------------")
	fmt.Printf("Memory Root: %s\n", cfg.MemoryRoot)
	fmt.Printf("Cache Dir: %s\n", cfg.Analysis.CacheDir)
	fmt.Printf("Daemon Enabled: %t\n", cfg.Daemon.Enabled)
	if cfg.Daemon.Enabled {
		fmt.Printf("Rebuild Interval: %d minutes\n", cfg.Daemon.FullRebuildIntervalMinutes)
		fmt.Printf("Workers: %d\n", cfg.Daemon.Workers)
		fmt.Printf("Rate Limit: %d/min\n", cfg.Daemon.RateLimitPerMin)
		if cfg.Daemon.HealthCheckPort > 0 {
			fmt.Printf("Health Check: http://localhost:%d\n", cfg.Daemon.HealthCheckPort)
		}
	}

	return nil
}

// prettyPrintJSON prints JSON with indentation
func prettyPrintJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
