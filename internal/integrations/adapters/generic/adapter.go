package generic

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations/output"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

const (
	// IntegrationVersion is the adapter version
	IntegrationVersion = "1.0.0"
)

// GenericAdapter provides a fallback adapter for unsupported frameworks
// It can output formatted content but does not support automatic setup
type GenericAdapter struct {
	name         string
	description  string
	outputFormat integrations.OutputFormat
}

// NewGenericAdapter creates a new generic integration adapter
func NewGenericAdapter(name, description string, format integrations.OutputFormat) *GenericAdapter {
	return &GenericAdapter{
		name:         name,
		description:  description,
		outputFormat: format,
	}
}

// GetName returns the integration name
func (a *GenericAdapter) GetName() string {
	return a.name
}

// GetDescription returns a human-readable description
func (a *GenericAdapter) GetDescription() string {
	if a.description != "" {
		return a.description
	}
	return "Generic integration adapter (manual setup required)"
}

// GetVersion returns the adapter version
func (a *GenericAdapter) GetVersion() string {
	return IntegrationVersion
}

// Detect always returns false for generic adapters
// Generic adapters cannot auto-detect frameworks
func (a *GenericAdapter) Detect() (bool, error) {
	return false, nil
}

// IsEnabled always returns false for generic adapters
// Generic adapters cannot check configuration status
func (a *GenericAdapter) IsEnabled() (bool, error) {
	return false, nil
}

// Setup returns an error with manual setup instructions
func (a *GenericAdapter) Setup(binaryPath string) error {
	cmd := a.GetCommand(binaryPath, a.outputFormat)
	return fmt.Errorf("automatic setup not supported for %s.\n\nPlease add this command to your framework's configuration:\n  %s\n\nConsult your framework's documentation for how to add custom commands or tools", a.name, cmd)
}

// Update returns an error - generic adapters don't support updates
func (a *GenericAdapter) Update(binaryPath string) error {
	return fmt.Errorf("automatic update not supported for %s - please update manually", a.name)
}

// Remove returns an error - generic adapters don't support removal
func (a *GenericAdapter) Remove() error {
	return fmt.Errorf("automatic removal not supported for %s - please remove manually", a.name)
}

// GetCommand returns the command that should be executed
func (a *GenericAdapter) GetCommand(binaryPath string, format integrations.OutputFormat) string {
	return fmt.Sprintf("%s read --format %s", binaryPath, format)
}

// FormatOutput formats the index without any framework-specific wrapping
// Just returns the formatted content (XML, Markdown, or JSON)
func (a *GenericAdapter) FormatOutput(index *types.Index, format integrations.OutputFormat) (string, error) {
	var processor output.OutputProcessor

	switch format {
	case integrations.FormatXML:
		processor = output.NewXMLProcessor()
	case integrations.FormatMarkdown:
		processor = output.NewMarkdownProcessor()
	case integrations.FormatJSON:
		processor = output.NewJSONProcessor()
	default:
		return "", fmt.Errorf("unsupported output format: %s", format)
	}

	return processor.Format(index)
}

// Validate always returns an error for generic adapters
func (a *GenericAdapter) Validate() error {
	return fmt.Errorf("%s is a generic adapter and cannot be validated - manual configuration required", a.name)
}

// Reload always returns an error for generic adapters
func (a *GenericAdapter) Reload(newConfig integrations.IntegrationConfig) error {
	return fmt.Errorf("%s is a generic adapter and does not support configuration reload", a.name)
}
