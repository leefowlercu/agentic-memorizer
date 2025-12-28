package claude

import "github.com/leefowlercu/agentic-memorizer/internal/integrations"

// init registers all Claude Code adapters with the global registry
func init() {
	integrations.GlobalRegistry().Register(NewClaudeCodeHookAdapter())
	integrations.GlobalRegistry().Register(NewClaudeCodeMCPAdapter())
}
