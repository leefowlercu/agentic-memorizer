package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/mcp/protocol"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

func TestServer_Run_ContextCancellation(t *testing.T) {
	// Create minimal index for server
	index := &types.Index{
		Stats: types.IndexStats{
			TotalFiles: 0,
		},
		Entries: []types.IndexEntry{},
	}

	logger := slog.Default()
	server := NewServer(index, logger)

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
	index := &types.Index{
		Stats: types.IndexStats{
			TotalFiles: 0,
		},
		Entries: []types.IndexEntry{},
	}

	logger := slog.Default()
	server := NewServer(index, logger)

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
	index := &types.Index{
		Stats: types.IndexStats{
			TotalFiles: 0,
		},
		Entries: []types.IndexEntry{},
	}

	logger := slog.Default()
	server := NewServer(index, logger)

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
	index := &types.Index{
		Stats: types.IndexStats{
			TotalFiles: 0,
		},
		Entries: []types.IndexEntry{},
	}

	logger := slog.Default()
	server := NewServer(index, logger)

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
	index := &types.Index{
		Stats: types.IndexStats{
			TotalFiles: 0,
		},
		Entries: []types.IndexEntry{},
	}

	logger := slog.Default()
	server := NewServer(index, logger)

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
	index := &types.Index{
		Stats: types.IndexStats{
			TotalFiles: 0,
		},
		Entries: []types.IndexEntry{},
	}

	logger := slog.Default()
	server := NewServer(index, logger)

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
	index := &types.Index{
		Stats: types.IndexStats{
			TotalFiles: 0,
		},
		Entries: []types.IndexEntry{},
	}

	logger := slog.Default()
	server := NewServer(index, logger)

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

	// Should not error, but response should indicate not implemented
	if err != nil {
		t.Errorf("handleMessage() error = %v, want nil", err)
	}

	// Check response contains error
	response := writeBuf.String()
	if response == "" {
		t.Error("No response written for prompts/list")
	}

	var resp protocol.JSONRPCResponse
	if err := json.Unmarshal([]byte(response), &resp); err == nil {
		if resp.Error == nil {
			t.Error("Expected error response for prompts/list (not implemented)")
		} else if resp.Error.Code != -32601 {
			t.Errorf("Expected error code -32601 (method not found), got %d", resp.Error.Code)
		}
	}
}

func TestServer_HandlePromptsGet(t *testing.T) {
	index := &types.Index{
		Stats: types.IndexStats{
			TotalFiles: 0,
		},
		Entries: []types.IndexEntry{},
	}

	logger := slog.Default()
	server := NewServer(index, logger)

	readBuf := bytes.NewBuffer(nil)
	writeBuf := bytes.NewBuffer(nil)
	server.transport = &mockTransport{
		readBuf:  readBuf,
		writeBuf: writeBuf,
	}

	server.initialized = true

	// Create prompts/get request
	req := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "prompts/get",
	}

	data, _ := json.Marshal(req)

	ctx := context.Background()
	err := server.handleMessage(ctx, data)

	// Should not error, but response should indicate not implemented
	if err != nil {
		t.Errorf("handleMessage() error = %v, want nil", err)
	}

	// Check response contains error
	response := writeBuf.String()
	if response == "" {
		t.Error("No response written for prompts/get")
	}

	var resp protocol.JSONRPCResponse
	if err := json.Unmarshal([]byte(response), &resp); err == nil {
		if resp.Error == nil {
			t.Error("Expected error response for prompts/get (not implemented)")
		} else if resp.Error.Code != -32601 {
			t.Errorf("Expected error code -32601 (method not found), got %d", resp.Error.Code)
		}
	}
}
