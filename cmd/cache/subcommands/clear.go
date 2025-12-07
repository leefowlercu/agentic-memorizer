package subcommands

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/format"
	_ "github.com/leefowlercu/agentic-memorizer/internal/format/formatters" // Register formatters
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
		status := format.NewStatus(format.StatusInfo, "Cache is already empty")
		return outputStatus(status)
	}

	// Show what we're clearing
	msg := fmt.Sprintf("Clearing all %d cached entries (%s)", stats.TotalEntries, format.FormatBytes(stats.TotalSize))
	clearing := format.NewStatus(format.StatusRunning, msg)
	if err := outputStatus(clearing); err != nil {
		return err
	}

	if err := manager.Clear(); err != nil {
		return fmt.Errorf("failed to clear cache; %w", err)
	}

	status := format.NewStatus(format.StatusSuccess, "Cache cleared successfully")
	status.AddDetail("Run 'agentic-memorizer daemon rebuild' to regenerate the cache")
	return outputStatus(status)
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
		msg := fmt.Sprintf("No stale entries to clear. All %d entries are current version (%s)",
			stats.TotalEntries, currentVersion)
		status := format.NewStatus(format.StatusInfo, msg)
		return outputStatus(status)
	}

	// Show what we're clearing
	msg := fmt.Sprintf("Clearing %d stale entries (keeping %d current entries)",
		entriesToClear, stats.TotalEntries-entriesToClear)
	clearing := format.NewStatus(format.StatusRunning, msg)
	if err := outputStatus(clearing); err != nil {
		return err
	}

	removed, err := manager.ClearOldVersions()
	if err != nil {
		return fmt.Errorf("failed to clear old versions; %w", err)
	}

	status := format.NewStatus(format.StatusSuccess, fmt.Sprintf("Removed %d stale cache entries", removed))
	status.AddDetail("Affected files will be re-analyzed on next daemon rebuild")
	return outputStatus(status)
}

// outputStatus formats and outputs a status message
func outputStatus(status *format.Status) error {
	formatter, err := format.GetFormatter("text")
	if err != nil {
		return fmt.Errorf("failed to get formatter; %w", err)
	}
	output, err := formatter.Format(status)
	if err != nil {
		return fmt.Errorf("failed to format status; %w", err)
	}
	fmt.Println(output)
	return nil
}
