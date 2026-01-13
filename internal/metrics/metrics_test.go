package metrics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMetricsHandler(t *testing.T) {
	handler := Handler()
	if handler == nil {
		t.Fatal("Handler returned nil")
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "memorizer_") {
		t.Error("response should contain memorizer_ metrics")
	}
}

func TestRecordAnalysis(t *testing.T) {
	// Record successful analysis
	RecordAnalysis("semantic", 1*time.Second, nil)

	// Record failed analysis
	RecordAnalysis("semantic", 500*time.Millisecond, errors.New("test error"))

	// Verify metrics are recorded (no panic)
}

func TestRecordProviderRequest(t *testing.T) {
	// Record successful request
	RecordProviderRequest("anthropic", "analyze", 2*time.Second, 100, 50, nil)

	// Record failed request
	RecordProviderRequest("openai", "embed", 1*time.Second, 200, 0, errors.New("rate limited"))

	// Verify metrics are recorded (no panic)
}

func TestRecordCacheAccess(t *testing.T) {
	// Record cache hit
	RecordCacheAccess("semantic", true)

	// Record cache miss
	RecordCacheAccess("embeddings", false)

	// Verify metrics are recorded (no panic)
}

func TestRecordGraphOperation(t *testing.T) {
	// Record successful operation
	RecordGraphOperation("upsert_file", 10*time.Millisecond, nil)

	// Record failed operation
	RecordGraphOperation("query", 50*time.Millisecond, errors.New("connection lost"))

	// Verify metrics are recorded (no panic)
}

func TestRecordWatcherEvent(t *testing.T) {
	RecordWatcherEvent("create")
	RecordWatcherEvent("modify")
	RecordWatcherEvent("delete")

	// Verify metrics are recorded (no panic)
}

func TestRecordMCPRequest(t *testing.T) {
	RecordMCPRequest("resources/list")
	RecordMCPRequest("resources/read")

	// Verify metrics are recorded (no panic)
}

func TestUpdateQueueMetrics(t *testing.T) {
	UpdateQueueMetrics(10, 2)

	// Verify metrics are recorded (no panic)
}

func TestUpdateGraphMetrics(t *testing.T) {
	UpdateGraphMetrics(100, 5, 500)

	// Verify metrics are recorded (no panic)
}

func TestUpdateWatcherMetrics(t *testing.T) {
	UpdateWatcherMetrics(25)

	// Verify metrics are recorded (no panic)
}

func TestUpdateMCPMetrics(t *testing.T) {
	UpdateMCPMetrics(3)

	// Verify metrics are recorded (no panic)
}

// mockProvider implements MetricsProvider for testing.
type mockProvider struct {
	shouldErr bool
}

func (m *mockProvider) CollectMetrics(ctx context.Context) error {
	if m.shouldErr {
		return errors.New("collection error")
	}
	return nil
}

func TestCollector_RegisterUnregister(t *testing.T) {
	c := NewCollector(1 * time.Second)

	provider := &mockProvider{}
	c.Register("test", provider)

	c.mu.RLock()
	_, ok := c.providers["test"]
	c.mu.RUnlock()
	if !ok {
		t.Error("provider should be registered")
	}

	c.Unregister("test")

	c.mu.RLock()
	_, ok = c.providers["test"]
	c.mu.RUnlock()
	if ok {
		t.Error("provider should be unregistered")
	}
}

func TestCollector_StartStop(t *testing.T) {
	c := NewCollector(100 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provider := &mockProvider{}
	c.Register("test", provider)

	// Start
	err := c.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	c.mu.RLock()
	running := c.running
	c.mu.RUnlock()
	if !running {
		t.Error("collector should be running after Start")
	}

	// Wait for at least one collection cycle
	time.Sleep(150 * time.Millisecond)

	// Stop
	err = c.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	c.mu.RLock()
	running = c.running
	c.mu.RUnlock()
	if running {
		t.Error("collector should not be running after Stop")
	}
}

func TestCollector_CollectWithError(t *testing.T) {
	c := NewCollector(100 * time.Millisecond)

	ctx := context.Background()

	// Register a provider that errors
	failProvider := &mockProvider{shouldErr: true}
	c.Register("failing", failProvider)

	// Register a provider that succeeds
	okProvider := &mockProvider{shouldErr: false}
	c.Register("healthy", okProvider)

	// Collect should set ComponentStatus appropriately
	c.collect(ctx)

	// Verify no panic occurred
}

func TestCollector_DoubleStart(t *testing.T) {
	c := NewCollector(100 * time.Millisecond)

	ctx := context.Background()

	err := c.Start(ctx)
	if err != nil {
		t.Fatalf("first Start failed: %v", err)
	}

	// Second start should be no-op
	err = c.Start(ctx)
	if err != nil {
		t.Fatalf("second Start failed: %v", err)
	}

	err = c.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestCollector_DoubleStop(t *testing.T) {
	c := NewCollector(100 * time.Millisecond)

	ctx := context.Background()

	err := c.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	err = c.Stop(ctx)
	if err != nil {
		t.Fatalf("first Stop failed: %v", err)
	}

	// Second stop should be no-op
	err = c.Stop(ctx)
	if err != nil {
		t.Fatalf("second Stop failed: %v", err)
	}
}
