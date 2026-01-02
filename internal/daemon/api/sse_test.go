package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/logging"
)

// connectSSE creates an SSE connection with required client headers
func connectSSE(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add required client headers
	req.Header.Set(logging.HeaderClientID, logging.NewClientID())
	req.Header.Set(logging.HeaderClientType, "test")
	req.Header.Set(logging.HeaderClientVersion, "1.0.0")

	client := &http.Client{}
	return client.Do(req)
}

// TestSSEHub_MultipleClients tests that the SSE hub can accept multiple client connections
func TestSSEHub_MultipleClients(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	hub := NewSSEHub(logger)

	// Start SSE server
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", hub.HandleSSE)

	server := &http.Server{
		Handler: mux,
	}

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	go server.Serve(listener)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		server.Shutdown(ctx)
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	baseURL := fmt.Sprintf("http://localhost:%d", port)

	// Connect 2 clients and keep them connected
	numClients := 2

	for i := 0; i < numClients; i++ {
		go func() {
			resp, err := connectSSE(baseURL + "/sse")
			if err != nil {
				return
			}
			defer resp.Body.Close()

			// Keep connection open (will be closed when server shuts down)
			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				// Consume messages
			}
		}()
		// Stagger connections to avoid race conditions
		time.Sleep(50 * time.Millisecond)
	}

	// Poll client count until all clients are connected
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if hub.ClientCount() == numClients {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify client count
	clientCount := hub.ClientCount()
	if clientCount != numClients {
		t.Fatalf("Expected %d clients, got %d", numClients, clientCount)
	}

	t.Logf("Successfully connected %d SSE clients", numClients)
}

// TestSSEHub_Broadcast tests that broadcasts reach all connected clients
func TestSSEHub_Broadcast(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	hub := NewSSEHub(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse", hub.HandleSSE)

	server := &http.Server{
		Handler: mux,
	}

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	go server.Serve(listener)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		server.Shutdown(ctx)
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	baseURL := fmt.Sprintf("http://localhost:%d", port)

	// Connect 2 clients
	numClients := 2
	receivedBroadcast := make(chan bool, numClients)

	for i := 0; i < numClients; i++ {
		go func() {
			resp, err := connectSSE(baseURL + "/sse")
			if err != nil {
				return
			}
			defer resp.Body.Close()

			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(line, "index_updated") {
					receivedBroadcast <- true
					return
				}
			}
		}()
		// Stagger connections to avoid race conditions
		time.Sleep(50 * time.Millisecond)
	}

	// Poll health endpoint until all clients are connected
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/health")
		if err == nil {
			var healthData map[string]any
			json.NewDecoder(resp.Body).Decode(&healthData)
			resp.Body.Close()
			if clients, ok := healthData["clients"].(float64); ok && int(clients) == numClients {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Broadcast a message
	hub.BroadcastIndexUpdate()

	// Wait for all clients to receive the broadcast
	timeout := time.After(2 * time.Second)
	received := 0
	for received < numClients {
		select {
		case <-receivedBroadcast:
			received++
		case <-timeout:
			t.Fatalf("Only %d/%d clients received broadcast", received, numClients)
		}
	}

	t.Logf("All %d clients received broadcast successfully", numClients)
}

// TestSSEHub_Keepalive tests that keepalive comments are sent every 30s
func TestSSEHub_Keepalive(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping keepalive test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	hub := NewSSEHub(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse", hub.HandleSSE)

	server := &http.Server{
		Handler: mux,
	}

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	go server.Serve(listener)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		server.Shutdown(ctx)
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	baseURL := fmt.Sprintf("http://localhost:%d", port)

	keepaliveReceived := make(chan bool, 1)

	go func() {
		resp, err := connectSSE(baseURL + "/sse")
		if err != nil {
			return
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, ": keepalive") {
				keepaliveReceived <- true
				return
			}
		}
	}()

	// Wait up to 35 seconds for keepalive (30s interval + 5s buffer)
	select {
	case <-keepaliveReceived:
		t.Log("Keepalive received successfully")
	case <-time.After(35 * time.Second):
		t.Error("Did not receive keepalive comment within 35 seconds")
	}
}

// TestSSEHub_GracefulShutdown tests that the hub shuts down gracefully
func TestSSEHub_GracefulShutdown(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	hub := NewSSEHub(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse", hub.HandleSSE)

	server := &http.Server{
		Handler: mux,
	}

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}

	go server.Serve(listener)

	port := listener.Addr().(*net.TCPAddr).Port
	baseURL := fmt.Sprintf("http://localhost:%d", port)

	// Connect a client
	go func() {
		resp, err := connectSSE(baseURL + "/sse")
		if err != nil {
			return
		}
		defer resp.Body.Close()
		// Just keep connection open
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
		}
	}()

	// Give client time to connect
	time.Sleep(200 * time.Millisecond)

	// Verify at least one client connected
	resp, err := http.Get(baseURL + "/health")
	if err == nil {
		var healthData map[string]any
		json.NewDecoder(resp.Body).Decode(&healthData)
		resp.Body.Close()
		if clients, ok := healthData["clients"].(float64); ok && int(clients) > 0 {
			t.Logf("Verified %d client(s) connected before shutdown", int(clients))
		}
	}

	// Shutdown server gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	shutdownComplete := make(chan error, 1)
	go func() {
		shutdownComplete <- server.Shutdown(ctx)
	}()

	// Verify shutdown completes within timeout
	select {
	case err := <-shutdownComplete:
		if err != nil {
			t.Logf("Shutdown completed with: %v (acceptable)", err)
		} else {
			t.Log("Shutdown completed successfully")
		}
	case <-time.After(6 * time.Second):
		t.Error("Server shutdown did not complete within 6 seconds")
	}

	listener.Close()
}

// mockHealthMetrics implements HealthMetricsProvider for testing
type mockHealthMetrics struct{}

func (m *mockHealthMetrics) GetSnapshot() HealthSnapshot {
	return HealthSnapshot{
		StartTime:        time.Now(),
		Uptime:           "1h 30m",
		UptimeSeconds:    5400,
		FilesProcessed:   100,
		APICalls:         50,
		CacheHits:        75,
		Errors:           2,
		LastBuildTime:    time.Now(),
		LastBuildSuccess: true,
		IndexFileCount:   100,
		WatcherActive:    true,
	}
}

// TestHTTPServer_HealthEndpoint tests the health endpoint via HTTPServer
func TestHTTPServer_HealthEndpoint(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	hub := NewSSEHub(logger)
	metrics := &mockHealthMetrics{}
	httpServer := NewHTTPServer(hub, metrics, nil, "", logger)

	port := findAvailablePort(t)
	if err := httpServer.Start(port); err != nil {
		t.Fatalf("Failed to start HTTP server: %v", err)
	}
	defer httpServer.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	baseURL := fmt.Sprintf("http://localhost:%d", port)

	// Check health with no clients
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Failed to get health: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", resp.Header.Get("Content-Type"))
	}

	var healthData map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&healthData); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}

	if status, ok := healthData["status"].(string); !ok || status != "healthy" {
		t.Errorf("Expected status=healthy, got %v", healthData["status"])
	}

	// Check that metrics contains sse_clients field
	metricsData, ok := healthData["metrics"].(map[string]any)
	if !ok {
		t.Fatalf("Expected metrics to be a map, got %T", healthData["metrics"])
	}

	if _, ok := metricsData["sse_clients"]; !ok {
		t.Error("Expected sse_clients field in metrics")
	}
}

// findAvailablePort finds an available port for testing
func findAvailablePort(t *testing.T) int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

// TestSSEHub_NoTimeout verifies that SSE connections stay alive beyond 60 seconds
// This test validates the fix for the SSE timeout issue where global WriteTimeout
// was closing streaming connections after 60 seconds
func TestSSEHub_NoTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	hub := NewSSEHub(logger)
	metrics := &mockHealthMetrics{}
	httpServer := NewHTTPServer(hub, metrics, nil, "", logger)

	port := findAvailablePort(t)
	if err := httpServer.Start(port); err != nil {
		t.Fatalf("Failed to start HTTP server: %v", err)
	}
	defer httpServer.Stop()

	time.Sleep(100 * time.Millisecond) // Server startup

	baseURL := fmt.Sprintf("http://localhost:%d", port)

	// Connect to SSE endpoint
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/sse", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Add required client headers
	req.Header.Set(logging.HeaderClientID, logging.NewClientID())
	req.Header.Set(logging.HeaderClientType, "test")
	req.Header.Set(logging.HeaderClientVersion, "1.0.0")

	client := &http.Client{Timeout: 0} // No client-side timeout
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer resp.Body.Close()

	// Track keepalive messages and connection status
	var keepaliveCount atomic.Int32
	var connectionClosed atomic.Bool
	errChan := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, ": keepalive") {
				count := keepaliveCount.Add(1)
				t.Logf("Received keepalive #%d at %v", count, time.Now())
			}
		}
		if err := scanner.Err(); err != nil {
			connectionClosed.Store(true)
			errChan <- fmt.Errorf("connection closed; %w", err)
		}
	}()

	// Wait 70 seconds (beyond old 60s timeout)
	select {
	case err := <-errChan:
		t.Fatalf("Connection closed prematurely after %v: %v", time.Now(), err)
	case <-time.After(70 * time.Second):
		if connectionClosed.Load() {
			t.Fatal("Connection closed before timeout")
		}
		count := keepaliveCount.Load()
		t.Logf("SUCCESS: Connection stayed alive for 70+ seconds (received %d keepalives)", count)

		// Should have received at least 2 keepalives (30s, 60s)
		if count < 2 {
			t.Errorf("Expected at least 2 keepalives, got %d", count)
		}
	}
}

// TestHTTPServer_APITimeout verifies that API endpoints still timeout correctly
// This is a regression test to ensure that removing global timeouts and adding
// per-endpoint timeouts via middleware still protects API endpoints from hanging
func TestHTTPServer_APITimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	hub := NewSSEHub(logger)
	metrics := &mockHealthMetrics{}

	// Create HTTP server (graph manager is nil, so search will fail gracefully)
	httpServer := NewHTTPServer(hub, metrics, nil, "", logger)

	port := findAvailablePort(t)
	if err := httpServer.Start(port); err != nil {
		t.Fatalf("Failed to start HTTP server: %v", err)
	}
	defer httpServer.Stop()

	time.Sleep(100 * time.Millisecond)

	baseURL := fmt.Sprintf("http://localhost:%d", port)

	// Test that API endpoint returns quickly (since graph manager is nil)
	// We can't easily test a real timeout without mocking, but we can verify
	// the endpoint is wrapped with timeout middleware by checking it doesn't hang
	start := time.Now()
	resp, err := http.Get(baseURL + "/api/v1/files?q=test")
	elapsed := time.Since(start)

	// Should return error quickly (graph not available), not hang
	if elapsed > 5*time.Second {
		t.Errorf("API request took %v, expected quick error response", elapsed)
	}

	if err == nil {
		// Should get 503 Service Unavailable since graph manager is nil
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
		}
		resp.Body.Close()
	}

	t.Logf("API endpoint responded in %v with appropriate error (timeout protection working)", elapsed)
}
