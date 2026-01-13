// Package subcommands provides integration CLI subcommands.
package subcommands

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/integrations"

	// Import integration packages to register them
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/claudecode"
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/codexcli"
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/geminicli"
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/opencodecli"
)

// registry returns the global integration registry.
func registry() *integrations.Registry {
	return integrations.GlobalRegistry()
}

// formatIntegrationType returns a human-readable integration type.
func formatIntegrationType(t integrations.IntegrationType) string {
	switch t {
	case integrations.IntegrationTypeHook:
		return "Hook"
	case integrations.IntegrationTypeMCP:
		return "MCP"
	case integrations.IntegrationTypePlugin:
		return "Plugin"
	default:
		return string(t)
	}
}

// formatStatus returns a human-readable status.
func formatStatus(s integrations.IntegrationStatus) string {
	switch s {
	case integrations.StatusNotInstalled:
		return "Not Installed"
	case integrations.StatusInstalled:
		return "Installed"
	case integrations.StatusError:
		return "Error"
	case integrations.StatusMissingHarness:
		return "Harness Missing"
	default:
		return string(s)
	}
}

// lookupIntegration finds an integration by name with helpful error messages.
func lookupIntegration(name string) (integrations.Integration, error) {
	integration, err := registry().Get(name)
	if err != nil {
		// Provide helpful message with available integrations
		names := registry().Names()
		if len(names) > 0 {
			return nil, fmt.Errorf("integration %q not found; available integrations: %v", name, names)
		}
		return nil, fmt.Errorf("integration %q not found; no integrations available", name)
	}
	return integration, nil
}
