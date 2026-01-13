package analysis

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/events"
)

// QueueState represents the current state of the analysis queue.
type QueueState int

const (
	QueueStateIdle QueueState = iota
	QueueStateRunning
	QueueStateStopping
	QueueStateStopped
)

// QueueStats contains statistics about queue operation.
type QueueStats struct {
	State           QueueState
	WorkerCount     int
	ActiveWorkers   int
	PendingItems    int
	ProcessedItems  int64
	FailedItems     int64
	AvgProcessTime  time.Duration
	Capacity        float64 // 0.0 - 1.0
	DegradationMode DegradationMode
}

// DegradationMode indicates the current analysis mode.
type DegradationMode int

const (
	DegradationFull     DegradationMode = iota // Full analysis
	DegradationNoEmbed                         // Skip embeddings
	DegradationMetadata                        // Metadata only
)

// Queue manages analysis work items and workers.
type Queue struct {
	mu            sync.RWMutex
	logger        *slog.Logger
	bus           events.Bus
	workerCount   int
	batchSize     int
	maxRetries    int
	retryDelay    time.Duration
	queueCapacity int

	state        QueueState
	workChan     chan WorkItem
	workers      []*Worker
	wg           sync.WaitGroup
	stopChan     chan struct{}
	ctx          context.Context
	cancelFn     context.CancelFunc

	// Stats
	processedCount atomic.Int64
	failedCount    atomic.Int64
	activeWorkers  atomic.Int32
	totalProcTime  atomic.Int64
}

// QueueOption configures the analysis queue.
type QueueOption func(*Queue)

// WithWorkerCount sets the number of concurrent workers.
func WithWorkerCount(n int) QueueOption {
	return func(q *Queue) {
		if n > 0 {
			q.workerCount = n
		}
	}
}

// WithBatchSize sets the batch size for processing.
func WithBatchSize(n int) QueueOption {
	return func(q *Queue) {
		if n > 0 {
			q.batchSize = n
		}
	}
}

// WithMaxRetries sets the maximum retry count.
func WithMaxRetries(n int) QueueOption {
	return func(q *Queue) {
		if n >= 0 {
			q.maxRetries = n
		}
	}
}

// WithQueueCapacity sets the maximum queue size.
func WithQueueCapacity(n int) QueueOption {
	return func(q *Queue) {
		if n > 0 {
			q.queueCapacity = n
		}
	}
}

// WithLogger sets the logger for the queue.
func WithLogger(logger *slog.Logger) QueueOption {
	return func(q *Queue) {
		q.logger = logger
	}
}

// NewQueue creates a new analysis queue.
func NewQueue(bus events.Bus, opts ...QueueOption) *Queue {
	q := &Queue{
		logger:        slog.Default(),
		bus:           bus,
		workerCount:   4,
		batchSize:     10,
		maxRetries:    3,
		retryDelay:    time.Second,
		queueCapacity: 1000,
		state:         QueueStateIdle,
	}

	for _, opt := range opts {
		opt(q)
	}

	return q
}

// Name returns the component name.
func (q *Queue) Name() string {
	return "analysis-queue"
}

// Start initializes and starts the queue workers.
func (q *Queue) Start(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.state == QueueStateRunning {
		return fmt.Errorf("queue already running")
	}

	q.ctx, q.cancelFn = context.WithCancel(ctx)
	q.stopChan = make(chan struct{})
	q.workChan = make(chan WorkItem, q.queueCapacity)
	q.state = QueueStateRunning

	// Start workers
	q.workers = make([]*Worker, q.workerCount)
	for i := 0; i < q.workerCount; i++ {
		worker := NewWorker(i, q)
		q.workers[i] = worker
		q.wg.Add(1)
		go func(w *Worker) {
			defer q.wg.Done()
			w.Run(q.ctx)
		}(worker)
	}

	// Subscribe to file events
	q.subscribeToEvents()

	q.logger.Info("analysis queue started",
		"workers", q.workerCount,
		"capacity", q.queueCapacity)

	return nil
}

// Stop gracefully shuts down the queue.
func (q *Queue) Stop(ctx context.Context) error {
	q.mu.Lock()
	if q.state != QueueStateRunning {
		q.mu.Unlock()
		return nil
	}
	q.state = QueueStateStopping
	q.mu.Unlock()

	// Signal stop
	close(q.stopChan)
	q.cancelFn()

	// Wait for workers with timeout
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		q.logger.Info("all workers stopped gracefully")
	case <-ctx.Done():
		q.logger.Warn("worker shutdown timed out")
	}

	q.mu.Lock()
	close(q.workChan)
	q.state = QueueStateStopped
	q.mu.Unlock()

	return nil
}

// subscribeToEvents registers event handlers.
func (q *Queue) subscribeToEvents() {
	// Subscribe to file discovery events
	q.bus.Subscribe(events.FileDiscovered, func(e events.Event) {
		if fe, ok := e.Payload.(*events.FileEvent); ok {
			q.Enqueue(WorkItem{
				FilePath:  fe.Path,
				FileSize:  fe.Size,
				ModTime:   fe.ModTime,
				EventType: WorkItemNew,
			})
		}
	})

	// Subscribe to file change events
	q.bus.Subscribe(events.FileChanged, func(e events.Event) {
		if fe, ok := e.Payload.(*events.FileEvent); ok {
			q.Enqueue(WorkItem{
				FilePath:  fe.Path,
				FileSize:  fe.Size,
				ModTime:   fe.ModTime,
				EventType: WorkItemChanged,
			})
		}
	})
}

// Enqueue adds a work item to the queue.
func (q *Queue) Enqueue(item WorkItem) error {
	q.mu.RLock()
	if q.state != QueueStateRunning {
		q.mu.RUnlock()
		return fmt.Errorf("queue not running")
	}
	q.mu.RUnlock()

	select {
	case q.workChan <- item:
		return nil
	default:
		return fmt.Errorf("queue full; capacity=%d", q.queueCapacity)
	}
}

// Stats returns current queue statistics.
func (q *Queue) Stats() QueueStats {
	q.mu.RLock()
	state := q.state
	workerCount := q.workerCount
	q.mu.RUnlock()

	pending := len(q.workChan)
	processed := q.processedCount.Load()
	failed := q.failedCount.Load()
	active := int(q.activeWorkers.Load())

	var avgTime time.Duration
	if processed > 0 {
		avgTime = time.Duration(q.totalProcTime.Load() / processed)
	}

	capacity := float64(pending) / float64(q.queueCapacity)
	mode := q.getDegradationMode(capacity)

	return QueueStats{
		State:           state,
		WorkerCount:     workerCount,
		ActiveWorkers:   active,
		PendingItems:    pending,
		ProcessedItems:  processed,
		FailedItems:     failed,
		AvgProcessTime:  avgTime,
		Capacity:        capacity,
		DegradationMode: mode,
	}
}

// getDegradationMode returns the current mode based on capacity.
func (q *Queue) getDegradationMode(capacity float64) DegradationMode {
	switch {
	case capacity >= 0.95:
		return DegradationMetadata
	case capacity >= 0.80:
		return DegradationNoEmbed
	default:
		return DegradationFull
	}
}

// SetWorkerCount adjusts the worker count dynamically.
func (q *Queue) SetWorkerCount(n int) {
	if n <= 0 {
		return
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	if q.state != QueueStateRunning {
		q.workerCount = n
		return
	}

	current := len(q.workers)
	if n == current {
		return
	}

	if n > current {
		// Add workers
		for i := current; i < n; i++ {
			worker := NewWorker(i, q)
			q.workers = append(q.workers, worker)
			q.wg.Add(1)
			go func(w *Worker) {
				defer q.wg.Done()
				w.Run(q.ctx)
			}(worker)
		}
	} else {
		// Signal excess workers to stop
		for i := n; i < current; i++ {
			q.workers[i].Stop()
		}
		q.workers = q.workers[:n]
	}

	q.workerCount = n
	q.logger.Info("worker count adjusted", "count", n)
}

// SetBatchSize adjusts the batch size.
func (q *Queue) SetBatchSize(n int) {
	if n <= 0 {
		return
	}
	q.mu.Lock()
	q.batchSize = n
	q.mu.Unlock()
}

// recordSuccess records a successful processing.
func (q *Queue) recordSuccess(duration time.Duration) {
	q.processedCount.Add(1)
	q.totalProcTime.Add(int64(duration))
}

// recordFailure records a failed processing.
func (q *Queue) recordFailure() {
	q.failedCount.Add(1)
}

// publishAnalysisComplete publishes a success event.
func (q *Queue) publishAnalysisComplete(path string, result *AnalysisResult) {
	q.bus.Publish(q.ctx, events.NewEvent(events.AnalysisComplete, &events.AnalysisEvent{
		Path:         path,
		ContentHash:  result.ContentHash,
		AnalysisType: events.AnalysisFull,
		Duration:     result.ProcessingTime,
	}))
}

// publishAnalysisFailed publishes a failure event.
func (q *Queue) publishAnalysisFailed(path string, err error) {
	q.bus.Publish(q.ctx, events.NewEvent(events.AnalysisFailed, &events.AnalysisEvent{
		Path:  path,
		Error: err.Error(),
	}))
}
