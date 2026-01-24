package analysis

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/storage"
)

// DrainConfig contains configuration for the drain worker.
type DrainConfig struct {
	// BatchSize is the number of items to dequeue in each batch.
	BatchSize int

	// MaxRetries is the maximum number of persistence attempts per item.
	MaxRetries int

	// RetryBackoff is the base delay between retry attempts.
	RetryBackoff time.Duration

	// CompletedRetention is how long to keep completed items before purging.
	CompletedRetention time.Duration

	// FailedRetention is how long to keep failed items before purging.
	FailedRetention time.Duration
}

// DefaultDrainConfig returns sensible defaults for the drain worker.
func DefaultDrainConfig() DrainConfig {
	return DrainConfig{
		BatchSize:          10,
		MaxRetries:         3,
		RetryBackoff:       time.Second,
		CompletedRetention: time.Hour,
		FailedRetention:    7 * 24 * time.Hour, // 7 days
	}
}

// DrainWorker automatically drains the persistence queue when the graph becomes available.
type DrainWorker struct {
	queue  storage.DurablePersistenceQueue
	graph  graph.Graph
	bus    events.Bus
	logger *slog.Logger
	config DrainConfig

	// draining prevents concurrent drain operations
	draining atomic.Bool

	// shutdown coordination
	stopChan  chan struct{}
	stopOnce  sync.Once
	wg        sync.WaitGroup
	unsubFn   func()
	unsubOnce sync.Once
}

// DrainWorkerOption configures the drain worker.
type DrainWorkerOption func(*DrainWorker)

// WithDrainConfig sets the drain worker configuration.
func WithDrainConfig(cfg DrainConfig) DrainWorkerOption {
	return func(w *DrainWorker) {
		w.config = cfg
	}
}

// WithDrainLogger sets the logger for the drain worker.
func WithDrainLogger(logger *slog.Logger) DrainWorkerOption {
	return func(w *DrainWorker) {
		w.logger = logger
	}
}

// NewDrainWorker creates a new drain worker.
func NewDrainWorker(queue storage.DurablePersistenceQueue, g graph.Graph, bus events.Bus, opts ...DrainWorkerOption) *DrainWorker {
	w := &DrainWorker{
		queue:    queue,
		graph:    g,
		bus:      bus,
		logger:   slog.Default(),
		config:   DefaultDrainConfig(),
		stopChan: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(w)
	}

	w.logger = w.logger.With("component", "drain_worker")
	return w
}

// Name returns the component name.
func (w *DrainWorker) Name() string {
	return "drain_worker"
}

// Start initializes the drain worker and begins monitoring for graph connectivity.
func (w *DrainWorker) Start(ctx context.Context) error {
	// Subscribe to GraphConnected events
	if w.bus != nil {
		w.unsubFn = w.bus.Subscribe(events.GraphConnected, w.handleGraphConnected)
	}

	// Check if graph is already connected at startup
	if w.graph != nil && w.graph.IsConnected() {
		w.logger.Info("graph connected at startup; triggering initial drain")
		w.wg.Go(func() {
			w.drain(ctx)
		})
	}

	return nil
}

// Stop shuts down the drain worker gracefully.
func (w *DrainWorker) Stop(ctx context.Context) error {
	w.stopOnce.Do(func() {
		close(w.stopChan)
	})

	// Unsubscribe from events
	w.unsubOnce.Do(func() {
		if w.unsubFn != nil {
			w.unsubFn()
		}
	})

	// Wait for in-flight drain to complete
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.logger.Debug("drain worker stopped cleanly")
	case <-ctx.Done():
		w.logger.Warn("drain worker stop timed out")
	}

	return nil
}

// handleGraphConnected is called when a GraphConnected event is received.
func (w *DrainWorker) handleGraphConnected(event events.Event) {
	w.logger.Info("graph connected; triggering drain")

	// Start drain in background goroutine
	w.wg.Go(func() {
		// Create context that respects stop signal
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Listen for stop signal in separate goroutine
		go func() {
			select {
			case <-w.stopChan:
				cancel()
			case <-ctx.Done():
			}
		}()

		w.drain(ctx)
	})
}

// drain processes all pending items in the queue.
// Uses atomic guard to prevent concurrent drain operations.
func (w *DrainWorker) drain(ctx context.Context) {
	// Atomic guard to prevent concurrent drains
	if !w.draining.CompareAndSwap(false, true) {
		w.logger.Debug("drain already in progress; skipping")
		return
	}
	defer w.draining.Store(false)

	drainStart := time.Now()
	var totalProcessed int64
	var totalFailed int64

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("drain interrupted by context cancellation",
				"processed", totalProcessed,
				"failed", totalFailed)
			return
		case <-w.stopChan:
			w.logger.Info("drain interrupted by stop signal",
				"processed", totalProcessed,
				"failed", totalFailed)
			return
		default:
		}

		// Check graph is still connected
		if w.graph == nil || !w.graph.IsConnected() {
			w.logger.Warn("graph disconnected during drain; stopping",
				"processed", totalProcessed,
				"failed", totalFailed)
			return
		}

		// Dequeue a batch
		batch, err := w.queue.DequeueBatch(ctx, w.config.BatchSize)
		if err != nil {
			w.logger.Error("failed to dequeue batch; stopping drain", "error", err)
			return
		}

		// No more items to process
		if len(batch) == 0 {
			break
		}

		// Process each item
		for _, item := range batch {
			if err := w.persistItem(ctx, item); err != nil {
				totalFailed++
				w.logger.Warn("failed to persist queued item",
					"id", item.ID,
					"path", item.FilePath,
					"error", err)

				// Mark as failed with retry tracking
				if failErr := w.queue.Fail(ctx, item.ID, w.config.MaxRetries, err.Error()); failErr != nil {
					w.logger.Error("failed to mark item as failed",
						"id", item.ID,
						"error", failErr)
				}
			} else {
				totalProcessed++
				if completeErr := w.queue.Complete(ctx, item.ID); completeErr != nil {
					w.logger.Error("failed to mark item as complete",
						"id", item.ID,
						"error", completeErr)
				}
			}
		}
	}

	// Purge old items after drain completes
	purged, err := w.queue.Purge(ctx, w.config.CompletedRetention, w.config.FailedRetention)
	if err != nil {
		w.logger.Warn("failed to purge old items", "error", err)
	} else if purged > 0 {
		w.logger.Debug("purged old items from queue", "count", purged)
	}

	w.logger.Info("persistence queue drain complete",
		"processed", totalProcessed,
		"failed", totalFailed,
		"purged", purged,
		"duration", time.Since(drainStart))
}

// persistItem persists a single queued result to the graph.
func (w *DrainWorker) persistItem(ctx context.Context, item *storage.QueuedResult) error {
	// Deserialize the analysis result
	var result AnalysisResult
	if err := storage.UnmarshalAnalysisResult(item.ResultJSON, &result); err != nil {
		return fmt.Errorf("failed to unmarshal result; %w", err)
	}

	// Use existing persistence stage logic
	persistenceStage := NewPersistenceStage(w.graph, WithPersistenceLogger(w.logger))
	if err := persistenceStage.Persist(ctx, &result); err != nil {
		return fmt.Errorf("failed to persist to graph; %w", err)
	}

	return nil
}

// TriggerDrain manually triggers a drain operation.
// This is useful for testing and for explicit drain requests.
func (w *DrainWorker) TriggerDrain(ctx context.Context) {
	w.wg.Go(func() {
		w.drain(ctx)
	})
}

// IsDraining returns true if a drain operation is currently in progress.
func (w *DrainWorker) IsDraining() bool {
	return w.draining.Load()
}
