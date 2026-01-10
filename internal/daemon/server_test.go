package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// T020: Tests for HTTP server /healthz endpoint

func TestServer_Healthz(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0, // Use any available port for testing
		Bind: "127.0.0.1",
	})

	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /healthz status = %d, want %d", w.Code, http.StatusOK)
	}

	var response struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != "alive" {
		t.Errorf("GET /healthz status = %q, want %q", response.Status, "alive")
	}
}

// T021: Tests for HTTP server /readyz endpoint

func TestServer_Readyz_Healthy(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /readyz status = %d, want %d", w.Code, http.StatusOK)
	}

	var response HealthStatus
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != "healthy" {
		t.Errorf("GET /readyz Status = %q, want %q", response.Status, "healthy")
	}

	if !response.Ready {
		t.Error("GET /readyz Ready = false, want true")
	}
}

func TestServer_Readyz_Degraded(t *testing.T) {
	hm := NewHealthManager()

	// Add a failed component
	hm.UpdateComponent("failed-component", ComponentHealth{
		Status:      ComponentStatusFailed,
		Error:       "test failure",
		LastChecked: time.Now(),
	})

	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Degraded should still return 200 OK (ready but degraded)
	if w.Code != http.StatusOK {
		t.Errorf("GET /readyz status = %d, want %d", w.Code, http.StatusOK)
	}

	var response HealthStatus
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != "degraded" {
		t.Errorf("GET /readyz Status = %q, want %q", response.Status, "degraded")
	}

	if !response.Ready {
		t.Error("GET /readyz Ready = false, want true for degraded state")
	}

	if len(response.Components) != 1 {
		t.Errorf("GET /readyz Components has %d entries, want 1", len(response.Components))
	}
}

func TestServer_Readyz_WithComponents(t *testing.T) {
	hm := NewHealthManager()

	// Add a healthy component
	hm.UpdateComponent("test-component", ComponentHealth{
		Status:      ComponentStatusRunning,
		LastChecked: time.Now(),
	})

	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var response HealthStatus
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if _, exists := response.Components["test-component"]; !exists {
		t.Error("GET /readyz missing component 'test-component'")
	}
}
