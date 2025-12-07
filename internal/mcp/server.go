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
	"github.com/leefowlercu/agentic-memorizer/internal/mcp/protocol"
	"github.com/leefowlercu/agentic-memorizer/internal/mcp/transport"
	"github.com/leefowlercu/agentic-memorizer/internal/search"
	"github.com/leefowlercu/agentic-memorizer/internal/version"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

type Server struct {
	transport    transport.Transport
	index        *types.GraphIndex
	indexMu      sync.RWMutex // Protects index access and reloading
	initialized  bool
	capabilities protocol.ServerCapabilities
	logger       *slog.Logger

	// Daemon API client (all data comes from daemon)
	daemonURL  string
	httpClient *http.Client

	// Subscription management
	subscriptions *SubscriptionManager

	// SSE client for daemon notifications
	sseClient *SSEClient

	// Prompt registry for MCP prompts
	promptRegistry *PromptRegistry

	// Handler registries (for Phase 2+)
	resourceHandlers map[string]ResourceHandler
	toolHandlers     map[string]ToolHandler
	promptHandlers   map[string]PromptHandler
}

type ResourceHandler func(ctx context.Context, uri string) (any, error)
type ToolHandler func(ctx context.Context, params json.RawMessage) (any, error)
type PromptHandler func(ctx context.Context, params json.RawMessage) (any, error)

// NewServer creates a new MCP server.
// daemonURL is the base URL of the daemon HTTP API (e.g., "http://localhost:8080").
// All data is fetched from the daemon - no direct FalkorDB connection.
func NewServer(index *types.GraphIndex, logger *slog.Logger, daemonURL string) *Server {
	s := &Server{
		transport:        transport.NewStdioTransport(),
		index:            index,
		logger:           logger,
		daemonURL:        strings.TrimSuffix(daemonURL, "/"),
		httpClient:       &http.Client{Timeout: 30 * time.Second},
		subscriptions:    NewSubscriptionManager(),
		promptRegistry:   NewPromptRegistry(),
		resourceHandlers: make(map[string]ResourceHandler),
		toolHandlers:     make(map[string]ToolHandler),
		promptHandlers:   make(map[string]PromptHandler),
	}

	// Register tool handlers
	s.toolHandlers["search_files"] = s.handleSearchFiles
	s.toolHandlers["get_file_metadata"] = s.handleGetFileMetadata
	s.toolHandlers["list_recent_files"] = s.handleListRecentFiles
	s.toolHandlers["get_related_files"] = s.handleGetRelatedFiles
	s.toolHandlers["search_entities"] = s.handleSearchEntities

	// Start SSE client for real-time daemon notifications.
	// Subscribes to index update events from daemon HTTP API.
	// Enables resource change notifications to MCP clients.
	// Only initialized if daemon URL configured (MCP can work standalone).
	if daemonURL != "" {
		sseURL := s.daemonURL + "/sse"
		s.sseClient = NewSSEClient(sseURL, s, logger)
		s.sseClient.Start()
		logger.Info("SSE client started", "daemon_url", daemonURL, "sse_url", sseURL)
	}

	return s
}

// Run starts the MCP server loop
func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("MCP server starting")

	// Stop SSE client on shutdown
	defer func() {
		if s.sseClient != nil {
			s.sseClient.Stop()
			s.logger.Info("SSE client stopped")
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
					s.logger.Info("Client disconnected")
					return nil
				}
				return fmt.Errorf("read error: %w", err)
			}

			s.logger.Debug("Received message", "raw_data", string(data))

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
		s.logger.Error("Failed to parse message", "error", err, "raw_data", string(data))
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
		s.logger.Warn("Invalid JSON-RPC version in notification", "version", msg.JSONRPC)
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
		s.logger.Warn("Unrecognized notification method", "method", msg.Method)
		return nil // Silently ignore unrecognized notifications
	}
}

// handleInitialize processes the initialize request
func (s *Server) handleInitialize(ctx context.Context, id any, params json.RawMessage) error {
	var req protocol.InitializeRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return s.sendError(id, protocol.InvalidParams, "Invalid initialize params", nil)
	}

	s.logger.Info("Received initialize request",
		"client", req.ClientInfo.Name,
		"version", req.ClientInfo.Version,
		"protocol", req.ProtocolVersion,
	)

	// Validate MCP protocol version - update supportedVersions when new versions released.
	// See MCP spec for version compatibility requirements.
	supportedVersions := []string{"2024-11-05", "2025-06-18"}
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
			Name:    "agentic-memorizer",
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

	return s.sendResponse(id, resp)
}

// handleInitialized processes the initialized notification
func (s *Server) handleInitialized(ctx context.Context, params json.RawMessage) error {
	s.logger.Info("Received initialized notification")
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
			URI:         "memorizer://index/markdown",
			Name:        "Memory Index (Markdown)",
			Description: "Human-readable markdown format of memory index",
			MimeType:    "text/markdown",
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

	s.logger.Info("Returning resources list", "count", len(resources))
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

	s.logger.Info("Reading resource", "uri", req.URI)

	// Parse URI and route to appropriate handler
	var content string
	var mimeType string
	var err error

	switch req.URI {
	case "memorizer://index":
		content, err = s.formatIndexXML()
		mimeType = "application/xml"
	case "memorizer://index/markdown":
		content, err = s.formatIndexMarkdown()
		mimeType = "text/markdown"
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

	s.logger.Info("Subscribing to resource", "uri", req.URI)

	// Validate URI - only allow memorizer:// URIs
	validURIs := map[string]bool{
		"memorizer://index":          true,
		"memorizer://index/markdown": true,
		"memorizer://index/json":     true,
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

	s.logger.Info("Unsubscribing from resource", "uri", req.URI)

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
	graphContent := format.NewGraphContent(s.index)
	return formatter.Format(graphContent)
}

// formatIndexMarkdown formats the index as Markdown
func (s *Server) formatIndexMarkdown() (string, error) {
	formatter, err := format.GetFormatter("markdown")
	if err != nil {
		return "", fmt.Errorf("failed to get formatter; %w", err)
	}
	graphContent := format.NewGraphContent(s.index)
	return formatter.Format(graphContent)
}

// formatIndexJSON formats the index as JSON
func (s *Server) formatIndexJSON() (string, error) {
	formatter, err := format.GetFormatter("json")
	if err != nil {
		return "", fmt.Errorf("failed to get formatter; %w", err)
	}
	graphContent := format.NewGraphContent(s.index)
	return formatter.Format(graphContent)
}

// handleSearchFiles performs semantic search across indexed files
func (s *Server) handleSearchFiles(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Query      string   `json:"query"`
		Categories []string `json:"categories,omitempty"`
		MaxResults int      `json:"max_results,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments; %w", err)
	}

	if params.Query == "" {
		return nil, fmt.Errorf("query parameter is required")
	}

	if params.MaxResults == 0 {
		params.MaxResults = 10 // default
	}

	// Try daemon API first if available
	if s.hasDaemonAPI() {
		// Determine category filter
		categoryFilter := ""
		if len(params.Categories) > 0 {
			categoryFilter = params.Categories[0] // Use first category for filter
		}

		reqBody := map[string]any{
			"query":    params.Query,
			"limit":    params.MaxResults,
			"category": categoryFilter,
		}

		respBody, err := s.callDaemonAPI(ctx, "POST", "/api/v1/search", reqBody)
		if err == nil {
			var searchResp struct {
				Results []struct {
					Path      string  `json:"path"`
					Name      string  `json:"name"`
					Category  string  `json:"category"`
					Score     float64 `json:"score"`
					MatchType string  `json:"match_type"`
					Summary   string  `json:"summary"`
				} `json:"results"`
				Count int `json:"count"`
			}
			if json.Unmarshal(respBody, &searchResp) == nil {
				// Format API results
				formattedResults := make([]map[string]any, len(searchResp.Results))
				for i, result := range searchResp.Results {
					formattedResults[i] = map[string]any{
						"path":       result.Path,
						"name":       result.Name,
						"category":   result.Category,
						"score":      result.Score,
						"match_type": result.MatchType,
						"summary":    result.Summary,
					}
				}

				return map[string]any{
					"query":        params.Query,
					"result_count": searchResp.Count,
					"source":       "daemon",
					"results":      formattedResults,
				}, nil
			}
		}
		// Fall through to index-based search on error
		s.logger.Debug("daemon search failed, falling back to index", "error", err)
	}

	// Fallback to index-based search
	searcher := search.NewSearcher(s.index)
	results := searcher.Search(search.SearchQuery{
		Query:      params.Query,
		Categories: params.Categories,
		MaxResults: params.MaxResults,
	})

	// Format results
	formattedResults := make([]map[string]any, len(results))
	for i, result := range results {
		formattedResults[i] = map[string]any{
			"path":       result.File.Path,
			"name":       result.File.Name,
			"category":   result.File.Category,
			"score":      result.Score,
			"match_type": result.MatchType,
			"size_human": result.File.SizeHuman,
			"modified":   result.File.Modified.Format(time.RFC3339),
		}

		// Add semantic fields if available
		if result.File.Summary != "" {
			formattedResults[i]["summary"] = result.File.Summary
			formattedResults[i]["tags"] = result.File.Tags
		}
	}

	return map[string]any{
		"query":        params.Query,
		"result_count": len(results),
		"source":       "index",
		"results":      formattedResults,
	}, nil
}

// handleGetFileMetadata returns complete metadata for a specific file
func (s *Server) handleGetFileMetadata(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Path string `json:"path"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments; %w", err)
	}

	if params.Path == "" {
		return nil, fmt.Errorf("path parameter is required")
	}

	// Try daemon API first if available
	if s.hasDaemonAPI() {
		// URL-encode the path for the API call
		encodedPath := strings.ReplaceAll(params.Path, "/", "%2F")
		respBody, err := s.callDaemonAPI(ctx, "GET", "/api/v1/files/"+encodedPath, nil)
		if err == nil {
			var fileResp struct {
				File json.RawMessage `json:"file"`
			}
			if json.Unmarshal(respBody, &fileResp) == nil {
				result := map[string]any{
					"source": "daemon",
				}
				// Parse file entry
				var file any
				if json.Unmarshal(fileResp.File, &file) == nil {
					result["file"] = file
				}
				return result, nil
			}
		}
		// Fall through to index-based lookup
		s.logger.Debug("daemon file lookup failed, falling back to index", "error", err)
	}

	// Fallback to index-based search - convert to simplified file format
	pathLower := strings.ToLower(params.Path)
	for _, file := range s.index.Files {
		filePathLower := strings.ToLower(file.Path)
		// Match if path contains the query or vice versa
		if strings.Contains(filePathLower, pathLower) || strings.Contains(pathLower, filePathLower) {
			// Convert FileEntry to a map for consistency
			result := map[string]any{
				"path":        file.Path,
				"name":        file.Name,
				"hash":        file.Hash,
				"type":        file.Type,
				"category":    file.Category,
				"size":        file.Size,
				"size_human":  file.SizeHuman,
				"modified":    file.Modified.Format(time.RFC3339),
				"is_readable": file.IsReadable,
			}
			if file.Summary != "" {
				result["summary"] = file.Summary
				result["document_type"] = file.DocumentType
				result["tags"] = file.Tags
				result["topics"] = file.Topics
			}
			return map[string]any{
				"file":   file,
				"source": "index",
			}, nil
		}
	}

	return nil, fmt.Errorf("file not found: %s", params.Path)
}

// handleListRecentFiles returns files modified within the specified time period
func (s *Server) handleListRecentFiles(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Days  int `json:"days,omitempty"`
		Limit int `json:"limit,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments; %w", err)
	}

	if params.Days == 0 {
		params.Days = 7 // default
	}
	if params.Limit == 0 {
		params.Limit = 20 // default
	}

	// Try daemon API first if available
	if s.hasDaemonAPI() {
		path := fmt.Sprintf("/api/v1/files/recent?days=%d&limit=%d", params.Days, params.Limit)
		respBody, err := s.callDaemonAPI(ctx, "GET", path, nil)
		if err == nil {
			var recentResp struct {
				Files []struct {
					Path     string `json:"path"`
					Name     string `json:"name"`
					Category string `json:"category"`
					Summary  string `json:"summary"`
				} `json:"files"`
				Count int `json:"count"`
			}
			if json.Unmarshal(respBody, &recentResp) == nil {
				// Format API results
				formattedResults := make([]map[string]any, len(recentResp.Files))
				for i, file := range recentResp.Files {
					formattedResults[i] = map[string]any{
						"path":     file.Path,
						"name":     file.Name,
						"category": file.Category,
						"summary":  file.Summary,
					}
				}

				return map[string]any{
					"days":         params.Days,
					"result_count": recentResp.Count,
					"source":       "daemon",
					"files":        formattedResults,
				}, nil
			}
		}
		// Fall through to index-based query
		s.logger.Debug("daemon recent files query failed, falling back to index", "error", err)
	}

	// Fallback to index-based query
	cutoff := time.Now().AddDate(0, 0, -params.Days)

	// Filter recent files
	var recentFiles []types.FileEntry
	for _, file := range s.index.Files {
		if file.Modified.After(cutoff) {
			recentFiles = append(recentFiles, file)
		}
	}

	// Sort by modified time descending
	for i := 0; i < len(recentFiles)-1; i++ {
		for j := i + 1; j < len(recentFiles); j++ {
			if recentFiles[j].Modified.After(recentFiles[i].Modified) {
				recentFiles[i], recentFiles[j] = recentFiles[j], recentFiles[i]
			}
		}
	}

	// Limit results
	if len(recentFiles) > params.Limit {
		recentFiles = recentFiles[:params.Limit]
	}

	// Format results
	formattedResults := make([]map[string]any, len(recentFiles))
	for i, file := range recentFiles {
		formattedResults[i] = map[string]any{
			"path":       file.Path,
			"name":       file.Name,
			"category":   file.Category,
			"size_human": file.SizeHuman,
			"modified":   file.Modified.Format(time.RFC3339),
		}

		// Add semantic fields if available
		if file.Summary != "" {
			formattedResults[i]["summary"] = file.Summary
			formattedResults[i]["tags"] = file.Tags
		}
	}

	return map[string]any{
		"days":         params.Days,
		"result_count": len(formattedResults),
		"source":       "index",
		"files":        formattedResults,
	}, nil
}

// handleGetRelatedFiles finds files related to a given file through graph relationships
func (s *Server) handleGetRelatedFiles(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Path  string `json:"path"`
		Limit int    `json:"limit,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments; %w", err)
	}

	if params.Path == "" {
		return nil, fmt.Errorf("path parameter is required")
	}

	if params.Limit == 0 {
		params.Limit = 10 // default
	}

	// Check if daemon API is available
	if !s.hasDaemonAPI() {
		return nil, fmt.Errorf("daemon API not available; related files search requires daemon connection")
	}

	// Get related files from daemon API
	path := fmt.Sprintf("/api/v1/files/related?path=%s&limit=%d", params.Path, params.Limit)
	respBody, err := s.callDaemonAPI(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get related files; %w", err)
	}

	var relatedResp struct {
		Files []struct {
			Path           string  `json:"path"`
			Name           string  `json:"name"`
			Summary        string  `json:"summary"`
			Strength       float64 `json:"strength"`
			ConnectionType string  `json:"connection_type"`
		} `json:"files"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal(respBody, &relatedResp); err != nil {
		return nil, fmt.Errorf("failed to parse response; %w", err)
	}

	// Format results
	formattedResults := make([]map[string]any, len(relatedResp.Files))
	for i, rel := range relatedResp.Files {
		formattedResults[i] = map[string]any{
			"path":            rel.Path,
			"name":            rel.Name,
			"summary":         rel.Summary,
			"strength":        rel.Strength,
			"connection_type": rel.ConnectionType,
		}
	}

	return map[string]any{
		"source_file":  params.Path,
		"result_count": len(formattedResults),
		"related":      formattedResults,
	}, nil
}

// handleSearchEntities searches for files by entity name
func (s *Server) handleSearchEntities(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Entity     string `json:"entity"`
		EntityType string `json:"entity_type,omitempty"`
		MaxResults int    `json:"max_results,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments; %w", err)
	}

	if params.Entity == "" {
		return nil, fmt.Errorf("entity parameter is required")
	}

	if params.MaxResults == 0 {
		params.MaxResults = 10 // default
	}

	// Check if daemon API is available
	if !s.hasDaemonAPI() {
		return nil, fmt.Errorf("daemon API not available; entity search requires daemon connection")
	}

	// Search by entity via daemon API
	path := fmt.Sprintf("/api/v1/entities/search?entity=%s&limit=%d", params.Entity, params.MaxResults)
	respBody, err := s.callDaemonAPI(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to search entities; %w", err)
	}

	var entityResp struct {
		Results []struct {
			Path      string `json:"path"`
			Name      string `json:"name"`
			Category  string `json:"category"`
			Summary   string `json:"summary"`
			MatchType string `json:"match_type"`
		} `json:"results"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal(respBody, &entityResp); err != nil {
		return nil, fmt.Errorf("failed to parse response; %w", err)
	}

	// Format results
	formattedResults := make([]map[string]any, len(entityResp.Results))
	for i, result := range entityResp.Results {
		formattedResults[i] = map[string]any{
			"path":       result.Path,
			"name":       result.Name,
			"category":   result.Category,
			"summary":    result.Summary,
			"match_type": result.MatchType,
		}
	}

	return map[string]any{
		"entity":       params.Entity,
		"entity_type":  params.EntityType,
		"result_count": len(formattedResults),
		"results":      formattedResults,
	}, nil
}

// formatSize formats bytes as human-readable string
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func (s *Server) handleToolsList(ctx context.Context, id any, params json.RawMessage) error {
	if !s.initialized {
		return s.sendError(id, protocol.ServerNotReady, "Server not initialized", nil)
	}

	tools := []protocol.Tool{
		{
			Name:        "search_files",
			Description: "Search for files in the memory index using semantic search. Returns ranked results based on relevance to the query.",
			InputSchema: protocol.InputSchema{
				Type: "object",
				Properties: map[string]protocol.Property{
					"query": {
						Type:        "string",
						Description: "Search query to match against filenames, summaries, tags, and topics",
					},
					"categories": {
						Type:        "array",
						Description: "Optional filter by file categories (e.g., documents, code, images)",
						Items:       &protocol.Items{Type: "string"},
					},
					"max_results": {
						Type:        "integer",
						Description: "Maximum number of results to return",
						Default:     10,
						Minimum:     ptrInt(1),
						Maximum:     ptrInt(100),
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "get_file_metadata",
			Description: "Get complete metadata and semantic analysis for a specific file by path. Returns all available information including summaries, tags, topics, entities, related files, and file-specific metadata in a unified FileEntry format.",
			InputSchema: protocol.InputSchema{
				Type: "object",
				Properties: map[string]protocol.Property{
					"path": {
						Type:        "string",
						Description: "File path (absolute or relative) to retrieve metadata for",
					},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "list_recent_files",
			Description: "List recently modified files within a specified time period. Returns files sorted by modification date (newest first).",
			InputSchema: protocol.InputSchema{
				Type: "object",
				Properties: map[string]protocol.Property{
					"days": {
						Type:        "integer",
						Description: "Number of days to look back for recent files",
						Default:     7,
						Minimum:     ptrInt(1),
						Maximum:     ptrInt(365),
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of files to return",
						Default:     20,
						Minimum:     ptrInt(1),
						Maximum:     ptrInt(100),
					},
				},
			},
		},
		{
			Name:        "get_related_files",
			Description: "Find files related to a given file through shared tags, topics, or entities in the knowledge graph. Requires FalkorDB to be running.",
			InputSchema: protocol.InputSchema{
				Type: "object",
				Properties: map[string]protocol.Property{
					"path": {
						Type:        "string",
						Description: "Path to the source file to find related files for",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of related files to return",
						Default:     10,
						Minimum:     ptrInt(1),
						Maximum:     ptrInt(50),
					},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "search_entities",
			Description: "Search for files that mention a specific entity (technology, person, concept, organization). Requires FalkorDB to be running.",
			InputSchema: protocol.InputSchema{
				Type: "object",
				Properties: map[string]protocol.Property{
					"entity": {
						Type:        "string",
						Description: "Entity name to search for (e.g., 'Go', 'FalkorDB', 'authentication')",
					},
					"entity_type": {
						Type:        "string",
						Description: "Optional entity type filter (technology, person, concept, organization)",
					},
					"max_results": {
						Type:        "integer",
						Description: "Maximum number of results to return",
						Default:     10,
						Minimum:     ptrInt(1),
						Maximum:     ptrInt(100),
					},
				},
				Required: []string{"entity"},
			},
		},
	}

	resp := protocol.ToolsListResponse{
		Tools: tools,
	}

	s.logger.Info("Returning tools list", "count", len(tools))
	return s.sendResponse(id, resp)
}

func ptrInt(i int) *int {
	return &i
}

func (s *Server) handleToolsCall(ctx context.Context, id any, params json.RawMessage) error {
	if !s.initialized {
		return s.sendError(id, protocol.ServerNotReady, "Server not initialized", nil)
	}

	var req protocol.ToolsCallRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return s.sendError(id, protocol.InvalidParams, "Invalid tool call params", nil)
	}

	s.logger.Info("Calling tool", "name", req.Name)

	// Look up handler
	handler, exists := s.toolHandlers[req.Name]
	if !exists {
		return s.sendError(id, protocol.MethodNotFound,
			fmt.Sprintf("Tool not found: %s", req.Name),
			nil,
		)
	}

	// Execute handler
	result, err := handler(ctx, req.Arguments)
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
	s.logger.Debug("Processing prompts/list request")

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

	s.logger.Debug("Processing prompts/get request", "name", req.Name)

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

	s.logger.Debug("Sending response", "raw_data", string(data))
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

	s.logger.Debug("Sending error", "raw_data", string(responseData))
	return s.transport.Write(responseData)
}

// GetIndex returns the current index (thread-safe)
func (s *Server) GetIndex() *types.GraphIndex {
	s.indexMu.RLock()
	defer s.indexMu.RUnlock()
	return s.index
}

// ReloadIndex atomically updates the server's index (thread-safe)
func (s *Server) ReloadIndex(newIndex *types.GraphIndex) {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()
	s.index = newIndex
	s.logger.Info("Index reloaded", "files", len(newIndex.Files))
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	s.logger.Info("MCP server shutting down")
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
