package codex

import "github.com/leefowlercu/agentic-memorizer/internal/integrations"

// init registers the Codex CLI MCP adapter with the global registry
func init() {
	integrations.GlobalRegistry().Register(NewCodexCLIMCPAdapter())
}
