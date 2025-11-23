package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/index"
	"github.com/leefowlercu/agentic-memorizer/internal/mcp/protocol"
)

// SSEClient connects to daemon's SSE notification stream
type SSEClient struct {
	daemonURL        string
	server           *Server
	logger           *slog.Logger
	reconnectBackoff time.Duration
	ctx              context.Context
	cancel           context.CancelFunc
}

// NewSSEClient creates a new SSE client
func NewSSEClient(daemonURL string, server *Server, logger *slog.Logger) *SSEClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &SSEClient{
		daemonURL:        daemonURL,
		server:           server,
		logger:           logger,
		reconnectBackoff: 5 * time.Second,
		ctx:              ctx,
		cancel:           cancel,
	}
}

// Start begins listening to SSE stream
func (c *SSEClient) Start() {
	go c.connectLoop()
}

// Stop stops the SSE client
func (c *SSEClient) Stop() {
	c.cancel()
}

// connectLoop handles connection with automatic reconnection
func (c *SSEClient) connectLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			c.connect()
			// Wait before reconnecting
			select {
			case <-c.ctx.Done():
				return
			case <-time.After(c.reconnectBackoff):
				c.logger.Info("Reconnecting to SSE stream")
			}
		}
	}
}

// connect establishes SSE connection and processes events
func (c *SSEClient) connect() {
	c.logger.Info("Connecting to daemon SSE stream", "url", c.daemonURL)

	req, err := http.NewRequestWithContext(c.ctx, "GET", c.daemonURL, nil)
	if err != nil {
		c.logger.Error("Failed to create request", "error", err)
		return
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	client := &http.Client{
		Timeout: 0, // No timeout for streaming connection
	}

	resp, err := client.Do(req)
	if err != nil {
		c.logger.Warn("Failed to connect to SSE stream", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("SSE stream returned non-200 status", "status", resp.StatusCode)
		return
	}

	c.logger.Info("Connected to SSE stream")

	scanner := bufio.NewScanner(resp.Body)
	var eventData string

	for scanner.Scan() {
		line := scanner.Text()

		// Parse SSE format
		if strings.HasPrefix(line, "data:") {
			eventData = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		} else if line == "" {
			// Empty line marks end of event
			if eventData != "" {
				c.handleEvent(eventData)
			}
			eventData = ""
		}
		// Ignore other lines (event:, :comments)
	}

	if err := scanner.Err(); err != nil {
		c.logger.Warn("SSE stream read error", "error", err)
	}

	c.logger.Info("SSE stream connection closed")
}

// handleEvent processes received SSE events
func (c *SSEClient) handleEvent(data string) {
	c.logger.Debug("Received SSE event", "data", data)

	var notification struct {
		Type      string                 `json:"type"`
		Timestamp string                 `json:"timestamp"`
		Metadata  map[string]interface{} `json:"metadata,omitempty"`
	}

	if err := json.Unmarshal([]byte(data), &notification); err != nil {
		c.logger.Error("Failed to parse notification", "error", err)
		return
	}

	c.logger.Info("Processing notification", "type", notification.Type)

	switch notification.Type {
	case "index_updated":
		c.handleIndexUpdate()
	default:
		c.logger.Debug("Ignoring notification type", "type", notification.Type)
	}
}

// handleIndexUpdate reloads index and sends MCP notifications
func (c *SSEClient) handleIndexUpdate() {
	// Reload index from disk
	indexPath, err := config.GetIndexPath()
	if err != nil {
		c.logger.Error("Failed to get index path", "error", err)
		return
	}

	indexManager := index.NewManager(indexPath)

	computed, err := indexManager.LoadComputed()
	if err != nil {
		c.logger.Error("Failed to reload index", "error", err)
		return
	}

	c.logger.Info("Reloaded index from disk", "files", len(computed.Index.Entries))
	c.server.ReloadIndex(computed.Index)

	// Send MCP notifications to client for subscribed resources
	c.sendMCPNotifications()
}

// sendMCPNotifications sends JSON-RPC notifications for subscribed resources
func (c *SSEClient) sendMCPNotifications() {
	subscribed := c.server.subscriptions.GetSubscriptions()

	if len(subscribed) == 0 {
		c.logger.Debug("No active subscriptions")
		return
	}

	c.logger.Info("Sending MCP notifications", "count", len(subscribed))

	for _, uri := range subscribed {
		notification := protocol.JSONRPCNotification{
			JSONRPC: "2.0",
			Method:  "notifications/resources/updated",
		}

		params := map[string]string{
			"uri": uri,
		}

		paramsJSON, _ := json.Marshal(params)
		notification.Params = paramsJSON

		data, err := json.Marshal(notification)
		if err != nil {
			c.logger.Error("Failed to marshal notification", "error", err)
			continue
		}

		if err := c.server.transport.Write(data); err != nil {
			c.logger.Error("Failed to write MCP notification", "error", err)
			continue
		}

		c.logger.Debug("Sent MCP notification", "uri", uri)
	}
}
