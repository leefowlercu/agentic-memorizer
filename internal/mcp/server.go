// Package mcp provides the Model Context Protocol server implementation.
package mcp

import (
	"context"
	"net/http"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/leefowlercu/agentic-memorizer/internal/export"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
)

// Server wraps the MCP server with memorizer-specific functionality.
type Server struct {
	mcpServer *server.MCPServer
	sseServer *server.SSEServer
	graph     graph.Graph
	exporter  *export.Exporter
	subs      *SubscriptionManager
	mu        sync.RWMutex
	running   bool
}

// Config contains MCP server configuration.
type Config struct {
	// Name is the server name advertised to clients.
	Name string
	// Version is the server version.
	Version string
	// BasePath is the URL base path for MCP endpoints.
	BasePath string
}

// DefaultConfig returns default MCP server configuration.
func DefaultConfig() Config {
	return Config{
		Name:     "memorizer",
		Version:  "1.0.0",
		BasePath: "/mcp",
	}
}

// NewServer creates a new MCP server.
func NewServer(g graph.Graph, cfg Config) *Server {
	s := &Server{
		graph:    g,
		exporter: export.NewExporter(g),
		subs:     NewSubscriptionManager(),
	}

	// Create MCP server with resource capabilities
	s.mcpServer = server.NewMCPServer(
		cfg.Name,
		cfg.Version,
		server.WithResourceCapabilities(true, true), // subscribe and listChanged
	)

	// Register resources
	s.registerResources()

	// Create SSE server for HTTP transport
	s.sseServer = server.NewSSEServer(
		s.mcpServer,
		server.WithStaticBasePath(cfg.BasePath),
		server.WithKeepAlive(true),
	)

	return s
}

// Start starts the MCP server.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.running = true
	return nil
}

// Stop stops the MCP server.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sseServer != nil {
		if err := s.sseServer.Shutdown(ctx); err != nil {
			return err
		}
	}

	s.running = false
	return nil
}

// Handler returns the HTTP handler for the MCP server.
func (s *Server) Handler() http.Handler {
	return s.sseServer
}

// SSEHandler returns the SSE endpoint handler.
func (s *Server) SSEHandler() http.Handler {
	return s.sseServer.SSEHandler()
}

// MessageHandler returns the JSON-RPC message handler.
func (s *Server) MessageHandler() http.Handler {
	return s.sseServer.MessageHandler()
}

// NotifyResourceChanged notifies all subscribers that a resource has changed.
func (s *Server) NotifyResourceChanged(uri string) {
	s.mcpServer.SendNotificationToAllClients("notifications/resources/updated", map[string]any{
		"uri": uri,
	})
}

// registerResources registers all memorizer resources with the MCP server.
func (s *Server) registerResources() {
	// Default index (XML format)
	s.mcpServer.AddResource(
		mcp.NewResource(
			ResourceURIIndex,
			"Knowledge Graph Index",
			mcp.WithResourceDescription("The complete memorizer knowledge graph index in XML format"),
			mcp.WithMIMEType("application/xml"),
		),
		s.handleReadResource,
	)

	// XML format
	s.mcpServer.AddResource(
		mcp.NewResource(
			ResourceURIIndexXML,
			"Knowledge Graph Index (XML)",
			mcp.WithResourceDescription("The memorizer knowledge graph index in XML format"),
			mcp.WithMIMEType("application/xml"),
		),
		s.handleReadResource,
	)

	// JSON format
	s.mcpServer.AddResource(
		mcp.NewResource(
			ResourceURIIndexJSON,
			"Knowledge Graph Index (JSON)",
			mcp.WithResourceDescription("The memorizer knowledge graph index in JSON format"),
			mcp.WithMIMEType("application/json"),
		),
		s.handleReadResource,
	)

	// TOON format
	s.mcpServer.AddResource(
		mcp.NewResource(
			ResourceURIIndexTOON,
			"Knowledge Graph Index (TOON)",
			mcp.WithResourceDescription("The memorizer knowledge graph index in token-optimized notation"),
			mcp.WithMIMEType("text/plain"),
		),
		s.handleReadResource,
	)
}

// handleReadResource handles read requests for all resource URIs.
func (s *Server) handleReadResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := request.Params.URI

	// Determine format from URI
	format := "xml"
	mimeType := "application/xml"

	switch uri {
	case ResourceURIIndex, ResourceURIIndexXML:
		format = "xml"
		mimeType = "application/xml"
	case ResourceURIIndexJSON:
		format = "json"
		mimeType = "application/json"
	case ResourceURIIndexTOON:
		format = "toon"
		mimeType = "text/plain"
	default:
		return nil, &ResourceNotFoundError{URI: uri}
	}

	// Export the graph in the requested format
	opts := export.ExportOptions{
		Format:   format,
		Envelope: "none",
	}

	output, _, err := s.exporter.Export(ctx, opts)
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: mimeType,
			Text:     string(output),
		},
	}, nil
}

// ResourceNotFoundError is returned when a requested resource doesn't exist.
type ResourceNotFoundError struct {
	URI string
}

func (e *ResourceNotFoundError) Error() string {
	return "resource not found: " + e.URI
}
