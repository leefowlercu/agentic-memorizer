package daemon

import (
	"fmt"
	"sync"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/daemon/api"
)

// ProviderInfoGetter is a function that returns semantic provider info
type ProviderInfoGetter func() (enabled bool, provider string, model string)

// HealthMetrics tracks daemon health statistics
type HealthMetrics struct {
	StartTime          time.Time `json:"start_time"`
	Uptime             string    `json:"uptime"`
	UptimeSeconds      int64     `json:"uptime_seconds"`
	FilesProcessed     int       `json:"files_processed"`
	APICalls           int       `json:"api_calls"`
	CacheHits          int       `json:"cache_hits"`
	Errors             int       `json:"errors"`
	LastBuildTime      time.Time `json:"last_build_time"`
	LastBuildSuccess   bool      `json:"last_build_success"`
	IndexFileCount     int       `json:"index_file_count"`
	WatcherActive      bool      `json:"watcher_active"`
	cacheManager       *cache.Manager
	providerInfoGetter ProviderInfoGetter
	mu                 sync.RWMutex
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

// RecordFileProcessed increments the files processed counter
func (h *HealthMetrics) RecordFileProcessed() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.FilesProcessed++
}

// RecordAPICall increments the API calls counter
func (h *HealthMetrics) RecordAPICall() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.APICalls++
}

// RecordCacheHit increments the cache hits counter
func (h *HealthMetrics) RecordCacheHit() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.CacheHits++
}

// IncrementIndexFileCount increments the index file count by 1
func (h *HealthMetrics) IncrementIndexFileCount() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.IndexFileCount++
}

// DecrementIndexFileCount decrements the index file count by 1
func (h *HealthMetrics) DecrementIndexFileCount() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.IndexFileCount > 0 {
		h.IndexFileCount--
	}
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

// SetCacheManager sets the cache manager for cache stats reporting
func (h *HealthMetrics) SetCacheManager(manager *cache.Manager) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cacheManager = manager
}

// SetProviderInfoGetter sets the function that returns semantic provider info
func (h *HealthMetrics) SetProviderInfoGetter(getter ProviderInfoGetter) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.providerInfoGetter = getter
}

// GetSnapshot returns a snapshot of current metrics
// Implements api.HealthMetricsProvider interface
func (h *HealthMetrics) GetSnapshot() api.HealthSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()

	uptime := time.Since(h.StartTime)
	snapshot := api.HealthSnapshot{
		StartTime:        h.StartTime,
		Uptime:           formatDuration(uptime),
		UptimeSeconds:    uptime.Seconds(),
		FilesProcessed:   h.FilesProcessed,
		APICalls:         h.APICalls,
		CacheHits:        h.CacheHits,
		Errors:           h.Errors,
		LastBuildTime:    h.LastBuildTime,
		LastBuildSuccess: h.LastBuildSuccess,
		IndexFileCount:   h.IndexFileCount,
		WatcherActive:    h.WatcherActive,
		CacheVersion:     cache.CacheVersion(),
	}

	// Get cache stats if cache manager is available
	if h.cacheManager != nil {
		if stats, err := h.cacheManager.GetStats(); err == nil {
			snapshot.CacheTotalEntries = stats.TotalEntries
			snapshot.CacheLegacyEntries = stats.LegacyEntries
			snapshot.CacheTotalSize = stats.TotalSize
		}
	}

	// Get semantic provider info if getter is available
	if h.providerInfoGetter != nil {
		snapshot.SemanticEnabled, snapshot.SemanticProvider, snapshot.SemanticModel = h.providerInfoGetter()
	}

	return snapshot
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
