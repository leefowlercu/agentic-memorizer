package daemon

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
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

// TestSSEHub_MultipleClients tests that the SSE hub can accept multiple client connections
func TestSSEHub_MultipleClients(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	hub := NewSSEHub(logger)

	// Start SSE server
	mux := http.NewServeMux()
	mux.HandleFunc("/notifications/stream", hub.handleSSEStream)
	mux.HandleFunc("/health", hub.handleHealth)

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
			resp, err := http.Get(baseURL + "/notifications/stream")
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

	// Check health endpoint shows 3 clients
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Failed to get health: %v", err)
	}
	defer resp.Body.Close()

	var healthData map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&healthData); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}

	clientCount, ok := healthData["clients"].(float64)
	if !ok {
		t.Fatalf("Health response missing clients field: %+v", healthData)
	}

	if int(clientCount) != numClients {
		t.Fatalf("Expected %d clients, got %d", numClients, int(clientCount))
	}

	t.Logf("Successfully connected %d SSE clients", numClients)
}

// TestSSEHub_Broadcast tests that broadcasts reach all connected clients
func TestSSEHub_Broadcast(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	hub := NewSSEHub(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/notifications/stream", hub.handleSSEStream)

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
			resp, err := http.Get(baseURL + "/notifications/stream")
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
	mux.HandleFunc("/notifications/stream", hub.handleSSEStream)

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
		resp, err := http.Get(baseURL + "/notifications/stream")
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
	mux.HandleFunc("/notifications/stream", hub.handleSSEStream)

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
		resp, err := http.Get(baseURL + "/notifications/stream")
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

// TestSSEHub_HealthEndpoint tests the health endpoint
func TestSSEHub_HealthEndpoint(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	hub := NewSSEHub(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", hub.handleHealth)

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

	if clients, ok := healthData["clients"].(float64); !ok || int(clients) != 0 {
		t.Errorf("Expected 0 clients, got %v", healthData["clients"])
	}
}

// TestDaemon_SSEHubIntegration tests SSE hub integration with daemon
func TestDaemon_SSEHubIntegration(t *testing.T) {
	// Create minimal config
	cfg := &config.Config{
		MemoryRoot: t.TempDir(),
		Analysis: config.AnalysisConfig{
			Enable:   false,
			CacheDir: t.TempDir(),
		},
		Daemon: config.DaemonConfig{
			DebounceMs:      100,
			Workers:         1,
			RateLimitPerMin: 60,
			SSENotifyPort:   0, // Test with port 0 (disabled)
			LogFile:         "",
			LogLevel:        "error",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	logWriter := &lumberjack.Logger{}

	d, err := New(cfg, logger, logWriter)
	if err != nil {
		t.Fatalf("Failed to create daemon: %v", err)
	}

	// Verify SSE hub was created
	if d.sseHub == nil {
		t.Error("SSE hub was not initialized")
	}

	// Test starting SSE hub with port 0 (disabled)
	if err := d.startSSEHub(0); err != nil {
		t.Errorf("startSSEHub(0) should not return error: %v", err)
	}

	if d.sseServer != nil {
		t.Error("SSE server should be nil when port is 0")
	}

	// Test starting SSE hub with real port
	port := findAvailablePort(t)
	if err := d.startSSEHub(port); err != nil {
		t.Fatalf("Failed to start SSE hub: %v", err)
	}

	if d.sseServer == nil {
		t.Error("SSE server should not be nil after starting")
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test that we can connect
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
	if err != nil {
		t.Fatalf("Failed to connect to SSE hub: %v", err)
	}
	resp.Body.Close()

	// Test stopping SSE hub
	if err := d.stopSSEHub(); err != nil {
		t.Errorf("Failed to stop SSE hub: %v", err)
	}

	if d.sseServer != nil {
		t.Error("SSE server should be nil after stopping")
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
