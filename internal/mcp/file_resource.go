package mcp

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/leefowlercu/agentic-memorizer/internal/graph"
)

// RegistryChecker provides path validation capabilities for the MCP server.
// This interface avoids direct dependency on the registry package.
type RegistryChecker interface {
	// IsPathRemembered returns true if the given file path is under a remembered directory.
	IsPathRemembered(ctx context.Context, filePath string) bool
}

// PathNotRememberedError is returned when a requested file path is not under a remembered directory.
type PathNotRememberedError struct {
	Path string
}

func (e *PathNotRememberedError) Error() string {
	return "path not remembered: " + e.Path
}

// isPathRemembered checks if the given path is under a remembered directory.
func (s *Server) isPathRemembered(ctx context.Context, path string) bool {
	if s.registry == nil {
		return false
	}
	return s.registry.IsPathRemembered(ctx, path)
}

// handleReadFileResource handles read requests for file resource URIs.
func (s *Server) handleReadFileResource(ctx context.Context, uri string) ([]mcp.ResourceContents, error) {
	// Extract path from URI (remove the memorizer://file/ prefix)
	if !strings.HasPrefix(uri, ResourceURIFilePrefix) {
		return nil, &ResourceNotFoundError{URI: uri}
	}

	path := strings.TrimPrefix(uri, ResourceURIFilePrefix)
	if path == "" {
		return nil, &ResourceNotFoundError{URI: uri}
	}

	// Validate path is remembered
	if !s.isPathRemembered(ctx, path) {
		return nil, &PathNotRememberedError{Path: path}
	}

	// Query graph for FileWithRelations
	fileData, err := s.graph.GetFileWithRelations(ctx, path)
	if err != nil {
		return nil, err
	}

	if fileData == nil {
		return nil, &ResourceNotFoundError{URI: uri}
	}

	// Serialize to JSON
	data, err := json.MarshalIndent(fileData, "", "  ")
	if err != nil {
		return nil, err
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

// FileNotFoundError is returned when a requested file does not exist in the graph.
type FileNotFoundError struct {
	Path string
}

func (e *FileNotFoundError) Error() string {
	return "file not found: " + e.Path
}

// Ensure Graph interface compatibility for type checking.
var _ interface {
	GetFileWithRelations(ctx context.Context, path string) (*graph.FileWithRelations, error)
} = (graph.Graph)(nil)
