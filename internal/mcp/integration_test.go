//go:build integration

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

// TestIntegration_FullResourcesFlow tests the complete initialization and resources flow
func TestIntegration_FullResourcesFlow(t *testing.T) {
	// Create a test index with sample data
	index := &types.FileIndex{
		Generated:  time.Now(),
		MemoryRoot: "/test",
		Files: []types.FileEntry{
			{
				Path:         "/test/document.md",
				Name:         "document.md",
				Category:     "documents",
				Size:         2048,
				Type:         "md",
				WordCount:    ptrInt(500),
				Summary:      "Test document about AI",
				Tags:         []string{"ai", "testing"},
				Topics:       []string{"artificial intelligence", "testing"},
				DocumentType: "technical-document",
				Confidence:   0.95,
			},
			{
				Path:         "/test/image.png",
				Name:         "image.png",
				Category:     "images",
				Size:         10000,
				Type:         "png",
				Dimensions:   &types.ImageDim{Width: 1920, Height: 1080},
				Summary:      "Architecture diagram",
				Tags:         []string{"diagram", "architecture"},
				Topics:       []string{"system design"},
				DocumentType: "diagram",
				Confidence:   0.90,
			},
		},
		Stats: types.IndexStats{
			TotalFiles:    2,
			AnalyzedFiles: 2,
			TotalSize:     12048,
		},
	}

	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	server := NewServer(index, logger, "")

	// Replace transport with mock
	readBuf := bytes.NewBuffer(nil)
	writeBuf := bytes.NewBuffer(nil)
	server.transport = &mockTransport{
		readBuf:  readBuf,
		writeBuf: writeBuf,
	}

	ctx := context.Background()

	// Step 1: Initialize
	t.Run("initialize", func(t *testing.T) {
		initReq := protocol.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "initialize",
			Params: mustMarshal(protocol.InitializeRequest{
				ProtocolVersion: "2024-11-05",
				Capabilities:    protocol.ClientCapabilities{},
				ClientInfo: protocol.ClientInfo{
					Name:    "integration-test",
					Version: "1.0.0",
				},
			}),
		}

		reqData, _ := json.Marshal(initReq)
		if err := server.handleMessage(ctx, reqData); err != nil {
			t.Fatalf("initialize failed: %v", err)
		}

		var resp protocol.JSONRPCResponse
		if err := json.Unmarshal(writeBuf.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal init response: %v", err)
		}

		if resp.Error != nil {
			t.Fatalf("Initialize error: %s", resp.Error.Message)
		}

		var initResp protocol.InitializeResponse
		if err := json.Unmarshal(resp.Result, &initResp); err != nil {
			t.Fatalf("Failed to unmarshal initialize response: %v", err)
		}

		if initResp.ServerInfo.Name != "memorizer" {
			t.Errorf("Server name = %s, want memorizer", initResp.ServerInfo.Name)
		}

		writeBuf.Reset()
	})

	// Step 2: Send initialized notification
	t.Run("initialized", func(t *testing.T) {
		notification := protocol.JSONRPCNotification{
			JSONRPC: "2.0",
			Method:  "initialized",
			Params:  json.RawMessage(`{}`),
		}

		notifData, _ := json.Marshal(notification)
		if err := server.handleMessage(ctx, notifData); err != nil {
			t.Fatalf("initialized notification failed: %v", err)
		}

		if !server.initialized {
			t.Error("Server should be initialized after notification")
		}
	})

	// Step 3: List resources
	t.Run("resources/list", func(t *testing.T) {
		listReq := protocol.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      2,
			Method:  "resources/list",
			Params:  json.RawMessage(`{}`),
		}

		reqData, _ := json.Marshal(listReq)
		writeBuf.Reset()

		if err := server.handleMessage(ctx, reqData); err != nil {
			t.Fatalf("resources/list failed: %v", err)
		}

		var resp protocol.JSONRPCResponse
		if err := json.Unmarshal(writeBuf.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal list response: %v", err)
		}

		if resp.Error != nil {
			t.Fatalf("List error: %s", resp.Error.Message)
		}

		var listResp protocol.ResourcesListResponse
		if err := json.Unmarshal(resp.Result, &listResp); err != nil {
			t.Fatalf("Failed to unmarshal resources list: %v", err)
		}

		if len(listResp.Resources) != 3 {
			t.Errorf("Resource count = %d, want 3", len(listResp.Resources))
		}
	})

	// Step 4: Read each resource format
	formats := []struct {
		name     string
		uri      string
		mimeType string
	}{
		{"XML", "memorizer://index", "application/xml"},
		{"Markdown", "memorizer://index/markdown", "text/markdown"},
		{"JSON", "memorizer://index/json", "application/json"},
	}

	for _, format := range formats {
		t.Run("resources/read_"+format.name, func(t *testing.T) {
			readReq := protocol.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      3,
				Method:  "resources/read",
				Params: mustMarshal(protocol.ResourcesReadRequest{
					URI: format.uri,
				}),
			}

			reqData, _ := json.Marshal(readReq)
			writeBuf.Reset()

			if err := server.handleMessage(ctx, reqData); err != nil {
				t.Fatalf("resources/read failed: %v", err)
			}

			var resp protocol.JSONRPCResponse
			if err := json.Unmarshal(writeBuf.Bytes(), &resp); err != nil {
				t.Fatalf("Failed to unmarshal read response: %v", err)
			}

			if resp.Error != nil {
				t.Fatalf("Read error: %s", resp.Error.Message)
			}

			var readResp protocol.ResourcesReadResponse
			if err := json.Unmarshal(resp.Result, &readResp); err != nil {
				t.Fatalf("Failed to unmarshal resources read: %v", err)
			}

			if len(readResp.Contents) != 1 {
				t.Errorf("Contents count = %d, want 1", len(readResp.Contents))
			}

			content := readResp.Contents[0]

			if content.URI != format.uri {
				t.Errorf("URI = %s, want %s", content.URI, format.uri)
			}

			if content.MimeType != format.mimeType {
				t.Errorf("MimeType = %s, want %s", content.MimeType, format.mimeType)
			}

			if content.Text == "" {
				t.Error("Content text should not be empty")
			}

			// Verify content contains expected data
			if format.name == "JSON" {
				// Parse JSON to verify structure
				var parsedIndex types.FileIndex
				if err := json.Unmarshal([]byte(content.Text), &parsedIndex); err != nil {
					t.Errorf("Invalid JSON content: %v", err)
				}
				if len(parsedIndex.Files) != 2 {
					t.Errorf("JSON files count = %d, want 2", len(parsedIndex.Files))
				}
			}
		})
	}
}

// TestIntegration_FullToolsFlow tests the complete tools workflow
func TestIntegration_FullToolsFlow(t *testing.T) {
	now := time.Now()

	// Create a test index with sample data for tools
	index := &types.FileIndex{
		Generated:  now,
		MemoryRoot: "/test/memory",
		Files: []types.FileEntry{
			{
				Path:         "/test/memory/terraform-guide.md",
				Name:         "terraform-guide.md",
				Category:     "documents",
				Size:         50000,
				Type:         "md",
				Modified:     now.AddDate(0, 0, -2), // 2 days ago
				WordCount:    ptrInt(5000),
				Summary:      "Comprehensive guide to HashiCorp Terraform",
				Tags:         []string{"terraform", "iac", "infrastructure"},
				Topics:       []string{"Terraform fundamentals", "Infrastructure as Code"},
				DocumentType: "technical-guide",
				Confidence:   0.95,
			},
			{
				Path:         "/test/memory/vault-config.hcl",
				Name:         "vault-config.hcl",
				Category:     "code",
				Size:         2048,
				Type:         "hcl",
				Modified:     now.AddDate(0, 0, -1), // 1 day ago
				Summary:      "Vault server configuration",
				Tags:         []string{"vault", "security", "config"},
				Topics:       []string{"HashiCorp Vault configuration"},
				DocumentType: "configuration-file",
				Confidence:   0.90,
			},
			{
				Path:         "/test/memory/old-notes.txt",
				Name:         "old-notes.txt",
				Category:     "documents",
				Size:         1024,
				Type:         "txt",
				Modified:     now.AddDate(0, 0, -30), // 30 days ago
				Summary:      "Old meeting notes",
				Tags:         []string{"notes", "meetings"},
				DocumentType: "notes",
				Confidence:   0.80,
			},
		},
		Stats: types.IndexStats{
			TotalFiles:    3,
			AnalyzedFiles: 3,
			TotalSize:     53072,
		},
	}

	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	server := NewServer(index, logger, "")

	// Replace transport with mock
	readBuf := bytes.NewBuffer(nil)
	writeBuf := bytes.NewBuffer(nil)
	server.transport = &mockTransport{
		readBuf:  readBuf,
		writeBuf: writeBuf,
	}

	ctx := context.Background()

	// Step 1: Initialize
	t.Run("initialize", func(t *testing.T) {
		initReq := protocol.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "initialize",
			Params: mustMarshal(protocol.InitializeRequest{
				ProtocolVersion: "2024-11-05",
				Capabilities:    protocol.ClientCapabilities{},
				ClientInfo: protocol.ClientInfo{
					Name:    "integration-test",
					Version: "1.0.0",
				},
			}),
		}

		reqData, _ := json.Marshal(initReq)
		if err := server.handleMessage(ctx, reqData); err != nil {
			t.Fatalf("initialize failed: %v", err)
		}

		var resp protocol.JSONRPCResponse
		if err := json.Unmarshal(writeBuf.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal init response: %v", err)
		}

		if resp.Error != nil {
			t.Fatalf("Initialize error: %s", resp.Error.Message)
		}

		writeBuf.Reset()
	})

	// Step 2: Send initialized notification
	t.Run("initialized", func(t *testing.T) {
		notification := protocol.JSONRPCNotification{
			JSONRPC: "2.0",
			Method:  "initialized",
			Params:  json.RawMessage(`{}`),
		}

		notifData, _ := json.Marshal(notification)
		if err := server.handleMessage(ctx, notifData); err != nil {
			t.Fatalf("initialized notification failed: %v", err)
		}

		if !server.initialized {
			t.Error("Server should be initialized after notification")
		}
	})

	// Step 3: List tools
	t.Run("tools/list", func(t *testing.T) {
		listReq := protocol.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      2,
			Method:  "tools/list",
			Params:  json.RawMessage(`{}`),
		}

		reqData, _ := json.Marshal(listReq)
		writeBuf.Reset()

		if err := server.handleMessage(ctx, reqData); err != nil {
			t.Fatalf("tools/list failed: %v", err)
		}

		var resp protocol.JSONRPCResponse
		if err := json.Unmarshal(writeBuf.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal list response: %v", err)
		}

		if resp.Error != nil {
			t.Fatalf("List error: %s", resp.Error.Message)
		}

		var listResp protocol.ToolsListResponse
		if err := json.Unmarshal(resp.Result, &listResp); err != nil {
			t.Fatalf("Failed to unmarshal tools list: %v", err)
		}

		if len(listResp.Tools) != 5 {
			t.Errorf("Tool count = %d, want 5", len(listResp.Tools))
		}
	})

	// Step 4: Call search_files tool
	t.Run("tools/call_search_files", func(t *testing.T) {
		callReq := protocol.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      3,
			Method:  "tools/call",
			Params: mustMarshal(protocol.ToolsCallRequest{
				Name:      "search_files",
				Arguments: mustMarshal(map[string]any{"query": "terraform"}),
			}),
		}

		reqData, _ := json.Marshal(callReq)
		writeBuf.Reset()

		if err := server.handleMessage(ctx, reqData); err != nil {
			t.Fatalf("tools/call search_files failed: %v", err)
		}

		var resp protocol.JSONRPCResponse
		if err := json.Unmarshal(writeBuf.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal call response: %v", err)
		}

		if resp.Error != nil {
			t.Fatalf("Call error: %s", resp.Error.Message)
		}

		var callResp protocol.ToolsCallResponse
		if err := json.Unmarshal(resp.Result, &callResp); err != nil {
			t.Fatalf("Failed to unmarshal tools call response: %v", err)
		}

		if callResp.IsError {
			t.Fatalf("Tool returned error: %s", callResp.Content[0].Text)
		}

		// Parse and verify result
		var result map[string]any
		if err := json.Unmarshal([]byte(callResp.Content[0].Text), &result); err != nil {
			t.Fatalf("Failed to parse result JSON: %v", err)
		}

		if result["query"] != "terraform" {
			t.Errorf("Query = %v, want terraform", result["query"])
		}

		if result["result_count"].(float64) < 1 {
			t.Error("Expected at least 1 search result for terraform")
		}
	})

	// Step 5: Call get_file_metadata tool
	t.Run("tools/call_get_file_metadata", func(t *testing.T) {
		callReq := protocol.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      4,
			Method:  "tools/call",
			Params: mustMarshal(protocol.ToolsCallRequest{
				Name:      "get_file_metadata",
				Arguments: mustMarshal(map[string]any{"path": "terraform"}),
			}),
		}

		reqData, _ := json.Marshal(callReq)
		writeBuf.Reset()

		if err := server.handleMessage(ctx, reqData); err != nil {
			t.Fatalf("tools/call get_file_metadata failed: %v", err)
		}

		var resp protocol.JSONRPCResponse
		if err := json.Unmarshal(writeBuf.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal call response: %v", err)
		}

		if resp.Error != nil {
			t.Fatalf("Call error: %s", resp.Error.Message)
		}

		var callResp protocol.ToolsCallResponse
		if err := json.Unmarshal(resp.Result, &callResp); err != nil {
			t.Fatalf("Failed to unmarshal tools call response: %v", err)
		}

		if callResp.IsError {
			t.Fatalf("Tool returned error: %s", callResp.Content[0].Text)
		}

		// Parse and verify result contains metadata
		var result struct {
			File   types.FileEntry `json:"file"`
			Source string          `json:"source"`
		}
		if err := json.Unmarshal([]byte(callResp.Content[0].Text), &result); err != nil {
			t.Fatalf("Failed to parse result JSON: %v", err)
		}

		if result.File.Summary == "" {
			t.Error("Expected summary in metadata")
		}
	})

	// Step 6: Call list_recent_files tool
	t.Run("tools/call_list_recent_files", func(t *testing.T) {
		callReq := protocol.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      5,
			Method:  "tools/call",
			Params: mustMarshal(protocol.ToolsCallRequest{
				Name:      "list_recent_files",
				Arguments: mustMarshal(map[string]any{"days": 7, "limit": 10}),
			}),
		}

		reqData, _ := json.Marshal(callReq)
		writeBuf.Reset()

		if err := server.handleMessage(ctx, reqData); err != nil {
			t.Fatalf("tools/call list_recent_files failed: %v", err)
		}

		var resp protocol.JSONRPCResponse
		if err := json.Unmarshal(writeBuf.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal call response: %v", err)
		}

		if resp.Error != nil {
			t.Fatalf("Call error: %s", resp.Error.Message)
		}

		var callResp protocol.ToolsCallResponse
		if err := json.Unmarshal(resp.Result, &callResp); err != nil {
			t.Fatalf("Failed to unmarshal tools call response: %v", err)
		}

		if callResp.IsError {
			t.Fatalf("Tool returned error: %s", callResp.Content[0].Text)
		}

		// Parse and verify result
		var result map[string]any
		if err := json.Unmarshal([]byte(callResp.Content[0].Text), &result); err != nil {
			t.Fatalf("Failed to parse result JSON: %v", err)
		}

		if result["days"].(float64) != 7 {
			t.Errorf("Days = %v, want 7", result["days"])
		}

		// Should have 2 recent files (terraform-guide and vault-config), not old-notes
		if result["result_count"].(float64) != 2 {
			t.Errorf("Result count = %v, want 2 (recent files only)", result["result_count"])
		}
	})
}
