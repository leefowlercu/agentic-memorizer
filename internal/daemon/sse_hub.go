package daemon

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// SSEClient represents a connected SSE client
type SSEClient struct {
	id       string
	messages chan string
	done     chan struct{}
}

// SSEHub manages Server-Sent Events broadcasting for index updates
type SSEHub struct {
	clients   map[string]*SSEClient
	clientsMu sync.RWMutex
	logger    *slog.Logger
}

// NewSSEHub creates a new SSE broadcast hub
func NewSSEHub(logger *slog.Logger) *SSEHub {
	return &SSEHub{
		clients: make(map[string]*SSEClient),
		logger:  logger,
	}
}

// register adds a new SSE client to the hub
func (h *SSEHub) register(client *SSEClient) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()
	h.clients[client.id] = client
	h.logger.Info("SSE client connected", "client_id", client.id, "total_clients", len(h.clients))
}

// unregister removes an SSE client from the hub
func (h *SSEHub) unregister(clientID string) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()

	if client, ok := h.clients[clientID]; ok {
		close(client.done)
		close(client.messages)
		delete(h.clients, clientID)
		h.logger.Info("SSE client disconnected", "client_id", clientID, "total_clients", len(h.clients))
	}
}

// ClientCount returns the number of connected SSE clients
func (h *SSEHub) ClientCount() int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	return len(h.clients)
}

// BroadcastIndexUpdate sends index update notification to all connected clients
func (h *SSEHub) BroadcastIndexUpdate() {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	if len(h.clients) == 0 {
		return
	}

	// Create notification payload
	notification := map[string]any{
		"type":      "index_updated",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(notification)
	if err != nil {
		h.logger.Error("failed to marshal index update notification", "error", err)
		return
	}

	message := fmt.Sprintf("data: %s\n\n", string(data))

	h.logger.Debug("broadcasting index update", "clients", len(h.clients))

	// Broadcast to all clients (non-blocking)
	for _, client := range h.clients {
		select {
		case client.messages <- message:
		case <-client.done:
			// Client disconnected, skip
		default:
			// Channel full, log warning but don't block
			h.logger.Warn("SSE client message queue full", "client_id", client.id)
		}
	}
}

// handleSSEStream handles SSE stream requests from MCP servers
func (h *SSEHub) handleSSEStream(w http.ResponseWriter, r *http.Request) {
	// Verify SSE headers
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create new client
	client := &SSEClient{
		id:       fmt.Sprintf("client-%d", time.Now().UnixNano()),
		messages: make(chan string, 10), // Buffer 10 messages
		done:     make(chan struct{}),
	}

	// Register client
	h.register(client)
	defer h.unregister(client.id)

	// Send initial connection message
	fmt.Fprintf(w, "data: {\"type\": \"connected\", \"client_id\": \"%s\"}\n\n", client.id)
	flusher.Flush()

	// Keepalive ticker (send comment every 30 seconds)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Stream messages to client
	for {
		select {
		case <-r.Context().Done():
			// Client disconnected
			return
		case <-client.done:
			// Hub closed this client
			return
		case msg := <-client.messages:
			// Send message
			fmt.Fprint(w, msg)
			flusher.Flush()
		case <-ticker.C:
			// Send keepalive comment
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}
