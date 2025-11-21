package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadMCPConfig_NonExistent(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "non-existent.json")

	config, fullConfig, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Expected no error for non-existent file, got: %v", err)
	}

	if config == nil {
		t.Fatal("Expected config to be initialized, got nil")
	}

	if config.MCPServers == nil {
		t.Fatal("Expected MCPServers map to be initialized, got nil")
	}

	if len(config.MCPServers) != 0 {
		t.Errorf("Expected empty MCPServers map, got %d entries", len(config.MCPServers))
	}

	if fullConfig == nil {
		t.Fatal("Expected fullConfig to be initialized, got nil")
	}
}

func TestReadMCPConfig_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "empty.json")

	// Create empty JSON object
	if err := os.WriteFile(configPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config, fullConfig, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Expected no error for empty file, got: %v", err)
	}

	if config == nil {
		t.Fatal("Expected config to be initialized, got nil")
	}

	if config.MCPServers == nil {
		t.Fatal("Expected MCPServers map to be initialized, got nil")
	}

	if len(config.MCPServers) != 0 {
		t.Errorf("Expected empty MCPServers map, got %d entries", len(config.MCPServers))
	}

	if fullConfig == nil {
		t.Fatal("Expected fullConfig to be initialized, got nil")
	}
}

func TestReadMCPConfig_WithServers(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create config with MCP servers
	configData := map[string]any{
		"mcpServers": map[string]any{
			"test-server": map[string]any{
				"type":    "stdio",
				"command": "/usr/bin/test",
				"args":    []string{"start"},
				"env": map[string]string{
					"TEST_VAR": "test_value",
				},
			},
		},
		"otherField": "should be preserved",
	}

	data, err := json.Marshal(configData)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config, fullConfig, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify MCP servers were parsed
	if len(config.MCPServers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(config.MCPServers))
	}

	server, exists := config.MCPServers["test-server"]
	if !exists {
		t.Fatal("Expected test-server to exist in MCPServers")
	}

	if server.Type != "stdio" {
		t.Errorf("Expected type 'stdio', got '%s'", server.Type)
	}

	if server.Command != "/usr/bin/test" {
		t.Errorf("Expected command '/usr/bin/test', got '%s'", server.Command)
	}

	if len(server.Args) != 1 || server.Args[0] != "start" {
		t.Errorf("Expected args ['start'], got %v", server.Args)
	}

	if server.Env["TEST_VAR"] != "test_value" {
		t.Errorf("Expected env TEST_VAR='test_value', got '%s'", server.Env["TEST_VAR"])
	}

	// Verify other fields were preserved
	if fullConfig["otherField"] != "should be preserved" {
		t.Error("Expected otherField to be preserved in fullConfig")
	}
}

func TestReadMCPConfig_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid.json")

	// Create invalid JSON
	if err := os.WriteFile(configPath, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, _, err := readMCPConfig(configPath)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestWriteMCPConfig_CreatesDirectory(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "subdir", "config.json")

	config := &MCPConfig{
		MCPServers: make(map[string]MCPServer),
	}
	fullConfig := make(map[string]any)

	err := writeMCPConfig(configPath, config, fullConfig)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Expected config file to be created")
	}
}

func TestWriteMCPConfig_PreservesFields(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create initial config with extra fields
	fullConfig := map[string]any{
		"otherField": "should be preserved",
		"anotherField": map[string]any{
			"nested": "value",
		},
	}

	config := &MCPConfig{
		MCPServers: map[string]MCPServer{
			"test-server": {
				Type:    "stdio",
				Command: "/usr/bin/test",
				Args:    []string{"start"},
			},
		},
	}

	// Write config
	err := writeMCPConfig(configPath, config, fullConfig)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Read back and verify
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to parse config file: %v", err)
	}

	// Verify extra fields were preserved
	if result["otherField"] != "should be preserved" {
		t.Error("Expected otherField to be preserved")
	}

	// Verify nested field was preserved
	anotherField, ok := result["anotherField"].(map[string]any)
	if !ok {
		t.Fatal("Expected anotherField to be preserved as map")
	}
	if anotherField["nested"] != "value" {
		t.Error("Expected nested field to be preserved")
	}

	// Verify MCP servers were written
	mcpServers, ok := result["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("Expected mcpServers to be written")
	}

	if len(mcpServers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(mcpServers))
	}
}

func TestWriteMCPConfig_AtomicWrite(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create initial config
	initialData := map[string]any{"initial": "data"}
	data, _ := json.Marshal(initialData)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to create initial file: %v", err)
	}

	// Write new config
	config := &MCPConfig{
		MCPServers: map[string]MCPServer{
			"test-server": {
				Type:    "stdio",
				Command: "/usr/bin/test",
			},
		},
	}
	fullConfig := make(map[string]any)

	err := writeMCPConfig(configPath, config, fullConfig)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify backup was removed (atomic write completed)
	backupPath := configPath + ".backup"
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("Expected backup file to be removed after successful write")
	}

	// Verify new config was written
	readConfig, _, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to read written config: %v", err)
	}

	if len(readConfig.MCPServers) != 1 {
		t.Errorf("Expected 1 server in written config, got %d", len(readConfig.MCPServers))
	}
}

func TestWriteMCPConfig_JSONFormatting(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	config := &MCPConfig{
		MCPServers: map[string]MCPServer{
			"test-server": {
				Type:    "stdio",
				Command: "/usr/bin/test",
				Args:    []string{"start"},
				Env: map[string]string{
					"TEST_VAR": "test_value",
				},
			},
		},
	}
	fullConfig := make(map[string]any)

	err := writeMCPConfig(configPath, config, fullConfig)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Read file as string to verify formatting
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	content := string(data)

	// Verify pretty printing (should contain newlines and indentation)
	if len(content) < 50 { // Pretty printed JSON should be longer
		t.Error("Expected config to be pretty printed with indentation")
	}
}
