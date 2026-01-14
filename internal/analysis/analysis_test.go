package analysis

import (
	"context"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/events"
)

func TestQueueStats(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	queue := NewQueue(bus)

	stats := queue.Stats()
	if stats.State != QueueStateIdle {
		t.Errorf("State = %v, want QueueStateIdle", stats.State)
	}
	if stats.WorkerCount != 4 {
		t.Errorf("WorkerCount = %d, want 4", stats.WorkerCount)
	}
}

func TestQueueOptions(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	queue := NewQueue(bus,
		WithWorkerCount(8),
		WithBatchSize(20),
		WithMaxRetries(5),
		WithQueueCapacity(2000),
	)

	if queue.workerCount != 8 {
		t.Errorf("workerCount = %d, want 8", queue.workerCount)
	}
	if queue.batchSize != 20 {
		t.Errorf("batchSize = %d, want 20", queue.batchSize)
	}
	if queue.maxRetries != 5 {
		t.Errorf("maxRetries = %d, want 5", queue.maxRetries)
	}
	if queue.queueCapacity != 2000 {
		t.Errorf("queueCapacity = %d, want 2000", queue.queueCapacity)
	}
}

func TestQueueStartStop(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	queue := NewQueue(bus, WithWorkerCount(2))

	// Start
	err := queue.Start(context.Background())
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	stats := queue.Stats()
	if stats.State != QueueStateRunning {
		t.Errorf("State = %v, want QueueStateRunning", stats.State)
	}

	// Double start should fail
	err = queue.Start(context.Background())
	if err == nil {
		t.Error("Expected error on double start")
	}

	// Stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = queue.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	stats = queue.Stats()
	if stats.State != QueueStateStopped {
		t.Errorf("State = %v, want QueueStateStopped", stats.State)
	}
}

func TestQueueEnqueue(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	queue := NewQueue(bus, WithWorkerCount(1), WithQueueCapacity(10))

	// Enqueue before start should fail
	err := queue.Enqueue(WorkItem{FilePath: "/test/file.txt"})
	if err == nil {
		t.Error("Expected error when enqueueing to stopped queue")
	}

	// Start queue
	err = queue.Start(context.Background())
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer queue.Stop(context.Background())

	// Enqueue should succeed
	err = queue.Enqueue(WorkItem{FilePath: "/test/file.txt"})
	if err != nil {
		t.Errorf("Enqueue failed: %v", err)
	}

	// Give workers time to process
	time.Sleep(100 * time.Millisecond)

	stats := queue.Stats()
	if stats.PendingItems < 0 {
		t.Errorf("PendingItems should not be negative")
	}
}

func TestDegradationMode(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	queue := NewQueue(bus)

	tests := []struct {
		capacity float64
		expected DegradationMode
	}{
		{0.0, DegradationFull},
		{0.5, DegradationFull},
		{0.79, DegradationFull},
		{0.80, DegradationNoEmbed},
		{0.90, DegradationNoEmbed},
		{0.95, DegradationMetadata},
		{1.0, DegradationMetadata},
	}

	for _, tt := range tests {
		result := queue.getDegradationMode(tt.capacity)
		if result != tt.expected {
			t.Errorf("getDegradationMode(%v) = %v, want %v", tt.capacity, result, tt.expected)
		}
	}
}

func TestSetWorkerCount(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	queue := NewQueue(bus, WithWorkerCount(2))

	// Before start
	queue.SetWorkerCount(4)
	if queue.workerCount != 4 {
		t.Errorf("workerCount = %d, want 4", queue.workerCount)
	}

	// Start queue
	err := queue.Start(context.Background())
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer queue.Stop(context.Background())

	// Increase workers
	queue.SetWorkerCount(6)
	time.Sleep(100 * time.Millisecond)

	if len(queue.workers) != 6 {
		t.Errorf("worker count = %d, want 6", len(queue.workers))
	}

	// Decrease workers
	queue.SetWorkerCount(3)
	time.Sleep(100 * time.Millisecond)

	if len(queue.workers) != 3 {
		t.Errorf("worker count = %d, want 3", len(queue.workers))
	}

	// Invalid count should be ignored
	queue.SetWorkerCount(0)
	if len(queue.workers) != 3 {
		t.Errorf("worker count = %d, want 3", len(queue.workers))
	}
}

func TestMergeSemanticResults(t *testing.T) {
	t.Run("EmptyResults", func(t *testing.T) {
		result, err := mergeSemanticResults(context.Background(), nil, nil)
		if err != nil {
			t.Errorf("mergeSemanticResults failed: %v", err)
		}
		if result == nil {
			t.Error("Expected non-nil result")
		}
	})

	t.Run("SingleResult", func(t *testing.T) {
		input := &SemanticResult{
			Summary: "Test summary",
			Tags:    []string{"tag1", "tag2"},
		}

		result, err := mergeSemanticResults(context.Background(), nil, []*SemanticResult{input})
		if err != nil {
			t.Errorf("mergeSemanticResults failed: %v", err)
		}
		if result.Summary != "Test summary" {
			t.Errorf("Summary = %q, want %q", result.Summary, "Test summary")
		}
	})

	t.Run("DeduplicateTags", func(t *testing.T) {
		results := []*SemanticResult{
			{Tags: []string{"tag1", "tag2"}},
			{Tags: []string{"tag2", "tag3"}},
			{Tags: []string{"TAG1", "tag4"}}, // Case-insensitive dedup
		}

		merged, err := mergeSemanticResults(context.Background(), nil, results)
		if err != nil {
			t.Errorf("mergeSemanticResults failed: %v", err)
		}

		// Should have 4 unique tags (tag1, tag2, tag3, tag4)
		if len(merged.Tags) != 4 {
			t.Errorf("Expected 4 unique tags, got %d: %v", len(merged.Tags), merged.Tags)
		}
	})

	t.Run("DeduplicateEntities", func(t *testing.T) {
		results := []*SemanticResult{
			{Entities: []Entity{{Name: "Go", Type: "language"}}},
			{Entities: []Entity{{Name: "Go", Type: "language"}}}, // Duplicate
			{Entities: []Entity{{Name: "Python", Type: "language"}}},
		}

		merged, err := mergeSemanticResults(context.Background(), nil, results)
		if err != nil {
			t.Errorf("mergeSemanticResults failed: %v", err)
		}

		if len(merged.Entities) != 2 {
			t.Errorf("Expected 2 unique entities, got %d", len(merged.Entities))
		}
	})

	t.Run("MaxComplexity", func(t *testing.T) {
		results := []*SemanticResult{
			{Complexity: 3},
			{Complexity: 7},
			{Complexity: 5},
		}

		merged, err := mergeSemanticResults(context.Background(), nil, results)
		if err != nil {
			t.Errorf("mergeSemanticResults failed: %v", err)
		}

		if merged.Complexity != 7 {
			t.Errorf("Complexity = %d, want 7", merged.Complexity)
		}
	})

	t.Run("ConcatenateSummaries", func(t *testing.T) {
		results := []*SemanticResult{
			{Summary: "First part."},
			{Summary: "Second part."},
		}

		merged, err := mergeSemanticResults(context.Background(), nil, results)
		if err != nil {
			t.Errorf("mergeSemanticResults failed: %v", err)
		}

		if merged.Summary == "" {
			t.Error("Expected non-empty summary")
		}
	})
}

func TestWorkerBackoff(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	queue := NewQueue(bus)
	queue.retryDelay = time.Second

	worker := NewWorker(0, queue)

	tests := []struct {
		retries  int
		expected time.Duration
	}{
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
	}

	for _, tt := range tests {
		result := worker.calculateBackoff(tt.retries)
		if result != tt.expected {
			t.Errorf("calculateBackoff(%d) = %v, want %v", tt.retries, result, tt.expected)
		}
	}
}

func TestDetectMIMEType(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/test/file.go", "text/x-go"},
		{"/test/file.py", "text/x-python"},
		{"/test/file.js", "text/javascript"},
		{"/test/file.ts", "text/typescript"},
		{"/test/file.md", "text/markdown"},
		{"/test/file.json", "application/json"},
		{"/test/file.yaml", "text/yaml"},
		{"/test/file.unknown", "application/octet-stream"},
	}

	for _, tt := range tests {
		result := detectMIMEType(tt.path, nil)
		if result != tt.expected {
			t.Errorf("detectMIMEType(%q) = %q, want %q", tt.path, result, tt.expected)
		}
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/test/file.go", "go"},
		{"/test/file.py", "python"},
		{"/test/file.js", "javascript"},
		{"/test/file.ts", "typescript"},
		{"/test/file.rs", "rust"},
		{"/test/file.rb", "ruby"},
		{"/test/file.unknown", ""},
	}

	for _, tt := range tests {
		result := detectLanguage(tt.path)
		if result != tt.expected {
			t.Errorf("detectLanguage(%q) = %q, want %q", tt.path, result, tt.expected)
		}
	}
}

func TestComputeHashes(t *testing.T) {
	t.Run("ContentHash", func(t *testing.T) {
		content1 := []byte("hello")
		content2 := []byte("world")

		hash1 := computeContentHash(content1)
		hash2 := computeContentHash(content2)

		if hash1 == hash2 {
			t.Error("Different content should produce different hashes")
		}
		// SHA256 produces 32 bytes = 64 hex characters
		if len(hash1) != 64 {
			t.Errorf("Hash length = %d, want 64 (SHA256)", len(hash1))
		}
	})

	t.Run("MetadataHash", func(t *testing.T) {
		now := time.Now()

		hash1 := computeMetadataHash("/path/to/file1", 100, now)
		hash2 := computeMetadataHash("/path/to/file2", 100, now)

		if hash1 == hash2 {
			t.Error("Different paths should produce different hashes")
		}
	})
}
