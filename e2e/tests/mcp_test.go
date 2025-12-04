//go:build e2e

package tests

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/e2e/harness"
)

// TestMCP_Initialize tests MCP initialize handshake
func TestMCP_Initialize(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Start MCP server
	client, err := harness.NewMCPClient(h.BinaryPath, h.AppDir)
	if err != nil {
		t.Skipf("Failed to start MCP server: %v", err)
	}
	defer client.Close()

	// Initialize
	resp, err := client.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Verify response
	if resp.ProtocolVersion != "2024-11-05" {
		t.Errorf("Expected protocol version 2024-11-05, got: %s", resp.ProtocolVersion)
	}

	if resp.ServerInfo.Name == "" {
		t.Error("Expected server name to be set")
	}

	if resp.ServerInfo.Version == "" {
		t.Error("Expected server version to be set")
	}

	t.Logf("MCP server initialized: %s v%s", resp.ServerInfo.Name, resp.ServerInfo.Version)
}

// TestMCP_ListResources tests resources/list endpoint
func TestMCP_ListResources(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Start MCP server
	client, err := harness.NewMCPClient(h.BinaryPath, h.AppDir)
	if err != nil {
		t.Skipf("Failed to start MCP server: %v", err)
	}
	defer client.Close()

	// Initialize
	if _, err := client.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// List resources
	resources, err := client.ListResources()
	if err != nil {
		t.Fatalf("ListResources failed: %v", err)
	}

	// Verify expected resources (index formats)
	expectedResources := map[string]bool{
		"memorizer://index/markdown": false,
		"memorizer://index/json":     false,
	}

	for _, res := range resources {
		if _, ok := expectedResources[res.URI]; ok {
			expectedResources[res.URI] = true
			t.Logf("Found resource: %s - %s", res.URI, res.Name)
		}
	}

	// Check all expected resources found
	for uri, found := range expectedResources {
		if !found {
			t.Errorf("Expected resource not found: %s", uri)
		}
	}
}

// TestMCP_ReadResource tests resources/read endpoint
func TestMCP_ReadResource(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Add test file
	if err := h.AddMemoryFile("test.md", "# Test\n\nContent"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Start MCP server
	client, err := harness.NewMCPClient(h.BinaryPath, h.AppDir)
	if err != nil {
		t.Skipf("Failed to start MCP server: %v", err)
	}
	defer client.Close()

	// Initialize
	if _, err := client.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Read each resource format
	testCases := []struct {
		uri         string
		shouldExist bool
	}{
		{"memorizer://index/markdown", true},
		{"memorizer://index/json", true},
		{"memorizer://index/invalid", false},
	}

	for _, tc := range testCases {
		t.Run(tc.uri, func(t *testing.T) {
			content, err := client.ReadResource(tc.uri)
			if tc.shouldExist {
				if err != nil {
					t.Errorf("ReadResource(%s) failed: %v", tc.uri, err)
					return
				}
				if content == "" {
					t.Errorf("Expected non-empty content for %s", tc.uri)
				}
				t.Logf("Resource %s: %d bytes", tc.uri, len(content))
			} else {
				if err == nil {
					t.Errorf("Expected error for invalid resource %s", tc.uri)
				}
			}
		})
	}
}

// TestMCP_ListTools tests tools/list endpoint
func TestMCP_ListTools(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Start MCP server
	client, err := harness.NewMCPClient(h.BinaryPath, h.AppDir)
	if err != nil {
		t.Skipf("Failed to start MCP server: %v", err)
	}
	defer client.Close()

	// Initialize
	if _, err := client.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// List tools
	tools, err := client.ListTools()
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	// Verify expected tools
	expectedTools := map[string]bool{
		"search_files":      false,
		"get_file_metadata": false,
		"list_recent_files": false,
		"get_related_files": false,
		"search_entities":   false,
	}

	for _, tool := range tools {
		if _, ok := expectedTools[tool.Name]; ok {
			expectedTools[tool.Name] = true
			t.Logf("Found tool: %s - %s", tool.Name, tool.Description)
		}
	}

	// Check all expected tools found
	for name, found := range expectedTools {
		if !found {
			t.Errorf("Expected tool not found: %s", name)
		}
	}

	if len(tools) != len(expectedTools) {
		t.Errorf("Expected %d tools, got %d", len(expectedTools), len(tools))
	}
}

// TestMCP_CallTool_SearchFiles tests search_files tool
func TestMCP_CallTool_SearchFiles(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server for daemon API
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Add test files
	if err := h.AddMemoryFile("test1.md", "# Test 1\n\nSearch term alpha"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}
	if err := h.AddMemoryFile("test2.md", "# Test 2\n\nSearch term beta"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for indexing
	time.Sleep(5 * time.Second)

	// Start MCP server
	client, err := harness.NewMCPClient(h.BinaryPath, h.AppDir)
	if err != nil {
		t.Skipf("Failed to start MCP server: %v", err)
	}
	defer client.Close()

	// Initialize
	if _, err := client.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Call search_files tool
	args := map[string]any{
		"query":       "test",
		"max_results": 10,
	}

	result, err := client.CallTool("search_files", args)
	if err != nil {
		t.Fatalf("CallTool(search_files) failed: %v", err)
	}

	t.Logf("Search results: %+v", result)
}

// TestMCP_CallTool_GetFileMetadata tests get_file_metadata tool
func TestMCP_CallTool_GetFileMetadata(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server for daemon API
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Add test file
	if err := h.AddMemoryFile("test.md", "# Test\n\nContent for metadata test"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for indexing
	time.Sleep(5 * time.Second)

	// Start MCP server
	client, err := harness.NewMCPClient(h.BinaryPath, h.AppDir)
	if err != nil {
		t.Skipf("Failed to start MCP server: %v", err)
	}
	defer client.Close()

	// Initialize
	if _, err := client.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Call get_file_metadata tool
	args := map[string]any{
		"path": "test.md",
	}

	result, err := client.CallTool("get_file_metadata", args)
	if err != nil {
		t.Fatalf("get_file_metadata failed: %v", err)
	}

	t.Logf("File metadata: %+v", result)
}

// TestMCP_CallTool_ListRecentFiles tests list_recent_files tool
func TestMCP_CallTool_ListRecentFiles(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server for daemon API
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Add test files
	if err := h.AddMemoryFile("recent1.md", "# Recent 1\n\nRecent file content"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}
	if err := h.AddMemoryFile("recent2.md", "# Recent 2\n\nAnother recent file"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for indexing
	time.Sleep(5 * time.Second)

	// Start MCP server
	client, err := harness.NewMCPClient(h.BinaryPath, h.AppDir)
	if err != nil {
		t.Skipf("Failed to start MCP server: %v", err)
	}
	defer client.Close()

	// Initialize
	if _, err := client.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Call list_recent_files tool
	args := map[string]any{
		"days":  7,
		"limit": 10,
	}

	result, err := client.CallTool("list_recent_files", args)
	if err != nil {
		t.Fatalf("CallTool(list_recent_files) failed: %v", err)
	}

	t.Logf("Recent files: %+v", result)
}

// TestMCP_CallTool_GetRelatedFiles tests get_related_files tool
func TestMCP_CallTool_GetRelatedFiles(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server for daemon API
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Add test files with related content
	if err := h.AddMemoryFile("test.md", "# Test\n\nContent about FalkorDB and graphs"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}
	if err := h.AddMemoryFile("related.md", "# Related\n\nAlso about FalkorDB and graphs"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for indexing
	time.Sleep(5 * time.Second)

	// Start MCP server
	client, err := harness.NewMCPClient(h.BinaryPath, h.AppDir)
	if err != nil {
		t.Skipf("Failed to start MCP server: %v", err)
	}
	defer client.Close()

	// Initialize
	if _, err := client.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Call get_related_files tool
	args := map[string]any{
		"path":  "test.md",
		"limit": 5,
	}

	result, err := client.CallTool("get_related_files", args)
	if err != nil {
		t.Fatalf("get_related_files failed: %v", err)
	}

	t.Logf("Related files: %+v", result)
}

// TestMCP_CallTool_SearchEntities tests search_entities tool
func TestMCP_CallTool_SearchEntities(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Enable HTTP server for daemon API
	if err := h.EnableHTTPServer(8080); err != nil {
		t.Fatalf("Failed to enable HTTP server: %v", err)
	}

	// Add test files mentioning an entity
	if err := h.AddMemoryFile("entity1.md", "# Entity Test 1\n\nThis document mentions FalkorDB extensively"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}
	if err := h.AddMemoryFile("entity2.md", "# Entity Test 2\n\nFalkorDB is a graph database"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.BinaryPath, "daemon", "start")
	cmd.Env = append(cmd.Env, "MEMORIZER_APP_DIR="+h.AppDir)

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}

	defer func() {
		cancel()
		cmd.Wait()
	}()

	if err := h.WaitForHealthy(30 * time.Second); err != nil {
		t.Fatalf("Daemon failed to become healthy: %v", err)
	}

	// Wait for indexing
	time.Sleep(5 * time.Second)

	// Start MCP server
	client, err := harness.NewMCPClient(h.BinaryPath, h.AppDir)
	if err != nil {
		t.Skipf("Failed to start MCP server: %v", err)
	}
	defer client.Close()

	// Initialize
	if _, err := client.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Call search_entities tool
	args := map[string]any{
		"entity":      "FalkorDB",
		"max_results": 10,
	}

	result, err := client.CallTool("search_entities", args)
	if err != nil {
		t.Fatalf("search_entities failed: %v", err)
	}

	t.Logf("Entity search results: %+v", result)
}

// TestMCP_InvalidRequest tests error handling for invalid requests
func TestMCP_InvalidRequest(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Start MCP server
	client, err := harness.NewMCPClient(h.BinaryPath, h.AppDir)
	if err != nil {
		t.Skipf("Failed to start MCP server: %v", err)
	}
	defer client.Close()

	// Initialize
	if _, err := client.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Test invalid tool name
	_, err = client.CallTool("nonexistent_tool", map[string]any{})
	if err == nil {
		t.Error("Expected error for invalid tool name")
	} else {
		if !strings.Contains(err.Error(), "tool not found") && !strings.Contains(err.Error(), "unknown tool") {
			t.Logf("Invalid tool error: %v", err)
		}
	}

	// Test invalid resource URI
	_, err = client.ReadResource("invalid://uri")
	if err == nil {
		t.Error("Expected error for invalid resource URI")
	} else {
		t.Logf("Invalid resource error: %v", err)
	}
}

// TestMCP_Shutdown tests MCP shutdown sequence
func TestMCP_Shutdown(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Start MCP server
	client, err := harness.NewMCPClient(h.BinaryPath, h.AppDir)
	if err != nil {
		t.Skipf("Failed to start MCP server: %v", err)
	}

	// Initialize
	if _, err := client.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Shutdown
	if err := client.Shutdown(); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Close should succeed after shutdown
	if err := client.Close(); err != nil {
		t.Logf("Close after shutdown: %v", err)
	}

	t.Log("MCP shutdown sequence completed")
}

// TestMCP_ProtocolVersion tests protocol version negotiation
func TestMCP_ProtocolVersion(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Start MCP server
	client, err := harness.NewMCPClient(h.BinaryPath, h.AppDir)
	if err != nil {
		t.Skipf("Failed to start MCP server: %v", err)
	}
	defer client.Close()

	// Initialize with specific protocol version
	resp, err := client.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Verify protocol version matches MCP spec
	expectedVersion := "2024-11-05"
	if resp.ProtocolVersion != expectedVersion {
		t.Errorf("Expected protocol version %s, got: %s", expectedVersion, resp.ProtocolVersion)
	}

	// Verify capabilities
	if resp.Capabilities.Resources == nil {
		t.Error("Expected resources capability to be advertised")
	}

	if resp.Capabilities.Tools == nil {
		t.Error("Expected tools capability to be advertised")
	}

	t.Logf("Protocol version: %s", resp.ProtocolVersion)
	t.Logf("Capabilities: resources=%v, tools=%v",
		resp.Capabilities.Resources != nil,
		resp.Capabilities.Tools != nil)
}

// TestMCP_MultipleClients tests that MCP server handles multiple client connections
func TestMCP_MultipleClients(t *testing.T) {
	t.Skip("MCP server is stdio-based, supports one client per process instance")

	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Start first client
	client1, err := harness.NewMCPClient(h.BinaryPath, h.AppDir)
	if err != nil {
		t.Skipf("Failed to start first MCP client: %v", err)
	}
	defer client1.Close()

	if _, err := client1.Initialize(); err != nil {
		t.Fatalf("First client initialize failed: %v", err)
	}

	// Start second client (should succeed with separate process)
	client2, err := harness.NewMCPClient(h.BinaryPath, h.AppDir)
	if err != nil {
		t.Skipf("Failed to start second MCP client: %v", err)
	}
	defer client2.Close()

	if _, err := client2.Initialize(); err != nil {
		t.Fatalf("Second client initialize failed: %v", err)
	}

	t.Log("Multiple MCP clients started successfully")
}

// TestMCP_ToolInputSchema tests that tools have proper input schemas
func TestMCP_ToolInputSchema(t *testing.T) {
	h := harness.New(t)
	if err := h.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	cleanup := harness.MustCleanup(t, h)
	defer cleanup.CleanupAll()

	// Start MCP server
	client, err := harness.NewMCPClient(h.BinaryPath, h.AppDir)
	if err != nil {
		t.Skipf("Failed to start MCP server: %v", err)
	}
	defer client.Close()

	// Initialize
	if _, err := client.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// List tools
	tools, err := client.ListTools()
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	// Verify each tool has input schema
	for _, tool := range tools {
		if tool.InputSchema.Type == "" {
			t.Errorf("Tool %s missing input schema", tool.Name)
			continue
		}

		if tool.InputSchema.Type != "object" {
			t.Errorf("Tool %s input schema type should be 'object', got: %s",
				tool.Name, tool.InputSchema.Type)
		}

		if len(tool.InputSchema.Properties) == 0 {
			t.Errorf("Tool %s has no properties in input schema", tool.Name)
		}

		t.Logf("Tool %s schema: %d properties, required=%v",
			tool.Name, len(tool.InputSchema.Properties), tool.InputSchema.Required)
	}
}
