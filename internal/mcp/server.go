// Package mcp provides the Model Context Protocol server implementation.
package mcp

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/export"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/providers"
	"github.com/leefowlercu/agentic-memorizer/internal/version"
)

// Server wraps the MCP server with memorizer-specific functionality.
type Server struct {
	mcpServer  *server.MCPServer
	httpServer *server.StreamableHTTPServer
	graph      graph.Graph
	embeddings providers.EmbeddingsProvider
	registry   RegistryChecker
	bus        *events.EventBus
	exporter   *export.Exporter
	subs       *SubscriptionManager
	mu         sync.RWMutex
	running    bool

	// stopChan signals the event listener to stop
	stopChan chan struct{}
	// unsubscribe is the function to unsubscribe from the event bus
	unsubscribe func()

	errChan chan error
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
		Version:  version.Get().Version,
		BasePath: "/mcp",
	}
}

// NewServer creates a new MCP server.
func NewServer(g graph.Graph, embeddings providers.EmbeddingsProvider, reg RegistryChecker, bus *events.EventBus, cfg Config) *Server {
	s := &Server{
		graph:      g,
		embeddings: embeddings,
		registry:   reg,
		bus:        bus,
		exporter:   export.NewExporter(g),
		subs:       NewSubscriptionManager(),
		errChan:    make(chan error, 1),
	}

	// Create MCP server with resource capabilities
	s.mcpServer = server.NewMCPServer(
		cfg.Name,
		cfg.Version,
		server.WithResourceCapabilities(true, true), // subscribe and listChanged
		server.WithToolCapabilities(true),
	)

	// Register resources
	s.registerResources()
	s.registerTools()

	// Create StreamableHTTP server for HTTP transport (MCP 2025-11-25 compliant)
	s.httpServer = server.NewStreamableHTTPServer(
		s.mcpServer,
		server.WithStateful(true),
		server.WithHeartbeatInterval(30*time.Second),
		server.WithEndpointPath(cfg.BasePath),
	)

	slog.Info("MCP StreamableHTTP server created",
		"name", cfg.Name,
		"version", cfg.Version,
		"base_path", cfg.BasePath,
	)

	return s
}

// Start starts the MCP server.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Start the event listener for notifications
	s.startEventListener(ctx)

	s.running = true
	slog.Info("MCP server started")
	return nil
}

// Stop stops the MCP server.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop the event listener first
	s.stopEventListener()

	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			slog.Warn("MCP server shutdown error", "error", err)
			return err
		}
	}

	s.running = false
	slog.Info("MCP server stopped")
	return nil
}

// Errors returns fatal server errors.
func (s *Server) Errors() <-chan error {
	return s.errChan
}

// Handler returns the HTTP handler for the MCP server.
func (s *Server) Handler() http.Handler {
	return s.httpServer
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

	// File resource template (RFC 6570)
	s.mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate(
			ResourceURIFileTemplate,
			"File Resource",
			mcp.WithTemplateDescription("Access analyzed file data from the knowledge graph. Returns file metadata, tags, topics, entities, and references."),
			mcp.WithTemplateMIMEType("application/json"),
		),
		s.handleReadResource,
	)
}

// handleReadResource handles read requests for all resource URIs.
func (s *Server) handleReadResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := request.Params.URI

	// Route file resource URIs to the file handler
	if strings.HasPrefix(uri, ResourceURIFilePrefix) {
		return s.handleReadFileResource(ctx, uri)
	}

	// Determine format from URI for index resources
	var format, mimeType string

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
