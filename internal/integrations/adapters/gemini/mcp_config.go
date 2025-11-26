package gemini

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// GeminiMCPConfig represents the Gemini CLI settings.json structure
// Only includes the mcpServers section we care about
type GeminiMCPConfig struct {
	MCPServers map[string]GeminiMCPServer `json:"mcpServers,omitempty"`
}

// GeminiMCPServer represents a single MCP server configuration
// Note: No "type" field - Gemini CLI defaults to stdio transport
type GeminiMCPServer struct {
	Command string            `json:"command,omitempty"` // For stdio: executable path
	Args    []string          `json:"args,omitempty"`    // For stdio: command arguments
	Env     map[string]string `json:"env,omitempty"`     // Environment variables
}

// readMCPConfig reads the Gemini CLI settings.json file
// Returns the parsed MCP config, the full config map (for preserving other fields), and any error
func readMCPConfig(path string) (*GeminiMCPConfig, map[string]any, error) {
	fullConfig := make(map[string]any)

	// If file doesn't exist, return empty config
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &GeminiMCPConfig{
			MCPServers: make(map[string]GeminiMCPServer),
		}, fullConfig, nil
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read settings file; %w", err)
	}

	// Parse into full config map to preserve all fields
	if err := json.Unmarshal(data, &fullConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to parse settings JSON; %w", err)
	}

	// Extract mcpServers if they exist
	var config GeminiMCPConfig
	if serversData, ok := fullConfig["mcpServers"]; ok {
		serversJSON, err := json.Marshal(serversData)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal mcpServers; %w", err)
		}
		var serversMap map[string]GeminiMCPServer
		if err := json.Unmarshal(serversJSON, &serversMap); err != nil {
			return nil, nil, fmt.Errorf("failed to parse mcpServers; %w", err)
		}
		config.MCPServers = serversMap
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]GeminiMCPServer)
	}

	return &config, fullConfig, nil
}

// writeMCPConfig writes the Gemini CLI settings.json file
// Performs atomic write with backup
func writeMCPConfig(path string, config *GeminiMCPConfig, fullConfig map[string]any) error {
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
		return fmt.Errorf("failed to marshal settings; %w", err)
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
		return fmt.Errorf("failed to write settings file; %w", err)
	}

	return nil
}
