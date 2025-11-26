package gemini

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
		t.Fatal("Expected config to be non-nil")
	}

	if config.MCPServers == nil {
		t.Fatal("Expected MCPServers to be initialized")
	}

	if len(config.MCPServers) != 0 {
		t.Errorf("Expected empty MCPServers, got %d entries", len(config.MCPServers))
	}

	if len(fullConfig) != 0 {
		t.Errorf("Expected empty fullConfig, got %d entries", len(fullConfig))
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
		t.Fatalf("Expected no error, got: %v", err)
	}

	if config == nil {
		t.Fatal("Expected config to be non-nil")
	}

	if config.MCPServers == nil {
		t.Fatal("Expected MCPServers to be initialized")
	}

	if len(config.MCPServers) != 0 {
		t.Errorf("Expected empty MCPServers, got %d entries", len(config.MCPServers))
	}

	if len(fullConfig) != 0 {
		t.Errorf("Expected fullConfig with empty object, got %d entries", len(fullConfig))
	}
}

func TestReadMCPConfig_ValidConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "valid.json")

	// Create valid config with MCP servers
	configData := map[string]any{
		"mcpServers": map[string]any{
			"test-server": map[string]any{
				"command": "/usr/bin/test",
				"args":    []string{"arg1", "arg2"},
				"env": map[string]any{
					"TEST_VAR": "value",
				},
			},
		},
	}

	data, _ := json.Marshal(configData)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config, fullConfig, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if config == nil {
		t.Fatal("Expected config to be non-nil")
	}

	if len(config.MCPServers) != 1 {
		t.Fatalf("Expected 1 server, got %d", len(config.MCPServers))
	}

	server, exists := config.MCPServers["test-server"]
	if !exists {
		t.Fatal("Expected test-server to exist")
	}

	if server.Command != "/usr/bin/test" {
		t.Errorf("Expected command '/usr/bin/test', got '%s'", server.Command)
	}

	if len(server.Args) != 2 {
		t.Errorf("Expected 2 args, got %d", len(server.Args))
	}

	if server.Env["TEST_VAR"] != "value" {
		t.Errorf("Expected TEST_VAR='value', got '%s'", server.Env["TEST_VAR"])
	}

	if len(fullConfig) == 0 {
		t.Error("Expected fullConfig to be populated")
	}
}

func TestReadMCPConfig_PreservesOtherFields(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "other-fields.json")

	// Create config with non-MCP fields
	configData := map[string]any{
		"model": map[string]any{
			"name": "gemini-2.5-pro",
		},
		"mcpServers": map[string]any{
			"test-server": map[string]any{
				"command": "/usr/bin/test",
			},
		},
		"context": map[string]any{
			"fileName": []string{"GEMINI.md"},
		},
	}

	data, _ := json.Marshal(configData)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config, fullConfig, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(config.MCPServers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(config.MCPServers))
	}

	// Verify other fields are preserved in fullConfig
	if _, exists := fullConfig["model"]; !exists {
		t.Error("Expected model field to be preserved")
	}

	if _, exists := fullConfig["context"]; !exists {
		t.Error("Expected context field to be preserved")
	}
}

func TestWriteMCPConfig_CreatesDirectory(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "subdir", "settings.json")

	config := &GeminiMCPConfig{
		MCPServers: map[string]GeminiMCPServer{
			"test-server": {
				Command: "/usr/bin/test",
			},
		},
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

	// Verify directory was created
	dir := filepath.Dir(configPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("Expected directory to be created")
	}
}

func TestWriteMCPConfig_AtomicWrite(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "settings.json")

	// Create initial config
	initialData := map[string]any{
		"mcpServers": map[string]any{
			"initial-server": map[string]any{
				"command": "/usr/bin/initial",
			},
		},
	}
	data, _ := json.Marshal(initialData)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to create initial file: %v", err)
	}

	// Write new config
	config := &GeminiMCPConfig{
		MCPServers: map[string]GeminiMCPServer{
			"new-server": {
				Command: "/usr/bin/new",
			},
		},
	}

	fullConfig := make(map[string]any)

	err := writeMCPConfig(configPath, config, fullConfig)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify backup was removed (cleanup successful)
	backupPath := configPath + ".backup"
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("Expected backup file to be removed after successful write")
	}

	// Verify new config was written
	newConfig, _, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to read new config: %v", err)
	}

	if _, exists := newConfig.MCPServers["new-server"]; !exists {
		t.Error("Expected new-server to exist")
	}

	if _, exists := newConfig.MCPServers["initial-server"]; exists {
		t.Error("Expected initial-server to be replaced")
	}
}

func TestWriteMCPConfig_PreservesOtherFields(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "settings.json")

	// Start with config that has non-MCP fields
	fullConfig := map[string]any{
		"model": map[string]any{
			"name": "gemini-2.5-pro",
		},
		"context": map[string]any{
			"fileName": []string{"GEMINI.md"},
		},
	}

	config := &GeminiMCPConfig{
		MCPServers: map[string]GeminiMCPServer{
			"test-server": {
				Command: "/usr/bin/test",
			},
		},
	}

	err := writeMCPConfig(configPath, config, fullConfig)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Read back and verify all fields are present
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	if _, exists := result["model"]; !exists {
		t.Error("Expected model field to be preserved")
	}

	if _, exists := result["context"]; !exists {
		t.Error("Expected context field to be preserved")
	}

	if _, exists := result["mcpServers"]; !exists {
		t.Error("Expected mcpServers field to exist")
	}
}
