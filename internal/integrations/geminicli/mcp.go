package geminicli

import (
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
)

const (
	mcpConfig = "~/.gemini/mcp.json"
	mcpKey    = "mcpServers"
)

// NewMCPIntegration creates a Gemini CLI MCP integration.
// This integration adds the memorizer MCP server to Gemini CLI's
// MCP server configuration.
func NewMCPIntegration() integrations.Integration {
	return integrations.NewMCPIntegration(
		"gemini-cli-mcp",
		harnessName,
		"Gemini CLI MCP integration that exposes knowledge graph via MCP protocol",
		binaryName,
		mcpConfig,
		"json",
		mcpKey,
		"memorizer",
		integrations.MCPServerConfig{
			Command: "memorizer",
			Args:    []string{"daemon", "mcp"},
			Type:    "stdio",
		},
	)
}

func init() {
	_ = integrations.RegisterIntegration(NewMCPIntegration())
}
