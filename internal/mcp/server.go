package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/internal/logging"
	"github.com/leefowlercu/agentic-memorizer/internal/mcp/handlers"
	"github.com/leefowlercu/agentic-memorizer/internal/mcp/protocol"
	"github.com/leefowlercu/agentic-memorizer/internal/mcp/transport"
	"github.com/leefowlercu/agentic-memorizer/internal/version"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

type Server struct {
	transport    transport.Transport
	index        *types.FileIndex
	indexMu      sync.RWMutex // Protects index access and reloading
	initialized  bool
	capabilities protocol.ServerCapabilities
	logger       *slog.Logger
	processID    string // UUIDv7 process identifier

	// Daemon API client (all data comes from daemon)
	daemonURL  string
	httpClient *http.Client

	// Subscription management
	subscriptions *SubscriptionManager

	// SSE client for daemon notifications
	sseClient *SSEClient

	// Prompt registry for MCP prompts
	promptRegistry *PromptRegistry

	// Handler registries
	resourceHandlers map[string]ResourceHandler
	toolHandlers     map[string]handlers.Handler
	promptHandlers   map[string]PromptHandler
}

type ResourceHandler func(ctx context.Context, uri string) (any, error)
type PromptHandler func(ctx context.Context, params json.RawMessage) (any, error)

// NewServer creates a new MCP server.
// daemonURL is the base URL of the daemon HTTP API (e.g., "http://localhost:8080").
// All data is fetched from the daemon - no direct FalkorDB connection.
func NewServer(index *types.FileIndex, logger *slog.Logger, daemonURL string) *Server {
	// Generate process_id and enrich logger with process context
	processID := logging.NewProcessID()
	enrichedLogger := logging.WithMCPProcess(logger, processID)

	httpClient := &http.Client{Timeout: 30 * time.Second}
	normalizedURL := strings.TrimSuffix(daemonURL, "/")

	s := &Server{
		transport:        transport.NewStdioTransport(),
		index:            index,
		logger:           enrichedLogger,
		processID:        processID,
		daemonURL:        normalizedURL,
		httpClient:       httpClient,
		subscriptions:    NewSubscriptionManager(),
		promptRegistry:   NewPromptRegistry(),
		resourceHandlers: make(map[string]ResourceHandler),
		toolHandlers:     make(map[string]handlers.Handler),
		promptHandlers:   make(map[string]PromptHandler),
	}

	// Create handler dependencies
	deps := &handlers.Dependencies{
		DaemonURL:  normalizedURL,
		HTTPClient: httpClient,
		Index:      s,
		Logger:     enrichedLogger,
	}

	// Register tool handlers
	s.registerToolHandler(handlers.NewSearchFilesHandler(deps))
	s.registerToolHandler(handlers.NewGetFileMetadataHandler(deps))
	s.registerToolHandler(handlers.NewListRecentFilesHandler(deps))
	s.registerToolHandler(handlers.NewGetRelatedFilesHandler(deps))
	s.registerToolHandler(handlers.NewSearchEntitiesHandler(deps))

	// Start SSE client for real-time daemon notifications.
	// Subscribes to index update events from daemon HTTP API.
	// Enables resource change notifications to MCP clients.
	// Only initialized if daemon URL configured (MCP can work standalone).
	if daemonURL != "" {
		sseURL := s.daemonURL + "/sse"
		// Pass process_id to SSEClient for header correlation
		sseLogger := logging.WithSSEClient(enrichedLogger)
		s.sseClient = NewSSEClient(sseURL, s, sseLogger, processID)
		s.sseClient.Start()
		enrichedLogger.Info("SSE client started", "daemon_url", daemonURL, "sse_url", sseURL)
	}

	return s
}

// registerToolHandler registers a tool handler
func (s *Server) registerToolHandler(h handlers.Handler) {
	s.toolHandlers[h.Name()] = h
}

// Run starts the MCP server loop
func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("mcp server starting")

	// Stop SSE client on shutdown
	defer func() {
		if s.sseClient != nil {
			s.sseClient.Stop()
			s.logger.Info("sse client stopped")
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Read message from transport
			data, err := s.transport.Read()
			if err != nil {
				if err == io.EOF {
					s.logger.Info("client disconnected")
					return nil
				}
				return fmt.Errorf("read error: %w", err)
			}

			s.logger.Debug("received message", "raw_data", string(data))

			// Handle message
			if err := s.handleMessage(ctx, data); err != nil {
				s.logger.Error("message handling error", "error", err)
				// Continue processing other messages
			}
		}
	}
}

// handleMessage routes incoming JSON-RPC messages
func (s *Server) handleMessage(ctx context.Context, data []byte) error {
	// Parse as generic JSON-RPC message
	var msg struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      any             `json:"id,omitempty"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
	}

	if err := json.Unmarshal(data, &msg); err != nil {
		s.logger.Error("failed to parse message", "error", err, "raw_data", string(data))
		// Only send error response if message has an ID (is a request, not a notification)
		if msg.ID != nil {
			return s.sendError(msg.ID, protocol.ParseError, "Parse error", nil)
		}
		return nil // Silently ignore malformed notifications
	}

	// Validate JSON-RPC version
	if msg.JSONRPC != "2.0" {
		// Only send error response if message has an ID (is a request, not a notification)
		if msg.ID != nil {
			return s.sendError(msg.ID, protocol.InvalidRequest, "Invalid JSON-RPC version", nil)
		}
		s.logger.Warn("invalid json-rpc version in notification", "version", msg.JSONRPC)
		return nil // Silently ignore notifications with invalid version
	}

	// Route based on method
	switch msg.Method {
	case "initialize":
		return s.handleInitialize(ctx, msg.ID, msg.Params)
	case "initialized", "notifications/initialized":
		return s.handleInitialized(ctx, msg.Params)
	case "resources/list":
		return s.handleResourcesList(ctx, msg.ID, msg.Params)
	case "resources/read":
		return s.handleResourcesRead(ctx, msg.ID, msg.Params)
	case "resources/subscribe":
		return s.handleResourcesSubscribe(ctx, msg.ID, msg.Params)
	case "resources/unsubscribe":
		return s.handleResourcesUnsubscribe(ctx, msg.ID, msg.Params)
	case "tools/list":
		return s.handleToolsList(ctx, msg.ID, msg.Params)
	case "tools/call":
		return s.handleToolsCall(ctx, msg.ID, msg.Params)
	case "prompts/list":
		return s.handlePromptsList(ctx, msg.ID, msg.Params)
	case "prompts/get":
		return s.handlePromptsGet(ctx, msg.ID, msg.Params)
	default:
		// Only send error response if message has an ID (is a request, not a notification)
		if msg.ID != nil {
			return s.sendError(msg.ID, protocol.MethodNotFound, fmt.Sprintf("Method not found: %s", msg.Method), nil)
		}
		s.logger.Warn("unrecognized notification method", "method", msg.Method)
		return nil // Silently ignore unrecognized notifications
	}
}

// handleInitialize processes the initialize request
func (s *Server) handleInitialize(ctx context.Context, id any, params json.RawMessage) error {
	var req protocol.InitializeRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return s.sendError(id, protocol.InvalidParams, "Invalid initialize params", nil)
	}

	s.logger.Info("received initialize request",
		"client", req.ClientInfo.Name,
		"version", req.ClientInfo.Version,
		"protocol", req.ProtocolVersion,
	)

	// Validate MCP protocol version - update supportedVersions when new versions released.
	// See MCP spec for version compatibility requirements.
	supportedVersions := []string{"2024-11-05", "2025-06-18", "2025-11-25"}
	supported := false
	for _, v := range supportedVersions {
		if req.ProtocolVersion == v {
			supported = true
			break
		}
	}
	if !supported {
		return s.sendError(id, protocol.InvalidRequest,
			fmt.Sprintf("Unsupported protocol version: %s (supported: %v)",
				req.ProtocolVersion, supportedVersions),
			nil,
		)
	}

	// Build capabilities response
	resp := protocol.InitializeResponse{
		ProtocolVersion: req.ProtocolVersion, // Echo back client's requested version
		ServerInfo: protocol.ServerInfo{
			Name:    "memorizer",
			Version: version.GetShortVersion(),
		},
		Capabilities: protocol.ServerCapabilities{
			Resources: &protocol.ResourcesCapability{
				Subscribe:   true,
				ListChanged: true,
			},
			Tools: &protocol.ToolsCapability{
				ListChanged: false, // Phase 5
			},
			Prompts: &protocol.PromptsCapability{
				ListChanged: false, // Phase 5
			},
		},
	}

	s.capabilities = resp.Capabilities

	// Generate session_id and enrich logger with session context
	sessionID := logging.NewSessionID()
	s.logger = logging.WithSessionID(s.logger, sessionID)
	s.logger = logging.WithClientInfo(s.logger, req.ClientInfo.Name, req.ClientInfo.Version)

	s.logger.Info("session initialized", "session_id", sessionID)

	return s.sendResponse(id, resp)
}

// handleInitialized processes the initialized notification
func (s *Server) handleInitialized(ctx context.Context, params json.RawMessage) error {
	s.logger.Info("received initialized notification")
	s.initialized = true
	// No response needed for notifications
	return nil
}

// handleResourcesList returns the list of available resources
func (s *Server) handleResourcesList(ctx context.Context, id any, params json.RawMessage) error {
	if !s.initialized {
		return s.sendError(id, protocol.ServerNotReady, "Server not initialized", nil)
	}

	resources := []protocol.Resource{
		{
			URI:         "memorizer://index",
			Name:        "Memory Index",
			Description: "Complete semantic index of files in memory directory (XML format)",
			MimeType:    "application/xml",
		},
		{
			URI:         "memorizer://index/json",
			Name:        "Memory Index (JSON)",
			Description: "Structured JSON format of memory index",
			MimeType:    "application/json",
		},
	}

	resp := protocol.ResourcesListResponse{
		Resources: resources,
	}

	s.logger.Info("returning resources list", "count", len(resources))
	return s.sendResponse(id, resp)
}

// handleResourcesRead reads and returns a specific resource
func (s *Server) handleResourcesRead(ctx context.Context, id any, params json.RawMessage) error {
	if !s.initialized {
		return s.sendError(id, protocol.ServerNotReady, "Server not initialized", nil)
	}

	var req protocol.ResourcesReadRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return s.sendError(id, protocol.InvalidParams, "Invalid resource read params", nil)
	}

	s.logger.Info("reading resource", "uri", req.URI)

	// Parse URI and route to appropriate handler
	var content string
	var mimeType string
	var err error

	switch req.URI {
	case "memorizer://index":
		content, err = s.formatIndexXML()
		mimeType = "application/xml"
	case "memorizer://index/json":
		content, err = s.formatIndexJSON()
		mimeType = "application/json"
	default:
		return s.sendError(id, protocol.InvalidParams,
			fmt.Sprintf("Resource not found: %s", req.URI),
			nil,
		)
	}

	if err != nil {
		return s.sendError(id, protocol.InternalError,
			fmt.Sprintf("Failed to format resource: %v", err),
			nil,
		)
	}

	resp := protocol.ResourcesReadResponse{
		Contents: []protocol.ResourceContent{
			{
				URI:      req.URI,
				MimeType: mimeType,
				Text:     content,
			},
		},
	}

	return s.sendResponse(id, resp)
}

// handleResourcesSubscribe handles resources/subscribe requests
func (s *Server) handleResourcesSubscribe(ctx context.Context, id any, params json.RawMessage) error {
	if !s.initialized {
		return s.sendError(id, protocol.ServerNotReady, "Server not initialized", nil)
	}

	var req protocol.ResourcesSubscribeRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return s.sendError(id, protocol.InvalidParams, "Invalid subscribe params", nil)
	}

	s.logger.Info("subscribing to resource", "uri", req.URI)

	// Validate URI - only allow memorizer:// URIs
	validURIs := map[string]bool{
		"memorizer://index":      true,
		"memorizer://index/json": true,
	}

	if !validURIs[req.URI] {
		return s.sendError(id, -32002, // Custom error code for invalid resource
			fmt.Sprintf("Invalid resource URI: %s", req.URI),
			nil,
		)
	}

	// Add subscription
	s.subscriptions.Subscribe(req.URI)

	// Return empty success response
	resp := protocol.ResourcesSubscribeResponse{}
	return s.sendResponse(id, resp)
}

// handleResourcesUnsubscribe handles resources/unsubscribe requests
func (s *Server) handleResourcesUnsubscribe(ctx context.Context, id any, params json.RawMessage) error {
	if !s.initialized {
		return s.sendError(id, protocol.ServerNotReady, "Server not initialized", nil)
	}

	var req protocol.ResourcesUnsubscribeRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return s.sendError(id, protocol.InvalidParams, "Invalid unsubscribe params", nil)
	}

	s.logger.Info("unsubscribing from resource", "uri", req.URI)

	// Remove subscription (no error if not subscribed)
	s.subscriptions.Unsubscribe(req.URI)

	// Return empty success response
	resp := protocol.ResourcesUnsubscribeResponse{}
	return s.sendResponse(id, resp)
}

// formatIndexXML formats the index as XML
func (s *Server) formatIndexXML() (string, error) {
	formatter, err := format.GetFormatter("xml")
	if err != nil {
		return "", fmt.Errorf("failed to get formatter; %w", err)
	}
	filesContent := format.NewFilesContent(s.index)
	return formatter.Format(filesContent)
}

// formatIndexJSON formats the index as JSON
func (s *Server) formatIndexJSON() (string, error) {
	formatter, err := format.GetFormatter("json")
	if err != nil {
		return "", fmt.Errorf("failed to get formatter; %w", err)
	}
	filesContent := format.NewFilesContent(s.index)
	return formatter.Format(filesContent)
}

func (s *Server) handleToolsList(ctx context.Context, id any, params json.RawMessage) error {
	if !s.initialized {
		return s.sendError(id, protocol.ServerNotReady, "Server not initialized", nil)
	}

	// Build tools list from registered handlers
	tools := make([]protocol.Tool, 0, len(s.toolHandlers))
	for _, handler := range s.toolHandlers {
		tools = append(tools, handler.ToolDefinition())
	}

	resp := protocol.ToolsListResponse{
		Tools: tools,
	}

	s.logger.Info("returning tools list", "count", len(tools))
	return s.sendResponse(id, resp)
}

func (s *Server) handleToolsCall(ctx context.Context, id any, params json.RawMessage) error {
	if !s.initialized {
		return s.sendError(id, protocol.ServerNotReady, "Server not initialized", nil)
	}

	var req protocol.ToolsCallRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return s.sendError(id, protocol.InvalidParams, "Invalid tool call params", nil)
	}

	s.logger.Info("calling tool", "name", req.Name)

	// Look up handler
	handler, exists := s.toolHandlers[req.Name]
	if !exists {
		return s.sendError(id, protocol.MethodNotFound,
			fmt.Sprintf("Tool not found: %s", req.Name),
			nil,
		)
	}

	// Execute handler
	result, err := handler.Execute(ctx, req.Arguments)
	if err != nil {
		// Tool execution error - return as tool response with isError flag
		resp := protocol.ToolsCallResponse{
			Content: []protocol.ToolContent{
				{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}
		return s.sendResponse(id, resp)
	}

	// Format successful result as JSON
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return s.sendError(id, protocol.InternalError,
			fmt.Sprintf("Failed to format tool result: %v", err),
			nil,
		)
	}

	resp := protocol.ToolsCallResponse{
		Content: []protocol.ToolContent{
			{
				Type: "text",
				Text: string(resultJSON),
			},
		},
		IsError: false,
	}

	return s.sendResponse(id, resp)
}

func (s *Server) handlePromptsList(ctx context.Context, id any, params json.RawMessage) error {
	s.logger.Debug("processing prompts/list request")

	// Get all prompts from registry
	prompts := s.promptRegistry.ListPrompts()

	resp := protocol.PromptsListResponse{
		Prompts: prompts,
	}

	return s.sendResponse(id, resp)
}

func (s *Server) handlePromptsGet(ctx context.Context, id any, params json.RawMessage) error {
	var req protocol.PromptsGetRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return s.sendError(id, protocol.InvalidParams, "Invalid prompts/get params", nil)
	}

	s.logger.Debug("processing prompts/get request", "name", req.Name)

	// Generate messages for the requested prompt
	messages, err := s.promptRegistry.GeneratePromptMessages(req.Name, req.Arguments, s)
	if err != nil {
		return s.sendError(id, protocol.InvalidParams, err.Error(), nil)
	}

	// Get prompt description from registry
	prompt, err := s.promptRegistry.GetPrompt(req.Name)
	if err != nil {
		return s.sendError(id, protocol.InvalidParams, err.Error(), nil)
	}

	resp := protocol.PromptsGetResponse{
		Description: prompt.Description,
		Messages:    messages,
	}

	return s.sendResponse(id, resp)
}

// sendResponse sends a JSON-RPC response
func (s *Server) sendResponse(id any, result any) error {
	response := protocol.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
	}

	resultData, err := json.Marshal(result)
	if err != nil {
		return err
	}
	response.Result = resultData

	data, err := json.Marshal(response)
	if err != nil {
		return err
	}

	s.logger.Debug("sending response", "raw_data", string(data))
	return s.transport.Write(data)
}

// sendError sends a JSON-RPC error response
func (s *Server) sendError(id any, code int, message string, data any) error {
	response := protocol.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &protocol.JSONRPCError{
			Code:    code,
			Message: message,
		},
	}

	if data != nil {
		errorData, err := json.Marshal(data)
		if err != nil {
			return err
		}
		response.Error.Data = errorData
	}

	responseData, err := json.Marshal(response)
	if err != nil {
		return err
	}

	s.logger.Debug("sending error", "raw_data", string(responseData))
	return s.transport.Write(responseData)
}

// GetIndex returns the current index (thread-safe)
func (s *Server) GetIndex() *types.FileIndex {
	s.indexMu.RLock()
	defer s.indexMu.RUnlock()
	return s.index
}

// ReloadIndex atomically updates the server's index (thread-safe)
func (s *Server) ReloadIndex(newIndex *types.FileIndex) {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()
	s.index = newIndex
	s.logger.Info("index reloaded", "files", len(newIndex.Files))
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	s.logger.Info("mcp server shutting down")
	return s.transport.Close()
}

// callDaemonAPI makes an HTTP request to the daemon API
func (s *Server) callDaemonAPI(ctx context.Context, method, path string, body any) ([]byte, error) {
	if s.daemonURL == "" {
		return nil, fmt.Errorf("daemon URL not configured")
	}

	url := s.daemonURL + path

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request; %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request; %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed; %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response; %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr struct {
			Error   string `json:"error"`
			Details string `json:"details"`
		}
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error != "" {
			return nil, fmt.Errorf("%s: %s", apiErr.Error, apiErr.Details)
		}
		return nil, fmt.Errorf("API error: %s", resp.Status)
	}

	return respBody, nil
}

// hasDaemonAPI returns true if the daemon API is configured
func (s *Server) hasDaemonAPI() bool {
	return s.daemonURL != ""
}
