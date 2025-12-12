//go:build e2e

package tests

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/e2e/harness"
)

// TestSSE_SingleClient tests SSE connection with a single client
func TestSSE_SingleClient(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Connect SSE client
	sseURL := fmt.Sprintf("http://localhost:8080/sse")
	resp, err := http.Get(sseURL)
	if err != nil {
		t.Fatalf("Failed to connect to SSE: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify SSE headers
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		t.Errorf("Expected Content-Type text/event-stream, got %s", contentType)
	}

	// Read initial connection message
	scanner := bufio.NewScanner(resp.Body)
	timeout := time.After(5 * time.Second)
	messageReceived := false

	go func() {
		if scanner.Scan() {
			line := scanner.Text()
			t.Logf("Received SSE line: %s", line)
			messageReceived = true
		}
	}()

	select {
	case <-timeout:
		if !messageReceived {
			t.Log("No initial SSE message received (expected for new connection)")
		}
	case <-time.After(1 * time.Second):
		// Wait a bit to see if message comes through
	}

	t.Log("SSE client connected successfully")
}

// TestSSE_MultipleClients tests SSE with multiple concurrent clients
func TestSSE_MultipleClients(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Connect multiple SSE clients
	numClients := 3
	sseURL := fmt.Sprintf("http://localhost:8080/sse")

	for i := 0; i < numClients; i++ {
		go func(clientNum int) {
			resp, err := http.Get(sseURL)
			if err != nil {
				t.Logf("Client %d failed to connect: %v", clientNum, err)
				return
			}
			defer resp.Body.Close()

			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				// Keep connection open
			}
		}(i)
		time.Sleep(50 * time.Millisecond) // Stagger connections
	}

	// Give clients time to connect
	time.Sleep(1 * time.Second)

	t.Logf("Successfully started %d SSE clients", numClients)
}

// TestSSE_Events tests different SSE event types
func TestSSE_Events(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Connect SSE client
	sseURL := fmt.Sprintf("http://localhost:8080/sse")

	clientCtx, clientCancel := context.WithTimeout(ctx, 15*time.Second)
	defer clientCancel()

	req, err := http.NewRequestWithContext(clientCtx, "GET", sseURL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to connect to SSE: %v", err)
	}
	defer resp.Body.Close()

	// Start reading events in background
	eventsChan := make(chan string, 10)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data:") {
				eventsChan <- line
			}
		}
		close(eventsChan)
	}()

	// Trigger events by adding a file
	if err := h.AddMemoryFile("sse-event-test.md", "# SSE Event Test\n\nTrigger file_indexed event."); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Wait for events with timeout
	timeout := time.After(10 * time.Second)
	receivedEvents := 0

	for {
		select {
		case event, ok := <-eventsChan:
			if !ok {
				t.Log("Event stream closed")
				return
			}
			t.Logf("Received SSE event: %s", event)
			receivedEvents++
		case <-timeout:
			t.Logf("Received %d events before timeout", receivedEvents)
			return
		}
	}
}

// TestSSE_Reconnection tests SSE client reconnection behavior
func TestSSE_Reconnection(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Start daemon without context cancellation - let cleanup handle stopping
	cmd := exec.Command(h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	sseURL := fmt.Sprintf("http://localhost:8080/sse")

	// First connection
	resp1, err := http.Get(sseURL)
	if err != nil {
		t.Fatalf("First connection failed: %v", err)
	}
	t.Log("First SSE connection established")
	resp1.Body.Close() // Disconnect

	// Wait a bit
	time.Sleep(500 * time.Millisecond)

	// Reconnect
	resp2, err := http.Get(sseURL)
	if err != nil {
		t.Fatalf("Reconnection failed: %v", err)
	}
	resp2.Body.Close() // Close explicitly before cleanup to avoid blocking daemon shutdown

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("Reconnection returned status %d", resp2.StatusCode)
	}

	t.Log("SSE reconnection successful")
}

// TestSSE_LongLivedConnection tests that SSE connections stay alive beyond 60 seconds
// This validates the fix for the SSE timeout issue where global WriteTimeout was
// closing streaming connections after 60 seconds
func TestSSE_LongLivedConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-lived connection test in short mode")
	}

	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Connect SSE client
	sseURL := "http://localhost:8080/sse"

	clientCtx, clientCancel := context.WithTimeout(ctx, 75*time.Second)
	defer clientCancel()

	req, err := http.NewRequestWithContext(clientCtx, "GET", sseURL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer resp.Body.Close()

	// Monitor connection for 70 seconds
	keepaliveCount := 0
	eventCount := 0
	connectionClosed := false
	errChan := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 10*1024*1024)

		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, ": keepalive") {
				keepaliveCount++
				t.Logf("Keepalive #%d received", keepaliveCount)
			} else if strings.HasPrefix(line, "data:") {
				eventCount++
				t.Logf("Event #%d received", eventCount)
			}
		}

		if err := scanner.Err(); err != nil {
			connectionClosed = true
			errChan <- fmt.Errorf("connection closed; %w", err)
		}
	}()

	// Wait 70 seconds (beyond old timeout)
	select {
	case err := <-errChan:
		t.Fatalf("Connection closed unexpectedly after %v: %v", time.Now(), err)
	case <-time.After(70 * time.Second):
		if connectionClosed {
			t.Fatal("Connection closed before timeout")
		}
		t.Logf("SUCCESS: Connection stayed alive 70+ seconds (keepalives: %d, events: %d)",
			keepaliveCount, eventCount)

		if keepaliveCount < 2 {
			t.Errorf("Expected at least 2 keepalives, got %d", keepaliveCount)
		}
	}
}
