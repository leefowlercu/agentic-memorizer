package events

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewBus(t *testing.T) {
	bus := NewBus()
	if bus == nil {
		t.Fatal("expected non-nil bus")
	}

	stats := bus.Stats()
	if stats.SubscriberCount != 0 {
		t.Errorf("expected 0 subscribers, got %d", stats.SubscriberCount)
	}
	if stats.IsClosed {
		t.Error("expected bus to not be closed")
	}
}

func TestBus_PublishSubscribe(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	var received atomic.Bool
	var receivedEvent Event

	unsubscribe := bus.Subscribe(FileDiscovered, func(event Event) {
		received.Store(true)
		receivedEvent = event
	})
	defer unsubscribe()

	event := NewEvent(FileDiscovered, FileEvent{
		Path:        "/test/file.go",
		ContentHash: "abc123",
		Size:        100,
		IsNew:       true,
	})

	err := bus.Publish(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for event to be processed
	time.Sleep(50 * time.Millisecond)

	if !received.Load() {
		t.Error("expected event to be received")
	}
	if receivedEvent.Type != FileDiscovered {
		t.Errorf("expected event type %s, got %s", FileDiscovered, receivedEvent.Type)
	}
}

func TestBus_MultipleSubscribers(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	var count atomic.Int32

	// Subscribe multiple handlers to the same event type
	for i := 0; i < 3; i++ {
		unsubscribe := bus.Subscribe(FileChanged, func(event Event) {
			count.Add(1)
		})
		defer unsubscribe()
	}

	event := NewEvent(FileChanged, FileEvent{Path: "/test/file.go"})
	err := bus.Publish(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for events to be processed
	time.Sleep(50 * time.Millisecond)

	if count.Load() != 3 {
		t.Errorf("expected 3 handlers to receive event, got %d", count.Load())
	}
}

func TestBus_SubscribeFiltersEventType(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	var receivedCount atomic.Int32

	// Subscribe only to FileDiscovered
	unsubscribe := bus.Subscribe(FileDiscovered, func(event Event) {
		receivedCount.Add(1)
	})
	defer unsubscribe()

	// Publish different event types
	bus.Publish(context.Background(), NewEvent(FileDiscovered, nil))
	bus.Publish(context.Background(), NewEvent(FileChanged, nil))
	bus.Publish(context.Background(), NewEvent(FileDeleted, nil))

	// Wait for events to be processed
	time.Sleep(50 * time.Millisecond)

	if receivedCount.Load() != 1 {
		t.Errorf("expected 1 event, got %d", receivedCount.Load())
	}
}

func TestBus_SubscribeAll(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	var receivedCount atomic.Int32

	unsubscribe := bus.SubscribeAll(func(event Event) {
		receivedCount.Add(1)
	})
	defer unsubscribe()

	// Publish different event types
	bus.Publish(context.Background(), NewEvent(FileDiscovered, nil))
	bus.Publish(context.Background(), NewEvent(FileChanged, nil))
	bus.Publish(context.Background(), NewEvent(FileDeleted, nil))
	bus.Publish(context.Background(), NewEvent(AnalysisComplete, nil))

	// Wait for events to be processed
	time.Sleep(50 * time.Millisecond)

	if receivedCount.Load() != 4 {
		t.Errorf("expected 4 events, got %d", receivedCount.Load())
	}
}

func TestBus_Unsubscribe(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	var receivedCount atomic.Int32

	unsubscribe := bus.Subscribe(FileDiscovered, func(event Event) {
		receivedCount.Add(1)
	})

	// Publish first event
	bus.Publish(context.Background(), NewEvent(FileDiscovered, nil))
	time.Sleep(50 * time.Millisecond)

	if receivedCount.Load() != 1 {
		t.Errorf("expected 1 event before unsubscribe, got %d", receivedCount.Load())
	}

	// Unsubscribe
	unsubscribe()

	// Publish second event
	bus.Publish(context.Background(), NewEvent(FileDiscovered, nil))
	time.Sleep(50 * time.Millisecond)

	// Should still be 1 (unsubscribed)
	if receivedCount.Load() != 1 {
		t.Errorf("expected 1 event after unsubscribe, got %d", receivedCount.Load())
	}
}

func TestBus_UnsubscribeIdempotent(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	unsubscribe := bus.Subscribe(FileDiscovered, func(event Event) {})

	// Call unsubscribe multiple times (should not panic)
	unsubscribe()
	unsubscribe()
	unsubscribe()
}

func TestBus_Close(t *testing.T) {
	bus := NewBus()

	var wg sync.WaitGroup
	wg.Add(1)

	bus.Subscribe(FileDiscovered, func(event Event) {
		wg.Done()
	})

	// Publish before close
	bus.Publish(context.Background(), NewEvent(FileDiscovered, nil))

	// Wait for event
	wg.Wait()

	// Close the bus
	err := bus.Close()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify bus is closed
	stats := bus.Stats()
	if !stats.IsClosed {
		t.Error("expected bus to be closed")
	}
	if stats.SubscriberCount != 0 {
		t.Errorf("expected 0 subscribers after close, got %d", stats.SubscriberCount)
	}

	// Publish after close should return error
	err = bus.Publish(context.Background(), NewEvent(FileDiscovered, nil))
	if err != ErrBusClosed {
		t.Errorf("expected ErrBusClosed, got %v", err)
	}
}

func TestBus_CloseIdempotent(t *testing.T) {
	bus := NewBus()

	// Close multiple times (should not panic or error)
	if err := bus.Close(); err != nil {
		t.Errorf("first close error: %v", err)
	}
	if err := bus.Close(); err != nil {
		t.Errorf("second close error: %v", err)
	}
}

func TestBus_SubscribeAfterClose(t *testing.T) {
	bus := NewBus()
	bus.Close()

	// Subscribe after close returns no-op unsubscribe
	unsubscribe := bus.Subscribe(FileDiscovered, func(event Event) {
		t.Error("handler should not be called")
	})

	// Should not panic
	unsubscribe()
}

func TestBus_ContextCancellation(t *testing.T) {
	bus := NewBus(WithBufferSize(1))
	defer bus.Close()

	// Fill the buffer with a blocking subscriber
	blocker := make(chan struct{})
	bus.Subscribe(FileDiscovered, func(event Event) {
		<-blocker // Block forever
	})

	// First event fills the buffer
	bus.Publish(context.Background(), NewEvent(FileDiscovered, nil))

	// Second event with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := bus.Publish(ctx, NewEvent(FileDiscovered, nil))
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	close(blocker)
}

func TestBus_HandlerPanicRecovery(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	var secondHandlerCalled atomic.Bool

	// First handler panics
	bus.Subscribe(FileDiscovered, func(event Event) {
		panic("test panic")
	})

	// Second handler should still receive events
	bus.Subscribe(FileDiscovered, func(event Event) {
		secondHandlerCalled.Store(true)
	})

	bus.Publish(context.Background(), NewEvent(FileDiscovered, nil))
	time.Sleep(50 * time.Millisecond)

	if !secondHandlerCalled.Load() {
		t.Error("expected second handler to be called despite first handler panic")
	}
}

func TestBus_ConcurrentPublish(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	var receivedCount atomic.Int32

	bus.Subscribe(FileDiscovered, func(event Event) {
		receivedCount.Add(1)
	})

	// Publish concurrently
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Publish(context.Background(), NewEvent(FileDiscovered, nil))
		}()
	}
	wg.Wait()

	// Wait for all events to be processed
	time.Sleep(100 * time.Millisecond)

	if receivedCount.Load() != 100 {
		t.Errorf("expected 100 events, got %d", receivedCount.Load())
	}
}

func TestBus_ConcurrentSubscribeUnsubscribe(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	// Concurrently subscribe and unsubscribe
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unsub := bus.Subscribe(FileDiscovered, func(event Event) {})
			time.Sleep(time.Millisecond)
			unsub()
		}()
	}
	wg.Wait()

	// Should have no subscribers left
	stats := bus.Stats()
	if stats.SubscriberCount != 0 {
		t.Errorf("expected 0 subscribers, got %d", stats.SubscriberCount)
	}
}

func TestBus_WithBufferSize(t *testing.T) {
	bus := NewBus(WithBufferSize(5))
	defer bus.Close()

	// Verify buffer size is applied (indirect test via stats)
	if bus.bufferSize != 5 {
		t.Errorf("expected buffer size 5, got %d", bus.bufferSize)
	}
}

func TestNewEvent(t *testing.T) {
	before := time.Now()
	event := NewEvent(FileDiscovered, FileEvent{Path: "/test"})
	after := time.Now()

	if event.Type != FileDiscovered {
		t.Errorf("expected type %s, got %s", FileDiscovered, event.Type)
	}

	if event.Timestamp.Before(before) || event.Timestamp.After(after) {
		t.Error("timestamp not within expected range")
	}

	payload, ok := event.Payload.(FileEvent)
	if !ok {
		t.Fatal("expected FileEvent payload")
	}
	if payload.Path != "/test" {
		t.Errorf("expected path /test, got %s", payload.Path)
	}
}

func TestEventTypes(t *testing.T) {
	tests := []struct {
		eventType EventType
		expected  string
	}{
		{FileDiscovered, "file.discovered"},
		{FileChanged, "file.changed"},
		{FileDeleted, "file.deleted"},
		{AnalysisComplete, "analysis.complete"},
		{AnalysisFailed, "analysis.failed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.eventType) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.eventType)
			}
		})
	}
}

func TestAnalysisTypes(t *testing.T) {
	tests := []struct {
		analysisType AnalysisType
		expected     string
	}{
		{AnalysisMetadata, "metadata"},
		{AnalysisSemantic, "semantic"},
		{AnalysisEmbeddings, "embeddings"},
		{AnalysisFull, "full"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.analysisType) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.analysisType)
			}
		})
	}
}
