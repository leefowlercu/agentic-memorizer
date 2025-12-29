package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/leefowlercu/agentic-memorizer/internal/mcp/protocol"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// Handler defines the interface for MCP tool handlers
type Handler interface {
	// Name returns the tool name
	Name() string

	// Execute handles the tool call
	Execute(ctx context.Context, args json.RawMessage) (any, error)

	// ToolDefinition returns the MCP tool definition for tools/list
	ToolDefinition() protocol.Tool
}

// IndexProvider provides access to the file index
type IndexProvider interface {
	GetIndex() *types.FileIndex
}

// Dependencies contains shared dependencies for all handlers
type Dependencies struct {
	DaemonURL  string
	HTTPClient *http.Client
	Index      IndexProvider
	Logger     *slog.Logger
}

// HasDaemonAPI returns true if the daemon API is configured
func (d *Dependencies) HasDaemonAPI() bool {
	return d.DaemonURL != ""
}

// CallDaemonAPI makes an HTTP request to the daemon API
func (d *Dependencies) CallDaemonAPI(ctx context.Context, method, path string, body any) ([]byte, error) {
	if d.DaemonURL == "" {
		return nil, fmt.Errorf("daemon URL not configured")
	}

	url := d.DaemonURL + path

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request; %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request; %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := d.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed; %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response; %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr struct {
			Error   string `json:"error"`
			Details string `json:"details"`
		}
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error != "" {
			return nil, fmt.Errorf("%s: %s", apiErr.Error, apiErr.Details)
		}
		return nil, fmt.Errorf("API error: %s", resp.Status)
	}

	return respBody, nil
}

// BaseHandler provides common functionality for handlers
type BaseHandler struct {
	deps *Dependencies
}

// NewBaseHandler creates a new base handler with dependencies
func NewBaseHandler(deps *Dependencies) BaseHandler {
	return BaseHandler{deps: deps}
}

// Deps returns the handler dependencies
func (h *BaseHandler) Deps() *Dependencies {
	return h.deps
}

// PtrInt returns a pointer to an int (helper for schema definitions)
func PtrInt(i int) *int {
	return &i
}
