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

// mockTransport implements a simple in-memory transport for testing
type mockTransport struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
}

func (m *mockTransport) Read() ([]byte, error) {
	return m.readBuf.ReadBytes('\n')
}

func (m *mockTransport) Write(data []byte) error {
	_, err := m.writeBuf.Write(data)
	if err != nil {
		return err
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		_, err = m.writeBuf.Write([]byte{'\n'})
	}
	return err
}

func (m *mockTransport) Close() error {
	return nil
}

func TestServer_Initialize(t *testing.T) {
	tests := []struct {
		name             string
		request          protocol.JSONRPCRequest
		wantError        bool
		wantErrorCode    int
		checkInitialized bool
	}{
		{
			name: "successful initialization with 2024-11-05",
			request: protocol.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "initialize",
				Params: mustMarshal(protocol.InitializeRequest{
					ProtocolVersion: "2024-11-05",
					Capabilities:    protocol.ClientCapabilities{},
					ClientInfo: protocol.ClientInfo{
						Name:    "test-client",
						Version: "1.0.0",
					},
				}),
			},
			wantError:        false,
			checkInitialized: false, // Not initialized until "initialized" notification
		},
		{
			name: "successful initialization with 2025-06-18",
			request: protocol.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "initialize",
				Params: mustMarshal(protocol.InitializeRequest{
					ProtocolVersion: "2025-06-18",
					Capabilities:    protocol.ClientCapabilities{},
					ClientInfo: protocol.ClientInfo{
						Name:    "test-client",
						Version: "1.0.0",
					},
				}),
			},
			wantError:        false,
			checkInitialized: false, // Not initialized until "initialized" notification
		},
		{
			name: "unsupported protocol version",
			request: protocol.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "initialize",
				Params: mustMarshal(protocol.InitializeRequest{
					ProtocolVersion: "1.0.0",
					Capabilities:    protocol.ClientCapabilities{},
					ClientInfo: protocol.ClientInfo{
						Name:    "test-client",
						Version: "1.0.0",
					},
				}),
			},
			wantError:     true,
			wantErrorCode: protocol.InvalidRequest,
		},
		{
			name: "empty protocol version",
			request: protocol.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "initialize",
				Params: mustMarshal(protocol.InitializeRequest{
					ProtocolVersion: "",
					Capabilities:    protocol.ClientCapabilities{},
					ClientInfo: protocol.ClientInfo{
						Name:    "test-client",
						Version: "1.0.0",
					},
				}),
			},
			wantError:     true,
			wantErrorCode: protocol.InvalidRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index := &types.Index{
				Generated: time.Now(),
				Root:      "/test",
				Entries:   []types.IndexEntry{},
				Stats:     types.IndexStats{},
			}

			logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
			server := NewServer(index, logger)

			// Replace transport with mock
			readBuf := bytes.NewBuffer(nil)
			writeBuf := bytes.NewBuffer(nil)
			server.transport = &mockTransport{
				readBuf:  readBuf,
				writeBuf: writeBuf,
			}

			// Write request to mock transport
			reqData, _ := json.Marshal(tt.request)
			readBuf.Write(reqData)
			readBuf.WriteByte('\n')

			// Handle the message
			ctx := context.Background()
			err := server.handleMessage(ctx, reqData)

			if tt.wantError {
				// Check that error response was written
				var resp protocol.JSONRPCResponse
				if err := json.Unmarshal(writeBuf.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to unmarshal error response: %v", err)
				}

				if resp.Error == nil {
					t.Fatal("Expected error response, got success")
				}

				if resp.Error.Code != tt.wantErrorCode {
					t.Errorf("Error code = %d, want %d", resp.Error.Code, tt.wantErrorCode)
				}
			} else {
				if err != nil {
					t.Fatalf("handleMessage() error = %v, want nil", err)
				}

				// Parse response
				var resp protocol.JSONRPCResponse
				if err := json.Unmarshal(writeBuf.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				if resp.Error != nil {
					t.Fatalf("Got error response: %s", resp.Error.Message)
				}

				// Parse initialize response
				var initResp protocol.InitializeResponse
				if err := json.Unmarshal(resp.Result, &initResp); err != nil {
					t.Fatalf("Failed to unmarshal initialize response: %v", err)
				}

				// Parse request to get expected protocol version
				var initReq protocol.InitializeRequest
				if err := json.Unmarshal(tt.request.Params, &initReq); err != nil {
					t.Fatalf("Failed to unmarshal initialize request: %v", err)
				}

				// Verify response echoes back client's protocol version
				if initResp.ProtocolVersion != initReq.ProtocolVersion {
					t.Errorf("ProtocolVersion = %s, want %s", initResp.ProtocolVersion, initReq.ProtocolVersion)
				}

				if initResp.ServerInfo.Name != "agentic-memorizer" {
					t.Errorf("ServerInfo.Name = %s, want agentic-memorizer", initResp.ServerInfo.Name)
				}

				if initResp.Capabilities.Resources == nil {
					t.Error("Expected Resources capability to be present")
				}

				if initResp.Capabilities.Tools == nil {
					t.Error("Expected Tools capability to be present")
				}

				if initResp.Capabilities.Prompts == nil {
					t.Error("Expected Prompts capability to be present")
				}

				// Check initialized flag
				if tt.checkInitialized && !server.initialized {
					t.Error("Server should be initialized after initialize request")
				} else if !tt.checkInitialized && server.initialized {
					t.Error("Server should not be initialized until initialized notification")
				}
			}
		})
	}
}

func TestServer_Initialized(t *testing.T) {
	index := &types.Index{
		Generated: time.Now(),
		Root:      "/test",
		Entries:   []types.IndexEntry{},
		Stats:     types.IndexStats{},
	}

	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	server := NewServer(index, logger)

	// Replace transport with mock
	readBuf := bytes.NewBuffer(nil)
	writeBuf := bytes.NewBuffer(nil)
	server.transport = &mockTransport{
		readBuf:  readBuf,
		writeBuf: writeBuf,
	}

	// Send initialized notification
	notification := protocol.JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  "initialized",
		Params:  json.RawMessage(`{}`),
	}

	ctx := context.Background()

	if !server.initialized {
		// Server should not be initialized before notification
		if err := server.handleInitialized(ctx, notification.Params); err != nil {
			t.Fatalf("handleInitialized() error = %v", err)
		}

		// Server should be initialized after notification
		if !server.initialized {
			t.Error("Server should be initialized after initialized notification")
		}

		// No response should be written for notifications
		if writeBuf.Len() > 0 {
			t.Error("No response should be written for notifications")
		}
	}
}

func TestServer_MethodNotFound(t *testing.T) {
	index := &types.Index{
		Generated: time.Now(),
		Root:      "/test",
		Entries:   []types.IndexEntry{},
		Stats:     types.IndexStats{},
	}

	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	server := NewServer(index, logger)

	// Replace transport with mock
	readBuf := bytes.NewBuffer(nil)
	writeBuf := bytes.NewBuffer(nil)
	server.transport = &mockTransport{
		readBuf:  readBuf,
		writeBuf: writeBuf,
	}

	// Send unknown method
	request := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "unknown/method",
		Params:  json.RawMessage(`{}`),
	}

	reqData, _ := json.Marshal(request)
	ctx := context.Background()

	if err := server.handleMessage(ctx, reqData); err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	// Parse response
	var resp protocol.JSONRPCResponse
	if err := json.Unmarshal(writeBuf.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error response for unknown method")
	}

	if resp.Error.Code != protocol.MethodNotFound {
		t.Errorf("Error code = %d, want %d", resp.Error.Code, protocol.MethodNotFound)
	}
}

func TestServer_ResourcesList(t *testing.T) {
	tests := []struct {
		name          string
		initialized   bool
		wantError     bool
		wantErrorCode int
		wantCount     int
	}{
		{
			name:        "successful list after initialization",
			initialized: true,
			wantError:   false,
			wantCount:   3, // XML, Markdown, JSON formats
		},
		{
			name:          "list before initialization",
			initialized:   false,
			wantError:     true,
			wantErrorCode: protocol.ServerNotReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index := &types.Index{
				Generated: time.Now(),
				Root:      "/test",
				Entries:   []types.IndexEntry{},
				Stats:     types.IndexStats{},
			}

			logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
			server := NewServer(index, logger)
			server.initialized = tt.initialized

			// Replace transport with mock
			writeBuf := bytes.NewBuffer(nil)
			server.transport = &mockTransport{
				readBuf:  bytes.NewBuffer(nil),
				writeBuf: writeBuf,
			}

			// Send resources/list request
			request := protocol.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "resources/list",
				Params:  json.RawMessage(`{}`),
			}

			reqData, _ := json.Marshal(request)
			ctx := context.Background()

			if err := server.handleMessage(ctx, reqData); err != nil {
				t.Fatalf("handleMessage() error = %v", err)
			}

			// Parse response
			var resp protocol.JSONRPCResponse
			if err := json.Unmarshal(writeBuf.Bytes(), &resp); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			if tt.wantError {
				if resp.Error == nil {
					t.Fatal("Expected error response, got success")
				}
				if resp.Error.Code != tt.wantErrorCode {
					t.Errorf("Error code = %d, want %d", resp.Error.Code, tt.wantErrorCode)
				}
			} else {
				if resp.Error != nil {
					t.Fatalf("Got error response: %s", resp.Error.Message)
				}

				// Parse resources list response
				var listResp protocol.ResourcesListResponse
				if err := json.Unmarshal(resp.Result, &listResp); err != nil {
					t.Fatalf("Failed to unmarshal resources list response: %v", err)
				}

				if len(listResp.Resources) != tt.wantCount {
					t.Errorf("Resource count = %d, want %d", len(listResp.Resources), tt.wantCount)
				}

				// Verify all three formats are present
				uris := make(map[string]bool)
				for _, r := range listResp.Resources {
					uris[r.URI] = true
				}

				expectedURIs := []string{
					"memorizer://index",
					"memorizer://index/markdown",
					"memorizer://index/json",
				}

				for _, uri := range expectedURIs {
					if !uris[uri] {
						t.Errorf("Expected URI %s not found in resources list", uri)
					}
				}
			}
		})
	}
}

func TestServer_ResourcesRead(t *testing.T) {
	tests := []struct {
		name          string
		uri           string
		initialized   bool
		wantError     bool
		wantErrorCode int
		wantMimeType  string
	}{
		{
			name:         "read XML format",
			uri:          "memorizer://index",
			initialized:  true,
			wantError:    false,
			wantMimeType: "application/xml",
		},
		{
			name:         "read Markdown format",
			uri:          "memorizer://index/markdown",
			initialized:  true,
			wantError:    false,
			wantMimeType: "text/markdown",
		},
		{
			name:         "read JSON format",
			uri:          "memorizer://index/json",
			initialized:  true,
			wantError:    false,
			wantMimeType: "application/json",
		},
		{
			name:          "read before initialization",
			uri:           "memorizer://index",
			initialized:   false,
			wantError:     true,
			wantErrorCode: protocol.ServerNotReady,
		},
		{
			name:          "read invalid URI",
			uri:           "memorizer://invalid",
			initialized:   true,
			wantError:     true,
			wantErrorCode: protocol.InvalidParams,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index := &types.Index{
				Generated: time.Now(),
				Root:      "/test",
				Entries: []types.IndexEntry{
					{
						Metadata: types.FileMetadata{
							FileInfo: types.FileInfo{
								Path:     "/test/file.txt",
								Category: "documents",
								Size:     100,
							},
						},
					},
				},
				Stats: types.IndexStats{
					TotalFiles:    1,
					AnalyzedFiles: 1,
				},
			}

			logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
			server := NewServer(index, logger)
			server.initialized = tt.initialized

			// Replace transport with mock
			writeBuf := bytes.NewBuffer(nil)
			server.transport = &mockTransport{
				readBuf:  bytes.NewBuffer(nil),
				writeBuf: writeBuf,
			}

			// Send resources/read request
			request := protocol.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "resources/read",
				Params: mustMarshal(protocol.ResourcesReadRequest{
					URI: tt.uri,
				}),
			}

			reqData, _ := json.Marshal(request)
			ctx := context.Background()

			if err := server.handleMessage(ctx, reqData); err != nil {
				t.Fatalf("handleMessage() error = %v", err)
			}

			// Parse response
			var resp protocol.JSONRPCResponse
			if err := json.Unmarshal(writeBuf.Bytes(), &resp); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			if tt.wantError {
				if resp.Error == nil {
					t.Fatal("Expected error response, got success")
				}
				if resp.Error.Code != tt.wantErrorCode {
					t.Errorf("Error code = %d, want %d", resp.Error.Code, tt.wantErrorCode)
				}
			} else {
				if resp.Error != nil {
					t.Fatalf("Got error response: %s", resp.Error.Message)
				}

				// Parse resources read response
				var readResp protocol.ResourcesReadResponse
				if err := json.Unmarshal(resp.Result, &readResp); err != nil {
					t.Fatalf("Failed to unmarshal resources read response: %v", err)
				}

				if len(readResp.Contents) != 1 {
					t.Errorf("Contents count = %d, want 1", len(readResp.Contents))
				}

				content := readResp.Contents[0]

				if content.URI != tt.uri {
					t.Errorf("Content URI = %s, want %s", content.URI, tt.uri)
				}

				if content.MimeType != tt.wantMimeType {
					t.Errorf("Content MimeType = %s, want %s", content.MimeType, tt.wantMimeType)
				}

				if content.Text == "" {
					t.Error("Content text should not be empty")
				}
			}
		})
	}
}

func TestServer_ToolsList(t *testing.T) {
	tests := []struct {
		name          string
		initialized   bool
		wantError     bool
		wantErrorCode int
		wantCount     int
	}{
		{
			name:        "successful list after initialization",
			initialized: true,
			wantError:   false,
			wantCount:   3, // search_files, get_file_metadata, list_recent_files
		},
		{
			name:          "list before initialization",
			initialized:   false,
			wantError:     true,
			wantErrorCode: protocol.ServerNotReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index := &types.Index{
				Generated: time.Now(),
				Root:      "/test",
				Entries:   []types.IndexEntry{},
				Stats:     types.IndexStats{},
			}

			logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
			server := NewServer(index, logger)
			server.initialized = tt.initialized

			// Replace transport with mock
			writeBuf := bytes.NewBuffer(nil)
			server.transport = &mockTransport{
				readBuf:  bytes.NewBuffer(nil),
				writeBuf: writeBuf,
			}

			// Send tools/list request
			request := protocol.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "tools/list",
				Params:  json.RawMessage(`{}`),
			}

			reqData, _ := json.Marshal(request)
			ctx := context.Background()

			if err := server.handleMessage(ctx, reqData); err != nil {
				t.Fatalf("handleMessage() error = %v", err)
			}

			// Parse response
			var resp protocol.JSONRPCResponse
			if err := json.Unmarshal(writeBuf.Bytes(), &resp); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			if tt.wantError {
				if resp.Error == nil {
					t.Fatal("Expected error response, got success")
				}
				if resp.Error.Code != tt.wantErrorCode {
					t.Errorf("Error code = %d, want %d", resp.Error.Code, tt.wantErrorCode)
				}
			} else {
				if resp.Error != nil {
					t.Fatalf("Got error response: %s", resp.Error.Message)
				}

				// Parse tools list response
				var listResp protocol.ToolsListResponse
				if err := json.Unmarshal(resp.Result, &listResp); err != nil {
					t.Fatalf("Failed to unmarshal tools list response: %v", err)
				}

				if len(listResp.Tools) != tt.wantCount {
					t.Errorf("Tool count = %d, want %d", len(listResp.Tools), tt.wantCount)
				}

				// Verify all three tools are present with correct names
				toolNames := make(map[string]bool)
				for _, tool := range listResp.Tools {
					toolNames[tool.Name] = true

					// Verify each tool has required fields
					if tool.Name == "" {
						t.Error("Tool has empty name")
					}
					if tool.Description == "" {
						t.Error("Tool has empty description")
					}
					if tool.InputSchema.Type == "" {
						t.Error("Tool has empty input schema type")
					}
				}

				expectedTools := []string{"search_files", "get_file_metadata", "list_recent_files"}
				for _, name := range expectedTools {
					if !toolNames[name] {
						t.Errorf("Expected tool %s not found in tools list", name)
					}
				}
			}
		})
	}
}

func TestServer_ToolsCall_SearchFiles(t *testing.T) {
	index := &types.Index{
		Generated: time.Now(),
		Root:      "/test",
		Entries: []types.IndexEntry{
			{
				Metadata: types.FileMetadata{
					FileInfo: types.FileInfo{
						Path:     "/test/terraform-guide.md",
						Category: "documents",
						Size:     1024,
						Modified: time.Now(),
					},
				},
				Semantic: &types.SemanticAnalysis{
					Summary: "Guide to Terraform",
					Tags:    []string{"terraform", "iac"},
				},
			},
		},
		Stats: types.IndexStats{TotalFiles: 1, AnalyzedFiles: 1},
	}

	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	server := NewServer(index, logger)
	server.initialized = true

	writeBuf := bytes.NewBuffer(nil)
	server.transport = &mockTransport{
		readBuf:  bytes.NewBuffer(nil),
		writeBuf: writeBuf,
	}

	// Send tools/call request for search_files
	request := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustMarshal(protocol.ToolsCallRequest{
			Name:      "search_files",
			Arguments: mustMarshal(map[string]any{"query": "terraform"}),
		}),
	}

	reqData, _ := json.Marshal(request)
	ctx := context.Background()

	if err := server.handleMessage(ctx, reqData); err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	// Parse response
	var resp protocol.JSONRPCResponse
	if err := json.Unmarshal(writeBuf.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("Got error response: %s", resp.Error.Message)
	}

	// Parse tool call response
	var callResp protocol.ToolsCallResponse
	if err := json.Unmarshal(resp.Result, &callResp); err != nil {
		t.Fatalf("Failed to unmarshal tool call response: %v", err)
	}

	if callResp.IsError {
		t.Fatalf("Tool returned error: %s", callResp.Content[0].Text)
	}

	if len(callResp.Content) != 1 {
		t.Errorf("Content count = %d, want 1", len(callResp.Content))
	}

	if callResp.Content[0].Type != "text" {
		t.Errorf("Content type = %s, want text", callResp.Content[0].Type)
	}

	// Verify result contains expected data
	var result map[string]any
	if err := json.Unmarshal([]byte(callResp.Content[0].Text), &result); err != nil {
		t.Fatalf("Failed to unmarshal result JSON: %v", err)
	}

	if result["query"] != "terraform" {
		t.Errorf("Query = %v, want terraform", result["query"])
	}

	if result["result_count"].(float64) != 1 {
		t.Errorf("Result count = %v, want 1", result["result_count"])
	}
}

func TestServer_ToolsCall_GetFileMetadata(t *testing.T) {
	index := &types.Index{
		Generated: time.Now(),
		Root:      "/test",
		Entries: []types.IndexEntry{
			{
				Metadata: types.FileMetadata{
					FileInfo: types.FileInfo{
						Path:     "/test/document.md",
						Category: "documents",
						Size:     2048,
					},
				},
				Semantic: &types.SemanticAnalysis{
					Summary: "Test document",
					Tags:    []string{"test"},
				},
			},
		},
		Stats: types.IndexStats{TotalFiles: 1},
	}

	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	server := NewServer(index, logger)
	server.initialized = true

	writeBuf := bytes.NewBuffer(nil)
	server.transport = &mockTransport{
		readBuf:  bytes.NewBuffer(nil),
		writeBuf: writeBuf,
	}

	// Send tools/call request for get_file_metadata
	request := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustMarshal(protocol.ToolsCallRequest{
			Name:      "get_file_metadata",
			Arguments: mustMarshal(map[string]any{"path": "document"}),
		}),
	}

	reqData, _ := json.Marshal(request)
	ctx := context.Background()

	if err := server.handleMessage(ctx, reqData); err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	// Parse response
	var resp protocol.JSONRPCResponse
	if err := json.Unmarshal(writeBuf.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("Got error response: %s", resp.Error.Message)
	}

	var callResp protocol.ToolsCallResponse
	if err := json.Unmarshal(resp.Result, &callResp); err != nil {
		t.Fatalf("Failed to unmarshal tool call response: %v", err)
	}

	if callResp.IsError {
		t.Fatalf("Tool returned error: %s", callResp.Content[0].Text)
	}
}

func TestServer_ToolsCall_ListRecentFiles(t *testing.T) {
	now := time.Now()
	index := &types.Index{
		Generated: now,
		Root:      "/test",
		Entries: []types.IndexEntry{
			{
				Metadata: types.FileMetadata{
					FileInfo: types.FileInfo{
						Path:     "/test/recent.md",
						Category: "documents",
						Modified: now.AddDate(0, 0, -1), // 1 day ago
					},
				},
			},
			{
				Metadata: types.FileMetadata{
					FileInfo: types.FileInfo{
						Path:     "/test/old.md",
						Category: "documents",
						Modified: now.AddDate(0, 0, -30), // 30 days ago
					},
				},
			},
		},
		Stats: types.IndexStats{TotalFiles: 2},
	}

	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	server := NewServer(index, logger)
	server.initialized = true

	writeBuf := bytes.NewBuffer(nil)
	server.transport = &mockTransport{
		readBuf:  bytes.NewBuffer(nil),
		writeBuf: writeBuf,
	}

	// Send tools/call request for list_recent_files
	request := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustMarshal(protocol.ToolsCallRequest{
			Name:      "list_recent_files",
			Arguments: mustMarshal(map[string]any{"days": 7}),
		}),
	}

	reqData, _ := json.Marshal(request)
	ctx := context.Background()

	if err := server.handleMessage(ctx, reqData); err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	// Parse response
	var resp protocol.JSONRPCResponse
	if err := json.Unmarshal(writeBuf.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("Got error response: %s", resp.Error.Message)
	}

	var callResp protocol.ToolsCallResponse
	if err := json.Unmarshal(resp.Result, &callResp); err != nil {
		t.Fatalf("Failed to unmarshal tool call response: %v", err)
	}

	if callResp.IsError {
		t.Fatalf("Tool returned error: %s", callResp.Content[0].Text)
	}

	// Verify result shows only recent file
	var result map[string]any
	if err := json.Unmarshal([]byte(callResp.Content[0].Text), &result); err != nil {
		t.Fatalf("Failed to unmarshal result JSON: %v", err)
	}

	if result["result_count"].(float64) != 1 {
		t.Errorf("Result count = %v, want 1 (only recent file)", result["result_count"])
	}
}

func TestServer_ToolsCall_InvalidTool(t *testing.T) {
	index := &types.Index{}
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	server := NewServer(index, logger)
	server.initialized = true

	writeBuf := bytes.NewBuffer(nil)
	server.transport = &mockTransport{
		readBuf:  bytes.NewBuffer(nil),
		writeBuf: writeBuf,
	}

	// Send tools/call request for non-existent tool
	request := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustMarshal(protocol.ToolsCallRequest{
			Name:      "invalid_tool",
			Arguments: json.RawMessage(`{}`),
		}),
	}

	reqData, _ := json.Marshal(request)
	ctx := context.Background()

	if err := server.handleMessage(ctx, reqData); err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	// Parse response
	var resp protocol.JSONRPCResponse
	if err := json.Unmarshal(writeBuf.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Should get JSON-RPC error for invalid tool
	if resp.Error == nil {
		t.Fatal("Expected error response for invalid tool")
	}

	if resp.Error.Code != protocol.MethodNotFound {
		t.Errorf("Error code = %d, want %d (MethodNotFound)", resp.Error.Code, protocol.MethodNotFound)
	}
}

func TestServer_ToolsCall_InvalidArguments(t *testing.T) {
	index := &types.Index{}
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	server := NewServer(index, logger)
	server.initialized = true

	writeBuf := bytes.NewBuffer(nil)
	server.transport = &mockTransport{
		readBuf:  bytes.NewBuffer(nil),
		writeBuf: writeBuf,
	}

	// Send tools/call request with missing required argument
	request := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustMarshal(protocol.ToolsCallRequest{
			Name:      "search_files",
			Arguments: mustMarshal(map[string]any{}), // Missing required "query"
		}),
	}

	reqData, _ := json.Marshal(request)
	ctx := context.Background()

	if err := server.handleMessage(ctx, reqData); err != nil {
		t.Fatalf("handleMessage() error = %v", err)
	}

	// Parse response
	var resp protocol.JSONRPCResponse
	if err := json.Unmarshal(writeBuf.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error != nil {
		t.Fatal("Should return tool response with isError, not JSON-RPC error")
	}

	var callResp protocol.ToolsCallResponse
	if err := json.Unmarshal(resp.Result, &callResp); err != nil {
		t.Fatalf("Failed to unmarshal tool call response: %v", err)
	}

	if !callResp.IsError {
		t.Error("Expected isError=true for invalid arguments")
	}
}

// mustMarshal is a test helper that panics on marshal errors
func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
