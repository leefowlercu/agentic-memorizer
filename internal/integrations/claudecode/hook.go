// Package claudecode provides Claude Code integrations.
package claudecode

import (
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
)

const (
	harnessName = "claude-code"
	binaryName  = "claude"
	hookConfig  = "~/.claude/settings.json"
	hookKey     = "hooks"
)

// NewHookIntegration creates a Claude Code hook integration.
// This integration injects knowledge graph data into Claude Code sessions
// via the SessionStart hook.
func NewHookIntegration() integrations.Integration {
	return integrations.NewHookIntegration(
		"claude-code-hook",
		harnessName,
		"Claude Code hook integration that injects knowledge graph data at session start",
		binaryName,
		hookConfig,
		hookKey,
		[]integrations.HookConfig{
			{
				HookType: "SessionStart",
				Matcher:  ".*",
				Command:  "memorizer read --format toon --envelope claude-code --quiet",
				Timeout:  30000,
			},
		},
	)
}

func init() {
	_ = integrations.RegisterIntegration(NewHookIntegration())
}
