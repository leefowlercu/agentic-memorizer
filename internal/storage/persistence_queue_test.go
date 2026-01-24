package storage

import (
	"context"
	"testing"
	"time"
)

func TestPersistenceQueue_EnqueueDequeue(t *testing.T) {
	s := newTestStorage(t)
	queue := s.PersistenceQueue()
	ctx := context.Background()

	// Enqueue an item
	resultJSON := []byte(`{"summary":"test summary","tags":["go","test"]}`)
	err := queue.Enqueue(ctx, "/test/file.go", "abc123", resultJSON)
	if err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	// Dequeue batch of 1
	items, err := queue.DequeueBatch(ctx, 1)
	if err != nil {
		t.Fatalf("failed to dequeue: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	item := items[0]
	if item.FilePath != "/test/file.go" {
		t.Errorf("expected file path '/test/file.go', got %q", item.FilePath)
	}
	if item.ContentHash != "abc123" {
		t.Errorf("expected content hash 'abc123', got %q", item.ContentHash)
	}
	if string(item.ResultJSON) != string(resultJSON) {
		t.Errorf("expected result JSON %q, got %q", resultJSON, item.ResultJSON)
	}
	if item.Status != QueueStatusInflight {
		t.Errorf("expected status %q, got %q", QueueStatusInflight, item.Status)
	}
	if item.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}
}

func TestPersistenceQueue_UpsertBehavior(t *testing.T) {
	s := newTestStorage(t)
	queue := s.PersistenceQueue()
	ctx := context.Background()

	// Enqueue initial item
	err := queue.Enqueue(ctx, "/test/file.go", "hash1", []byte(`{"version":1}`))
	if err != nil {
		t.Fatalf("failed to enqueue first: %v", err)
	}

	// Enqueue with same path and hash - should replace
	err = queue.Enqueue(ctx, "/test/file.go", "hash1", []byte(`{"version":2}`))
	if err != nil {
		t.Fatalf("failed to enqueue second: %v", err)
	}

	// Should only have 1 item
	stats, err := queue.Stats(ctx)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}
	if stats.Pending != 1 {
		t.Errorf("expected 1 pending item, got %d", stats.Pending)
	}

	// Dequeue and verify latest version
	items, err := queue.DequeueBatch(ctx, 1)
	if err != nil {
		t.Fatalf("failed to dequeue: %v", err)
	}

	if string(items[0].ResultJSON) != `{"version":2}` {
		t.Errorf("expected version 2, got %s", items[0].ResultJSON)
	}
}

func TestPersistenceQueue_DifferentHashCreatesNewEntry(t *testing.T) {
	s := newTestStorage(t)
	queue := s.PersistenceQueue()
	ctx := context.Background()

	// Enqueue with different hashes for same file - should create two entries
	err := queue.Enqueue(ctx, "/test/file.go", "hash1", []byte(`{"version":1}`))
	if err != nil {
		t.Fatalf("failed to enqueue first: %v", err)
	}

	err = queue.Enqueue(ctx, "/test/file.go", "hash2", []byte(`{"version":2}`))
	if err != nil {
		t.Fatalf("failed to enqueue second: %v", err)
	}

	// Should have 2 items
	stats, err := queue.Stats(ctx)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}
	if stats.Pending != 2 {
		t.Errorf("expected 2 pending items, got %d", stats.Pending)
	}
}

func TestPersistenceQueue_DequeueBatchEmpty(t *testing.T) {
	s := newTestStorage(t)
	queue := s.PersistenceQueue()
	ctx := context.Background()

	// Dequeue from empty queue
	items, err := queue.DequeueBatch(ctx, 10)
	if err != nil {
		t.Fatalf("failed to dequeue: %v", err)
	}

	if items != nil {
		t.Errorf("expected nil items from empty queue, got %d items", len(items))
	}
}

func TestPersistenceQueue_DequeueBatchZero(t *testing.T) {
	s := newTestStorage(t)
	queue := s.PersistenceQueue()
	ctx := context.Background()

	// Enqueue an item
	queue.Enqueue(ctx, "/test/file.go", "hash", []byte(`{}`))

	// Dequeue with n=0
	items, err := queue.DequeueBatch(ctx, 0)
	if err != nil {
		t.Fatalf("failed to dequeue: %v", err)
	}

	if items != nil {
		t.Errorf("expected nil items for n=0, got %d items", len(items))
	}
}

func TestPersistenceQueue_DequeueBatchLimit(t *testing.T) {
	s := newTestStorage(t)
	queue := s.PersistenceQueue()
	ctx := context.Background()

	// Enqueue multiple items
	for i := range 5 {
		err := queue.Enqueue(ctx, "/test/file"+string(rune('A'+i))+".go", "hash", []byte(`{}`))
		if err != nil {
			t.Fatalf("failed to enqueue %d: %v", i, err)
		}
	}

	// Dequeue with limit of 3
	items, err := queue.DequeueBatch(ctx, 3)
	if err != nil {
		t.Fatalf("failed to dequeue: %v", err)
	}

	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}

	// Verify remaining items are still pending
	stats, _ := queue.Stats(ctx)
	if stats.Pending != 2 {
		t.Errorf("expected 2 pending items, got %d", stats.Pending)
	}
	if stats.Inflight != 3 {
		t.Errorf("expected 3 inflight items, got %d", stats.Inflight)
	}
}

func TestPersistenceQueue_Complete(t *testing.T) {
	s := newTestStorage(t)
	queue := s.PersistenceQueue()
	ctx := context.Background()

	// Enqueue and dequeue
	queue.Enqueue(ctx, "/test/file.go", "hash", []byte(`{}`))
	items, _ := queue.DequeueBatch(ctx, 1)

	// Complete the item
	err := queue.Complete(ctx, items[0].ID)
	if err != nil {
		t.Fatalf("failed to complete: %v", err)
	}

	// Verify status
	stats, _ := queue.Stats(ctx)
	if stats.Completed != 1 {
		t.Errorf("expected 1 completed item, got %d", stats.Completed)
	}
	if stats.Inflight != 0 {
		t.Errorf("expected 0 inflight items, got %d", stats.Inflight)
	}
}

func TestPersistenceQueue_Complete_NotFound(t *testing.T) {
	s := newTestStorage(t)
	queue := s.PersistenceQueue()
	ctx := context.Background()

	err := queue.Complete(ctx, 99999)
	if err == nil {
		t.Error("expected error for non-existent item")
	}
}

func TestPersistenceQueue_FailWithRetry(t *testing.T) {
	s := newTestStorage(t)
	queue := s.PersistenceQueue()
	ctx := context.Background()

	// Enqueue and dequeue
	queue.Enqueue(ctx, "/test/file.go", "hash", []byte(`{}`))
	items, _ := queue.DequeueBatch(ctx, 1)

	// Fail the item (max retries = 3)
	err := queue.Fail(ctx, items[0].ID, 3, "connection refused")
	if err != nil {
		t.Fatalf("failed to fail item: %v", err)
	}

	// Should return to pending for retry
	stats, _ := queue.Stats(ctx)
	if stats.Pending != 1 {
		t.Errorf("expected 1 pending item, got %d", stats.Pending)
	}
	if stats.Failed != 0 {
		t.Errorf("expected 0 failed items, got %d", stats.Failed)
	}
}

func TestPersistenceQueue_FailMaxRetries(t *testing.T) {
	s := newTestStorage(t)
	queue := s.PersistenceQueue()
	ctx := context.Background()

	// Enqueue and fail multiple times
	queue.Enqueue(ctx, "/test/file.go", "hash", []byte(`{}`))

	// Fail 3 times (max retries = 3)
	for i := range 3 {
		items, _ := queue.DequeueBatch(ctx, 1)
		if len(items) == 0 {
			// If no pending items, break (item already failed)
			break
		}
		err := queue.Fail(ctx, items[0].ID, 3, "connection refused")
		if err != nil {
			t.Fatalf("failed to fail item on attempt %d: %v", i+1, err)
		}
	}

	// Should be marked as failed after 3 retries
	stats, _ := queue.Stats(ctx)
	if stats.Failed != 1 {
		t.Errorf("expected 1 failed item, got %d", stats.Failed)
	}
	if stats.Pending != 0 {
		t.Errorf("expected 0 pending items, got %d", stats.Pending)
	}
}

func TestPersistenceQueue_Fail_NotFound(t *testing.T) {
	s := newTestStorage(t)
	queue := s.PersistenceQueue()
	ctx := context.Background()

	err := queue.Fail(ctx, 99999, 3, "error")
	if err == nil {
		t.Error("expected error for non-existent item")
	}
}

func TestPersistenceQueue_Stats(t *testing.T) {
	s := newTestStorage(t)
	queue := s.PersistenceQueue()
	ctx := context.Background()

	// Empty queue
	stats, err := queue.Stats(ctx)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}
	if stats.Total() != 0 {
		t.Errorf("expected total 0, got %d", stats.Total())
	}

	// Create items in specific states, dequeuing all before creating next batch
	// This ensures FIFO ordering doesn't interfere with test expectations

	// 1. Create and complete one item
	queue.Enqueue(ctx, "/test/completed.go", "hash", []byte(`{}`))
	completedItems, _ := queue.DequeueBatch(ctx, 1)
	queue.Complete(ctx, completedItems[0].ID)

	// 2. Create and fail one item (exhaust retries)
	queue.Enqueue(ctx, "/test/failed.go", "hash", []byte(`{}`))
	for range 3 {
		failedItems, _ := queue.DequeueBatch(ctx, 1)
		if len(failedItems) > 0 {
			queue.Fail(ctx, failedItems[0].ID, 3, "permanent error")
		}
	}

	// 3. Create one inflight item
	queue.Enqueue(ctx, "/test/inflight.go", "hash", []byte(`{}`))
	inflightItems, _ := queue.DequeueBatch(ctx, 1)

	// 4. Create two pending items (added last so they stay pending)
	queue.Enqueue(ctx, "/test/pending1.go", "hash", []byte(`{}`))
	queue.Enqueue(ctx, "/test/pending2.go", "hash", []byte(`{}`))

	// Verify stats
	stats, _ = queue.Stats(ctx)
	if stats.Pending != 2 {
		t.Errorf("expected 2 pending, got %d", stats.Pending)
	}
	if stats.Inflight != 1 {
		t.Errorf("expected 1 inflight, got %d", stats.Inflight)
	}
	if stats.Completed != 1 {
		t.Errorf("expected 1 completed, got %d", stats.Completed)
	}
	if stats.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", stats.Failed)
	}
	if stats.Total() != 5 {
		t.Errorf("expected total 5, got %d", stats.Total())
	}

	// Verify OldestPending is set
	if stats.OldestPending == nil {
		t.Error("expected OldestPending to be set")
	}

	// Clean up inflight item
	queue.Complete(ctx, inflightItems[0].ID)
}

func TestPersistenceQueue_StatsOldestPending(t *testing.T) {
	s := newTestStorage(t)
	queue := s.PersistenceQueue()
	ctx := context.Background()

	// Track time before first enqueue
	beforeEnqueue := time.Now().UTC().Add(-time.Second)

	// Enqueue items with slight delay
	queue.Enqueue(ctx, "/test/first.go", "hash", []byte(`{}`))
	time.Sleep(10 * time.Millisecond)
	queue.Enqueue(ctx, "/test/second.go", "hash", []byte(`{}`))

	stats, _ := queue.Stats(ctx)

	if stats.OldestPending == nil {
		t.Fatal("expected OldestPending to be set")
	}

	// OldestPending should be around the time of first enqueue
	if stats.OldestPending.Before(beforeEnqueue) {
		t.Errorf("OldestPending %v is before first enqueue %v", stats.OldestPending, beforeEnqueue)
	}
}

func TestPersistenceQueue_Purge(t *testing.T) {
	s := newTestStorage(t)
	queue := s.PersistenceQueue()
	ctx := context.Background()

	// Create completed item
	queue.Enqueue(ctx, "/test/completed.go", "hash", []byte(`{}`))
	items, _ := queue.DequeueBatch(ctx, 1)
	queue.Complete(ctx, items[0].ID)

	// Create failed item
	queue.Enqueue(ctx, "/test/failed.go", "hash", []byte(`{}`))
	for range 3 {
		failedItems, _ := queue.DequeueBatch(ctx, 1)
		if len(failedItems) > 0 {
			queue.Fail(ctx, failedItems[0].ID, 3, "error")
		}
	}

	// Create pending item (should not be purged)
	queue.Enqueue(ctx, "/test/pending.go", "hash", []byte(`{}`))

	// Purge with zero duration (purge everything)
	purged, err := queue.Purge(ctx, 0, 0)
	if err != nil {
		t.Fatalf("failed to purge: %v", err)
	}

	if purged != 2 {
		t.Errorf("expected 2 purged items, got %d", purged)
	}

	// Verify only pending remains
	stats, _ := queue.Stats(ctx)
	if stats.Pending != 1 {
		t.Errorf("expected 1 pending, got %d", stats.Pending)
	}
	if stats.Completed != 0 {
		t.Errorf("expected 0 completed, got %d", stats.Completed)
	}
	if stats.Failed != 0 {
		t.Errorf("expected 0 failed, got %d", stats.Failed)
	}
}

func TestPersistenceQueue_PurgeWithRetention(t *testing.T) {
	s := newTestStorage(t)
	queue := s.PersistenceQueue()
	ctx := context.Background()

	// Create completed item
	queue.Enqueue(ctx, "/test/completed.go", "hash", []byte(`{}`))
	items, _ := queue.DequeueBatch(ctx, 1)
	queue.Complete(ctx, items[0].ID)

	// Purge with retention period longer than item age (should not purge)
	purged, err := queue.Purge(ctx, time.Hour, time.Hour)
	if err != nil {
		t.Fatalf("failed to purge: %v", err)
	}

	if purged != 0 {
		t.Errorf("expected 0 purged items, got %d", purged)
	}

	// Verify item still exists
	stats, _ := queue.Stats(ctx)
	if stats.Completed != 1 {
		t.Errorf("expected 1 completed, got %d", stats.Completed)
	}
}

func TestPersistenceQueue_FIFOOrder(t *testing.T) {
	s := newTestStorage(t)
	queue := s.PersistenceQueue()
	ctx := context.Background()

	// Enqueue items in order
	paths := []string{"/test/a.go", "/test/b.go", "/test/c.go"}
	for _, p := range paths {
		err := queue.Enqueue(ctx, p, "hash", []byte(`{}`))
		if err != nil {
			t.Fatalf("failed to enqueue %s: %v", p, err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure distinct timestamps
	}

	// Dequeue one at a time and verify order
	for i, expectedPath := range paths {
		items, err := queue.DequeueBatch(ctx, 1)
		if err != nil {
			t.Fatalf("failed to dequeue %d: %v", i, err)
		}
		if len(items) != 1 {
			t.Fatalf("expected 1 item at iteration %d, got %d", i, len(items))
		}
		if items[0].FilePath != expectedPath {
			t.Errorf("expected path %q at iteration %d, got %q", expectedPath, i, items[0].FilePath)
		}
		queue.Complete(ctx, items[0].ID)
	}
}

func TestPersistenceQueue_UpsertResetsRetryCount(t *testing.T) {
	s := newTestStorage(t)
	queue := s.PersistenceQueue()
	ctx := context.Background()

	// Enqueue and fail once
	queue.Enqueue(ctx, "/test/file.go", "hash", []byte(`{}`))
	items, _ := queue.DequeueBatch(ctx, 1)
	queue.Fail(ctx, items[0].ID, 5, "first error")

	// Re-enqueue same file+hash - should reset retry count
	queue.Enqueue(ctx, "/test/file.go", "hash", []byte(`{"new":"data"}`))

	// Dequeue and verify retry count is 0
	items, _ = queue.DequeueBatch(ctx, 1)
	if items[0].RetryCount != 0 {
		t.Errorf("expected retry count 0 after upsert, got %d", items[0].RetryCount)
	}
	if items[0].LastError != "" {
		t.Errorf("expected empty last error after upsert, got %q", items[0].LastError)
	}
}

func TestMarshalUnmarshalAnalysisResult(t *testing.T) {
	type testResult struct {
		Summary string   `json:"summary"`
		Tags    []string `json:"tags"`
	}

	original := testResult{
		Summary: "Test file analysis",
		Tags:    []string{"go", "test"},
	}

	// Marshal
	data, err := MarshalAnalysisResult(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal
	var result testResult
	err = UnmarshalAnalysisResult(data, &result)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result.Summary != original.Summary {
		t.Errorf("expected summary %q, got %q", original.Summary, result.Summary)
	}
	if len(result.Tags) != len(original.Tags) {
		t.Errorf("expected %d tags, got %d", len(original.Tags), len(result.Tags))
	}
}
