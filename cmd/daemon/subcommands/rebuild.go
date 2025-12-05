package subcommands

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/spf13/cobra"
)

var (
	forceRebuild  bool
	clearOldCache bool
)

var RebuildCmd = &cobra.Command{
	Use:   "rebuild",
	Short: "Force immediate index rebuild",
	Long: "\nForce the daemon to perform an immediate full index rebuild.\n\n" +
		"This triggers a rebuild via the daemon's HTTP API. The daemon will re-process " +
		"all files in the memory directory, extracting metadata, performing semantic " +
		"analysis, and rebuilding all graph relationships.\n\n" +
		"Use --force to clear the graph before rebuilding (otherwise, existing entries " +
		"are updated in place).\n\n" +
		"Use --clear-old-cache to remove stale cache entries before rebuilding. This ensures " +
		"files are re-analyzed with the current analysis version.",
	PreRunE: validateRebuild,
	RunE:    runRebuild,
}

func init() {
	RebuildCmd.Flags().BoolVarP(&forceRebuild, "force", "f", false, "Clear graph before rebuilding")
	RebuildCmd.Flags().BoolVar(&clearOldCache, "clear-old-cache", false, "Clear stale cache entries before rebuilding")
}

func validateRebuild(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runRebuild(cmd *cobra.Command, args []string) error {
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config; %w", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config; %w", err)
	}

	// Clear stale cache entries if requested
	if clearOldCache {
		cacheManager, err := cache.NewManager(cfg.Analysis.CacheDir)
		if err != nil {
			return fmt.Errorf("failed to initialize cache manager; %w", err)
		}

		stats, err := cacheManager.GetStats()
		if err != nil {
			return fmt.Errorf("failed to get cache stats; %w", err)
		}

		// Count stale entries
		currentVersion := cache.CacheVersion()
		staleCount := 0
		for version, count := range stats.VersionCounts {
			if version != currentVersion {
				staleCount += count
			}
		}

		if staleCount > 0 {
			fmt.Printf("Clearing %d stale cache entries...\n", staleCount)
			removed, err := cacheManager.ClearOldVersions()
			if err != nil {
				return fmt.Errorf("failed to clear old cache versions; %w", err)
			}
			fmt.Printf("Removed %d stale cache entries\n\n", removed)
		} else {
			fmt.Printf("No stale cache entries to clear\n\n")
		}
	}

	// Check if daemon is running by hitting health endpoint
	daemonURL := fmt.Sprintf("http://localhost:%d", cfg.Daemon.HTTPPort)
	healthURL := fmt.Sprintf("%s/health", daemonURL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request; %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("daemon is not running; start with 'agentic-memorizer daemon start'")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon health check failed; status %d", resp.StatusCode)
	}

	// Trigger rebuild via daemon API
	rebuildURL := fmt.Sprintf("%s/api/v1/rebuild", daemonURL)
	if forceRebuild {
		rebuildURL += "?force=true"
	}

	fmt.Printf("Triggering index rebuild via daemon...\n")
	if forceRebuild {
		fmt.Printf("Note: --force flag will clear the graph before rebuilding\n")
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()

	req2, err := http.NewRequestWithContext(ctx2, "POST", rebuildURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create rebuild request; %w", err)
	}

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		return fmt.Errorf("failed to trigger rebuild; %w", err)
	}
	defer resp2.Body.Close()

	switch resp2.StatusCode {
	case http.StatusOK, http.StatusAccepted:
		fmt.Printf("Rebuild started successfully\n")
		fmt.Printf("\nThe daemon is now rebuilding the index in the background.\n")
		fmt.Printf("Use 'agentic-memorizer daemon status' to check progress.\n")
	case http.StatusConflict:
		fmt.Printf("A rebuild is already in progress\n")
	default:
		return fmt.Errorf("rebuild request failed; status %d", resp2.StatusCode)
	}

	return nil
}
