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
			Command: "memorizer",
			Args:    []string{"daemon", "mcp"},
			Type:    "stdio",
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

	t.Run("SetupAndTeardown", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "mcp.json")

		// Create a mock binary
		binPath := filepath.Join(tmpDir, "testbin")
		_ = os.WriteFile(binPath, []byte("#!/bin/sh\necho test"), 0755)

		// Update PATH
		origPath := os.Getenv("PATH")
		_ = os.Setenv("PATH", tmpDir+":"+origPath)
		defer func() { _ = os.Setenv("PATH", origPath) }()

		serverConfig := MCPServerConfig{
			Command: "memorizer",
			Args:    []string{"daemon", "mcp"},
			Type:    "stdio",
			Env:     map[string]string{"TEST_VAR": "test"},
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

		// Verify server added
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

		if serverEntry["command"] != "memorizer" {
			t.Errorf("command = %v, want %v", serverEntry["command"], "memorizer")
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
			MCPServerConfig{Command: "memorizer"},
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
