package daemon

import (
	"os"
	"sync"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/cache"
)

func TestRecordFileProcessed(t *testing.T) {
	metrics := NewHealthMetrics()

	metrics.RecordFileProcessed()
	snapshot := metrics.GetSnapshot()

	if snapshot.FilesProcessed != 1 {
		t.Errorf("Expected FilesProcessed to be 1, got %d", snapshot.FilesProcessed)
	}

	// Record multiple times
	for i := 0; i < 5; i++ {
		metrics.RecordFileProcessed()
	}

	snapshot = metrics.GetSnapshot()
	if snapshot.FilesProcessed != 6 {
		t.Errorf("Expected FilesProcessed to be 6, got %d", snapshot.FilesProcessed)
	}
}

func TestRecordAPICall(t *testing.T) {
	metrics := NewHealthMetrics()

	metrics.RecordAPICall()
	snapshot := metrics.GetSnapshot()

	if snapshot.APICalls != 1 {
		t.Errorf("Expected APICalls to be 1, got %d", snapshot.APICalls)
	}

	// Record multiple times
	for i := 0; i < 10; i++ {
		metrics.RecordAPICall()
	}

	snapshot = metrics.GetSnapshot()
	if snapshot.APICalls != 11 {
		t.Errorf("Expected APICalls to be 11, got %d", snapshot.APICalls)
	}
}

func TestRecordCacheHit(t *testing.T) {
	metrics := NewHealthMetrics()

	metrics.RecordCacheHit()
	snapshot := metrics.GetSnapshot()

	if snapshot.CacheHits != 1 {
		t.Errorf("Expected CacheHits to be 1, got %d", snapshot.CacheHits)
	}

	// Record multiple times
	for i := 0; i < 7; i++ {
		metrics.RecordCacheHit()
	}

	snapshot = metrics.GetSnapshot()
	if snapshot.CacheHits != 8 {
		t.Errorf("Expected CacheHits to be 8, got %d", snapshot.CacheHits)
	}
}

func TestIncrementIndexFileCount(t *testing.T) {
	metrics := NewHealthMetrics()

	metrics.IncrementIndexFileCount()
	snapshot := metrics.GetSnapshot()

	if snapshot.IndexFileCount != 1 {
		t.Errorf("Expected IndexFileCount to be 1, got %d", snapshot.IndexFileCount)
	}

	// Increment multiple times
	for i := 0; i < 3; i++ {
		metrics.IncrementIndexFileCount()
	}

	snapshot = metrics.GetSnapshot()
	if snapshot.IndexFileCount != 4 {
		t.Errorf("Expected IndexFileCount to be 4, got %d", snapshot.IndexFileCount)
	}
}

func TestDecrementIndexFileCount(t *testing.T) {
	metrics := NewHealthMetrics()

	// Set initial count
	metrics.IncrementIndexFileCount()
	metrics.IncrementIndexFileCount()
	metrics.IncrementIndexFileCount()

	metrics.DecrementIndexFileCount()
	snapshot := metrics.GetSnapshot()

	if snapshot.IndexFileCount != 2 {
		t.Errorf("Expected IndexFileCount to be 2, got %d", snapshot.IndexFileCount)
	}

	// Decrement to zero
	metrics.DecrementIndexFileCount()
	metrics.DecrementIndexFileCount()

	snapshot = metrics.GetSnapshot()
	if snapshot.IndexFileCount != 0 {
		t.Errorf("Expected IndexFileCount to be 0, got %d", snapshot.IndexFileCount)
	}

	// Attempt to decrement below zero (should stay at zero)
	metrics.DecrementIndexFileCount()
	snapshot = metrics.GetSnapshot()
	if snapshot.IndexFileCount != 0 {
		t.Errorf("Expected IndexFileCount to remain at 0, got %d", snapshot.IndexFileCount)
	}
}

func TestHealthMetrics_ConcurrentIncrement(t *testing.T) {
	metrics := NewHealthMetrics()
	iterations := 100
	goroutines := 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Concurrently increment all metrics
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.RecordFileProcessed()
				metrics.RecordAPICall()
				metrics.RecordCacheHit()
				metrics.IncrementIndexFileCount()
			}
		}()
	}

	wg.Wait()

	expected := iterations * goroutines
	snapshot := metrics.GetSnapshot()

	if snapshot.FilesProcessed != expected {
		t.Errorf("Expected FilesProcessed to be %d, got %d", expected, snapshot.FilesProcessed)
	}
	if snapshot.APICalls != expected {
		t.Errorf("Expected APICalls to be %d, got %d", expected, snapshot.APICalls)
	}
	if snapshot.CacheHits != expected {
		t.Errorf("Expected CacheHits to be %d, got %d", expected, snapshot.CacheHits)
	}
	if snapshot.IndexFileCount != expected {
		t.Errorf("Expected IndexFileCount to be %d, got %d", expected, snapshot.IndexFileCount)
	}
}

func TestHealthMetrics_ConcurrentMixed(t *testing.T) {
	metrics := NewHealthMetrics()
	iterations := 50

	var wg sync.WaitGroup
	wg.Add(3)

	// Increment files processed
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			metrics.RecordFileProcessed()
		}
	}()

	// Increment API calls
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			metrics.RecordAPICall()
		}
	}()

	// Increment and decrement index count sequentially in the same goroutine
	go func() {
		defer wg.Done()
		// First increment all
		for i := 0; i < iterations; i++ {
			metrics.IncrementIndexFileCount()
		}
		// Then decrement half
		for i := 0; i < iterations/2; i++ {
			metrics.DecrementIndexFileCount()
		}
	}()

	wg.Wait()

	snapshot := metrics.GetSnapshot()

	if snapshot.FilesProcessed != iterations {
		t.Errorf("Expected FilesProcessed to be %d, got %d", iterations, snapshot.FilesProcessed)
	}
	if snapshot.APICalls != iterations {
		t.Errorf("Expected APICalls to be %d, got %d", iterations, snapshot.APICalls)
	}
	// Index count should be iterations - (iterations/2) = iterations/2
	expectedIndexCount := iterations / 2
	if snapshot.IndexFileCount != expectedIndexCount {
		t.Errorf("Expected IndexFileCount to be %d, got %d", expectedIndexCount, snapshot.IndexFileCount)
	}
}

func TestHealthMetrics_ConcurrentReadsAndWrites(t *testing.T) {
	metrics := NewHealthMetrics()
	iterations := 100

	var wg sync.WaitGroup
	wg.Add(3)

	// Writer goroutine
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			metrics.RecordFileProcessed()
			metrics.RecordAPICall()
			metrics.IncrementIndexFileCount()
		}
	}()

	// Reader goroutines (should not panic or deadlock)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				snapshot := metrics.GetSnapshot()
				// Just verify we can read without issues
				_ = snapshot.FilesProcessed
				_ = snapshot.APICalls
				_ = snapshot.IndexFileCount
			}
		}()
	}

	wg.Wait()

	snapshot := metrics.GetSnapshot()
	if snapshot.FilesProcessed != iterations {
		t.Errorf("Expected FilesProcessed to be %d, got %d", iterations, snapshot.FilesProcessed)
	}
}

func TestHealthMetrics_CacheStats(t *testing.T) {
	// Create temp cache directory
	tempDir, err := os.MkdirTemp("", "health-test-cache")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create cache manager
	cacheManager, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("failed to create cache manager: %v", err)
	}

	// Create metrics without cache manager
	metrics := NewHealthMetrics()
	snapshot := metrics.GetSnapshot()

	// Should have cache version even without manager
	if snapshot.CacheVersion != cache.CacheVersion() {
		t.Errorf("Expected CacheVersion %q, got %q", cache.CacheVersion(), snapshot.CacheVersion)
	}

	// Without cache manager, stats should be zero
	if snapshot.CacheTotalEntries != 0 {
		t.Errorf("Expected CacheTotalEntries to be 0 without manager, got %d", snapshot.CacheTotalEntries)
	}

	// Set cache manager
	metrics.SetCacheManager(cacheManager)
	snapshot = metrics.GetSnapshot()

	// With empty cache, stats should still be zero
	if snapshot.CacheTotalEntries != 0 {
		t.Errorf("Expected CacheTotalEntries to be 0 for empty cache, got %d", snapshot.CacheTotalEntries)
	}

	// Verify version is reported
	if snapshot.CacheVersion == "" {
		t.Error("Expected CacheVersion to be set")
	}
}

func TestHealthMetrics_SetCacheManager(t *testing.T) {
	metrics := NewHealthMetrics()

	// Test setting nil (should not panic)
	metrics.SetCacheManager(nil)
	snapshot := metrics.GetSnapshot()

	// Should still have version but no stats
	if snapshot.CacheVersion == "" {
		t.Error("Expected CacheVersion to be set even with nil manager")
	}

	// Create temp cache directory
	tempDir, err := os.MkdirTemp("", "health-test-cache")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cacheManager, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("failed to create cache manager: %v", err)
	}

	// Set valid cache manager
	metrics.SetCacheManager(cacheManager)
	snapshot = metrics.GetSnapshot()

	// Verify cache manager is used
	if snapshot.CacheVersion != cache.CacheVersion() {
		t.Errorf("Expected CacheVersion %q, got %q", cache.CacheVersion(), snapshot.CacheVersion)
	}
}
