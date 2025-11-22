package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/integrations/output"
	"github.com/leefowlercu/agentic-memorizer/internal/mcp/protocol"
	"github.com/leefowlercu/agentic-memorizer/internal/mcp/transport"
	"github.com/leefowlercu/agentic-memorizer/internal/search"
	"github.com/leefowlercu/agentic-memorizer/internal/version"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

type Server struct {
	transport    transport.Transport
	index        *types.Index
	initialized  bool
	capabilities protocol.ServerCapabilities
	logger       *slog.Logger

	// Handler registries (for Phase 2+)
	resourceHandlers map[string]ResourceHandler
	toolHandlers     map[string]ToolHandler
	promptHandlers   map[string]PromptHandler
}

type ResourceHandler func(ctx context.Context, uri string) (any, error)
type ToolHandler func(ctx context.Context, params json.RawMessage) (any, error)
type PromptHandler func(ctx context.Context, params json.RawMessage) (any, error)

func NewServer(index *types.Index, logger *slog.Logger) *Server {
	s := &Server{
		transport:        transport.NewStdioTransport(),
		index:            index,
		logger:           logger,
		resourceHandlers: make(map[string]ResourceHandler),
		toolHandlers:     make(map[string]ToolHandler),
		promptHandlers:   make(map[string]PromptHandler),
	}

	// Register tool handlers
	s.toolHandlers["search_files"] = s.handleSearchFiles
	s.toolHandlers["get_file_metadata"] = s.handleGetFileMetadata
	s.toolHandlers["list_recent_files"] = s.handleListRecentFiles

	return s
}

// Run starts the MCP server loop
func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("MCP server starting")

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

	// Validate protocol version (support both 2024-11-05 and 2025-06-18)
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
				Subscribe:   false, // Phase 5
				ListChanged: false, // Phase 5
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

// formatIndexXML formats the index as XML
func (s *Server) formatIndexXML() (string, error) {
	formatter := output.NewXMLProcessor()
	return formatter.Format(s.index)
}

// formatIndexMarkdown formats the index as Markdown
func (s *Server) formatIndexMarkdown() (string, error) {
	formatter := output.NewMarkdownProcessor()
	return formatter.Format(s.index)
}

// formatIndexJSON formats the index as JSON
func (s *Server) formatIndexJSON() (string, error) {
	formatter := output.NewJSONProcessor()
	return formatter.Format(s.index)
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

	// Perform search
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
			"path":       result.Entry.Metadata.Path,
			"name":       filepath.Base(result.Entry.Metadata.Path),
			"category":   result.Entry.Metadata.Category,
			"score":      result.Score,
			"match_type": result.MatchType,
			"size_human": formatSize(result.Entry.Metadata.Size),
			"modified":   result.Entry.Metadata.Modified.Format(time.RFC3339),
		}

		// Add semantic fields if available
		if result.Entry.Semantic != nil {
			formattedResults[i]["summary"] = result.Entry.Semantic.Summary
			formattedResults[i]["tags"] = result.Entry.Semantic.Tags
		}
	}

	return map[string]any{
		"query":        params.Query,
		"result_count": len(results),
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

	// Search for matching entry
	pathLower := strings.ToLower(params.Path)
	for _, entry := range s.index.Entries {
		entryPathLower := strings.ToLower(entry.Metadata.Path)
		// Match if path contains the query or vice versa
		if strings.Contains(entryPathLower, pathLower) || strings.Contains(pathLower, entryPathLower) {
			return entry, nil
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

	// Calculate cutoff time
	cutoff := time.Now().AddDate(0, 0, -params.Days)

	// Filter recent entries
	var recentEntries []types.IndexEntry
	for _, entry := range s.index.Entries {
		if entry.Metadata.Modified.After(cutoff) {
			recentEntries = append(recentEntries, entry)
		}
	}

	// Sort by modified time descending
	for i := 0; i < len(recentEntries)-1; i++ {
		for j := i + 1; j < len(recentEntries); j++ {
			if recentEntries[j].Metadata.Modified.After(recentEntries[i].Metadata.Modified) {
				recentEntries[i], recentEntries[j] = recentEntries[j], recentEntries[i]
			}
		}
	}

	// Limit results
	if len(recentEntries) > params.Limit {
		recentEntries = recentEntries[:params.Limit]
	}

	// Format results
	formattedResults := make([]map[string]any, len(recentEntries))
	for i, entry := range recentEntries {
		formattedResults[i] = map[string]any{
			"path":       entry.Metadata.Path,
			"name":       filepath.Base(entry.Metadata.Path),
			"category":   entry.Metadata.Category,
			"size_human": formatSize(entry.Metadata.Size),
			"modified":   entry.Metadata.Modified.Format(time.RFC3339),
		}

		// Add semantic fields if available
		if entry.Semantic != nil {
			formattedResults[i]["summary"] = entry.Semantic.Summary
			formattedResults[i]["tags"] = entry.Semantic.Tags
		}
	}

	return map[string]any{
		"days":         params.Days,
		"result_count": len(formattedResults),
		"files":        formattedResults,
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
			Description: "Get complete metadata and semantic analysis for a specific file by path. Returns all available information including summaries, tags, topics, and file-specific metadata.",
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
	}

	resp := protocol.ToolsListResponse{
		Tools: tools,
	}

	s.logger.Info("Returning tools list", "count", len(tools))
	return s.sendResponse(id, resp)
}

// ptrInt returns a pointer to an int
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
	return s.sendError(id, protocol.MethodNotFound, "prompts/list not yet implemented", nil)
}

func (s *Server) handlePromptsGet(ctx context.Context, id any, params json.RawMessage) error {
	return s.sendError(id, protocol.MethodNotFound, "prompts/get not yet implemented", nil)
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

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	s.logger.Info("MCP server shutting down")
	return s.transport.Close()
}
