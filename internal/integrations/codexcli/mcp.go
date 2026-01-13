// Package codexcli provides Codex CLI integrations.
package codexcli

import (
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
)

const (
	harnessName = "codex-cli"
	binaryName  = "codex"
	mcpConfig   = "~/.codex/config.toml"
	mcpKey      = "mcp_servers"
)

// NewMCPIntegration creates a Codex CLI MCP integration.
// This integration adds the memorizer MCP server to Codex CLI's
// MCP server configuration.
func NewMCPIntegration() integrations.Integration {
	return integrations.NewMCPIntegration(
		"codex-cli-mcp",
		harnessName,
		"Codex CLI MCP integration that exposes knowledge graph via MCP protocol",
		binaryName,
		mcpConfig,
		"toml",
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
