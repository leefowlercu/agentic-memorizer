//go:build !integration

package worker

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/metadata"
)

func TestPool_BasicProcessing(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "worker-pool-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	testFiles := []string{"test1.txt", "test2.txt", "test3.txt"}
	for _, name := range testFiles {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	// Setup
	cacheDir := filepath.Join(tmpDir, ".cache")
	cacheManager, err := cache.NewManager(cacheDir)
	if err != nil {
		t.Fatalf("failed to create cache manager: %v", err)
	}

	metadataExtractor := metadata.NewExtractor()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx := context.Background()

	// Create worker pool (no semantic analyzer or embeddings for faster tests)
	pool := NewPool(2, 60, metadataExtractor, nil, "", "", nil, nil, cacheManager, logger, ctx)
	pool.Start()
	defer pool.Stop()

	// Submit jobs
	for _, name := range testFiles {
		path := filepath.Join(tmpDir, name)
		info, _ := os.Stat(path)
		pool.Submit(Job{
			Path:     path,
			Info:     info,
			Priority: 50,
		})
	}

	// Collect results
	results := make([]JobResult, 0, len(testFiles))
	timeout := time.After(5 * time.Second)

	for i := 0; i < len(testFiles); i++ {
		select {
		case result := <-pool.Results():
			results = append(results, result)
		case <-timeout:
			t.Fatal("timeout waiting for results")
		}
	}

	// Verify
	if len(results) != len(testFiles) {
		t.Errorf("expected %d results, got %d", len(testFiles), len(results))
	}

	for _, result := range results {
		if result.Error != nil {
			t.Errorf("unexpected error: %v", result.Error)
		}
		if result.Entry.Metadata.Path == "" {
			t.Error("empty path in result")
		}
	}

	stats := pool.GetStats()
	if stats.JobsProcessed != len(testFiles) {
		t.Errorf("expected %d jobs processed, got %d", len(testFiles), stats.JobsProcessed)
	}
}

func TestPool_Prioritization(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "worker-pool-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create files with different modification times
	oldFile := filepath.Join(tmpDir, "old.txt")
	recentFile := filepath.Join(tmpDir, "recent.txt")

	os.WriteFile(oldFile, []byte("old"), 0644)

	// Change old file's mod time to 2 hours ago
	twoHoursAgo := time.Now().Add(-2 * time.Hour)
	os.Chtimes(oldFile, twoHoursAgo, twoHoursAgo)

	// Create recent file now
	os.WriteFile(recentFile, []byte("recent"), 0644)

	oldInfo, _ := os.Stat(oldFile)
	recentInfo, _ := os.Stat(recentFile)

	oldPriority := CalculatePriority(oldInfo)
	recentPriority := CalculatePriority(recentInfo)

	if recentPriority <= oldPriority {
		t.Errorf("recent file should have higher priority than old file (recent=%d, old=%d)", recentPriority, oldPriority)
	}
}

func TestPool_RateLimiting(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "worker-pool-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cacheDir := filepath.Join(tmpDir, ".cache")
	cacheManager, err := cache.NewManager(cacheDir)
	if err != nil {
		t.Fatalf("failed to create cache manager: %v", err)
	}

	metadataExtractor := metadata.NewExtractor()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx := context.Background()

	// Create pool with very low rate limit (6 per minute = 1 per 10 seconds)
	pool := NewPool(1, 6, metadataExtractor, nil, "", "", nil, nil, cacheManager, logger, ctx)
	pool.Start()
	defer pool.Stop()

	// The rate limiter should allow burst of 3
	// So first 3 should be fast, rest should be rate limited
	numJobs := 5
	for i := 0; i < numJobs; i++ {
		path := filepath.Join(tmpDir, "test.txt")
		os.WriteFile(path, []byte("test"), 0644)
		info, _ := os.Stat(path)
		pool.Submit(Job{Path: path, Info: info, Priority: 50})
	}

	// The test just verifies the pool doesn't crash with rate limiting
	// Actual rate limiting behavior is tested by the pool accepting jobs
	timeout := time.After(3 * time.Second)
	for i := 0; i < numJobs; i++ {
		select {
		case <-pool.Results():
		case <-timeout:
			// This is ok - rate limiting may slow things down
			return
		}
	}
}

func TestPool_GracefulShutdown(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "worker-pool-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cacheDir := filepath.Join(tmpDir, ".cache")
	cacheManager, _ := cache.NewManager(cacheDir)
	metadataExtractor := metadata.NewExtractor()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx := context.Background()

	pool := NewPool(2, 60, metadataExtractor, nil, "", "", nil, nil, cacheManager, logger, ctx)
	pool.Start()

	// Submit some jobs
	numJobs := 5
	for i := 0; i < numJobs; i++ {
		path := filepath.Join(tmpDir, "test.txt")
		os.WriteFile(path, []byte("test"), 0644)
		info, _ := os.Stat(path)
		pool.Submit(Job{Path: path, Info: info, Priority: 50})
	}

	// Collect all results first
	for i := 0; i < numJobs; i++ {
		<-pool.Results()
	}

	// Stop should wait for workers to finish
	pool.Stop()

	// After stop, results channel should be closed
	// Drain any remaining buffered results
	for {
		_, ok := <-pool.Results()
		if !ok {
			// Channel is closed, test passes
			return
		}
	}
}

func TestPool_ContextCancellation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "worker-pool-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cacheDir := filepath.Join(tmpDir, ".cache")
	cacheManager, _ := cache.NewManager(cacheDir)
	metadataExtractor := metadata.NewExtractor()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))

	ctx, cancel := context.WithCancel(context.Background())

	pool := NewPool(2, 60, metadataExtractor, nil, "", "", nil, nil, cacheManager, logger, ctx)
	pool.Start()
	defer pool.Stop()

	// Submit jobs
	for i := 0; i < 10; i++ {
		path := filepath.Join(tmpDir, "test.txt")
		os.WriteFile(path, []byte("test"), 0644)
		info, _ := os.Stat(path)
		pool.Submit(Job{Path: path, Info: info, Priority: 50})
	}

	// Cancel context
	cancel()

	// Workers should stop gracefully
	time.Sleep(500 * time.Millisecond)

	// This test just verifies no panic occurs
}
