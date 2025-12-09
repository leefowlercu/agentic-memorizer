package claude

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

const (
	// MCPIntegrationName is the unique identifier for this integration
	MCPIntegrationName = "claude-code-mcp"

	// MCPIntegrationVersion is the adapter version
	MCPIntegrationVersion = "2.0.0"

	// MCPServerName is the identifier used in ~/.claude.json
	MCPServerName = "memorizer"
)

// ClaudeCodeMCPAdapter implements the Integration interface for Claude Code MCP server
type ClaudeCodeMCPAdapter struct {
	configPath string
	serverName string
}

// NewClaudeCodeMCPAdapter creates a new Claude Code MCP integration adapter
func NewClaudeCodeMCPAdapter() *ClaudeCodeMCPAdapter {
	return &ClaudeCodeMCPAdapter{
		configPath: getDefaultMCPConfigPath(),
		serverName: MCPServerName,
	}
}

// GetName returns the integration name
func (a *ClaudeCodeMCPAdapter) GetName() string {
	return MCPIntegrationName
}

// GetDescription returns a human-readable description
func (a *ClaudeCodeMCPAdapter) GetDescription() string {
	return "Claude Code MCP server integration"
}

// GetVersion returns the adapter version
func (a *ClaudeCodeMCPAdapter) GetVersion() string {
	return MCPIntegrationVersion
}

// Detect checks if Claude Code with MCP support is installed on the system
func (a *ClaudeCodeMCPAdapter) Detect() (bool, error) {
	// Check if ~/.claude.json exists
	if _, err := os.Stat(a.configPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check MCP config file; %w", err)
	}

	// Check if `claude` CLI is available
	if _, err := exec.LookPath("claude"); err != nil {
		return false, nil
	}

	return true, nil
}

// IsEnabled checks if the integration is currently configured
func (a *ClaudeCodeMCPAdapter) IsEnabled() (bool, error) {
	// Check if config file exists
	if _, err := os.Stat(a.configPath); os.IsNotExist(err) {
		return false, nil
	}

	// Read config and check for agentic-memorizer server
	mcpConfig, _, err := readMCPConfig(a.configPath)
	if err != nil {
		return false, fmt.Errorf("failed to read MCP config; %w", err)
	}

	// Check if our server is registered
	_, exists := mcpConfig.MCPServers[a.serverName]
	return exists, nil
}

// Setup configures the Claude Code MCP integration
func (a *ClaudeCodeMCPAdapter) Setup(binaryPath string) error {
	mcpConfig, fullConfig, err := readMCPConfig(a.configPath)
	if err != nil {
		return fmt.Errorf("failed to read MCP config; %w", err)
	}

	// Get memory root from config for environment variable
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get configuration; %w", err)
	}

	// Create MCP server entry
	server := MCPServer{
		Type:    "stdio",
		Command: binaryPath,
		Args:    []string{"mcp", "start"},
		Env: map[string]string{
			"MEMORIZER_MEMORY_ROOT": cfg.MemoryRoot,
		},
	}

	// Add or update server
	mcpConfig.MCPServers[a.serverName] = server

	if err := writeMCPConfig(a.configPath, mcpConfig, fullConfig); err != nil {
		return fmt.Errorf("failed to write MCP config; %w", err)
	}

	return nil
}

// Update updates the integration configuration
func (a *ClaudeCodeMCPAdapter) Update(binaryPath string) error {
	// For Claude Code MCP, update is the same as setup
	return a.Setup(binaryPath)
}

// Remove removes the integration configuration
func (a *ClaudeCodeMCPAdapter) Remove() error {
	mcpConfig, fullConfig, err := readMCPConfig(a.configPath)
	if err != nil {
		return fmt.Errorf("failed to read MCP config; %w", err)
	}

	// Remove our server if it exists
	delete(mcpConfig.MCPServers, a.serverName)

	if err := writeMCPConfig(a.configPath, mcpConfig, fullConfig); err != nil {
		return fmt.Errorf("failed to write MCP config; %w", err)
	}

	return nil
}

// GetCommand returns the command that should be executed by Claude Code
func (a *ClaudeCodeMCPAdapter) GetCommand(binaryPath string, format integrations.OutputFormat) string {
	// MCP servers ignore format parameter - format selection happens via resource URIs
	// This method is primarily for documentation/display purposes
	return fmt.Sprintf("%s mcp start", binaryPath)
}

// FormatOutput is not used by MCP integrations
// MCP servers provide output through resources and tools, not direct formatting
func (a *ClaudeCodeMCPAdapter) FormatOutput(index *types.GraphIndex, format integrations.OutputFormat) (string, error) {
	return "", fmt.Errorf("MCP integrations do not use FormatOutput; output is provided through MCP resources and tools")
}

// Validate checks the health of the integration
func (a *ClaudeCodeMCPAdapter) Validate() error {
	// Check if config file exists
	if _, err := os.Stat(a.configPath); os.IsNotExist(err) {
		return fmt.Errorf("MCP config file not found at %s", a.configPath)
	}

	// Try to read config
	mcpConfig, _, err := readMCPConfig(a.configPath)
	if err != nil {
		return fmt.Errorf("failed to read MCP config; %w", err)
	}

	// Check if our server is configured
	server, exists := mcpConfig.MCPServers[a.serverName]
	if !exists {
		return fmt.Errorf("MCP server %q not found in configuration", a.serverName)
	}

	// Validate server configuration
	if server.Type != "stdio" {
		return fmt.Errorf("MCP server has invalid type %q (expected stdio)", server.Type)
	}

	if server.Command == "" {
		return fmt.Errorf("MCP server missing command")
	}

	if len(server.Args) == 0 || server.Args[0] != "mcp" {
		return fmt.Errorf("MCP server has invalid arguments")
	}

	// Check for old binary name and reject
	if strings.Contains(server.Command, "agentic-memorizer") {
		return fmt.Errorf("integration uses old binary name 'agentic-memorizer'; run 'memorizer integrations remove %s && memorizer integrations setup %s'",
			MCPIntegrationName, MCPIntegrationName)
	}

	// Check if binary exists
	if _, err := os.Stat(server.Command); os.IsNotExist(err) {
		return fmt.Errorf("MCP server binary not found at %s", server.Command)
	}

	return nil
}

// Reload applies configuration changes
func (a *ClaudeCodeMCPAdapter) Reload(newConfig integrations.IntegrationConfig) error {
	// Update server name if provided
	if serverName, ok := newConfig.Settings["server_name"].(string); ok && serverName != "" {
		a.serverName = serverName
	}

	// Update config path if provided
	if configPath, ok := newConfig.Settings["config_path"].(string); ok && configPath != "" {
		expanded, err := integrations.ExpandPath(configPath)
		if err != nil {
			return fmt.Errorf("invalid config path; %w", err)
		}
		a.configPath = expanded
	}

	return nil
}

// getDefaultMCPConfigPath returns the default path to Claude Code MCP configuration
func getDefaultMCPConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude.json")
}

// VerifyRegistration verifies the MCP server is properly registered
// This can be called after setup to confirm the configuration
func (a *ClaudeCodeMCPAdapter) VerifyRegistration() error {
	// Try running `claude mcp get` to verify registration
	cmd := exec.Command("claude", "mcp", "get", a.serverName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("MCP server not registered (claude mcp get failed); %w; output: %s", err, string(output))
	}

	// Check output contains our server name
	if !strings.Contains(string(output), a.serverName) {
		return fmt.Errorf("MCP server registration verification failed; server not found in output")
	}

	return nil
}
