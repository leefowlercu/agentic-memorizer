package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/mcp/protocol"
)

// SearchEntitiesHandler handles the search_entities tool
type SearchEntitiesHandler struct {
	BaseHandler
}

// NewSearchEntitiesHandler creates a new search entities handler
func NewSearchEntitiesHandler(deps *Dependencies) *SearchEntitiesHandler {
	return &SearchEntitiesHandler{
		BaseHandler: NewBaseHandler(deps),
	}
}

// Name returns the tool name
func (h *SearchEntitiesHandler) Name() string {
	return "search_entities"
}

// Execute searches for files by entity name
func (h *SearchEntitiesHandler) Execute(ctx context.Context, args json.RawMessage) (any, error) {
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
	if !h.deps.HasDaemonAPI() {
		return nil, fmt.Errorf("daemon API not available; entity search requires daemon connection")
	}

	// Search by entity via daemon API
	path := fmt.Sprintf("/api/v1/entities/search?entity=%s&limit=%d", params.Entity, params.MaxResults)
	respBody, err := h.deps.CallDaemonAPI(ctx, "GET", path, nil)
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

// ToolDefinition returns the MCP tool definition
func (h *SearchEntitiesHandler) ToolDefinition() protocol.Tool {
	return protocol.Tool{
		Name:        "search_entities",
		Description: "Search for files that mention a specific entity (technology, person, concept, organization). Requires FalkorDB to be running.",
		InputSchema: protocol.InputSchema{
			Schema: "https://json-schema.org/draft/2020-12/schema",
			Type:   "object",
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
					Minimum:     PtrInt(1),
					Maximum:     PtrInt(100),
				},
			},
			Required: []string{"entity"},
		},
	}
}
