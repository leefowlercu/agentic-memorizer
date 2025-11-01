package output

import (
	"fmt"
	"sort"
	"time"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// formatSize formats bytes into human-readable size
func formatSize(bytes int64) string {
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

// getRecentEntries returns entries modified within the specified number of days
func getRecentEntries(entries []types.IndexEntry, days int) []types.IndexEntry {
	cutoff := time.Now().AddDate(0, 0, -days)
	recent := []types.IndexEntry{}

	for _, entry := range entries {
		if entry.Metadata.Modified.After(cutoff) {
			recent = append(recent, entry)
		}
	}

	sort.Slice(recent, func(i, j int) bool {
		return recent[i].Metadata.Modified.After(recent[j].Metadata.Modified)
	})

	if len(recent) > 10 {
		recent = recent[:10]
	}

	return recent
}

// groupByCategory groups index entries by their category
func groupByCategory(entries []types.IndexEntry) map[string][]types.IndexEntry {
	categories := make(map[string][]types.IndexEntry)

	for _, entry := range entries {
		category := entry.Metadata.Category
		categories[category] = append(categories[category], entry)
	}

	return categories
}
