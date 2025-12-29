package shared

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
)

// GetConfig initializes and loads the application configuration.
// This combines config.InitConfig() and config.GetConfig() into a single call.
func GetConfig() (*config.Config, error) {
	if err := config.InitConfig(); err != nil {
		return nil, fmt.Errorf("failed to initialize config; %w", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config; %w", err)
	}

	return cfg, nil
}
