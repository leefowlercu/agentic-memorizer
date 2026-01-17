package integrations

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMCPIntegration(t *testing.T) {
	t.Run("NewMCPIntegration", func(t *testing.T) {
		serverConfig := MCPServerConfig{
			Transport: MCPTransportRemote,
			URL:       "http://localhost:7600/mcp",
		}

		integration := NewMCPIntegration(
			"test-mcp",
			"test-harness",
			"Test MCP integration",
			"testbin",
			"~/.test/mcp.json",
			"json",
			"mcpServers",
			"memorizer",
			serverConfig,
		)

		if integration.Name() != "test-mcp" {
			t.Errorf("Name() = %q, want %q", integration.Name(), "test-mcp")
		}
		if integration.Harness() != "test-harness" {
			t.Errorf("Harness() = %q, want %q", integration.Harness(), "test-harness")
		}
		if integration.Type() != IntegrationTypeMCP {
			t.Errorf("Type() = %q, want %q", integration.Type(), IntegrationTypeMCP)
		}
		if integration.Description() != "Test MCP integration" {
			t.Errorf("Description() = %q, want %q", integration.Description(), "Test MCP integration")
		}
	})

	t.Run("SetupAndTeardownRemote", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "mcp.json")

		// Create a mock binary
		binPath := filepath.Join(tmpDir, "testbin")
		_ = os.WriteFile(binPath, []byte("#!/bin/sh\necho test"), 0755)

		// Update PATH
		origPath := os.Getenv("PATH")
		_ = os.Setenv("PATH", tmpDir+":"+origPath)
		defer func() { _ = os.Setenv("PATH", origPath) }()

		// Test remote transport (default)
		serverConfig := MCPServerConfig{
			Transport: MCPTransportRemote,
			URL:       "http://127.0.0.1:7600/mcp",
			Env:       map[string]string{"TEST_VAR": "test"},
		}

		integration := NewMCPIntegration(
			"test-mcp",
			"test-harness",
			"Test",
			"testbin",
			configPath,
			"json",
			"mcpServers",
			"memorizer",
			serverConfig,
		)

		ctx := context.Background()

		// Setup
		err := integration.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Verify server added with URL-based config
		data, _ := os.ReadFile(configPath)
		var config map[string]any
		_ = json.Unmarshal(data, &config)

		serversSection, ok := config["mcpServers"].(map[string]any)
		if !ok {
			t.Fatal("mcpServers section not found")
		}

		serverEntry, exists := serversSection["memorizer"].(map[string]any)
		if !exists {
			t.Fatal("memorizer server not found")
		}

		// For remote transport, should have URL not command
		if serverEntry["url"] != "http://127.0.0.1:7600/mcp" {
			t.Errorf("url = %v, want %v", serverEntry["url"], "http://127.0.0.1:7600/mcp")
		}
		if _, hasCommand := serverEntry["command"]; hasCommand {
			t.Error("remote transport should not have command field")
		}

		// IsInstalled should return true
		installed, err := integration.IsInstalled()
		if err != nil {
			t.Fatalf("IsInstalled failed: %v", err)
		}
		if !installed {
			t.Error("IsInstalled should return true after setup")
		}

		// Teardown
		err = integration.Teardown(ctx)
		if err != nil {
			t.Fatalf("Teardown failed: %v", err)
		}

		// Verify server removed (use fresh map to avoid json.Unmarshal merge behavior)
		data, _ = os.ReadFile(configPath)
		var afterConfig map[string]any
		_ = json.Unmarshal(data, &afterConfig)

		if serversSection, ok := afterConfig["mcpServers"].(map[string]any); ok {
			if _, exists := serversSection["memorizer"]; exists {
				t.Error("memorizer server should be removed after teardown")
			}
		}

		// IsInstalled should return false
		installed, err = integration.IsInstalled()
		if err != nil {
			t.Fatalf("IsInstalled failed: %v", err)
		}
		if installed {
			t.Error("IsInstalled should return false after teardown")
		}
	})

	t.Run("SetupAndTeardownStdio", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "mcp.json")

		// Create a mock binary
		binPath := filepath.Join(tmpDir, "testbin")
		_ = os.WriteFile(binPath, []byte("#!/bin/sh\necho test"), 0755)

		// Update PATH
		origPath := os.Getenv("PATH")
		_ = os.Setenv("PATH", tmpDir+":"+origPath)
		defer func() { _ = os.Setenv("PATH", origPath) }()

		// Test stdio transport
		serverConfig := MCPServerConfig{
			Transport: MCPTransportStdio,
			Command:   "memorizer",
			Args:      []string{"daemon", "mcp"},
			Env:       map[string]string{"TEST_VAR": "test"},
		}

		integration := NewMCPIntegration(
			"test-mcp",
			"test-harness",
			"Test",
			"testbin",
			configPath,
			"json",
			"mcpServers",
			"memorizer",
			serverConfig,
		)

		ctx := context.Background()

		// Setup
		err := integration.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Verify server added with command-based config
		data, _ := os.ReadFile(configPath)
		var config map[string]any
		_ = json.Unmarshal(data, &config)

		serversSection, ok := config["mcpServers"].(map[string]any)
		if !ok {
			t.Fatal("mcpServers section not found")
		}

		serverEntry, exists := serversSection["memorizer"].(map[string]any)
		if !exists {
			t.Fatal("memorizer server not found")
		}

		// For stdio transport, should have command not URL
		if serverEntry["command"] != "memorizer" {
			t.Errorf("command = %v, want %v", serverEntry["command"], "memorizer")
		}
		if serverEntry["type"] != "stdio" {
			t.Errorf("type = %v, want %v", serverEntry["type"], "stdio")
		}
		if _, hasURL := serverEntry["url"]; hasURL {
			t.Error("stdio transport should not have url field")
		}

		// Teardown
		err = integration.Teardown(ctx)
		if err != nil {
			t.Fatalf("Teardown failed: %v", err)
		}
	})

	t.Run("SetupPreservesExistingServers", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "mcp.json")

		// Create existing config with another server
		existingConfig := map[string]any{
			"mcpServers": map[string]any{
				"other-server": map[string]any{
					"command": "other",
					"args":    []string{"arg1"},
				},
			},
		}
		data, _ := json.Marshal(existingConfig)
		_ = os.WriteFile(configPath, data, 0644)

		// Create a mock binary
		binPath := filepath.Join(tmpDir, "testbin")
		_ = os.WriteFile(binPath, []byte("#!/bin/sh\necho test"), 0755)

		origPath := os.Getenv("PATH")
		_ = os.Setenv("PATH", tmpDir+":"+origPath)
		defer func() { _ = os.Setenv("PATH", origPath) }()

		integration := NewMCPIntegration(
			"test-mcp",
			"test",
			"Test",
			"testbin",
			configPath,
			"json",
			"mcpServers",
			"memorizer",
			MCPServerConfig{
				Transport: MCPTransportRemote,
				URL:       "http://localhost:7600/mcp",
			},
		)

		ctx := context.Background()
		err := integration.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Verify both servers exist
		data, _ = os.ReadFile(configPath)
		var config map[string]any
		_ = json.Unmarshal(data, &config)

		serversSection := config["mcpServers"].(map[string]any)
		if _, exists := serversSection["other-server"]; !exists {
			t.Error("other-server should still exist")
		}
		if _, exists := serversSection["memorizer"]; !exists {
			t.Error("memorizer should be added")
		}
	})

	t.Run("SetupDefaultsToRemoteTransport", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "mcp.json")

		// Create a mock binary
		binPath := filepath.Join(tmpDir, "testbin")
		_ = os.WriteFile(binPath, []byte("#!/bin/sh\necho test"), 0755)

		origPath := os.Getenv("PATH")
		_ = os.Setenv("PATH", tmpDir+":"+origPath)
		defer func() { _ = os.Setenv("PATH", origPath) }()

		// Empty config should default to remote transport with default URL
		integration := NewMCPIntegration(
			"test-mcp",
			"test",
			"Test",
			"testbin",
			configPath,
			"json",
			"mcpServers",
			"memorizer",
			MCPServerConfig{}, // Empty config
		)

		ctx := context.Background()
		err := integration.Setup(ctx)
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Verify URL is set to default
		data, _ := os.ReadFile(configPath)
		var config map[string]any
		_ = json.Unmarshal(data, &config)

		serversSection := config["mcpServers"].(map[string]any)
		serverEntry := serversSection["memorizer"].(map[string]any)

		// Should have default URL
		if serverEntry["url"] != DefaultMCPURL {
			t.Errorf("url = %v, want %v", serverEntry["url"], DefaultMCPURL)
		}
	})

	t.Run("ValidateMissingBinary", func(t *testing.T) {
		integration := NewMCPIntegration(
			"test",
			"test",
			"Test",
			"nonexistent-binary-12345",
			"/tmp/mcp.json",
			"json",
			"mcpServers",
			"memorizer",
			MCPServerConfig{},
		)

		err := integration.Validate()
		if err == nil {
			t.Error("Validate should fail for missing binary")
		}
	})

	t.Run("StatusMissingHarness", func(t *testing.T) {
		integration := NewMCPIntegration(
			"test",
			"test",
			"Test",
			"nonexistent-binary-12345",
			"/tmp/mcp.json",
			"json",
			"mcpServers",
			"memorizer",
			MCPServerConfig{},
		)

		status, err := integration.Status()
		if err != nil {
			t.Fatalf("Status failed: %v", err)
		}

		if status.Status != StatusMissingHarness {
			t.Errorf("Status = %q, want %q", status.Status, StatusMissingHarness)
		}
	})
}
