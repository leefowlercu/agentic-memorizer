package daemon

import (
	"fmt"
	"sync"
	"time"
)

// HealthMetrics tracks daemon health statistics
type HealthMetrics struct {
	StartTime        time.Time `json:"start_time"`
	Uptime           string    `json:"uptime"`
	UptimeSeconds    int64     `json:"uptime_seconds"`
	FilesProcessed   int       `json:"files_processed"`
	APICalls         int       `json:"api_calls"`
	CacheHits        int       `json:"cache_hits"`
	Errors           int       `json:"errors"`
	LastBuildTime    time.Time `json:"last_build_time"`
	LastBuildSuccess bool      `json:"last_build_success"`
	IndexFileCount   int       `json:"index_file_count"`
	WatcherActive    bool      `json:"watcher_active"`
	mu               sync.RWMutex
}

// HealthSnapshot represents a point-in-time snapshot of health metrics
// without the mutex, safe for copying and serialization
type HealthSnapshot struct {
	StartTime        time.Time `json:"start_time"`
	Uptime           string    `json:"uptime"`
	UptimeSeconds    int64     `json:"uptime_seconds"`
	FilesProcessed   int       `json:"files_processed"`
	APICalls         int       `json:"api_calls"`
	CacheHits        int       `json:"cache_hits"`
	Errors           int       `json:"errors"`
	LastBuildTime    time.Time `json:"last_build_time"`
	LastBuildSuccess bool      `json:"last_build_success"`
	IndexFileCount   int       `json:"index_file_count"`
	WatcherActive    bool      `json:"watcher_active"`
}

// NewHealthMetrics creates a new health metrics tracker
func NewHealthMetrics() *HealthMetrics {
	return &HealthMetrics{
		StartTime: time.Now(),
	}
}

// RecordBuild records a build attempt
func (h *HealthMetrics) RecordBuild(filesProcessed, apiCalls, cacheHits, errors int, success bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.FilesProcessed += filesProcessed
	h.APICalls += apiCalls
	h.CacheHits += cacheHits
	h.Errors += errors
	h.LastBuildTime = time.Now()
	h.LastBuildSuccess = success
}

// RecordError increments the error counter
func (h *HealthMetrics) RecordError() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Errors++
}

// SetIndexFileCount updates the current index file count
func (h *HealthMetrics) SetIndexFileCount(count int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.IndexFileCount = count
}

// SetWatcherActive sets the watcher active status
func (h *HealthMetrics) SetWatcherActive(active bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.WatcherActive = active
}

// GetSnapshot returns a snapshot of current metrics
func (h *HealthMetrics) GetSnapshot() HealthSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()

	uptime := time.Since(h.StartTime)
	return HealthSnapshot{
		StartTime:        h.StartTime,
		Uptime:           formatDuration(uptime),
		UptimeSeconds:    int64(uptime.Seconds()),
		FilesProcessed:   h.FilesProcessed,
		APICalls:         h.APICalls,
		CacheHits:        h.CacheHits,
		Errors:           h.Errors,
		LastBuildTime:    h.LastBuildTime,
		LastBuildSuccess: h.LastBuildSuccess,
		IndexFileCount:   h.IndexFileCount,
		WatcherActive:    h.WatcherActive,
	}
}

// formatDuration formats a duration in human-readable form
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
