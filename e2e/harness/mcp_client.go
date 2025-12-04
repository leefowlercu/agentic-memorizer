package harness

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/leefowlercu/agentic-memorizer/internal/mcp/protocol"
)

// MCPClient simulates an MCP client for testing the MCP server
type MCPClient struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    *bufio.Reader
	mu        sync.Mutex
	requestID int
}

// NewMCPClient creates a new MCP client that communicates with the server via stdio
func NewMCPClient(binaryPath, appDir string) (*MCPClient, error) {
	cmd := exec.Command(binaryPath, "mcp", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+appDir)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe; %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe; %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start MCP server; %w", err)
	}

	return &MCPClient{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
	}, nil
}

// Initialize sends the initialize request
func (c *MCPClient) Initialize() (*protocol.InitializeResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.requestID++
	reqID := c.requestID

	req := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  "initialize",
		Params: mustMarshal(protocol.InitializeRequest{
			ProtocolVersion: "2024-11-05",
			Capabilities:    protocol.ClientCapabilities{},
			ClientInfo: protocol.ClientInfo{
				Name:    "e2e-test-client",
				Version: "1.0.0",
			},
		}),
	}

	if err := c.sendRequest(req); err != nil {
		return nil, err
	}

	var resp protocol.JSONRPCResponse
	if err := c.readResponse(&resp); err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("initialize error; code=%d; message=%s", resp.Error.Code, resp.Error.Message)
	}

	var initResp protocol.InitializeResponse
	if err := json.Unmarshal(resp.Result, &initResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal initialize response; %w", err)
	}

	// Send initialized notification
	notification := protocol.JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  "initialized",
		Params:  json.RawMessage(`{}`),
	}

	if err := c.sendNotification(notification); err != nil {
		return nil, err
	}

	return &initResp, nil
}

// ListResources requests the list of available resources
func (c *MCPClient) ListResources() ([]protocol.Resource, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.requestID++
	reqID := c.requestID

	req := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  "resources/list",
		Params:  json.RawMessage(`{}`),
	}

	if err := c.sendRequest(req); err != nil {
		return nil, err
	}

	var resp protocol.JSONRPCResponse
	if err := c.readResponse(&resp); err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("resources/list error; code=%d; message=%s", resp.Error.Code, resp.Error.Message)
	}

	var listResp protocol.ResourcesListResponse
	if err := json.Unmarshal(resp.Result, &listResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resources/list response; %w", err)
	}

	return listResp.Resources, nil
}

// ReadResource reads a specific resource by URI
func (c *MCPClient) ReadResource(uri string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.requestID++
	reqID := c.requestID

	req := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  "resources/read",
		Params: mustMarshal(protocol.ResourcesReadRequest{
			URI: uri,
		}),
	}

	if err := c.sendRequest(req); err != nil {
		return "", err
	}

	var resp protocol.JSONRPCResponse
	if err := c.readResponse(&resp); err != nil {
		return "", err
	}

	if resp.Error != nil {
		return "", fmt.Errorf("resources/read error; code=%d; message=%s", resp.Error.Code, resp.Error.Message)
	}

	var readResp protocol.ResourcesReadResponse
	if err := json.Unmarshal(resp.Result, &readResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal resources/read response; %w", err)
	}

	if len(readResp.Contents) == 0 {
		return "", fmt.Errorf("no content returned")
	}

	return readResp.Contents[0].Text, nil
}

// ListTools requests the list of available tools
func (c *MCPClient) ListTools() ([]protocol.Tool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.requestID++
	reqID := c.requestID

	req := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  "tools/list",
		Params:  json.RawMessage(`{}`),
	}

	if err := c.sendRequest(req); err != nil {
		return nil, err
	}

	var resp protocol.JSONRPCResponse
	if err := c.readResponse(&resp); err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("tools/list error; code=%d; message=%s", resp.Error.Code, resp.Error.Message)
	}

	var listResp protocol.ToolsListResponse
	if err := json.Unmarshal(resp.Result, &listResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools/list response; %w", err)
	}

	return listResp.Tools, nil
}

// CallTool calls a tool with the given arguments
func (c *MCPClient) CallTool(name string, args any) (any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.requestID++
	reqID := c.requestID

	req := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  "tools/call",
		Params: mustMarshal(protocol.ToolsCallRequest{
			Name:      name,
			Arguments: mustMarshal(args),
		}),
	}

	if err := c.sendRequest(req); err != nil {
		return nil, err
	}

	var resp protocol.JSONRPCResponse
	if err := c.readResponse(&resp); err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("tools/call error; code=%d; message=%s", resp.Error.Code, resp.Error.Message)
	}

	var callResp protocol.ToolsCallResponse
	if err := json.Unmarshal(resp.Result, &callResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools/call response; %w", err)
	}

	if callResp.IsError {
		return nil, fmt.Errorf("tool returned error; %s", callResp.Content[0].Text)
	}

	// Parse the result JSON
	var result any
	if err := json.Unmarshal([]byte(callResp.Content[0].Text), &result); err != nil {
		// Return raw text if not JSON
		return callResp.Content[0].Text, nil
	}

	return result, nil
}

// Shutdown sends the shutdown request
func (c *MCPClient) Shutdown() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.requestID++
	reqID := c.requestID

	req := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  "shutdown",
		Params:  json.RawMessage(`{}`),
	}

	if err := c.sendRequest(req); err != nil {
		return err
	}

	// Don't wait for response, just send exit notification
	notification := protocol.JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  "exit",
		Params:  json.RawMessage(`{}`),
	}

	return c.sendNotification(notification)
}

// Close closes the MCP client connection
func (c *MCPClient) Close() error {
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_ = c.cmd.Wait()
	}
	return nil
}

// sendRequest sends a JSON-RPC request
func (c *MCPClient) sendRequest(req protocol.JSONRPCRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request; %w", err)
	}

	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write request; %w", err)
	}

	return nil
}

// sendNotification sends a JSON-RPC notification
func (c *MCPClient) sendNotification(notif protocol.JSONRPCNotification) error {
	data, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("failed to marshal notification; %w", err)
	}

	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write notification; %w", err)
	}

	return nil
}

// readResponse reads a JSON-RPC response
func (c *MCPClient) readResponse(resp *protocol.JSONRPCResponse) error {
	line, err := c.stdout.ReadBytes('\n')
	if err != nil {
		return fmt.Errorf("failed to read response; %w", err)
	}

	if err := json.Unmarshal(line, resp); err != nil {
		return fmt.Errorf("failed to unmarshal response; %w", err)
	}

	return nil
}

// mustMarshal marshals data to JSON or panics
func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal: %v", err))
	}
	return data
}
