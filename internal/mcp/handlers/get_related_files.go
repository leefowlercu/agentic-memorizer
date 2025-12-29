package handlers

import (
	"context"
	"encoding/json"
	"fmt"

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

	// Get related files from daemon API
	path := fmt.Sprintf("/api/v1/files/related?path=%s&limit=%d", params.Path, params.Limit)
	respBody, err := h.deps.CallDaemonAPI(ctx, "GET", path, nil)
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
