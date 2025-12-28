package subcommands

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/format"
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

	manager, err := cache.NewManager(cfg.Semantic.CacheDir)
	if err != nil {
		return fmt.Errorf("failed to initialize cache manager; %w", err)
	}

	stats, err := manager.GetStats()
	if err != nil {
		return fmt.Errorf("failed to get cache stats; %w", err)
	}

	// Build output using format package
	section := format.NewSection("Cache Status").AddDivider()
	section.AddKeyValue("Location", cfg.Semantic.CacheDir)
	section.AddKeyValue("Current Version", cache.CacheVersion())

	// Add statistics subsection
	statsSection := format.NewSection("Statistics").SetLevel(1).AddDivider()
	statsSection.AddKeyValuef("Total Entries", "%d", stats.TotalEntries)
	statsSection.AddKeyValue("Total Size", format.FormatBytes(stats.TotalSize))
	statsSection.AddKeyValuef("Legacy Entries", "%d", stats.LegacyEntries)
	section.AddSubsection(statsSection)

	// Add version distribution if available
	if len(stats.VersionCounts) > 0 {
		versionSection := format.NewSection("Version Distribution").SetLevel(1).AddDivider()
		for version, count := range stats.VersionCounts {
			marker := ""
			if version == cache.CacheVersion() {
				marker = " (current)"
			} else if version == "v0.0.0" {
				marker = " (legacy)"
			} else {
				marker = " (stale)"
			}
			versionSection.AddKeyValuef(version, "%d%s", count, marker)
		}
		section.AddSubsection(versionSection)
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

	// Add note based on cache state
	if stats.TotalEntries > 0 {
		// Calculate total stale entries (legacy + other stale versions)
		staleCount := stats.LegacyEntries
		currentVersion := cache.CacheVersion()
		for version, count := range stats.VersionCounts {
			if version != currentVersion && version != "v0.0.0" {
				staleCount += count
			}
		}

		if staleCount == 0 {
			// No stale entries - all entries are current
			fmt.Printf("\nTo clear the cache, run: memorizer cache clear --all\n")
		} else if staleCount < stats.TotalEntries {
			// Some entries are stale
			fmt.Printf("\nTo clear stale entries, run: memorizer cache clear --stale\n")
			fmt.Printf("To clear all entries, run: memorizer cache clear --all\n")
		} else {
			// All entries are stale
			fmt.Printf("\nAll cache entries are stale.\n")
			fmt.Printf("To clear the cache, run: memorizer cache clear {--all|--stale}\n")
		}
	}

	return nil
}
