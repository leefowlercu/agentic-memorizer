package daemon

import (
	"context"

	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

// registryAdapter adapts registry.Registry to mcp.RegistryChecker interface.
type registryAdapter struct {
	registry registry.Registry
}

// newRegistryAdapter creates a new registry adapter.
func newRegistryAdapter(reg registry.Registry) *registryAdapter {
	return &registryAdapter{registry: reg}
}

// IsPathRemembered returns true if the given file path is under a remembered directory.
func (a *registryAdapter) IsPathRemembered(ctx context.Context, filePath string) bool {
	if a.registry == nil {
		return false
	}
	_, err := a.registry.FindContainingPath(ctx, filePath)
	return err == nil
}
