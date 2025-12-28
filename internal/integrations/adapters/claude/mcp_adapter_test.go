package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

func TestNewClaudeCodeMCPAdapter(t *testing.T) {
	adapter := NewClaudeCodeMCPAdapter()
	if adapter == nil {
		t.Fatal("Expected adapter to be initialized, got nil")
	}

	if adapter.configPath == "" {
		t.Error("Expected configPath to be set")
	}

	if adapter.serverName != MCPServerName {
		t.Errorf("Expected serverName '%s', got '%s'", MCPServerName, adapter.serverName)
	}
}

func TestClaudeCodeMCPAdapter_GetName(t *testing.T) {
	adapter := NewClaudeCodeMCPAdapter()
	if adapter.GetName() != MCPIntegrationName {
		t.Errorf("Expected name '%s', got '%s'", MCPIntegrationName, adapter.GetName())
	}
}

func TestClaudeCodeMCPAdapter_GetDescription(t *testing.T) {
	adapter := NewClaudeCodeMCPAdapter()
	description := adapter.GetDescription()
	if description == "" {
		t.Error("Expected non-empty description")
	}
}

func TestClaudeCodeMCPAdapter_GetVersion(t *testing.T) {
	adapter := NewClaudeCodeMCPAdapter()
	if adapter.GetVersion() != MCPIntegrationVersion {
		t.Errorf("Expected version '%s', got '%s'", MCPIntegrationVersion, adapter.GetVersion())
	}
}

func TestClaudeCodeMCPAdapter_IsEnabled_NoConfig(t *testing.T) {
	tempDir := t.TempDir()
	adapter := &ClaudeCodeMCPAdapter{
		configPath: filepath.Join(tempDir, "non-existent.json"),
		serverName: MCPServerName,
	}

	enabled, err := adapter.IsEnabled()
	if err != nil {
		t.Fatalf("Expected no error for non-existent config, got: %v", err)
	}

	if enabled {
		t.Error("Expected adapter to not be enabled when config doesn't exist")
	}
}

func TestClaudeCodeMCPAdapter_IsEnabled_EmptyConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create empty config
	if err := os.WriteFile(configPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &ClaudeCodeMCPAdapter{
		configPath: configPath,
		serverName: MCPServerName,
	}

	enabled, err := adapter.IsEnabled()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if enabled {
		t.Error("Expected adapter to not be enabled when server not in config")
	}
}

func TestClaudeCodeMCPAdapter_IsEnabled_ServerExists(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create config with our server
	configData := map[string]any{
		"mcpServers": map[string]any{
			MCPServerName: map[string]any{
				"type":    "stdio",
				"command": "/usr/bin/test",
				"args":    []string{"mcp", "start"},
			},
		},
	}

	data, _ := json.Marshal(configData)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &ClaudeCodeMCPAdapter{
		configPath: configPath,
		serverName: MCPServerName,
	}

	enabled, err := adapter.IsEnabled()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !enabled {
		t.Error("Expected adapter to be enabled when server exists in config")
	}
}

func TestClaudeCodeMCPAdapter_Setup(t *testing.T) {
	// Create temporary config directory
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create temporary config file for agentic-memorizer
	tempConfigDir := t.TempDir()
	memorizerConfigPath := filepath.Join(tempConfigDir, "config.yaml")
	memoryRoot := filepath.Join(tempConfigDir, "memory")

	// Create minimal config with memory root
	configContent := "memory_root: " + memoryRoot + "\n"
	if err := os.WriteFile(memorizerConfigPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Set environment variable to point to test config
	oldAppDir := os.Getenv("MEMORIZER_APP_DIR")
	os.Setenv("MEMORIZER_APP_DIR", tempConfigDir)
	defer os.Setenv("MEMORIZER_APP_DIR", oldAppDir)

	// Reset config and initialize with new path
	config.ResetForTesting()
	if err := config.InitConfig(); err != nil {
		t.Fatalf("Failed to initialize config: %v", err)
	}

	adapter := &ClaudeCodeMCPAdapter{
		configPath: configPath,
		serverName: MCPServerName,
	}

	binaryPath := "/usr/local/bin/memorizer"
	err := adapter.Setup(binaryPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify config was written
	mcpConfig, _, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to read config after setup: %v", err)
	}

	server, exists := mcpConfig.MCPServers[MCPServerName]
	if !exists {
		t.Fatal("Expected server to exist after setup")
	}

	if server.Type != "stdio" {
		t.Errorf("Expected type 'stdio', got '%s'", server.Type)
	}

	if server.Command != binaryPath {
		t.Errorf("Expected command '%s', got '%s'", binaryPath, server.Command)
	}

	if len(server.Args) != 2 || server.Args[0] != "mcp" || server.Args[1] != "start" {
		t.Errorf("Expected args ['mcp', 'start'], got %v", server.Args)
	}

	if server.Env["MEMORIZER_MEMORY_ROOT"] != memoryRoot {
		t.Errorf("Expected MEMORIZER_MEMORY_ROOT='%s', got '%s'", memoryRoot, server.Env["MEMORIZER_MEMORY_ROOT"])
	}
}

func TestClaudeCodeMCPAdapter_Update(t *testing.T) {
	// Update should behave same as Setup
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create temporary config file for agentic-memorizer
	tempConfigDir := t.TempDir()
	memorizerConfigPath := filepath.Join(tempConfigDir, "config.yaml")
	memoryRoot := filepath.Join(tempConfigDir, "memory")

	configContent := "memory_root: " + memoryRoot + "\n"
	if err := os.WriteFile(memorizerConfigPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	oldAppDir := os.Getenv("MEMORIZER_APP_DIR")
	os.Setenv("MEMORIZER_APP_DIR", tempConfigDir)
	defer os.Setenv("MEMORIZER_APP_DIR", oldAppDir)

	config.ResetForTesting()
	if err := config.InitConfig(); err != nil {
		t.Fatalf("Failed to initialize config: %v", err)
	}

	adapter := &ClaudeCodeMCPAdapter{
		configPath: configPath,
		serverName: MCPServerName,
	}

	binaryPath := "/usr/local/bin/memorizer"
	err := adapter.Update(binaryPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify config was written
	mcpConfig, _, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to read config after update: %v", err)
	}

	if _, exists := mcpConfig.MCPServers[MCPServerName]; !exists {
		t.Fatal("Expected server to exist after update")
	}
}

func TestClaudeCodeMCPAdapter_Remove(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create config with our server
	configData := map[string]any{
		"mcpServers": map[string]any{
			MCPServerName: map[string]any{
				"type":    "stdio",
				"command": "/usr/bin/test",
				"args":    []string{"mcp", "start"},
			},
			"other-server": map[string]any{
				"type": "stdio",
			},
		},
	}

	data, _ := json.Marshal(configData)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &ClaudeCodeMCPAdapter{
		configPath: configPath,
		serverName: MCPServerName,
	}

	err := adapter.Remove()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify our server was removed
	mcpConfig, _, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to read config after remove: %v", err)
	}

	if _, exists := mcpConfig.MCPServers[MCPServerName]; exists {
		t.Error("Expected server to be removed")
	}

	// Verify other server still exists
	if _, exists := mcpConfig.MCPServers["other-server"]; !exists {
		t.Error("Expected other-server to still exist")
	}
}

func TestClaudeCodeMCPAdapter_GetCommand(t *testing.T) {
	adapter := NewClaudeCodeMCPAdapter()
	binaryPath := "/usr/local/bin/memorizer"

	command := adapter.GetCommand(binaryPath, integrations.FormatXML)
	if command == "" {
		t.Error("Expected non-empty command")
	}

	// Command should contain binary path and mcp start
	if command != binaryPath+" mcp start" {
		t.Errorf("Expected command '%s mcp start', got '%s'", binaryPath, command)
	}
}

func TestClaudeCodeMCPAdapter_FormatOutput(t *testing.T) {
	adapter := NewClaudeCodeMCPAdapter()

	// FormatOutput should return an error for MCP integrations
	_, err := adapter.FormatOutput(&types.FileIndex{}, integrations.FormatXML)
	if err == nil {
		t.Error("Expected error from FormatOutput, got nil")
	}
}

func TestClaudeCodeMCPAdapter_Validate_NoConfig(t *testing.T) {
	tempDir := t.TempDir()
	adapter := &ClaudeCodeMCPAdapter{
		configPath: filepath.Join(tempDir, "non-existent.json"),
		serverName: MCPServerName,
	}

	err := adapter.Validate()
	if err == nil {
		t.Error("Expected error for non-existent config")
	}
}

func TestClaudeCodeMCPAdapter_Validate_NoServer(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create empty config
	if err := os.WriteFile(configPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &ClaudeCodeMCPAdapter{
		configPath: configPath,
		serverName: MCPServerName,
	}

	err := adapter.Validate()
	if err == nil {
		t.Error("Expected error when server not found")
	}
}

func TestClaudeCodeMCPAdapter_Validate_InvalidType(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create config with wrong type
	configData := map[string]any{
		"mcpServers": map[string]any{
			MCPServerName: map[string]any{
				"type":    "http",
				"command": "/usr/bin/test",
				"args":    []string{"mcp", "start"},
			},
		},
	}

	data, _ := json.Marshal(configData)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &ClaudeCodeMCPAdapter{
		configPath: configPath,
		serverName: MCPServerName,
	}

	err := adapter.Validate()
	if err == nil {
		t.Error("Expected error for invalid type")
	}
}

func TestClaudeCodeMCPAdapter_Validate_MissingCommand(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create config without command
	configData := map[string]any{
		"mcpServers": map[string]any{
			MCPServerName: map[string]any{
				"type": "stdio",
				"args": []string{"mcp", "start"},
			},
		},
	}

	data, _ := json.Marshal(configData)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &ClaudeCodeMCPAdapter{
		configPath: configPath,
		serverName: MCPServerName,
	}

	err := adapter.Validate()
	if err == nil {
		t.Error("Expected error for missing command")
	}
}

func TestClaudeCodeMCPAdapter_Validate_InvalidArgs(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create config with wrong args
	configData := map[string]any{
		"mcpServers": map[string]any{
			MCPServerName: map[string]any{
				"type":    "stdio",
				"command": "/usr/bin/test",
				"args":    []string{"wrong", "args"},
			},
		},
	}

	data, _ := json.Marshal(configData)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &ClaudeCodeMCPAdapter{
		configPath: configPath,
		serverName: MCPServerName,
	}

	err := adapter.Validate()
	if err == nil {
		t.Error("Expected error for invalid args")
	}
}

func TestClaudeCodeMCPAdapter_Validate_BinaryNotFound(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create config with non-existent binary
	configData := map[string]any{
		"mcpServers": map[string]any{
			MCPServerName: map[string]any{
				"type":    "stdio",
				"command": "/non/existent/binary",
				"args":    []string{"mcp", "start"},
			},
		},
	}

	data, _ := json.Marshal(configData)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &ClaudeCodeMCPAdapter{
		configPath: configPath,
		serverName: MCPServerName,
	}

	err := adapter.Validate()
	if err == nil {
		t.Error("Expected error for non-existent binary")
	}
}

func TestClaudeCodeMCPAdapter_Validate_Success(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create a temporary binary file
	binaryPath := filepath.Join(tempDir, "test-binary")
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	// Create valid config
	configData := map[string]any{
		"mcpServers": map[string]any{
			MCPServerName: map[string]any{
				"type":    "stdio",
				"command": binaryPath,
				"args":    []string{"mcp", "start"},
			},
		},
	}

	data, _ := json.Marshal(configData)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	adapter := &ClaudeCodeMCPAdapter{
		configPath: configPath,
		serverName: MCPServerName,
	}

	err := adapter.Validate()
	if err != nil {
		t.Errorf("Expected no error for valid config, got: %v", err)
	}
}

func TestClaudeCodeMCPAdapter_Reload_ServerName(t *testing.T) {
	adapter := NewClaudeCodeMCPAdapter()
	originalServerName := adapter.serverName

	newConfig := integrations.IntegrationConfig{
		Settings: map[string]any{
			"server_name": "custom-server-name",
		},
	}

	err := adapter.Reload(newConfig)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if adapter.serverName == originalServerName {
		t.Error("Expected server name to be updated")
	}

	if adapter.serverName != "custom-server-name" {
		t.Errorf("Expected server name 'custom-server-name', got '%s'", adapter.serverName)
	}
}

func TestClaudeCodeMCPAdapter_Reload_ConfigPath(t *testing.T) {
	adapter := NewClaudeCodeMCPAdapter()
	originalConfigPath := adapter.configPath

	newConfig := integrations.IntegrationConfig{
		Settings: map[string]any{
			"config_path": "~/.test-claude.json",
		},
	}

	err := adapter.Reload(newConfig)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if adapter.configPath == originalConfigPath {
		t.Error("Expected config path to be updated")
	}

	// Should expand tilde
	if adapter.configPath == "~/.test-claude.json" {
		t.Error("Expected config path to be expanded")
	}
}

func TestClaudeCodeMCPAdapter_Reload_EmptySettings(t *testing.T) {
	adapter := NewClaudeCodeMCPAdapter()
	originalServerName := adapter.serverName
	originalConfigPath := adapter.configPath

	newConfig := integrations.IntegrationConfig{
		Settings: map[string]any{},
	}

	err := adapter.Reload(newConfig)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Nothing should change
	if adapter.serverName != originalServerName {
		t.Error("Expected server name to remain unchanged")
	}

	if adapter.configPath != originalConfigPath {
		t.Error("Expected config path to remain unchanged")
	}
}
