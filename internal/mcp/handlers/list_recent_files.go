package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/mcp/protocol"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// ListRecentFilesHandler handles the list_recent_files tool
type ListRecentFilesHandler struct {
	BaseHandler
}

// NewListRecentFilesHandler creates a new list recent files handler
func NewListRecentFilesHandler(deps *Dependencies) *ListRecentFilesHandler {
	return &ListRecentFilesHandler{
		BaseHandler: NewBaseHandler(deps),
	}
}

// Name returns the tool name
func (h *ListRecentFilesHandler) Name() string {
	return "list_recent_files"
}

// Execute returns files modified within the specified time period
func (h *ListRecentFilesHandler) Execute(ctx context.Context, args json.RawMessage) (any, error) {
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
	if h.deps.HasDaemonAPI() {
		result, err := h.executeDaemon(ctx, params.Days, params.Limit)
		if err == nil {
			return result, nil
		}
		// Fall through to index-based query
		h.deps.Logger.Debug("daemon recent files query failed, falling back to index", "error", err)
	}

	// Fallback to index-based query
	return h.executeIndex(params.Days, params.Limit)
}

func (h *ListRecentFilesHandler) executeDaemon(ctx context.Context, days, limit int) (any, error) {
	path := fmt.Sprintf("/api/v1/files/recent?days=%d&limit=%d", days, limit)
	respBody, err := h.deps.CallDaemonAPI(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var recentResp struct {
		Files []struct {
			Path     string `json:"path"`
			Name     string `json:"name"`
			Category string `json:"category"`
			Summary  string `json:"summary"`
		} `json:"files"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal(respBody, &recentResp); err != nil {
		return nil, fmt.Errorf("failed to parse response; %w", err)
	}

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
		"days":         days,
		"result_count": recentResp.Count,
		"source":       "daemon",
		"files":        formattedResults,
	}, nil
}

func (h *ListRecentFilesHandler) executeIndex(days, limit int) (any, error) {
	index := h.deps.Index.GetIndex()
	cutoff := time.Now().AddDate(0, 0, -days)

	// Filter recent files
	var recentFiles []types.FileEntry
	for _, file := range index.Files {
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
	if len(recentFiles) > limit {
		recentFiles = recentFiles[:limit]
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
		"days":         days,
		"result_count": len(formattedResults),
		"source":       "index",
		"files":        formattedResults,
	}, nil
}

// ToolDefinition returns the MCP tool definition
func (h *ListRecentFilesHandler) ToolDefinition() protocol.Tool {
	return protocol.Tool{
		Name:        "list_recent_files",
		Description: "List recently modified files within a specified time period. Returns files sorted by modification date (newest first).",
		InputSchema: protocol.InputSchema{
			Schema: "https://json-schema.org/draft/2020-12/schema",
			Type:   "object",
			Properties: map[string]protocol.Property{
				"days": {
					Type:        "integer",
					Description: "Number of days to look back for recent files",
					Default:     7,
					Minimum:     PtrInt(1),
					Maximum:     PtrInt(365),
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of files to return",
					Default:     20,
					Minimum:     PtrInt(1),
					Maximum:     PtrInt(100),
				},
			},
		},
	}
}
