package claude

import "github.com/leefowlercu/agentic-memorizer/internal/integrations"

// init registers all Claude Code adapters with the global registry
func init() {
	integrations.GlobalRegistry().Register(NewClaudeCodeAdapter())
	integrations.GlobalRegistry().Register(NewClaudeCodeMCPAdapter())
}
