package gemini

import "github.com/leefowlercu/agentic-memorizer/internal/integrations"

// init registers the Gemini CLI MCP adapter with the global registry
func init() {
	integrations.GlobalRegistry().Register(NewGeminiCLIMCPAdapter())
}
