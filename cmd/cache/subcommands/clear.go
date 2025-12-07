package subcommands

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/spf13/cobra"
)

var (
	clearAll         bool
	clearOldVersions bool
)

var ClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the semantic analysis cache",
	Long: "\nClear entries from the semantic analysis cache.\n\n" +
		"By default, this command requires a flag to specify what to clear:\n" +
		"  --all          Clear all cached entries\n" +
		"  --old-versions Clear only stale entries (non-current versions)\n\n" +
		"Cleared entries will be re-analyzed on next daemon rebuild or when " +
		"the corresponding files are modified.",
	PreRunE: validateClear,
	RunE:    runClear,
}

func init() {
	ClearCmd.Flags().BoolVar(&clearAll, "all", false, "Clear all cached entries")
	ClearCmd.Flags().BoolVar(&clearOldVersions, "old-versions", false, "Clear only stale/legacy entries")
}

func validateClear(cmd *cobra.Command, args []string) error {
	if !clearAll && !clearOldVersions {
		return fmt.Errorf("please specify --all or --old-versions")
	}
	if clearAll && clearOldVersions {
		return fmt.Errorf("cannot use both --all and --old-versions")
	}
	cmd.SilenceUsage = true
	return nil
}

func runClear(cmd *cobra.Command, args []string) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config; %w", err)
	}

	manager, err := cache.NewManager(cfg.Analysis.CacheDir)
	if err != nil {
		return fmt.Errorf("failed to initialize cache manager; %w", err)
	}

	if clearAll {
		return runClearAll(manager)
	}

	return runClearOldVersions(manager)
}

func runClearAll(manager *cache.Manager) error {
	// Get stats first to report what we're clearing
	stats, err := manager.GetStats()
	if err != nil {
		return fmt.Errorf("failed to get cache stats; %w", err)
	}

	if stats.TotalEntries == 0 {
		fmt.Printf("Cache is already empty.\n")
		return nil
	}

	fmt.Printf("Clearing all %d cached entries (%s)...\n", stats.TotalEntries, format.FormatBytes(stats.TotalSize))

	if err := manager.Clear(); err != nil {
		return fmt.Errorf("failed to clear cache; %w", err)
	}

	fmt.Printf("Cache cleared successfully.\n")
	fmt.Printf("\nNote: Run 'agentic-memorizer daemon rebuild' to regenerate the cache.\n")

	return nil
}

func runClearOldVersions(manager *cache.Manager) error {
	// Get stats first to show what versions exist
	stats, err := manager.GetStats()
	if err != nil {
		return fmt.Errorf("failed to get cache stats; %w", err)
	}

	// Count entries to be cleared
	currentVersion := cache.CacheVersion()
	entriesToClear := 0
	for version, count := range stats.VersionCounts {
		if version != currentVersion {
			entriesToClear += count
		}
	}

	if entriesToClear == 0 {
		fmt.Printf("No stale entries to clear. All %d entries are current version (%s).\n",
			stats.TotalEntries, currentVersion)
		return nil
	}

	fmt.Printf("Clearing %d stale entries (keeping %d current entries)...\n",
		entriesToClear, stats.TotalEntries-entriesToClear)

	removed, err := manager.ClearOldVersions()
	if err != nil {
		return fmt.Errorf("failed to clear old versions; %w", err)
	}

	fmt.Printf("Removed %d stale cache entries.\n", removed)
	fmt.Printf("\nNote: Affected files will be re-analyzed on next daemon rebuild.\n")

	return nil
}
