package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/mcp/protocol"
)

// GetFileMetadataHandler handles the get_file_metadata tool
type GetFileMetadataHandler struct {
	BaseHandler
}

// NewGetFileMetadataHandler creates a new get file metadata handler
func NewGetFileMetadataHandler(deps *Dependencies) *GetFileMetadataHandler {
	return &GetFileMetadataHandler{
		BaseHandler: NewBaseHandler(deps),
	}
}

// Name returns the tool name
func (h *GetFileMetadataHandler) Name() string {
	return "get_file_metadata"
}

// Execute returns complete metadata for a specific file
func (h *GetFileMetadataHandler) Execute(ctx context.Context, args json.RawMessage) (any, error) {
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
	if h.deps.HasDaemonAPI() {
		result, err := h.executeDaemon(ctx, params.Path)
		if err == nil {
			return result, nil
		}
		// Fall through to index-based lookup
		h.deps.Logger.Debug("daemon file lookup failed, falling back to index", "error", err)
	}

	// Fallback to index-based search
	return h.executeIndex(params.Path)
}

func (h *GetFileMetadataHandler) executeDaemon(ctx context.Context, path string) (any, error) {
	// URL-encode the path for the API call
	encodedPath := strings.ReplaceAll(path, "/", "%2F")
	respBody, err := h.deps.CallDaemonAPI(ctx, "GET", "/api/v1/files/"+encodedPath, nil)
	if err != nil {
		return nil, err
	}

	var fileResp struct {
		File json.RawMessage `json:"file"`
	}
	if err := json.Unmarshal(respBody, &fileResp); err != nil {
		return nil, fmt.Errorf("failed to parse response; %w", err)
	}

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

func (h *GetFileMetadataHandler) executeIndex(path string) (any, error) {
	index := h.deps.Index.GetIndex()
	pathLower := strings.ToLower(path)

	for _, file := range index.Files {
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

	return nil, fmt.Errorf("file not found: %s", path)
}

// ToolDefinition returns the MCP tool definition
func (h *GetFileMetadataHandler) ToolDefinition() protocol.Tool {
	return protocol.Tool{
		Name:        "get_file_metadata",
		Description: "Get complete metadata and semantic analysis for a specific file by path. Returns all available information including summaries, tags, topics, entities, related files, and file-specific metadata in a unified FileEntry format.",
		InputSchema: protocol.InputSchema{
			Schema: "https://json-schema.org/draft/2020-12/schema",
			Type:   "object",
			Properties: map[string]protocol.Property{
				"path": {
					Type:        "string",
					Description: "File path (absolute or relative) to retrieve metadata for",
				},
			},
			Required: []string{"path"},
		},
	}
}
