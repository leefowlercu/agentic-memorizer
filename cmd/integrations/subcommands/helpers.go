package subcommands

import (
	"fmt"
	"os"
)

// FindBinaryPath attempts to locate the agentic-memorizer binary
func FindBinaryPath() (string, error) {
	// Try to get the current executable path
	execPath, err := os.Executable()
	if err == nil {
		// Check if this is the agentic-memorizer binary
		if baseName := execPath; len(baseName) > 0 {
			return execPath, nil
		}
	}

	// Try common installation paths
	home, err := os.UserHomeDir()
	if err == nil {
		commonPaths := []string{
			home + "/.local/bin/agentic-memorizer",
			home + "/go/bin/agentic-memorizer",
			"/usr/local/bin/agentic-memorizer",
		}

		for _, path := range commonPaths {
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	// Try PATH
	// Note: exec.LookPath would normally be used here, but we're avoiding
	// it to keep dependencies minimal
	return "", fmt.Errorf("could not locate agentic-memorizer binary")
}
