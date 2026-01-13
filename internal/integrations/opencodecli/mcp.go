// Package opencodecli provides OpenCode CLI integrations.
package opencodecli

import (
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
)

const (
	harnessName = "opencode"
	binaryName  = "opencode"
	mcpConfig   = "~/.config/opencode/opencode.json"
	mcpKey      = "mcpServers"
)

// NewMCPIntegration creates an OpenCode MCP integration.
// This integration adds the memorizer MCP server to OpenCode's
// MCP server configuration.
func NewMCPIntegration() integrations.Integration {
	return integrations.NewMCPIntegration(
		"opencode-mcp",
		harnessName,
		"OpenCode MCP integration that exposes knowledge graph via MCP protocol",
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
