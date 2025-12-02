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

// getRecentFileEntries returns file entries modified within the specified number of days
func getRecentFileEntries(files []types.FileEntry, days int) []types.FileEntry {
	cutoff := time.Now().AddDate(0, 0, -days)
	recent := []types.FileEntry{}

	for _, file := range files {
		if file.Modified.After(cutoff) {
			recent = append(recent, file)
		}
	}

	sort.Slice(recent, func(i, j int) bool {
		return recent[i].Modified.After(recent[j].Modified)
	})

	if len(recent) > 10 {
		recent = recent[:10]
	}

	return recent
}

// groupFilesByCategory groups file entries by their category
func groupFilesByCategory(files []types.FileEntry) map[string][]types.FileEntry {
	categories := make(map[string][]types.FileEntry)

	for _, file := range files {
		categories[file.Category] = append(categories[file.Category], file)
	}

	return categories
}
