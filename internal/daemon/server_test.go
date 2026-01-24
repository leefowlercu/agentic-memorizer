package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/export"
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

// Edge case tests for Server

func TestServer_Rebuild_NoHandler(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	// Don't set rebuild func

	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodPost, "/rebuild", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("POST /rebuild without handler status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	var response RebuildResult
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != "error" {
		t.Errorf("Response status = %q, want %q", response.Status, "error")
	}

	if response.Error != "rebuild not available" {
		t.Errorf("Response error = %q, want %q", response.Error, "rebuild not available")
	}
}

func TestServer_Remember_NoHandler(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	req := httptest.NewRequest(http.MethodPost, "/remember", bytes.NewBufferString(`{"path":"/tmp/test"}`))
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("POST /remember without handler status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	var response errorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Error != "remember not available" {
		t.Errorf("response error = %q, want %q", response.Error, "remember not available")
	}
}

func TestServer_Remember_Success(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	srv.SetRememberFunc(func(ctx context.Context, req RememberRequest) (*RememberResponse, error) {
		if req.Path == "" {
			t.Error("expected path in request")
		}
		return &RememberResponse{Status: RememberStatusAdded, Path: req.Path}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/remember", bytes.NewBufferString(`{"path":"/tmp/test"}`))
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /remember status = %d, want %d", w.Code, http.StatusOK)
	}

	var response RememberResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Status != RememberStatusAdded {
		t.Errorf("response status = %q, want %q", response.Status, RememberStatusAdded)
	}
	if response.Path != "/tmp/test" {
		t.Errorf("response path = %q, want %q", response.Path, "/tmp/test")
	}
}

func TestServer_List_NoHandler(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("GET /list without handler status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	var response errorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Error != "list not available" {
		t.Errorf("response error = %q, want %q", response.Error, "list not available")
	}
}

func TestServer_List_Success(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	count := 3
	now := time.Now().UTC()
	srv.SetListFunc(func(ctx context.Context) (*ListResponse, error) {
		return &ListResponse{
			Paths: []ListEntry{
				{
					Path:       "/projects/app",
					Status:     "ok",
					FileCount:  &count,
					LastWalkAt: &now,
					CreatedAt:  now,
					UpdatedAt:  now,
				},
			},
		}, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /list status = %d, want %d", w.Code, http.StatusOK)
	}

	var response ListResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(response.Paths) != 1 {
		t.Fatalf("response paths = %d, want 1", len(response.Paths))
	}
	if response.Paths[0].Path != "/projects/app" {
		t.Errorf("response path = %q, want %q", response.Paths[0].Path, "/projects/app")
	}
	if response.Paths[0].Status != "ok" {
		t.Errorf("response status = %q, want %q", response.Paths[0].Status, "ok")
	}
	if response.Paths[0].FileCount == nil || *response.Paths[0].FileCount != count {
		t.Errorf("response file count = %v, want %d", response.Paths[0].FileCount, count)
	}
}

func TestServer_Read_NoHandler(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	req := httptest.NewRequest(http.MethodPost, "/read", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("POST /read without handler status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	var response errorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Error != "read not available" {
		t.Errorf("response error = %q, want %q", response.Error, "read not available")
	}
}

func TestServer_Read_InvalidBody(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	srv.SetReadFunc(func(ctx context.Context, req ReadRequest) (*ReadResponse, error) {
		return &ReadResponse{}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/read", bytes.NewBufferString("{bad"))
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /read invalid body status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var response errorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Error != "invalid request body" {
		t.Errorf("response error = %q, want %q", response.Error, "invalid request body")
	}
}

func TestServer_Read_Unavailable(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	srv.SetReadFunc(func(ctx context.Context, req ReadRequest) (*ReadResponse, error) {
		return nil, ErrReadUnavailable
	})

	req := httptest.NewRequest(http.MethodPost, "/read", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("POST /read unavailable status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	var response errorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Error != ErrReadUnavailable.Error() {
		t.Errorf("response error = %q, want %q", response.Error, ErrReadUnavailable.Error())
	}
}

func TestServer_Read_Success(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	srv.SetReadFunc(func(ctx context.Context, req ReadRequest) (*ReadResponse, error) {
		return &ReadResponse{
			Output: "<graph/>",
			Stats: &export.ExportStats{
				FileCount:      1,
				DirectoryCount: 2,
				OutputSize:     8,
				Duration:       250 * time.Millisecond,
			},
		}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/read", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /read status = %d, want %d", w.Code, http.StatusOK)
	}

	var response ReadResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Output != "<graph/>" {
		t.Errorf("response output = %q, want %q", response.Output, "<graph/>")
	}
	if response.Stats == nil || response.Stats.FileCount != 1 || response.Stats.DirectoryCount != 2 {
		t.Errorf("response stats = %+v, want file_count=1 directory_count=2", response.Stats)
	}
}

func TestServer_Forget_NoHandler(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	req := httptest.NewRequest(http.MethodPost, "/forget", bytes.NewBufferString(`{"path":"/tmp/test","keep_data":true}`))
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("POST /forget without handler status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	var response errorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Error != "forget not available" {
		t.Errorf("response error = %q, want %q", response.Error, "forget not available")
	}
}

func TestServer_Forget_Success(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	srv.SetForgetFunc(func(ctx context.Context, req ForgetRequest) (*ForgetResponse, error) {
		if req.Path == "" {
			t.Error("expected path in request")
		}
		return &ForgetResponse{Status: ForgetStatusForgotten, Path: req.Path, KeepData: req.KeepData}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/forget", bytes.NewBufferString(`{"path":"/tmp/test","keep_data":false}`))
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /forget status = %d, want %d", w.Code, http.StatusOK)
	}

	var response ForgetResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Status != ForgetStatusForgotten {
		t.Errorf("response status = %q, want %q", response.Status, ForgetStatusForgotten)
	}
	if response.Path != "/tmp/test" {
		t.Errorf("response path = %q, want %q", response.Path, "/tmp/test")
	}
	if response.KeepData {
		t.Error("expected keep_data to be false")
	}
}

func TestServer_Rebuild_Success(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	// Set rebuild func that succeeds
	srv.SetRebuildFunc(func(ctx context.Context, full bool) (*RebuildResult, error) {
		return &RebuildResult{
			Status:        "completed",
			FilesQueued:   42,
			DirsProcessed: 5,
			Duration:      "1.5s",
		}, nil
	})

	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodPost, "/rebuild", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /rebuild status = %d, want %d", w.Code, http.StatusOK)
	}

	var response RebuildResult
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != "completed" {
		t.Errorf("Response status = %q, want %q", response.Status, "completed")
	}

	if response.FilesQueued != 42 {
		t.Errorf("Response files_queued = %d, want %d", response.FilesQueued, 42)
	}

	if response.DirsProcessed != 5 {
		t.Errorf("Response dirs_processed = %d, want %d", response.DirsProcessed, 5)
	}
}

func TestServer_Rebuild_Failure(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	// Set rebuild func that fails
	srv.SetRebuildFunc(func(ctx context.Context, full bool) (*RebuildResult, error) {
		return nil, errors.New("rebuild failed: database error")
	})

	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodPost, "/rebuild", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("POST /rebuild with error status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	var response RebuildResult
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != "error" {
		t.Errorf("Response status = %q, want %q", response.Status, "error")
	}

	if response.Error != "rebuild failed: database error" {
		t.Errorf("Response error = %q, want %q", response.Error, "rebuild failed: database error")
	}
}

func TestServer_Rebuild_FullFlag(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	var receivedFull bool
	srv.SetRebuildFunc(func(ctx context.Context, full bool) (*RebuildResult, error) {
		receivedFull = full
		return &RebuildResult{Status: "completed"}, nil
	})

	handler := srv.Handler()

	// Test without full flag
	req := httptest.NewRequest(http.MethodPost, "/rebuild", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if receivedFull {
		t.Error("Expected full=false without query param")
	}

	// Test with full=true
	req = httptest.NewRequest(http.MethodPost, "/rebuild?full=true", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !receivedFull {
		t.Error("Expected full=true with ?full=true query param")
	}

	// Test with full=false (explicit)
	receivedFull = true // reset
	req = httptest.NewRequest(http.MethodPost, "/rebuild?full=false", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if receivedFull {
		t.Error("Expected full=false with ?full=false query param")
	}
}

func TestServer_Rebuild_WrongMethod(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	srv.SetRebuildFunc(func(ctx context.Context, full bool) (*RebuildResult, error) {
		return &RebuildResult{Status: "completed"}, nil
	})

	handler := srv.Handler()

	// GET should not work for /rebuild
	req := httptest.NewRequest(http.MethodGet, "/rebuild", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /rebuild status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestServer_MetricsHandler(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	// Create a simple metrics handler
	metricsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# HELP test_metric A test metric\n"))
	})

	srv.SetMetricsHandler(metricsHandler)

	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /metrics status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if body != "# HELP test_metric A test metric\n" {
		t.Errorf("GET /metrics body = %q, unexpected content", body)
	}
}

func TestServer_ContentType(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	handler := srv.Handler()

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/healthz"},
		{http.MethodGet, "/readyz"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("%s %s Content-Type = %q, want %q", tt.method, tt.path, contentType, "application/json")
			}
		})
	}
}

// Additional edge case tests

func TestServer_404_UnknownPath(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/unknown/path", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("GET /unknown/path status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestServer_SetMCPHandler(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	// Create a simple MCP handler
	mcpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"mcp": "response"}`))
	})

	srv.SetMCPHandler(mcpHandler)

	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/mcp/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /mcp/test status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestServer_SetMCPHandler_Nil(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	// Setting nil handler should not panic
	srv.SetMCPHandler(nil)

	handler := srv.Handler()

	// Should still be able to access other endpoints
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /healthz status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestServer_SetMetricsHandler_Nil(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	// Setting nil handler should not panic
	srv.SetMetricsHandler(nil)

	handler := srv.Handler()

	// Should still be able to access other endpoints
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /healthz status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestServer_Rebuild_ContentType(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	srv.SetRebuildFunc(func(ctx context.Context, full bool) (*RebuildResult, error) {
		return &RebuildResult{Status: "completed"}, nil
	})

	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodPost, "/rebuild", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("POST /rebuild Content-Type = %q, want %q", contentType, "application/json")
	}
}

func TestServer_ConcurrentRequests(t *testing.T) {
	hm := NewHealthManager()

	// Add some components
	for i := range 5 {
		name := "component-" + string(rune('a'+i))
		hm.UpdateComponent(name, ComponentHealth{
			Status:      ComponentStatusRunning,
			LastChecked: time.Now(),
		})
	}

	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	handler := srv.Handler()

	// Spawn concurrent requests
	var wg sync.WaitGroup
	numRequests := 100

	for range numRequests {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("GET /readyz status = %d, want %d", w.Code, http.StatusOK)
			}
		}()
	}

	wg.Wait()
}

// TestServer_MCP_EndpointAccessible tests that the MCP endpoint is accessible
// when an MCP handler is configured.
func TestServer_MCP_EndpointAccessible(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	// Create an MCP handler that responds with proper MCP-style response
	mcpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25"}}`))
	})

	srv.SetMCPHandler(mcpHandler)

	handler := srv.Handler()

	// Test various MCP paths are routed correctly
	paths := []string{
		"/mcp",
		"/mcp/",
		"/mcp/sse",
		"/mcp/message",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			// MCP handler should receive the request
			if w.Code != http.StatusOK {
				t.Errorf("POST %s status = %d, want %d", path, w.Code, http.StatusOK)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("POST %s Content-Type = %q, want %q", path, contentType, "application/json")
			}
		})
	}
}

// TestServer_MCP_NotFound tests that /mcp returns 404 when no handler is set.
func TestServer_MCP_NotFound_NoHandler(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	// Don't set MCP handler

	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("POST /mcp without handler status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// TestServer_MCP_MethodHandling tests that the MCP handler receives all HTTP methods.
func TestServer_MCP_MethodHandling(t *testing.T) {
	hm := NewHealthManager()
	srv := NewServer(hm, ServerConfig{
		Port: 0,
		Bind: "127.0.0.1",
	})

	var receivedMethod string
	mcpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.WriteHeader(http.StatusOK)
	})

	srv.SetMCPHandler(mcpHandler)

	handler := srv.Handler()

	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodDelete,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			receivedMethod = ""
			req := httptest.NewRequest(method, "/mcp", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if receivedMethod != method {
				t.Errorf("MCP handler received method %q, want %q", receivedMethod, method)
			}
		})
	}
}
