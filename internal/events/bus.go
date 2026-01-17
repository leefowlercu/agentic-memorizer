package events

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/metrics"
)

// Bus is the interface for the event bus.
type Bus interface {
	// Publish sends an event to all subscribers of the event type.
	// Returns an error if the bus is closed.
	Publish(ctx context.Context, event Event) error

	// Subscribe registers a handler for a specific event type.
	// Returns an unsubscribe function that removes the subscription.
	Subscribe(eventType EventType, handler EventHandler) (unsubscribe func())

	// SubscribeAll registers a handler for all event types.
	// Returns an unsubscribe function that removes the subscription.
	SubscribeAll(handler EventHandler) (unsubscribe func())

	// Close shuts down the event bus and drains pending events.
	Close() error
}

// subscription represents a registered event handler.
type subscription struct {
	id           uint64
	eventType    EventType // empty string means subscribe to all
	handler      EventHandler
	events       chan Event
	done         chan struct{}
	unsubscribed atomic.Bool
}

// EventBus is the default implementation of the Bus interface.
type EventBus struct {
	mu            sync.RWMutex
	subscriptions map[uint64]*subscription
	nextID        atomic.Uint64
	closed        atomic.Bool
	logger        *slog.Logger

	// bufferSize is the size of each subscriber's event buffer.
	bufferSize int

	// dropCount tracks how many events were dropped due to backpressure.
	dropCount atomic.Int64

	lastStatsTime  time.Time
	lastStatsDrops int64

	criticalQueue CriticalQueue
	criticalTypes map[EventType]bool
	stopDrain     chan struct{}
}

// BusOption configures the event bus.
type BusOption func(*EventBus)

// WithBufferSize sets the buffer size for subscriber event channels.
func WithBufferSize(size int) BusOption {
	return func(b *EventBus) {
		if size > 0 {
			b.bufferSize = size
		}
	}
}

// WithLogger sets the logger for the event bus.
func WithLogger(logger *slog.Logger) BusOption {
	return func(b *EventBus) {
		b.logger = logger
	}
}

// WithCriticalQueue enables a durable queue for critical events.
func WithCriticalQueue(queue CriticalQueue, criticalTypes []EventType) BusOption {
	return func(b *EventBus) {
		b.criticalQueue = queue
		if len(criticalTypes) > 0 {
			b.criticalTypes = make(map[EventType]bool)
			for _, t := range criticalTypes {
				b.criticalTypes[t] = true
			}
		}
	}
}

// NewBus creates a new event bus with the given options.
func NewBus(opts ...BusOption) *EventBus {
	b := &EventBus{
		subscriptions: make(map[uint64]*subscription),
		bufferSize:    100, // default buffer size
		logger:        slog.Default(),
		stopDrain:     make(chan struct{}),
	}

	for _, opt := range opts {
		opt(b)
	}

	// Start critical drain if configured
	if b.criticalQueue != nil {
		go b.drainCritical()
	}

	return b
}

// Publish sends an event to all subscribers of the event type.
func (b *EventBus) Publish(ctx context.Context, event Event) error {
	if b.closed.Load() {
		return ErrBusClosed
	}

	// Critical events go to durable queue if configured
	if b.criticalQueue != nil && b.criticalTypes != nil && b.criticalTypes[event.Type] {
		if err := b.criticalQueue.Enqueue(event); err != nil {
			b.logger.Warn("critical queue enqueue failed", "error", err, "event_type", event.Type)
			b.dropCount.Add(1)
			return err
		}
		return nil
	}

	return b.publishToSubscribers(ctx, event)
}

func (b *EventBus) publishToSubscribers(ctx context.Context, event Event) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subscriptions {
		if sub.eventType != "" && sub.eventType != event.Type {
			continue
		}
		select {
		case sub.events <- event:
		case <-ctx.Done():
			return ctx.Err()
		default:
			b.logger.Warn("event bus subscriber buffer full, dropping event",
				"event_type", event.Type,
				"subscriber_id", sub.id,
			)
			b.dropCount.Add(1)
			metrics.EventBusDroppedEvents.WithLabelValues(string(event.Type)).Inc()
		}
	}
	return nil
}

// Subscribe registers a handler for a specific event type.
func (b *EventBus) Subscribe(eventType EventType, handler EventHandler) func() {
	return b.subscribe(eventType, handler)
}

// SubscribeAll registers a handler for all event types.
func (b *EventBus) SubscribeAll(handler EventHandler) func() {
	return b.subscribe("", handler)
}

func (b *EventBus) subscribe(eventType EventType, handler EventHandler) func() {
	if b.closed.Load() {
		// Return no-op unsubscribe if bus is closed
		return func() {}
	}

	id := b.nextID.Add(1)
	sub := &subscription{
		id:        id,
		eventType: eventType,
		handler:   handler,
		events:    make(chan Event, b.bufferSize),
		done:      make(chan struct{}),
	}

	b.mu.Lock()
	b.subscriptions[id] = sub
	b.mu.Unlock()

	// Start goroutine to process events for this subscriber
	go b.processEvents(sub)

	// Return unsubscribe function
	return func() {
		b.unsubscribe(id)
	}
}

// processEvents handles events for a single subscription.
func (b *EventBus) processEvents(sub *subscription) {
	for {
		select {
		case event, ok := <-sub.events:
			if !ok {
				// Channel closed, subscription removed
				return
			}
			// Call handler (recover from panics to not crash the bus)
			b.safeCall(sub, event)
		case <-sub.done:
			// Drain remaining events before exiting
			for {
				select {
				case event, ok := <-sub.events:
					if !ok {
						return
					}
					b.safeCall(sub, event)
				default:
					return
				}
			}
		}
	}
}

// safeCall invokes the handler with panic recovery.
func (b *EventBus) safeCall(sub *subscription, event Event) {
	defer func() {
		if r := recover(); r != nil {
			b.logger.Error("event handler panicked",
				"subscriber_id", sub.id,
				"event_type", event.Type,
				"panic", r,
			)
		}
	}()

	sub.handler(event)
}

// unsubscribe removes a subscription by ID.
func (b *EventBus) unsubscribe(id uint64) {
	b.mu.Lock()
	sub, ok := b.subscriptions[id]
	if ok {
		delete(b.subscriptions, id)
	}
	b.mu.Unlock()

	if ok && sub.unsubscribed.CompareAndSwap(false, true) {
		// Signal done and close channels (only once)
		close(sub.done)
		close(sub.events)
	}
}

// Close shuts down the event bus and drains pending events.
func (b *EventBus) Close() error {
	if b.closed.Swap(true) {
		// Already closed
		return nil
	}

	if b.stopDrain != nil {
		close(b.stopDrain)
	}

	if b.criticalQueue != nil {
		_ = b.criticalQueue.Close()
	}

	b.mu.Lock()
	subs := make([]*subscription, 0, len(b.subscriptions))
	for _, sub := range b.subscriptions {
		subs = append(subs, sub)
	}
	b.subscriptions = make(map[uint64]*subscription)
	b.mu.Unlock()

	// Close all subscriptions (with double-close protection)
	for _, sub := range subs {
		if sub.unsubscribed.CompareAndSwap(false, true) {
			close(sub.done)
			close(sub.events)
		}
	}

	return nil
}

// Stats returns current bus statistics.
func (b *EventBus) Stats() BusStats {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastStatsTime)
	if b.lastStatsTime.IsZero() {
		elapsed = 0
	}
	drops := b.dropCount.Load()
	deltaDrops := drops - b.lastStatsDrops
	var rate float64
	if elapsed > 0 {
		rate = float64(deltaDrops) / elapsed.Seconds()
	}
	b.lastStatsTime = now
	b.lastStatsDrops = drops

	var critLen int
	var critCap int
	if b.criticalQueue != nil {
		if l, err := b.criticalQueue.Len(); err == nil {
			critLen = l
		}
		critCap = b.criticalQueue.Cap()
	}

	return BusStats{
		SubscriberCount: len(b.subscriptions),
		IsClosed:        b.closed.Load(),
		Dropped:         b.dropCount.Load(),
		DropRatePerSec:  rate,
		CriticalLen:     critLen,
		CriticalCap:     critCap,
	}
}

// drainCritical moves events from the durable queue to subscribers.
func (b *EventBus) drainCritical() {
	ctx := context.Background()
	for {
		select {
		case <-b.stopDrain:
			return
		default:
		}

		event, err := b.criticalQueue.Dequeue(ctx)
		if err != nil {
			if err == context.Canceled || err.Error() == "queue closed" {
				return
			}
			b.logger.Warn("critical queue dequeue failed", "error", err)
			time.Sleep(time.Second)
			continue
		}

		if err := b.publishToSubscribers(ctx, event); err != nil {
			b.logger.Warn("failed to deliver critical event", "error", err, "event_type", event.Type)
		}
	}
}

// BusStats contains event bus statistics.
type BusStats struct {
	SubscriberCount int
	IsClosed        bool
	Dropped         int64
	DropRatePerSec  float64
	CriticalLen     int
	CriticalCap     int
}
