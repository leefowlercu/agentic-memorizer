package subcommands

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/spf13/cobra"
)

var StatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cache statistics",
	Long: "\nShow statistics about the semantic analysis cache.\n\n" +
		"Displays the total number of cached entries, their size, and version distribution. " +
		"Legacy entries (v0.0.0) are entries created before versioning was implemented and " +
		"will be re-analyzed on next daemon rebuild.",
	PreRunE: validateStatus,
	RunE:    runStatus,
}

func validateStatus(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config; %w", err)
	}

	manager, err := cache.NewManager(cfg.Analysis.CacheDir)
	if err != nil {
		return fmt.Errorf("failed to initialize cache manager; %w", err)
	}

	stats, err := manager.GetStats()
	if err != nil {
		return fmt.Errorf("failed to get cache stats; %w", err)
	}

	fmt.Printf("Cache Status\n")
	fmt.Printf("============\n\n")

	fmt.Printf("Location:       %s\n", cfg.Analysis.CacheDir)
	fmt.Printf("Current Version: %s\n\n", cache.CacheVersion())

	fmt.Printf("Statistics\n")
	fmt.Printf("----------\n")
	fmt.Printf("Total Entries:  %d\n", stats.TotalEntries)
	fmt.Printf("Total Size:     %s\n", formatBytes(stats.TotalSize))
	fmt.Printf("Legacy Entries: %d\n", stats.LegacyEntries)

	if len(stats.VersionCounts) > 0 {
		fmt.Printf("\nVersion Distribution\n")
		fmt.Printf("--------------------\n")
		for version, count := range stats.VersionCounts {
			marker := ""
			if version == cache.CacheVersion() {
				marker = " (current)"
			} else if version == "v0.0.0" {
				marker = " (legacy)"
			} else {
				marker = " (stale)"
			}
			fmt.Printf("  %s: %d%s\n", version, count, marker)
		}
	}

	if stats.LegacyEntries > 0 || hasStaleEntries(stats) {
		fmt.Printf("\nNote: Run 'agentic-memorizer cache clear --old-versions' to remove stale entries.\n")
		fmt.Printf("      Stale entries will be re-analyzed automatically on next daemon rebuild.\n")
	}

	return nil
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// hasStaleEntries checks if there are any non-current, non-legacy entries
func hasStaleEntries(stats *cache.CacheStats) bool {
	currentVersion := cache.CacheVersion()
	for version, count := range stats.VersionCounts {
		if version != currentVersion && version != "v0.0.0" && count > 0 {
			return true
		}
	}
	return false
}
