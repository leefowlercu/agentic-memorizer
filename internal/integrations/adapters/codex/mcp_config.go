package codex

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// CodexMCPConfig represents the Codex CLI config.toml structure
// Only includes the mcp_servers section we care about
type CodexMCPConfig struct {
	MCPServers map[string]CodexMCPServer `toml:"mcp_servers,omitempty"`
}

// CodexMCPServer represents a single MCP server configuration
// Codex CLI uses TOML with environment variables in nested table
type CodexMCPServer struct {
	Command           string            `toml:"command,omitempty"`
	Args              []string          `toml:"args,omitempty"`
	Enabled           *bool             `toml:"enabled,omitempty"` // pointer for optional
	StartupTimeoutSec *int              `toml:"startup_timeout_sec,omitempty"`
	ToolTimeoutSec    *int              `toml:"tool_timeout_sec,omitempty"`
	Env               map[string]string `toml:"env,omitempty"`
}

// readMCPConfig reads ~/.codex/config.toml
// Returns: parsed config, full config map (for preserving other fields), error
func readMCPConfig(path string) (*CodexMCPConfig, map[string]any, error) {
	fullConfig := make(map[string]any)

	// If file doesn't exist, return empty config
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &CodexMCPConfig{
			MCPServers: make(map[string]CodexMCPServer),
		}, fullConfig, nil
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read config file; %w", err)
	}

	// Parse into full config map to preserve all fields
	if err := toml.Unmarshal(data, &fullConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to parse TOML; %w", err)
	}

	// Extract mcp_servers if they exist
	var config CodexMCPConfig
	if serversData, ok := fullConfig["mcp_servers"]; ok {
		serversBytes, err := toml.Marshal(map[string]any{"mcp_servers": serversData})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal mcp_servers; %w", err)
		}
		if err := toml.Unmarshal(serversBytes, &config); err != nil {
			return nil, nil, fmt.Errorf("failed to parse mcp_servers; %w", err)
		}
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]CodexMCPServer)
	}

	return &config, fullConfig, nil
}

// writeMCPConfig writes ~/.codex/config.toml with atomic write
// Merges mcp_servers into fullConfig to preserve other settings
func writeMCPConfig(path string, config *CodexMCPConfig, fullConfig map[string]any) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory; %w", err)
	}

	// Update mcp_servers in full config
	fullConfig["mcp_servers"] = config.MCPServers

	// Marshal to TOML
	data, err := toml.Marshal(fullConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config; %w", err)
	}

	// Create backup if file exists
	if _, err := os.Stat(path); err == nil {
		backupPath := path + ".backup"
		if err := os.Rename(path, backupPath); err != nil {
			return fmt.Errorf("failed to create backup; %w", err)
		}
		// Remove backup on success
		defer os.Remove(backupPath)
	}

	// Write file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file; %w", err)
	}

	return nil
}
