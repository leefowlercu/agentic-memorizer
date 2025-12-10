package gemini

import "github.com/leefowlercu/agentic-memorizer/internal/integrations"

// init registers all Gemini CLI adapters with the global registry
func init() {
	integrations.GlobalRegistry().Register(NewGeminiCLIHookAdapter())
	integrations.GlobalRegistry().Register(NewGeminiCLIMCPAdapter())
}
