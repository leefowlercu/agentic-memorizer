package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/mcp/protocol"
	"github.com/leefowlercu/agentic-memorizer/internal/search"
)

// SearchFilesHandler handles the search_files tool
type SearchFilesHandler struct {
	BaseHandler
}

// NewSearchFilesHandler creates a new search files handler
func NewSearchFilesHandler(deps *Dependencies) *SearchFilesHandler {
	return &SearchFilesHandler{
		BaseHandler: NewBaseHandler(deps),
	}
}

// Name returns the tool name
func (h *SearchFilesHandler) Name() string {
	return "search_files"
}

// Execute performs semantic search across indexed files
func (h *SearchFilesHandler) Execute(ctx context.Context, args json.RawMessage) (any, error) {
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
	if h.deps.HasDaemonAPI() {
		result, err := h.executeDaemon(ctx, params)
		if err == nil {
			return result, nil
		}
		// Fall through to index-based search on error
		h.deps.Logger.Debug("daemon search failed, falling back to index", "error", err)
	}

	// Fallback to index-based search
	return h.executeIndex(params)
}

func (h *SearchFilesHandler) executeDaemon(ctx context.Context, params struct {
	Query      string   `json:"query"`
	Categories []string `json:"categories,omitempty"`
	MaxResults int      `json:"max_results,omitempty"`
}) (any, error) {
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

	respBody, err := h.deps.CallDaemonAPI(ctx, "POST", "/api/v1/search", reqBody)
	if err != nil {
		return nil, err
	}

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
	if err := json.Unmarshal(respBody, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse response; %w", err)
	}

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

func (h *SearchFilesHandler) executeIndex(params struct {
	Query      string   `json:"query"`
	Categories []string `json:"categories,omitempty"`
	MaxResults int      `json:"max_results,omitempty"`
}) (any, error) {
	index := h.deps.Index.GetIndex()
	searcher := search.NewSearcher(index)
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

// ToolDefinition returns the MCP tool definition
func (h *SearchFilesHandler) ToolDefinition() protocol.Tool {
	return protocol.Tool{
		Name:        "search_files",
		Description: "Search for files in the memory index using semantic search. Returns ranked results based on relevance to the query.",
		InputSchema: protocol.InputSchema{
			Schema: "https://json-schema.org/draft/2020-12/schema",
			Type:   "object",
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
					Minimum:     PtrInt(1),
					Maximum:     PtrInt(100),
				},
			},
			Required: []string{"query"},
		},
	}
}
