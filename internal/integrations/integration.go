// Package integrations provides the integration system for AI harness connections.
package integrations

import (
	"context"
	"time"
)

// IntegrationType represents the type of integration.
type IntegrationType string

const (
	// IntegrationTypeHook represents a hook-based integration that modifies harness settings.
	IntegrationTypeHook IntegrationType = "hook"
	// IntegrationTypeMCP represents an MCP server integration.
	IntegrationTypeMCP IntegrationType = "mcp"
	// IntegrationTypePlugin represents a plugin-based integration.
	IntegrationTypePlugin IntegrationType = "plugin"
)

// IntegrationStatus represents the current status of an integration.
type IntegrationStatus string

const (
	// StatusNotInstalled indicates the integration is not configured.
	StatusNotInstalled IntegrationStatus = "not_installed"
	// StatusInstalled indicates the integration is configured and ready.
	StatusInstalled IntegrationStatus = "installed"
	// StatusError indicates the integration has configuration errors.
	StatusError IntegrationStatus = "error"
	// StatusMissingHarness indicates the target harness is not installed.
	StatusMissingHarness IntegrationStatus = "missing_harness"
)

// StatusInfo contains detailed status information about an integration.
type StatusInfo struct {
	Status      IntegrationStatus
	Message     string
	ConfigPath  string
	BackupPath  string
	InstalledAt time.Time
}

// Integration defines the interface for harness integrations.
type Integration interface {
	// Name returns the unique identifier for this integration.
	Name() string

	// Harness returns the name of the target AI harness.
	Harness() string

	// Type returns the integration type (hook, mcp, plugin).
	Type() IntegrationType

	// Description returns a human-readable description.
	Description() string

	// Setup installs the integration by modifying harness configuration.
	Setup(ctx context.Context) error

	// Teardown removes the integration configuration.
	Teardown(ctx context.Context) error

	// IsInstalled checks if the integration is currently installed.
	IsInstalled() (bool, error)

	// Status returns detailed status information.
	Status() (*StatusInfo, error)

	// Validate checks if the integration can be set up.
	Validate() error
}

// HarnessInfo contains information about an AI harness.
type HarnessInfo struct {
	Name           string
	BinaryName     string
	ConfigPath     string
	ConfigFormat   string // "json", "toml", "yaml"
	Description    string
	Documentation  string
	IsInstalled    bool
	InstalledPath  string
}

// SetupResult contains the result of a setup operation.
type SetupResult struct {
	Success    bool
	BackupPath string
	Message    string
}

// TeardownResult contains the result of a teardown operation.
type TeardownResult struct {
	Success        bool
	BackupRestored bool
	Message        string
}
