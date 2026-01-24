package daemonclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/daemon"
)

const (
	DefaultTimeout = 5 * time.Second
	RebuildTimeout = 5 * time.Minute
	RewalkTimeout  = 30 * time.Second
	ReadTimeout    = 5 * time.Minute
)

// Client provides a shared HTTP client for daemon endpoints.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if timeout > 0 {
			c.httpClient.Timeout = timeout
		}
	}
}

// New creates a Client using daemon configuration.
func New(cfg config.DaemonConfig, opts ...Option) *Client {
	client := &Client{
		baseURL: ResolveBaseURL(cfg),
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// NewFromConfig creates a Client from the root config.
func NewFromConfig(cfg *config.Config, opts ...Option) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config not initialized")
	}
	return New(cfg.Daemon, opts...), nil
}

// ResolveBaseURL builds the daemon base URL from config.
func ResolveBaseURL(cfg config.DaemonConfig) string {
	bind := NormalizeBind(cfg.HTTPBind)
	return fmt.Sprintf("http://%s:%d", bind, cfg.HTTPPort)
}

// NormalizeBind maps wildcard binds to loopback for local clients.
func NormalizeBind(bind string) string {
	if bind == "" || bind == "0.0.0.0" {
		return "127.0.0.1"
	}
	if strings.Contains(bind, ":") && !strings.HasPrefix(bind, "[") {
		return "[" + bind + "]"
	}
	return bind
}

// Ready fetches /readyz health status.
func (c *Client) Ready(ctx context.Context) (*daemon.HealthStatus, error) {
	var status daemon.HealthStatus
	if err := c.doJSON(ctx, http.MethodGet, "/readyz", nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// Rebuild triggers /rebuild with optional full flag.
func (c *Client) Rebuild(ctx context.Context, full bool) (*daemon.RebuildResult, error) {
	path := "/rebuild"
	if full {
		path += "?full=true"
	}

	var result daemon.RebuildResult
	if err := c.doJSON(ctx, http.MethodPost, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Remember registers a path with the daemon.
func (c *Client) Remember(ctx context.Context, req daemon.RememberRequest) (*daemon.RememberResponse, error) {
	var result daemon.RememberResponse
	if err := c.doJSON(ctx, http.MethodPost, "/remember", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Forget removes a remembered path from the daemon.
func (c *Client) Forget(ctx context.Context, req daemon.ForgetRequest) (*daemon.ForgetResponse, error) {
	var result daemon.ForgetResponse
	if err := c.doJSON(ctx, http.MethodPost, "/forget", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// List fetches remembered paths from the daemon.
func (c *Client) List(ctx context.Context) (*daemon.ListResponse, error) {
	var result daemon.ListResponse
	if err := c.doJSON(ctx, http.MethodGet, "/list", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Read exports the knowledge graph via the daemon.
func (c *Client) Read(ctx context.Context, req daemon.ReadRequest) (*daemon.ReadResponse, error) {
	var result daemon.ReadResponse
	if err := c.doJSON(ctx, http.MethodPost, "/read", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		buf := &bytes.Buffer{}
		if err := json.NewEncoder(buf).Encode(in); err != nil {
			return fmt.Errorf("failed to encode request; %w", err)
		}
		body = buf
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("failed to create request; %w", err)
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon; %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp errorResponse
		if decodeErr := json.NewDecoder(resp.Body).Decode(&errResp); decodeErr == nil && errResp.Error != "" {
			return fmt.Errorf("daemon request failed; %s", errResp.Error)
		}
		return fmt.Errorf("daemon request failed; status %d", resp.StatusCode)
	}

	if out == nil {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to parse response; %w", err)
	}

	return nil
}

type errorResponse struct {
	Error string `json:"error"`
}
