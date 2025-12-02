package worker

import (
	"os"
	"time"
)

// CalculatePriority calculates job priority based on file modification time
// More recently modified files get higher priority
func CalculatePriority(info os.FileInfo) int {
	age := time.Since(info.ModTime())

	// Files modified in last hour: priority 100
	if age < time.Hour {
		return 100
	}

	// Files modified in last day: priority 50
	if age < 24*time.Hour {
		return 50
	}

	// Files modified in last week: priority 25
	if age < 7*24*time.Hour {
		return 25
	}

	// Older files: priority 10
	return 10
}
