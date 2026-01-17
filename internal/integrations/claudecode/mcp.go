package claudecode

import (
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
)

const (
	mcpConfig = "~/.claude.json"
	mcpKey    = "mcpServers"
)

// NewMCPIntegration creates a Claude Code MCP integration.
// This integration adds the memorizer MCP server to Claude Code's
// MCP server configuration using remote HTTP transport.
func NewMCPIntegration() integrations.Integration {
	return integrations.NewMCPIntegration(
		"claude-code-mcp",
		harnessName,
		"Claude Code MCP integration that exposes knowledge graph via MCP protocol",
		binaryName,
		mcpConfig,
		"json",
		mcpKey,
		"memorizer",
		integrations.MCPServerConfig{
			Transport: integrations.MCPTransportRemote,
			// URL is prompted during setup, defaults to http://127.0.0.1:7600/mcp
		},
	)
}

func init() {
	_ = integrations.RegisterIntegration(NewMCPIntegration())
}
