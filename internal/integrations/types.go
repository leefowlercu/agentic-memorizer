package integrations

// OutputFormat represents the base format for rendering the memory index.
// This is distinct from integration-specific wrappers.
type OutputFormat string

const (
	// FormatXML renders the index as XML (existing format)
	FormatXML OutputFormat = "xml"

	// FormatMarkdown renders the index as Markdown (existing format)
	FormatMarkdown OutputFormat = "markdown"

	// FormatJSON renders the index as JSON (new format - not the storage format)
	// This is a human-readable/agent-readable JSON representation
	FormatJSON OutputFormat = "json"
)

// IsValid checks if the output format is valid
func (f OutputFormat) IsValid() bool {
	switch f {
	case FormatXML, FormatMarkdown, FormatJSON:
		return true
	default:
		return false
	}
}

// String returns the string representation of the output format
func (f OutputFormat) String() string {
	return string(f)
}

// IntegrationConfig represents the configuration for a specific integration.
// This is stored in the main config file under the integrations section.
type IntegrationConfig struct {
	// Type is the integration type (e.g., "claude-code-hook", "continue", "cline")
	Type string `mapstructure:"type" yaml:"type"`

	// Enabled indicates if this integration is active
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`

	// OutputFormat is the preferred output format for this integration
	OutputFormat string `mapstructure:"output_format" yaml:"output_format"`

	// Settings contains integration-specific settings
	// For Claude Code: settings_path, matchers
	// For Continue: config_path
	// For Cline: config_path
	Settings map[string]any `mapstructure:"settings" yaml:"settings"`
}

// IntegrationsConfig represents the complete integrations configuration section.
// The Enabled list tracks which integrations have been configured via setup commands.
// Integration-specific configuration (hooks, tools, etc.) is stored in framework-specific
// files (e.g., ~/.claude.json, ~/.claude/settings.json) rather than in this config file.
type IntegrationsConfig struct {
	// Enabled is a list of integration names that are enabled
	Enabled []string `mapstructure:"enabled" yaml:"enabled"`
}

// HealthStatus represents the health status of an integration
type HealthStatus struct {
	// Healthy indicates if the integration is functioning correctly
	Healthy bool

	// Status is a human-readable status message
	Status string

	// Details provides additional information about the health check
	Details map[string]any

	// LastChecked is the timestamp of the last health check
	LastChecked string
}
