package cmdutil

import (
	"path/filepath"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
)

// ResolvePath expands "~" and returns an absolute, cleaned path.
// Empty input returns an empty string.
func ResolvePath(path string) (string, error) {
	expanded := config.ExpandPath(path)
	if expanded == "" {
		return "", nil
	}

	absPath, err := filepath.Abs(expanded)
	if err != nil {
		return "", err
	}

	return filepath.Clean(absPath), nil
}
