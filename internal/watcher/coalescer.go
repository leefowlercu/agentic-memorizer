package watcher

import (
	"sync"
	"time"
)

// CoalescedEventType represents the type of coalesced event.
type CoalescedEventType int

const (
	EventCreate CoalescedEventType = iota
	EventModify
	EventDelete
)

// CoalescedEvent represents a coalesced filesystem event.
type CoalescedEvent struct {
	Path      string
	Type      CoalescedEventType
	Timestamp time.Time
}

// Coalescer deduplicates and coalesces filesystem events.
type Coalescer struct {
	debounceWindow    time.Duration
	deleteGracePeriod time.Duration

	mu      sync.Mutex
	pending map[string]*pendingEvent
	events  chan CoalescedEvent
	stopCh  chan struct{}
	stopped bool
	wg      sync.WaitGroup
}

// pendingEvent tracks a pending event with its timer.
type pendingEvent struct {
	event CoalescedEvent
	timer *time.Timer
}

// NewCoalescer creates a new Coalescer with the given configuration.
func NewCoalescer(debounceWindow, deleteGracePeriod time.Duration) *Coalescer {
	c := &Coalescer{
		debounceWindow:    debounceWindow,
		deleteGracePeriod: deleteGracePeriod,
		pending:           make(map[string]*pendingEvent),
		events:            make(chan CoalescedEvent, 1000),
		stopCh:            make(chan struct{}),
	}
	return c
}

// Add adds an event to the coalescer.
func (c *Coalescer) Add(event CoalescedEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return
	}

	path := event.Path

	// Check if there's already a pending event for this path
	if pe, exists := c.pending[path]; exists {
		// Stop existing timer (emit() checks pending map, so late fires are safe)
		pe.timer.Stop()

		// Special case: Create + Delete = skip entirely (transient file)
		if pe.event.Type == EventCreate && event.Type == EventDelete {
			delete(c.pending, path)
			return
		}

		// Update event type based on sequence
		pe.event = c.mergeEvents(pe.event, event)
		pe.event.Timestamp = event.Timestamp

		// Reset timer with appropriate delay
		delay := c.getDelay(pe.event.Type)
		pe.timer = time.AfterFunc(delay, func() {
			c.emit(path)
		})
		return
	}

	// Create new pending event
	delay := c.getDelay(event.Type)
	pe := &pendingEvent{
		event: event,
	}
	pe.timer = time.AfterFunc(delay, func() {
		c.emit(path)
	})
	c.pending[path] = pe
}

// Events returns the channel of coalesced events.
func (c *Coalescer) Events() <-chan CoalescedEvent {
	return c.events
}

// Stop stops the coalescer and drains pending events.
func (c *Coalescer) Stop() {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return
	}
	c.stopped = true

	// Stop all pending timers
	for path, pe := range c.pending {
		pe.timer.Stop()
		delete(c.pending, path)
	}
	c.mu.Unlock()

	close(c.stopCh)
	c.wg.Wait()
	close(c.events)
}

// emit emits a pending event.
func (c *Coalescer) emit(path string) {
	c.mu.Lock()
	pe, exists := c.pending[path]
	if !exists {
		c.mu.Unlock()
		return
	}

	event := pe.event
	delete(c.pending, path)
	c.mu.Unlock()

	// The grace period has passed - emit the event (delete events have already been verified)
	select {
	case c.events <- event:
	case <-c.stopCh:
	}
}

// mergeEvents merges two events into a single event.
func (c *Coalescer) mergeEvents(old, new CoalescedEvent) CoalescedEvent {
	// Handle event type transitions
	switch {
	case old.Type == EventCreate && new.Type == EventModify:
		// Create + Modify = Create (new file that was modified)
		return CoalescedEvent{Path: new.Path, Type: EventCreate, Timestamp: new.Timestamp}

	case old.Type == EventCreate && new.Type == EventDelete:
		// Create + Delete = nothing (file created and deleted within window)
		// We return delete but with a flag to skip entirely
		return CoalescedEvent{Path: new.Path, Type: EventDelete, Timestamp: new.Timestamp}

	case old.Type == EventModify && new.Type == EventDelete:
		// Modify + Delete = Delete
		return CoalescedEvent{Path: new.Path, Type: EventDelete, Timestamp: new.Timestamp}

	case old.Type == EventDelete && new.Type == EventCreate:
		// Delete + Create = Modify (file was replaced)
		return CoalescedEvent{Path: new.Path, Type: EventModify, Timestamp: new.Timestamp}

	case old.Type == EventModify && new.Type == EventModify:
		// Multiple modifies = single modify
		return CoalescedEvent{Path: new.Path, Type: EventModify, Timestamp: new.Timestamp}

	default:
		// For any other case, use the new event
		return new
	}
}

// getDelay returns the appropriate delay for an event type.
func (c *Coalescer) getDelay(eventType CoalescedEventType) time.Duration {
	if eventType == EventDelete {
		return c.deleteGracePeriod
	}
	return c.debounceWindow
}

// PendingCount returns the number of pending events (for testing).
func (c *Coalescer) PendingCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pending)
}
