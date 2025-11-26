package codex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadMCPConfig_NonExistent(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	config, fullConfig, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Expected no error for non-existent file, got: %v", err)
	}

	if config == nil {
		t.Fatal("Expected config to be non-nil")
	}

	if len(config.MCPServers) != 0 {
		t.Errorf("Expected empty MCPServers map, got %d entries", len(config.MCPServers))
	}

	if len(fullConfig) != 0 {
		t.Errorf("Expected empty fullConfig map, got %d entries", len(fullConfig))
	}
}

func TestReadMCPConfig_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	// Create empty file
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	config, fullConfig, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Expected no error for empty file, got: %v", err)
	}

	if config == nil {
		t.Fatal("Expected config to be non-nil")
	}

	if len(config.MCPServers) != 0 {
		t.Errorf("Expected empty MCPServers map, got %d entries", len(config.MCPServers))
	}

	if len(fullConfig) != 0 {
		t.Errorf("Expected empty fullConfig map, got %d entries", len(fullConfig))
	}
}

func TestReadMCPConfig_ValidConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	tomlContent := `
[mcp_servers.test-server]
command = "/usr/bin/test"
args = ["arg1", "arg2"]

[mcp_servers.test-server.env]
TEST_VAR = "test_value"
`

	if err := os.WriteFile(configPath, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config, fullConfig, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
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

	if len(server.Args) != 2 || server.Args[0] != "arg1" || server.Args[1] != "arg2" {
		t.Errorf("Expected args [arg1, arg2], got %v", server.Args)
	}

	if server.Env["TEST_VAR"] != "test_value" {
		t.Errorf("Expected TEST_VAR='test_value', got '%s'", server.Env["TEST_VAR"])
	}

	if _, ok := fullConfig["mcp_servers"]; !ok {
		t.Error("Expected mcp_servers in fullConfig")
	}
}

func TestReadMCPConfig_WithOptionalFields(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	tomlContent := `
[mcp_servers.test-server]
command = "/usr/bin/test"
args = ["mcp", "start"]
enabled = true
startup_timeout_sec = 20
tool_timeout_sec = 90

[mcp_servers.test-server.env]
VAR = "value"
`

	if err := os.WriteFile(configPath, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config, _, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	server := config.MCPServers["test-server"]

	if server.Enabled == nil || !*server.Enabled {
		t.Error("Expected enabled=true")
	}

	if server.StartupTimeoutSec == nil || *server.StartupTimeoutSec != 20 {
		t.Errorf("Expected startup_timeout_sec=20, got %v", server.StartupTimeoutSec)
	}

	if server.ToolTimeoutSec == nil || *server.ToolTimeoutSec != 90 {
		t.Errorf("Expected tool_timeout_sec=90, got %v", server.ToolTimeoutSec)
	}
}

func TestReadMCPConfig_PreservesOtherFields(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	tomlContent := `
other_setting = "value"
number_setting = 42

[other_section]
key = "value"

[mcp_servers.test-server]
command = "/usr/bin/test"
`

	if err := os.WriteFile(configPath, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, fullConfig, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if fullConfig["other_setting"] != "value" {
		t.Error("Expected other_setting to be preserved")
	}

	if fullConfig["number_setting"] != int64(42) {
		t.Error("Expected number_setting to be preserved")
	}

	if _, ok := fullConfig["other_section"]; !ok {
		t.Error("Expected other_section to be preserved")
	}
}

func TestWriteMCPConfig_CreatesDirectory(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "subdir", "config.toml")

	config := &CodexMCPConfig{
		MCPServers: map[string]CodexMCPServer{
			"test-server": {
				Command: "/usr/bin/test",
				Args:    []string{"mcp", "start"},
			},
		},
	}

	fullConfig := make(map[string]any)

	err := writeMCPConfig(configPath, config, fullConfig)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(filepath.Dir(configPath)); os.IsNotExist(err) {
		t.Error("Expected directory to be created")
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Expected config file to be created")
	}
}

func TestWriteMCPConfig_AtomicWrite(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	// Create initial config
	initialContent := `
[mcp_servers.existing]
command = "/usr/bin/existing"
`
	if err := os.WriteFile(configPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to create initial file: %v", err)
	}

	// Write new config
	config := &CodexMCPConfig{
		MCPServers: map[string]CodexMCPServer{
			"test-server": {
				Command: "/usr/bin/test",
				Args:    []string{"mcp", "start"},
			},
		},
	}

	fullConfig := make(map[string]any)

	err := writeMCPConfig(configPath, config, fullConfig)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify backup was removed
	backupPath := configPath + ".backup"
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("Expected backup file to be removed")
	}

	// Verify new content
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	content := string(data)
	if len(content) == 0 {
		t.Error("Expected non-empty config file")
	}
}

func TestWriteMCPConfig_PreservesOtherFields(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	fullConfig := map[string]any{
		"other_setting":  "preserved",
		"number_setting": 42,
		"other_section": map[string]any{
			"key": "value",
		},
	}

	config := &CodexMCPConfig{
		MCPServers: map[string]CodexMCPServer{
			"test-server": {
				Command: "/usr/bin/test",
			},
		},
	}

	err := writeMCPConfig(configPath, config, fullConfig)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Read back and verify
	readConfig, readFullConfig, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	if readFullConfig["other_setting"] != "preserved" {
		t.Error("Expected other_setting to be preserved")
	}

	if readFullConfig["number_setting"] != int64(42) {
		t.Error("Expected number_setting to be preserved")
	}

	if _, ok := readFullConfig["other_section"]; !ok {
		t.Error("Expected other_section to be preserved")
	}

	if _, ok := readConfig.MCPServers["test-server"]; !ok {
		t.Error("Expected test-server to exist")
	}
}

func TestWriteMCPConfig_NestedEnvTable(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	enabled := true
	config := &CodexMCPConfig{
		MCPServers: map[string]CodexMCPServer{
			"test-server": {
				Command: "/usr/bin/test",
				Args:    []string{"mcp", "start"},
				Enabled: &enabled,
				Env: map[string]string{
					"VAR1": "value1",
					"VAR2": "value2",
				},
			},
		},
	}

	fullConfig := make(map[string]any)

	err := writeMCPConfig(configPath, config, fullConfig)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Read back and verify env vars
	readConfig, _, err := readMCPConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	server := readConfig.MCPServers["test-server"]
	if server.Env["VAR1"] != "value1" {
		t.Errorf("Expected VAR1='value1', got '%s'", server.Env["VAR1"])
	}

	if server.Env["VAR2"] != "value2" {
		t.Errorf("Expected VAR2='value2', got '%s'", server.Env["VAR2"])
	}
}
