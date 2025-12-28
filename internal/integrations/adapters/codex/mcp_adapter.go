package codex

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
	MCPIntegrationName = "codex-cli-mcp"

	// MCPIntegrationVersion is the adapter version
	MCPIntegrationVersion = "2.0.0"

	// MCPServerName is the identifier used in ~/.codex/config.toml
	MCPServerName = "memorizer"
)

// CodexCLIMCPAdapter implements the Integration interface for Codex CLI MCP server
type CodexCLIMCPAdapter struct {
	configPath string
	serverName string
}

// NewCodexCLIMCPAdapter creates a new Codex CLI MCP integration adapter
func NewCodexCLIMCPAdapter() *CodexCLIMCPAdapter {
	return &CodexCLIMCPAdapter{
		configPath: getDefaultMCPConfigPath(),
		serverName: MCPServerName,
	}
}

// GetName returns the integration name
func (a *CodexCLIMCPAdapter) GetName() string {
	return MCPIntegrationName
}

// GetDescription returns a human-readable description
func (a *CodexCLIMCPAdapter) GetDescription() string {
	return "OpenAI Codex CLI MCP server integration"
}

// GetVersion returns the adapter version
func (a *CodexCLIMCPAdapter) GetVersion() string {
	return MCPIntegrationVersion
}

// Detect checks if Codex CLI with MCP support is installed on the system
func (a *CodexCLIMCPAdapter) Detect() (bool, error) {
	// Check if ~/.codex/ directory exists
	home, err := os.UserHomeDir()
	if err != nil {
		return false, fmt.Errorf("failed to get home directory; %w", err)
	}

	codexDir := filepath.Join(home, ".codex")
	if _, err := os.Stat(codexDir); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check .codex directory; %w", err)
	}

	// Check if `codex` CLI is available
	if _, err := exec.LookPath("codex"); err != nil {
		return false, nil
	}

	return true, nil
}

// IsEnabled checks if the integration is currently configured
func (a *CodexCLIMCPAdapter) IsEnabled() (bool, error) {
	// Check if config file exists
	if _, err := os.Stat(a.configPath); os.IsNotExist(err) {
		return false, nil
	}

	// Read config and check for agentic-memorizer server
	mcpConfig, _, err := readMCPConfig(a.configPath)
	if err != nil {
		return false, fmt.Errorf("failed to read config; %w", err)
	}

	// Check if our server is registered
	server, exists := mcpConfig.MCPServers[a.serverName]
	if !exists {
		return false, nil
	}

	// Check if explicitly disabled
	if server.Enabled != nil && !*server.Enabled {
		return false, nil
	}

	return true, nil
}

// Setup configures the Codex CLI MCP integration
func (a *CodexCLIMCPAdapter) Setup(binaryPath string) error {
	mcpConfig, fullConfig, err := readMCPConfig(a.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config; %w", err)
	}

	// Get memory root from config for environment variable
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get configuration; %w", err)
	}

	// Create MCP server entry
	enabled := true
	server := CodexMCPServer{
		Command: binaryPath,
		Args:    []string{"mcp", "start"},
		Enabled: &enabled,
		Env: map[string]string{
			"MEMORIZER_MEMORY_ROOT": cfg.MemoryRoot,
		},
	}

	// Add or update server
	mcpConfig.MCPServers[a.serverName] = server

	if err := writeMCPConfig(a.configPath, mcpConfig, fullConfig); err != nil {
		return fmt.Errorf("failed to write config; %w", err)
	}

	return nil
}

// Update updates the integration configuration
func (a *CodexCLIMCPAdapter) Update(binaryPath string) error {
	// For Codex CLI MCP, update is the same as setup
	return a.Setup(binaryPath)
}

// Remove removes the integration configuration
func (a *CodexCLIMCPAdapter) Remove() error {
	mcpConfig, fullConfig, err := readMCPConfig(a.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config; %w", err)
	}

	// Remove our server if it exists
	delete(mcpConfig.MCPServers, a.serverName)

	if err := writeMCPConfig(a.configPath, mcpConfig, fullConfig); err != nil {
		return fmt.Errorf("failed to write config; %w", err)
	}

	return nil
}

// GetCommand returns the command that should be executed by Codex CLI
func (a *CodexCLIMCPAdapter) GetCommand(binaryPath string, format integrations.OutputFormat) string {
	// MCP servers ignore format parameter - format selection happens via resource URIs
	// This method is primarily for documentation/display purposes
	return fmt.Sprintf("%s mcp start", binaryPath)
}

// FormatOutput is not used by MCP integrations
// MCP servers provide output through resources and tools, not direct formatting
func (a *CodexCLIMCPAdapter) FormatOutput(index *types.GraphIndex, format integrations.OutputFormat) (string, error) {
	return "", fmt.Errorf("MCP integrations do not use FormatOutput; output is provided through MCP resources and tools")
}

// FormatFactsOutput is not used by MCP integrations
// MCP servers provide output through resources and tools, not direct formatting
func (a *CodexCLIMCPAdapter) FormatFactsOutput(facts *types.FactsIndex, format integrations.OutputFormat) (string, error) {
	return "", fmt.Errorf("MCP integrations do not use FormatFactsOutput; output is provided through MCP resources and tools")
}

// Validate checks the health of the integration
func (a *CodexCLIMCPAdapter) Validate() error {
	// Check if config file exists
	if _, err := os.Stat(a.configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found at %s", a.configPath)
	}

	// Try to read config
	mcpConfig, _, err := readMCPConfig(a.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config; %w", err)
	}

	// Check if our server is configured
	server, exists := mcpConfig.MCPServers[a.serverName]
	if !exists {
		return fmt.Errorf("MCP server %q not found in configuration", a.serverName)
	}

	// Check if explicitly disabled
	if server.Enabled != nil && !*server.Enabled {
		return fmt.Errorf("MCP server is disabled in configuration")
	}

	// Validate server configuration
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
func (a *CodexCLIMCPAdapter) Reload(newConfig integrations.IntegrationConfig) error {
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

// getDefaultMCPConfigPath returns the default path to Codex CLI config
func getDefaultMCPConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".codex", "config.toml")
}
