package integrations

import "github.com/leefowlercu/agentic-memorizer/pkg/types"

// Integration defines the interface that all agent framework integrations must implement.
// Each integration adapter (e.g., Claude Code, Continue, Cline) provides framework-specific
// implementation for configuration management, output formatting, and lifecycle operations.
type Integration interface {
	// Metadata returns identifying information about the integration

	// GetName returns the unique identifier for this integration (e.g., "claude-code-hook")
	GetName() string

	// GetDescription returns a human-readable description of the integration
	GetDescription() string

	// GetVersion returns the integration adapter version
	GetVersion() string

	// Detection methods determine if the framework is installed and configured

	// Detect checks if the agent framework is installed on the system
	// Returns true if the framework's configuration directory/files exist
	Detect() (bool, error)

	// IsEnabled checks if this integration is currently configured and active
	// Returns true if hooks/configuration have been set up
	IsEnabled() (bool, error)

	// Lifecycle methods manage integration configuration

	// Setup configures the integration by installing hooks or modifying framework config
	// binaryPath is the path to the agentic-memorizer binary
	// Returns error if setup fails
	Setup(binaryPath string) error

	// Update modifies an existing integration configuration
	// Used when the binary path or integration settings change
	// Returns error if update fails
	Update(binaryPath string) error

	// Remove uninstalls the integration by removing hooks or configuration
	// Returns error if removal fails
	Remove() error

	// Command generation

	// GetCommand returns the full command string that the framework should execute
	// binaryPath is the path to the agentic-memorizer binary
	// format is the desired output format (xml, markdown, json)
	// Returns a command string like "agentic-memorizer read --format xml --integration claude-code-hook"
	GetCommand(binaryPath string, format OutputFormat) string

	// Output formatting

	// FormatOutput formats the graph index for this specific integration
	// Applies integration-specific wrapping (e.g., SessionStart JSON for Claude Code)
	// index is the graph-native memory index to format
	// format is the base output format (xml, markdown, json)
	// Returns formatted string ready for the framework to consume
	FormatOutput(index *types.GraphIndex, format OutputFormat) (string, error)

	// Validation

	// Validate checks the health of the integration configuration
	// Returns error if configuration is invalid or inconsistent
	Validate() error

	// Configuration management

	// Reload applies configuration changes without full teardown/setup
	// newConfig contains the updated integration configuration
	// Returns error if reload fails (caller should rollback)
	Reload(newConfig IntegrationConfig) error
}
