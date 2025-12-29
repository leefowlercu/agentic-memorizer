package shared

import (
	"fmt"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
)

// GetFilesCommand returns the command for file index hooks (SessionStart)
func GetFilesCommand(binaryPath string, format integrations.OutputFormat, integrationName string) string {
	return fmt.Sprintf("%s read files --format %s --integration %s", binaryPath, format, integrationName)
}

// GetFactsCommand returns the command for facts hooks (UserPromptSubmit/BeforeAgent)
func GetFactsCommand(binaryPath string, format integrations.OutputFormat, integrationName string) string {
	return fmt.Sprintf("%s read facts --format %s --integration %s", binaryPath, format, integrationName)
}

// ContainsMemorizer checks if a command contains the memorizer binary name
// Returns true for both current "memorizer" and old "agentic-memorizer" names
func ContainsMemorizer(command string) bool {
	return strings.Contains(command, "memorizer")
}

// ContainsOldBinaryName checks if a command uses the old binary name
func ContainsOldBinaryName(command string) bool {
	return strings.Contains(command, "agentic-memorizer")
}

// IsMemorizerHook checks if a command is a memorizer hook (current name, not old)
func IsMemorizerHook(command string) bool {
	return ContainsMemorizer(command) && !ContainsOldBinaryName(command)
}

// AggregateErrors joins multiple error strings with semicolon separator
func AggregateErrors(errors []string) string {
	return strings.Join(errors, "; ")
}
