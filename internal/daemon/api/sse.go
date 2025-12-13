package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/logging"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// IndexProvider is an interface for getting the current index
type IndexProvider interface {
	GetIndex(ctx context.Context) (*types.GraphIndex, error)
}

// SSEClient represents a connected SSE client
type SSEClient struct {
	id            string
	clientType    string
	clientVersion string
	messages      chan string
	done          chan struct{}
	logger        *slog.Logger // Client-specific logger with correlation IDs
}

// SSEHub manages Server-Sent Events broadcasting for index updates
type SSEHub struct {
	clients       map[string]*SSEClient
	clientsMu     sync.RWMutex
	indexProvider IndexProvider
	logger        *slog.Logger
}

// NewSSEHub creates a new SSE broadcast hub
func NewSSEHub(logger *slog.Logger) *SSEHub {
	return &SSEHub{
		clients: make(map[string]*SSEClient),
		logger:  logger,
	}
}

// SetIndexProvider sets the index provider for including index data in events
func (h *SSEHub) SetIndexProvider(provider IndexProvider) {
	h.indexProvider = provider
}

// register adds a new SSE client to the hub
func (h *SSEHub) register(client *SSEClient) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()
	h.clients[client.id] = client

	// Use client-specific logger if available (includes correlation IDs)
	logger := h.logger
	if client.logger != nil {
		logger = client.logger
	}
	logger.Info("sse client connected", "total_clients", len(h.clients))
}

// unregister removes an SSE client from the hub
func (h *SSEHub) unregister(clientID string) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()

	if client, ok := h.clients[clientID]; ok {
		close(client.done)
		close(client.messages)
		delete(h.clients, clientID)

		// Use client-specific logger if available (includes correlation IDs)
		logger := h.logger
		if client.logger != nil {
			logger = client.logger
		}
		logger.Info("sse client disconnected", "total_clients", len(h.clients))
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

	// Get index data if provider is available
	var index *types.GraphIndex
	if h.indexProvider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var err error
		index, err = h.indexProvider.GetIndex(ctx)
		if err != nil {
			h.logger.Warn("failed to get index for SSE broadcast", "error", err)
		}
	}

	// Create notification payload
	event := SSEEvent{
		Type:      "index_updated",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      index,
	}

	data, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("failed to marshal index update notification", "error", err)
		return
	}

	message := fmt.Sprintf("data: %s\n\n", string(data))

	h.logger.Debug("broadcasting index update", "clients", len(h.clients), "has_index", index != nil)

	// Broadcast to all clients (non-blocking)
	for _, client := range h.clients {
		select {
		case client.messages <- message:
		case <-client.done:
			// Client disconnected, skip
		default:
			// Channel full, log warning but don't block
			h.logger.Warn("sse client message queue full", "client_id", client.id)
		}
	}
}

// HandleSSE handles SSE stream requests from MCP servers
func (h *SSEHub) HandleSSE(w http.ResponseWriter, r *http.Request) {
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

	// Extract and validate required client headers
	clientID := r.Header.Get(logging.HeaderClientID)
	clientType := r.Header.Get(logging.HeaderClientType)
	clientVersion := r.Header.Get(logging.HeaderClientVersion)

	// Validate all headers are present
	if clientID == "" || clientType == "" || clientVersion == "" {
		h.logger.Warn("sse connection rejected: missing required client headers",
			"has_client_id", clientID != "",
			"has_client_type", clientType != "",
			"has_client_version", clientVersion != "",
		)
		http.Error(w, "Missing required headers: X-Client-ID, X-Client-Type, X-Client-Version", http.StatusBadRequest)
		return
	}

	// Create client-specific logger with correlation IDs
	clientLogger := logging.WithDaemonSSE(h.logger, clientID, clientType, clientVersion)

	// Create new client
	client := &SSEClient{
		id:            clientID,
		clientType:    clientType,
		clientVersion: clientVersion,
		messages:      make(chan string, 10), // Buffer 10 messages
		done:          make(chan struct{}),
		logger:        clientLogger,
	}

	// Register client
	h.register(client)
	defer h.unregister(client.id)

	// Send initial index_snapshot with full index data
	h.sendIndexSnapshot(w, flusher, client.id, clientLogger)

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

// sendIndexSnapshot sends the initial index snapshot to a newly connected client
func (h *SSEHub) sendIndexSnapshot(w http.ResponseWriter, flusher http.Flusher, clientID string, logger *slog.Logger) {
	// Get index data if provider is available
	var index *types.GraphIndex
	if h.indexProvider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var err error
		index, err = h.indexProvider.GetIndex(ctx)
		if err != nil {
			logger.Warn("failed to get index for snapshot", "error", err)
		}
	}

	// Create snapshot event
	event := SSEEvent{
		Type:      "index_snapshot",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      index,
	}

	data, err := json.Marshal(event)
	if err != nil {
		logger.Error("failed to marshal index snapshot", "error", err)
		// Send fallback connected message
		fmt.Fprintf(w, "data: {\"type\": \"connected\", \"client_id\": \"%s\"}\n\n", clientID)
		flusher.Flush()
		return
	}

	fmt.Fprintf(w, "data: %s\n\n", string(data))
	flusher.Flush()

	logger.Info("sent index snapshot to client",
		"has_index", index != nil,
		"file_count", func() int {
			if index != nil {
				return len(index.Files)
			}
			return 0
		}(),
	)
}
