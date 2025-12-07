package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	_ "github.com/leefowlercu/agentic-memorizer/internal/format/formatters" // Register formatters
	"github.com/leefowlercu/agentic-memorizer/internal/mcp/protocol"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

func TestServer_Run_ContextCancellation(t *testing.T) {
	// Create minimal index for server
	index := &types.GraphIndex{
		Stats: types.IndexStats{
			TotalFiles: 0,
		},
		Files: []types.FileEntry{},
	}

	logger := slog.Default()
	server := NewServer(index, logger, "")

	// Replace transport with mock
	readBuf := bytes.NewBuffer(nil)
	writeBuf := bytes.NewBuffer(nil)
	server.transport = &mockTransport{
		readBuf:  readBuf,
		writeBuf: writeBuf,
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	// Run should exit quickly due to cancelled context
	done := make(chan error, 1)
	go func() {
		done <- server.Run(ctx)
	}()

	select {
	case <-done:
		// Server exited - this is expected
	case <-time.After(1 * time.Second):
		t.Fatal("Run() did not exit after context cancellation")
	}
}

func TestServer_Run_ClientDisconnect(t *testing.T) {
	index := &types.GraphIndex{
		Stats: types.IndexStats{
			TotalFiles: 0,
		},
		Files: []types.FileEntry{},
	}

	logger := slog.Default()
	server := NewServer(index, logger, "")

	// Transport with empty buffer will return EOF immediately
	readBuf := bytes.NewBuffer(nil)
	writeBuf := bytes.NewBuffer(nil)
	server.transport = &mockTransport{
		readBuf:  readBuf,
		writeBuf: writeBuf,
	}

	ctx := context.Background()

	// Run should exit when client disconnects (EOF from empty buffer)
	err := server.Run(ctx)

	// Server logs client disconnect but returns nil (not EOF)
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
}

func TestServer_Shutdown(t *testing.T) {
	index := &types.GraphIndex{
		Stats: types.IndexStats{
			TotalFiles: 0,
		},
		Files: []types.FileEntry{},
	}

	logger := slog.Default()
	server := NewServer(index, logger, "")

	readBuf := bytes.NewBuffer(nil)
	writeBuf := bytes.NewBuffer(nil)
	server.transport = &mockTransport{
		readBuf:  readBuf,
		writeBuf: writeBuf,
	}

	// Shutdown should not error
	err := server.Shutdown()
	if err != nil {
		t.Errorf("Shutdown() error = %v, want nil", err)
	}
}

func TestServer_HandleMessage_NotificationWithInvalidVersion(t *testing.T) {
	index := &types.GraphIndex{
		Stats: types.IndexStats{
			TotalFiles: 0,
		},
		Files: []types.FileEntry{},
	}

	logger := slog.Default()
	server := NewServer(index, logger, "")

	readBuf := bytes.NewBuffer(nil)
	writeBuf := bytes.NewBuffer(nil)
	server.transport = &mockTransport{
		readBuf:  readBuf,
		writeBuf: writeBuf,
	}

	// Initialize server first
	server.initialized = true

	// Create notification with invalid JSON-RPC version
	notification := map[string]any{
		"jsonrpc": "1.0", // Invalid version
		"method":  "notifications/initialized",
		// No "id" field makes this a notification
	}

	data, _ := json.Marshal(notification)

	ctx := context.Background()

	// Should not error - notifications with invalid version are logged but don't return errors
	err := server.handleMessage(ctx, data)

	// Notifications don't produce responses even on errors
	if err != nil {
		t.Errorf("handleMessage() error = %v, want nil for notification", err)
	}
}

func TestServer_HandleMessage_UnknownNotification(t *testing.T) {
	index := &types.GraphIndex{
		Stats: types.IndexStats{
			TotalFiles: 0,
		},
		Files: []types.FileEntry{},
	}

	logger := slog.Default()
	server := NewServer(index, logger, "")

	readBuf := bytes.NewBuffer(nil)
	writeBuf := bytes.NewBuffer(nil)
	server.transport = &mockTransport{
		readBuf:  readBuf,
		writeBuf: writeBuf,
	}

	server.initialized = true

	// Create notification with unknown method
	notification := map[string]any{
		"jsonrpc": "2.0",
		"method":  "unknown/notification",
		// No "id" field makes this a notification
	}

	data, _ := json.Marshal(notification)

	ctx := context.Background()

	// Unknown notifications should be ignored (no error)
	err := server.handleMessage(ctx, data)

	if err != nil {
		t.Errorf("handleMessage() error = %v, want nil for unknown notification", err)
	}
}

func TestServer_HandleMessage_MalformedNotification(t *testing.T) {
	index := &types.GraphIndex{
		Stats: types.IndexStats{
			TotalFiles: 0,
		},
		Files: []types.FileEntry{},
	}

	logger := slog.Default()
	server := NewServer(index, logger, "")

	readBuf := bytes.NewBuffer(nil)
	writeBuf := bytes.NewBuffer(nil)
	server.transport = &mockTransport{
		readBuf:  readBuf,
		writeBuf: writeBuf,
	}

	// Malformed JSON is logged but doesn't return error
	malformed := []byte("{invalid json")

	ctx := context.Background()
	err := server.handleMessage(ctx, malformed)

	// handleMessage logs the error but returns nil for malformed JSON
	if err != nil {
		t.Errorf("handleMessage() error = %v, want nil (logs error instead)", err)
	}
}

func TestServer_HandlePromptsList(t *testing.T) {
	index := &types.GraphIndex{
		Stats: types.IndexStats{
			TotalFiles: 0,
		},
		Files: []types.FileEntry{},
	}

	logger := slog.Default()
	server := NewServer(index, logger, "")

	readBuf := bytes.NewBuffer(nil)
	writeBuf := bytes.NewBuffer(nil)
	server.transport = &mockTransport{
		readBuf:  readBuf,
		writeBuf: writeBuf,
	}

	server.initialized = true

	// Create prompts/list request
	req := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "prompts/list",
	}

	data, _ := json.Marshal(req)

	ctx := context.Background()
	err := server.handleMessage(ctx, data)

	// Should not error
	if err != nil {
		t.Errorf("handleMessage() error = %v, want nil", err)
	}

	// Check response contains prompts list
	response := writeBuf.String()
	if response == "" {
		t.Fatal("No response written for prompts/list")
	}

	var resp protocol.JSONRPCResponse
	if err := json.Unmarshal([]byte(response), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("Unexpected error in response: %v", resp.Error)
	}

	// Unmarshal result
	var listResp protocol.PromptsListResponse
	if err := json.Unmarshal(resp.Result, &listResp); err != nil {
		t.Fatalf("Failed to unmarshal prompts list result: %v", err)
	}

	// Verify we have the 3 default prompts
	if len(listResp.Prompts) != 3 {
		t.Errorf("Expected 3 prompts, got %d", len(listResp.Prompts))
	}

	// Verify prompt names
	expectedPrompts := map[string]bool{
		"analyze-file":    false,
		"search-context":  false,
		"explain-summary": false,
	}
	for _, prompt := range listResp.Prompts {
		if _, ok := expectedPrompts[prompt.Name]; ok {
			expectedPrompts[prompt.Name] = true
		}
	}
	for name, found := range expectedPrompts {
		if !found {
			t.Errorf("Expected prompt %q not found in list", name)
		}
	}
}

func TestServer_HandlePromptsGet(t *testing.T) {
	index := &types.GraphIndex{
		Stats: types.IndexStats{
			TotalFiles: 0,
		},
		Files: []types.FileEntry{},
	}

	logger := slog.Default()
	server := NewServer(index, logger, "")

	readBuf := bytes.NewBuffer(nil)
	writeBuf := bytes.NewBuffer(nil)
	server.transport = &mockTransport{
		readBuf:  readBuf,
		writeBuf: writeBuf,
	}

	server.initialized = true

	// Create prompts/get request for search-context prompt
	params := protocol.PromptsGetRequest{
		Name: "search-context",
		Arguments: map[string]string{
			"topic":    "authentication",
			"category": "code",
		},
	}
	paramsData, _ := json.Marshal(params)

	req := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "prompts/get",
		Params:  paramsData,
	}

	data, _ := json.Marshal(req)

	ctx := context.Background()
	err := server.handleMessage(ctx, data)

	// Should not error
	if err != nil {
		t.Errorf("handleMessage() error = %v, want nil", err)
	}

	// Check response contains prompt messages
	response := writeBuf.String()
	if response == "" {
		t.Fatal("No response written for prompts/get")
	}

	var resp protocol.JSONRPCResponse
	if err := json.Unmarshal([]byte(response), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("Unexpected error in response: %v", resp.Error)
	}

	// Unmarshal result
	var getResp protocol.PromptsGetResponse
	if err := json.Unmarshal(resp.Result, &getResp); err != nil {
		t.Fatalf("Failed to unmarshal prompts/get result: %v", err)
	}

	// Verify description is present
	if getResp.Description == "" {
		t.Error("Expected non-empty description in response")
	}

	// Verify we have at least one message
	if len(getResp.Messages) == 0 {
		t.Error("Expected at least one message in response")
	}

	// Verify message structure
	for i, msg := range getResp.Messages {
		if msg.Role == "" {
			t.Errorf("Message %d has empty role", i)
		}
		if msg.Content.Type == "" {
			t.Errorf("Message %d has empty content type", i)
		}
		if msg.Content.Text == "" {
			t.Errorf("Message %d has empty text content", i)
		}
	}
}
