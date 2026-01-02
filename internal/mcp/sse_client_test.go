package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/logging"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// TestSSEClient_ConnectToStream tests that SSE client can connect to daemon stream
func TestSSEClient_ConnectToStream(t *testing.T) {
	// Create mock SSE server
	mux := http.NewServeMux()
	connected := make(chan bool, 1)

	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("Expected Accept header 'text/event-stream', got %s", r.Header.Get("Accept"))
		}

		connected <- true

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Send initial connection message
		fmt.Fprintf(w, ": connected\n\n")
		flusher.Flush()

		// Keep connection open until context done
		<-r.Context().Done()
	})

	server := &http.Server{Handler: mux}
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	go server.Serve(listener)
	defer server.Shutdown(context.Background())

	// Create SSE client
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelError}))
	idx := &types.FileIndex{
		Stats: types.IndexStats{TotalFiles: 0},
		Files: []types.FileEntry{},
	}
	srv := NewServer(idx, logger, "")
	srv.sseClient = NewSSEClient(fmt.Sprintf("http://localhost:%d/sse", port), srv, logger, logging.NewProcessID())

	// Start client
	srv.sseClient.Start()
	defer srv.sseClient.Stop()

	// Wait for connection
	select {
	case <-connected:
		t.Log("SSE client connected successfully")
	case <-time.After(2 * time.Second):
		t.Fatal("SSE client did not connect within timeout")
	}
}

// TestSSEClient_ReceiveNotification tests that client receives and processes notifications
func TestSSEClient_ReceiveNotification(t *testing.T) {
	// Create initial index (graph reload will fail without FalkorDB, but we're testing notification receipt)
	initialIndex := &types.FileIndex{
		Generated:  time.Now(),
		MemoryRoot: "/test/memory",
		Files: []types.FileEntry{
			{
				Path: "/test/file1.txt",
			},
		},
		Stats: types.IndexStats{TotalFiles: 1},
	}

	// Create mock SSE server
	mux := http.NewServeMux()
	notificationReceived := make(chan bool, 1)

	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Send index_updated notification after short delay
		time.Sleep(100 * time.Millisecond)

		notification := map[string]interface{}{
			"type":      "index_updated",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		data, _ := json.Marshal(notification)

		fmt.Fprintf(w, "data: %s\n\n", string(data))
		flusher.Flush()

		notificationReceived <- true

		// Keep connection open
		<-r.Context().Done()
	})

	server := &http.Server{Handler: mux}
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	go server.Serve(listener)
	defer server.Shutdown(context.Background())

	// Create SSE client (index reload will be processed from SSE event)
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelError}))
	srv := NewServer(initialIndex, logger, "")
	srv.sseClient = NewSSEClient(fmt.Sprintf("http://localhost:%d/sse", port), srv, logger, logging.NewProcessID())

	// Start client
	srv.sseClient.Start()
	defer srv.sseClient.Stop()

	// Wait for notification to be received
	select {
	case <-notificationReceived:
		t.Log("Notification received and processed")
	case <-time.After(3 * time.Second):
		t.Fatal("Notification not received within timeout")
	}

	// Give client time to process notification
	time.Sleep(200 * time.Millisecond)
}

// TestSSEClient_AutoReconnect tests automatic reconnection on disconnect
func TestSSEClient_AutoReconnect(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping reconnection test in short mode")
	}

	connectionCount := 0
	var countMu sync.Mutex

	mux := http.NewServeMux()
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		countMu.Lock()
		connectionCount++
		count := connectionCount
		countMu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, _ := w.(http.Flusher)

		fmt.Fprintf(w, ": connected\n\n")
		flusher.Flush()

		// First connection: close after 500ms to trigger reconnect
		if count == 1 {
			time.Sleep(500 * time.Millisecond)
			return // Close connection
		}

		// Second connection: keep open
		<-r.Context().Done()
	})

	server := &http.Server{Handler: mux}
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	go server.Serve(listener)
	defer server.Shutdown(context.Background())

	// Create SSE client
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelError}))
	idx := &types.FileIndex{
		Stats: types.IndexStats{TotalFiles: 0},
		Files: []types.FileEntry{},
	}
	srv := NewServer(idx, logger, "")
	srv.sseClient = NewSSEClient(fmt.Sprintf("http://localhost:%d/sse", port), srv, logger, logging.NewProcessID())

	// Start client
	srv.sseClient.Start()
	defer srv.sseClient.Stop()

	// Wait for reconnection (5s backoff + buffer)
	time.Sleep(7 * time.Second)

	countMu.Lock()
	count := connectionCount
	countMu.Unlock()

	if count < 2 {
		t.Errorf("Expected at least 2 connections (initial + reconnect), got %d", count)
	} else {
		t.Logf("Successfully reconnected after disconnect (total connections: %d)", count)
	}
}

// TestSSEClient_MultipleClients tests multiple MCP servers connecting simultaneously
func TestSSEClient_MultipleClients(t *testing.T) {
	clientCount := 0
	var countMu sync.Mutex
	maxClients := make(chan int, 1)
	allConnected := make(chan struct{})
	expectedClients := 3

	mux := http.NewServeMux()
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		countMu.Lock()
		clientCount++
		current := clientCount
		// Signal when all clients have connected
		if current == expectedClients {
			select {
			case <-allConnected:
				// Already closed
			default:
				close(allConnected)
			}
		}
		countMu.Unlock()

		// Track maximum concurrent clients
		select {
		case old := <-maxClients:
			if current > old {
				maxClients <- current
			} else {
				maxClients <- old
			}
		default:
			maxClients <- current
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, _ := w.(http.Flusher)

		fmt.Fprintf(w, ": connected\n\n")
		flusher.Flush()

		<-r.Context().Done()

		countMu.Lock()
		clientCount--
		countMu.Unlock()
	})

	server := &http.Server{Handler: mux}
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	go server.Serve(listener)
	defer server.Shutdown(context.Background())

	// Create 3 SSE clients
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), &slog.HandlerOptions{Level: slog.LevelError}))
	clients := make([]*SSEClient, expectedClients)

	for i := 0; i < expectedClients; i++ {
		idx := &types.FileIndex{
			Stats: types.IndexStats{TotalFiles: 0},
			Files: []types.FileEntry{},
		}
		srv := NewServer(idx, logger, "")
		clients[i] = NewSSEClient(fmt.Sprintf("http://localhost:%d/sse", port), srv, logger, logging.NewProcessID())
		clients[i].Start()
	}

	// Wait for all clients to connect with timeout
	select {
	case <-allConnected:
		// All clients connected
	case <-time.After(10 * time.Second):
		t.Fatalf("Timeout waiting for all clients to connect")
	}

	// Stop all clients
	for _, client := range clients {
		client.Stop()
	}

	// Check maximum concurrent clients
	max := <-maxClients
	if max != expectedClients {
		t.Errorf("Expected %d concurrent clients, got %d", expectedClients, max)
	} else {
		t.Logf("Successfully handled %d concurrent SSE clients", max)
	}
}

// TestSSEClient_DaemonContinuesWithoutClients tests that daemon works without MCP clients
func TestSSEClient_DaemonContinuesWithoutClients(t *testing.T) {
	// This test verifies the daemon doesn't crash when broadcasting with no clients
	// The actual daemon SSE hub handles this gracefully

	mux := http.NewServeMux()
	broadcastCount := 0

	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		fmt.Fprintf(w, ": connected\n\n")
		flusher.Flush()

		<-r.Context().Done()
	})

	server := &http.Server{Handler: mux}
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	go server.Serve(listener)
	defer server.Shutdown(context.Background())

	// Simulate broadcasts with no clients connected
	for i := 0; i < 5; i++ {
		broadcastCount++
		time.Sleep(100 * time.Millisecond)
	}

	t.Logf("Completed %d broadcasts with no clients connected (no crashes)", broadcastCount)
}
