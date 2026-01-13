// Package geminicli provides Gemini CLI integrations.
package geminicli

import (
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
)

const (
	harnessName = "gemini-cli"
	binaryName  = "gemini"
	hookConfig  = "~/.gemini/settings.json"
	hookKey     = "hooks"
)

// NewHookIntegration creates a Gemini CLI hook integration.
// This integration injects knowledge graph data into Gemini CLI sessions
// via the SessionStart hook.
func NewHookIntegration() integrations.Integration {
	return integrations.NewHookIntegration(
		"gemini-cli-hook",
		harnessName,
		"Gemini CLI hook integration that injects knowledge graph data at session start",
		binaryName,
		hookConfig,
		hookKey,
		[]integrations.HookConfig{
			{
				HookType: "SessionStart",
				Matcher:  ".*",
				Command:  "memorizer read --format toon --envelope gemini-cli --quiet",
				Timeout:  30000,
			},
		},
	)
}

func init() {
	_ = integrations.RegisterIntegration(NewHookIntegration())
}
