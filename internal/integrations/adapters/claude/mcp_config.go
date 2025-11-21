package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MCPConfig represents the Claude Code MCP configuration structure
type MCPConfig struct {
	MCPServers map[string]MCPServer `json:"mcpServers,omitempty"`
}

// MCPServer represents a single MCP server configuration
type MCPServer struct {
	Type    string            `json:"type"`              // stdio, http, sse
	Command string            `json:"command,omitempty"` // For stdio: executable path
	Args    []string          `json:"args,omitempty"`    // For stdio: command arguments
	URL     string            `json:"url,omitempty"`     // For http/sse: server URL
	Env     map[string]string `json:"env,omitempty"`     // Environment variables
}

// readMCPConfig reads the Claude Code MCP configuration file (~/.claude.json)
// Returns the parsed MCP config, the full config map (for preserving other fields), and any error
func readMCPConfig(path string) (*MCPConfig, map[string]any, error) {
	fullConfig := make(map[string]any)

	// If file doesn't exist, return empty config
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &MCPConfig{
			MCPServers: make(map[string]MCPServer),
		}, fullConfig, nil
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read MCP config file; %w", err)
	}

	// Parse into full config map to preserve all fields
	if err := json.Unmarshal(data, &fullConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to parse MCP config JSON; %w", err)
	}

	// Extract mcpServers if they exist
	var config MCPConfig
	if serversData, ok := fullConfig["mcpServers"]; ok {
		serversJSON, err := json.Marshal(serversData)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal mcpServers; %w", err)
		}
		var serversMap map[string]MCPServer
		if err := json.Unmarshal(serversJSON, &serversMap); err != nil {
			return nil, nil, fmt.Errorf("failed to parse mcpServers; %w", err)
		}
		config.MCPServers = serversMap
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]MCPServer)
	}

	return &config, fullConfig, nil
}

// writeMCPConfig writes the Claude Code MCP configuration file
// Performs atomic write with backup
func writeMCPConfig(path string, config *MCPConfig, fullConfig map[string]any) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory; %w", err)
	}

	// Update mcpServers in full config
	fullConfig["mcpServers"] = config.MCPServers

	// Marshal to JSON with pretty printing
	data, err := json.MarshalIndent(fullConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal MCP config; %w", err)
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
		return fmt.Errorf("failed to write MCP config file; %w", err)
	}

	return nil
}
