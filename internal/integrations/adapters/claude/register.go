package claude

import "github.com/leefowlercu/agentic-memorizer/internal/integrations"

// init registers the Claude Code adapter with the global registry
func init() {
	integrations.GlobalRegistry().Register(NewClaudeCodeAdapter())
}
