package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/events"
)

func TestStartStopEventListener(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, reg, bus, DefaultConfig())

	ctx := context.Background()

	// Start the server (which starts the event listener)
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify the listener is started
	if s.unsubscribe == nil {
		t.Error("unsubscribe function should be set after Start")
	}

	// Stop the server
	if err := s.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify cleanup
	if s.unsubscribe != nil {
		t.Error("unsubscribe function should be nil after Stop")
	}
}

func TestStartEventListenerWithoutBus(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	// Create server without event bus
	s := NewServer(g, reg, nil, DefaultConfig())

	ctx := context.Background()

	// Start should succeed even without event bus
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// unsubscribe should be nil since there's no bus
	if s.unsubscribe != nil {
		t.Error("unsubscribe function should be nil when no bus")
	}

	if err := s.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestHandleAnalysisCompleteEvent(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, reg, bus, DefaultConfig())

	ctx := context.Background()

	// Start the server
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer s.Stop(ctx)

	// Subscribe to the file resource
	subscriber := &Subscriber{
		ID:        "test-subscriber",
		SessionID: "test-session",
	}
	s.subs.Subscribe(ResourceURIFilePrefix+"/test/file.go", subscriber)

	// Verify subscription
	if !s.subs.HasSubscribers(ResourceURIFilePrefix + "/test/file.go") {
		t.Error("expected subscribers for file URI")
	}

	// Publish an analysis complete event
	event := events.NewEvent(events.AnalysisComplete, events.AnalysisEvent{
		Path:         "/test/file.go",
		ContentHash:  "abc123",
		AnalysisType: events.AnalysisFull,
		Duration:     100 * time.Millisecond,
	})

	if err := bus.Publish(ctx, event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Give the event listener time to process
	time.Sleep(50 * time.Millisecond)

	// The notification is sent via mcpServer.SendNotificationToAllClients
	// We can't easily verify it was sent without more mocking, but the
	// code path should have been exercised without panic
}

func TestNotifyRebuildComplete(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, reg, bus, DefaultConfig())

	// Subscribe to index resources
	subscriber := &Subscriber{
		ID:        "test-subscriber",
		SessionID: "test-session",
	}
	s.subs.Subscribe(ResourceURIIndex, subscriber)
	s.subs.Subscribe(ResourceURIIndexJSON, subscriber)

	// Verify subscriptions
	if !s.subs.HasSubscribers(ResourceURIIndex) {
		t.Error("expected subscribers for index URI")
	}
	if !s.subs.HasSubscribers(ResourceURIIndexJSON) {
		t.Error("expected subscribers for JSON index URI")
	}

	// Call NotifyRebuildComplete directly - should not panic
	s.NotifyRebuildComplete()
}

func TestHandleRebuildCompleteEvent(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, reg, bus, DefaultConfig())

	ctx := context.Background()

	// Start the server (which starts the event listener)
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer s.Stop(ctx)

	// Subscribe to index resources
	subscriber := &Subscriber{
		ID:        "test-subscriber",
		SessionID: "test-session",
	}
	s.subs.Subscribe(ResourceURIIndex, subscriber)

	// Publish a RebuildComplete event
	event := events.NewEvent(events.RebuildComplete, events.RebuildCompleteEvent{
		FilesQueued:   100,
		DirsProcessed: 10,
		Duration:      5 * time.Second,
		Full:          true,
	})

	if err := bus.Publish(ctx, event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Give the event listener time to process
	time.Sleep(50 * time.Millisecond)

	// The notification should have been sent via mcpServer.SendNotificationToAllClients
	// We can't easily verify it was sent without more mocking, but the
	// code path should have been exercised without panic
}

func TestHandleInvalidRebuildEventPayload(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, reg, bus, DefaultConfig())

	// Create event with invalid payload type
	event := events.Event{
		Type:      events.RebuildComplete,
		Timestamp: time.Now(),
		Payload:   "invalid payload type", // Should be events.RebuildCompleteEvent
	}

	// Should not panic
	s.handleRebuildCompleteEvent(event)
}

func TestNotifyFileChanged(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, reg, bus, DefaultConfig())

	// Subscribe to a file resource
	subscriber := &Subscriber{
		ID:        "test-subscriber",
		SessionID: "test-session",
	}
	filePath := "/test/file.go"
	s.subs.Subscribe(ResourceURIFilePrefix+filePath, subscriber)

	// Verify subscription
	if !s.subs.HasSubscribers(ResourceURIFilePrefix + filePath) {
		t.Error("expected subscribers for file URI")
	}

	// Call NotifyFileChanged - should not panic
	s.NotifyFileChanged(filePath)

	// Empty path should be a no-op
	s.NotifyFileChanged("")
}

func TestGetSubscribedURIsForPath(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, reg, bus, DefaultConfig())

	// No subscriptions initially
	uris := s.GetSubscribedURIsForPath("/test/file.go")
	if len(uris) != 0 {
		t.Errorf("expected 0 URIs, got %d", len(uris))
	}

	// Subscribe to file and index resources
	subscriber := &Subscriber{
		ID:        "test-subscriber",
		SessionID: "test-session",
	}
	filePath := "/test/file.go"
	s.subs.Subscribe(ResourceURIFilePrefix+filePath, subscriber)
	s.subs.Subscribe(ResourceURIIndex, subscriber)
	s.subs.Subscribe(ResourceURIIndexJSON, subscriber)

	// Should return file URI and index URIs
	uris = s.GetSubscribedURIsForPath(filePath)
	if len(uris) != 3 {
		t.Errorf("expected 3 URIs, got %d: %v", len(uris), uris)
	}

	// Check specific URIs are present
	hasFileURI := false
	hasIndexURI := false
	hasJSONURI := false
	for _, uri := range uris {
		switch uri {
		case ResourceURIFilePrefix + filePath:
			hasFileURI = true
		case ResourceURIIndex:
			hasIndexURI = true
		case ResourceURIIndexJSON:
			hasJSONURI = true
		}
	}

	if !hasFileURI {
		t.Error("expected file URI in result")
	}
	if !hasIndexURI {
		t.Error("expected index URI in result")
	}
	if !hasJSONURI {
		t.Error("expected JSON index URI in result")
	}
}

func TestIsIndexURI(t *testing.T) {
	tests := []struct {
		uri      string
		expected bool
	}{
		{ResourceURIIndex, true},
		{ResourceURIIndexXML, true},
		{ResourceURIIndexJSON, true},
		{ResourceURIIndexTOON, true},
		{ResourceURIFilePrefix + "/test/file.go", false},
		{"memorizer://other", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			result := isIndexURI(tt.uri)
			if result != tt.expected {
				t.Errorf("isIndexURI(%q) = %v, want %v", tt.uri, result, tt.expected)
			}
		})
	}
}

func TestIsFileURI(t *testing.T) {
	tests := []struct {
		uri      string
		expected bool
	}{
		{ResourceURIFilePrefix + "/test/file.go", true},
		{ResourceURIFilePrefix + "/path/to/file", true},
		{ResourceURIFilePrefix, true}, // Just the prefix
		{ResourceURIIndex, false},
		{ResourceURIIndexJSON, false},
		{"memorizer://other", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			result := isFileURI(tt.uri)
			if result != tt.expected {
				t.Errorf("isFileURI(%q) = %v, want %v", tt.uri, result, tt.expected)
			}
		})
	}
}

func TestNotifyIndexSubscribersNoSubscribers(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, reg, bus, DefaultConfig())

	// No subscribers - should not panic
	s.notifyIndexSubscribers()
}

func TestHandleInvalidEventPayload(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, reg, bus, DefaultConfig())

	// Create event with invalid payload type
	event := events.Event{
		Type:      events.AnalysisComplete,
		Timestamp: time.Now(),
		Payload:   "invalid payload type", // Should be events.AnalysisEvent
	}

	// Should not panic
	s.handleAnalysisCompleteEvent(event)
}

func TestHandleEmptyPathEvent(t *testing.T) {
	g := newMockGraph()
	reg := newMockRegistry()
	bus := events.NewBus()
	s := NewServer(g, reg, bus, DefaultConfig())

	// Create event with empty path
	event := events.NewEvent(events.AnalysisComplete, events.AnalysisEvent{
		Path:         "",
		ContentHash:  "abc123",
		AnalysisType: events.AnalysisFull,
	})

	// Should not panic and should exit early
	s.handleAnalysisCompleteEvent(event)
}
