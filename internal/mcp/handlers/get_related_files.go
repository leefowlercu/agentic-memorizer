package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/mcp/protocol"
)

// GetRelatedFilesHandler handles the get_related_files tool
type GetRelatedFilesHandler struct {
	BaseHandler
}

// NewGetRelatedFilesHandler creates a new get related files handler
func NewGetRelatedFilesHandler(deps *Dependencies) *GetRelatedFilesHandler {
	return &GetRelatedFilesHandler{
		BaseHandler: NewBaseHandler(deps),
	}
}

// Name returns the tool name
func (h *GetRelatedFilesHandler) Name() string {
	return "get_related_files"
}

// Execute finds files related to a given file through graph relationships
func (h *GetRelatedFilesHandler) Execute(ctx context.Context, args json.RawMessage) (any, error) {
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
	if !h.deps.HasDaemonAPI() {
		return nil, fmt.Errorf("daemon API not available; related files search requires daemon connection")
	}

	// Related files are now included in the file metadata endpoint
	// GET /api/v1/files/{path}?related_limit=N
	encodedPath := strings.ReplaceAll(params.Path, "/", "%2F")
	apiPath := fmt.Sprintf("/api/v1/files/%s?related_limit=%d", encodedPath, params.Limit)
	respBody, err := h.deps.CallDaemonAPI(ctx, "GET", apiPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get file metadata; %w", err)
	}

	var fileResp struct {
		File struct {
			RelatedFiles []struct {
				Path           string  `json:"path"`
				Name           string  `json:"name"`
				Summary        string  `json:"summary"`
				Strength       float64 `json:"strength"`
				ConnectionType string  `json:"connection_type"`
			} `json:"related_files"`
		} `json:"file"`
	}
	if err := json.Unmarshal(respBody, &fileResp); err != nil {
		return nil, fmt.Errorf("failed to parse response; %w", err)
	}

	// Format results
	relatedFiles := fileResp.File.RelatedFiles
	formattedResults := make([]map[string]any, len(relatedFiles))
	for i, rel := range relatedFiles {
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

// ToolDefinition returns the MCP tool definition
func (h *GetRelatedFilesHandler) ToolDefinition() protocol.Tool {
	return protocol.Tool{
		Name:        "get_related_files",
		Description: "Find files related to a given file through shared tags, topics, or entities in the knowledge graph. Requires FalkorDB to be running.",
		InputSchema: protocol.InputSchema{
			Schema: "https://json-schema.org/draft/2020-12/schema",
			Type:   "object",
			Properties: map[string]protocol.Property{
				"path": {
					Type:        "string",
					Description: "Path to the source file to find related files for",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of related files to return",
					Default:     10,
					Minimum:     PtrInt(1),
					Maximum:     PtrInt(50),
				},
			},
			Required: []string{"path"},
		},
	}
}
