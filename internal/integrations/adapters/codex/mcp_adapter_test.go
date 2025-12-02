package codex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

func TestNewCodexCLIMCPAdapter(t *testing.T) {
	adapter := NewCodexCLIMCPAdapter()

	if adapter == nil {
		t.Fatal("Expected non-nil adapter")
	}

	if adapter.serverName != MCPServerName {
		t.Errorf("Expected server name %q, got %q", MCPServerName, adapter.serverName)
	}

	if adapter.configPath == "" {
		t.Error("Expected non-empty config path")
	}
}

func TestCodexCLIMCPAdapter_GetName(t *testing.T) {
	adapter := NewCodexCLIMCPAdapter()

	if adapter.GetName() != MCPIntegrationName {
		t.Errorf("Expected name %q, got %q", MCPIntegrationName, adapter.GetName())
	}
}

func TestCodexCLIMCPAdapter_GetDescription(t *testing.T) {
	adapter := NewCodexCLIMCPAdapter()

	desc := adapter.GetDescription()
	if desc == "" {
		t.Error("Expected non-empty description")
	}

	if desc != "OpenAI Codex CLI MCP server integration" {
		t.Errorf("Unexpected description: %s", desc)
	}
}

func TestCodexCLIMCPAdapter_GetVersion(t *testing.T) {
	adapter := NewCodexCLIMCPAdapter()

	if adapter.GetVersion() != MCPIntegrationVersion {
		t.Errorf("Expected version %q, got %q", MCPIntegrationVersion, adapter.GetVersion())
	}
}

func TestCodexCLIMCPAdapter_Detect_NoDirectory(t *testing.T) {
	adapter := NewCodexCLIMCPAdapter()

	// This test assumes ~/.codex/ doesn't exist
	// Skip if it does exist to avoid false negatives
	home, _ := os.UserHomeDir()
	codexDir := filepath.Join(home, ".codex")
	if _, err := os.Stat(codexDir); err == nil {
		t.Skip("~/.codex/ directory exists, skipping test")
	}

	detected, err := adapter.Detect()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if detected {
		t.Error("Expected not detected when directory doesn't exist")
	}
}

func TestCodexCLIMCPAdapter_Detect_DirectoryExists(t *testing.T) {
	adapter := NewCodexCLIMCPAdapter()

	// This test checks if codex CLI is available when directory exists
	home, _ := os.UserHomeDir()
	codexDir := filepath.Join(home, ".codex")

	// Only run if ~/.codex exists
	if _, err := os.Stat(codexDir); os.IsNotExist(err) {
		t.Skip("~/.codex/ directory doesn't exist, skipping test")
	}

	detected, err := adapter.Detect()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Detection depends on whether `codex` CLI is in PATH
	// We just verify no error occurred
	_ = detected
}

func TestCodexCLIMCPAdapter_IsEnabled_NoConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	adapter := NewCodexCLIMCPAdapter()
	adapter.configPath = configPath

	enabled, err := adapter.IsEnabled()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if enabled {
		t.Error("Expected not enabled when config doesn't exist")
	}
}

func TestCodexCLIMCPAdapter_IsEnabled_EmptyConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	// Create empty config
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty config: %v", err)
	}

	adapter := NewCodexCLIMCPAdapter()
	adapter.configPath = configPath

	enabled, err := adapter.IsEnabled()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if enabled {
		t.Error("Expected not enabled with empty config")
	}
}

func TestCodexCLIMCPAdapter_IsEnabled_ServerExists(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	tomlContent := `
[mcp_servers.agentic-memorizer]
command = "/usr/bin/test"
args = ["mcp", "start"]
enabled = true
`

	if err := os.WriteFile(configPath, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	adapter := NewCodexCLIMCPAdapter()
	adapter.configPath = configPath

	enabled, err := adapter.IsEnabled()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !enabled {
		t.Error("Expected enabled when server exists")
	}
}

func TestCodexCLIMCPAdapter_IsEnabled_ServerNotExists(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	tomlContent := `
[mcp_servers.other-server]
command = "/usr/bin/other"
`

	if err := os.WriteFile(configPath, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	adapter := NewCodexCLIMCPAdapter()
	adapter.configPath = configPath

	enabled, err := adapter.IsEnabled()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if enabled {
		t.Error("Expected not enabled when our server doesn't exist")
	}
}

func TestCodexCLIMCPAdapter_IsEnabled_ExplicitlyDisabled(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	tomlContent := `
[mcp_servers.agentic-memorizer]
command = "/usr/bin/test"
args = ["mcp", "start"]
enabled = false
`

	if err := os.WriteFile(configPath, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	adapter := NewCodexCLIMCPAdapter()
	adapter.configPath = configPath

	enabled, err := adapter.IsEnabled()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if enabled {
		t.Error("Expected not enabled when explicitly disabled")
	}
}

func TestCodexCLIMCPAdapter_Setup(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	binaryPath := "/usr/bin/agentic-memorizer"

	adapter := NewCodexCLIMCPAdapter()
	adapter.configPath = configPath

	err := adapter.Setup(binaryPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify config was written
	config, _, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	server, exists := config.MCPServers[MCPServerName]
	if !exists {
		t.Fatal("Expected server to be added")
	}

	if server.Command != binaryPath {
		t.Errorf("Expected command %q, got %q", binaryPath, server.Command)
	}

	if len(server.Args) != 2 || server.Args[0] != "mcp" || server.Args[1] != "start" {
		t.Errorf("Expected args [mcp, start], got %v", server.Args)
	}

	if server.Enabled == nil || !*server.Enabled {
		t.Error("Expected enabled=true")
	}

	if server.Env == nil {
		t.Error("Expected Env map to exist")
	}

	// Note: MEMORIZER_MEMORY_ROOT value depends on config initialization
	// In test environment, config.GetConfig() may fail, resulting in empty value
	// We just verify the Env map exists
}

func TestCodexCLIMCPAdapter_Setup_PreservesExistingServers(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	initialContent := `
[mcp_servers.existing-server]
command = "/usr/bin/existing"
args = ["arg1"]
`

	if err := os.WriteFile(configPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to create initial config: %v", err)
	}

	adapter := NewCodexCLIMCPAdapter()
	adapter.configPath = configPath
	binaryPath := "/usr/bin/agentic-memorizer"

	err := adapter.Setup(binaryPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	config, _, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	// Check both servers exist
	if len(config.MCPServers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(config.MCPServers))
	}

	if _, exists := config.MCPServers["existing-server"]; !exists {
		t.Error("Expected existing server to be preserved")
	}

	if _, exists := config.MCPServers[MCPServerName]; !exists {
		t.Error("Expected new server to be added")
	}
}

func TestCodexCLIMCPAdapter_Setup_SetsEnabledTrue(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	adapter := NewCodexCLIMCPAdapter()
	adapter.configPath = configPath

	err := adapter.Setup("/usr/bin/test")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	config, _, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	server := config.MCPServers[MCPServerName]
	if server.Enabled == nil {
		t.Fatal("Expected enabled field to be set")
	}

	if !*server.Enabled {
		t.Error("Expected enabled=true")
	}
}

func TestCodexCLIMCPAdapter_Update(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	binaryPath := "/usr/bin/new-path"

	adapter := NewCodexCLIMCPAdapter()
	adapter.configPath = configPath

	// Setup first
	if err := adapter.Setup("/usr/bin/old-path"); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Update
	err := adapter.Update(binaryPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify updated
	config, _, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	server := config.MCPServers[MCPServerName]
	if server.Command != binaryPath {
		t.Errorf("Expected command %q, got %q", binaryPath, server.Command)
	}
}

func TestCodexCLIMCPAdapter_Remove(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	adapter := NewCodexCLIMCPAdapter()
	adapter.configPath = configPath

	// Setup first
	if err := adapter.Setup("/usr/bin/test"); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Remove
	err := adapter.Remove()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify removed
	config, _, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	if _, exists := config.MCPServers[MCPServerName]; exists {
		t.Error("Expected server to be removed")
	}
}

func TestCodexCLIMCPAdapter_Remove_ServerNotExists(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	adapter := NewCodexCLIMCPAdapter()
	adapter.configPath = configPath

	// Remove without setup (should not error)
	err := adapter.Remove()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestCodexCLIMCPAdapter_GetCommand(t *testing.T) {
	adapter := NewCodexCLIMCPAdapter()
	binaryPath := "/usr/bin/agentic-memorizer"

	cmd := adapter.GetCommand(binaryPath, integrations.FormatXML)

	expected := "/usr/bin/agentic-memorizer mcp start"
	if cmd != expected {
		t.Errorf("Expected command %q, got %q", expected, cmd)
	}
}

func TestCodexCLIMCPAdapter_FormatOutput_ReturnsError(t *testing.T) {
	adapter := NewCodexCLIMCPAdapter()
	index := &types.GraphIndex{}

	_, err := adapter.FormatOutput(index, integrations.FormatXML)
	if err == nil {
		t.Error("Expected error for FormatOutput on MCP adapter")
	}
}

func TestCodexCLIMCPAdapter_Validate_NoConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	adapter := NewCodexCLIMCPAdapter()
	adapter.configPath = configPath

	err := adapter.Validate()
	if err == nil {
		t.Error("Expected error when config doesn't exist")
	}
}

func TestCodexCLIMCPAdapter_Validate_NoServer(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	tomlContent := `
[mcp_servers.other-server]
command = "/usr/bin/other"
`

	if err := os.WriteFile(configPath, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	adapter := NewCodexCLIMCPAdapter()
	adapter.configPath = configPath

	err := adapter.Validate()
	if err == nil {
		t.Error("Expected error when our server doesn't exist")
	}
}

func TestCodexCLIMCPAdapter_Validate_Disabled(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	tomlContent := `
[mcp_servers.agentic-memorizer]
command = "/usr/bin/test"
args = ["mcp", "start"]
enabled = false
`

	if err := os.WriteFile(configPath, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	adapter := NewCodexCLIMCPAdapter()
	adapter.configPath = configPath

	err := adapter.Validate()
	if err == nil {
		t.Error("Expected error when server is disabled")
	}
}

func TestCodexCLIMCPAdapter_Validate_MissingCommand(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	tomlContent := `
[mcp_servers.agentic-memorizer]
args = ["mcp", "start"]
`

	if err := os.WriteFile(configPath, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	adapter := NewCodexCLIMCPAdapter()
	adapter.configPath = configPath

	err := adapter.Validate()
	if err == nil {
		t.Error("Expected error when command is missing")
	}
}

func TestCodexCLIMCPAdapter_Validate_InvalidArgs(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	tomlContent := `
[mcp_servers.agentic-memorizer]
command = "/usr/bin/test"
args = ["wrong", "args"]
`

	if err := os.WriteFile(configPath, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	adapter := NewCodexCLIMCPAdapter()
	adapter.configPath = configPath

	err := adapter.Validate()
	if err == nil {
		t.Error("Expected error when args are invalid")
	}
}

func TestCodexCLIMCPAdapter_Validate_BinaryNotFound(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	tomlContent := `
[mcp_servers.agentic-memorizer]
command = "/nonexistent/binary"
args = ["mcp", "start"]
`

	if err := os.WriteFile(configPath, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	adapter := NewCodexCLIMCPAdapter()
	adapter.configPath = configPath

	err := adapter.Validate()
	if err == nil {
		t.Error("Expected error when binary doesn't exist")
	}
}

func TestCodexCLIMCPAdapter_Validate_Success(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	// Create a test binary
	binaryPath := filepath.Join(tempDir, "test-binary")
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	tomlContent := `
[mcp_servers.agentic-memorizer]
command = "` + binaryPath + `"
args = ["mcp", "start"]
enabled = true

[mcp_servers.agentic-memorizer.env]
MEMORIZER_MEMORY_ROOT = "/path/to/memory"
`

	if err := os.WriteFile(configPath, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	adapter := NewCodexCLIMCPAdapter()
	adapter.configPath = configPath

	err := adapter.Validate()
	if err != nil {
		t.Errorf("Expected validation to pass, got error: %v", err)
	}
}

func TestCodexCLIMCPAdapter_Reload_ServerName(t *testing.T) {
	adapter := NewCodexCLIMCPAdapter()

	newConfig := integrations.IntegrationConfig{
		Settings: map[string]any{
			"server_name": "new-server-name",
		},
	}

	err := adapter.Reload(newConfig)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if adapter.serverName != "new-server-name" {
		t.Errorf("Expected server name to be updated to 'new-server-name', got %q", adapter.serverName)
	}
}

func TestCodexCLIMCPAdapter_Reload_ConfigPath(t *testing.T) {
	tempDir := t.TempDir()
	newPath := filepath.Join(tempDir, "custom-config.toml")

	adapter := NewCodexCLIMCPAdapter()

	newConfig := integrations.IntegrationConfig{
		Settings: map[string]any{
			"config_path": newPath,
		},
	}

	err := adapter.Reload(newConfig)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if adapter.configPath != newPath {
		t.Errorf("Expected config path to be updated to %q, got %q", newPath, adapter.configPath)
	}
}
