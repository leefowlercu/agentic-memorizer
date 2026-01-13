package integrations

import (
	"context"
	"fmt"
	"os"
	"time"
)

// MCPServerConfig defines the MCP server configuration.
type MCPServerConfig struct {
	// Command is the executable to run (or URL for HTTP servers).
	Command string
	// Args are the command arguments.
	Args []string
	// Env contains environment variables.
	Env map[string]string
	// Type is the server type (e.g., "stdio", "http", "sse").
	Type string
}

// MCPIntegration is the base type for MCP-based integrations.
type MCPIntegration struct {
	name         string
	harness      string
	description  string
	binaryName   string
	configPath   string
	configFormat string // "json", "toml"
	serverKey    string // Key in config for MCP servers section
	serverName   string // Name of the memorizer server entry
	serverConfig MCPServerConfig
}

// NewMCPIntegration creates a new MCP-based integration.
func NewMCPIntegration(
	name string,
	harness string,
	description string,
	binaryName string,
	configPath string,
	configFormat string,
	serverKey string,
	serverName string,
	serverConfig MCPServerConfig,
) *MCPIntegration {
	return &MCPIntegration{
		name:         name,
		harness:      harness,
		description:  description,
		binaryName:   binaryName,
		configPath:   configPath,
		configFormat: configFormat,
		serverKey:    serverKey,
		serverName:   serverName,
		serverConfig: serverConfig,
	}
}

// Name returns the integration name.
func (m *MCPIntegration) Name() string {
	return m.name
}

// Harness returns the target harness name.
func (m *MCPIntegration) Harness() string {
	return m.harness
}

// Type returns the integration type.
func (m *MCPIntegration) Type() IntegrationType {
	return IntegrationTypeMCP
}

// Description returns the integration description.
func (m *MCPIntegration) Description() string {
	return m.description
}

// Validate checks if the integration can be set up.
func (m *MCPIntegration) Validate() error {
	// Check if harness binary exists
	if _, found := FindBinary(m.binaryName); !found {
		return fmt.Errorf("%s binary not found in PATH", m.binaryName)
	}

	return nil
}

// Setup installs the MCP integration.
func (m *MCPIntegration) Setup(ctx context.Context) error {
	if err := m.Validate(); err != nil {
		return err
	}

	// Backup existing config
	backupPath, err := BackupConfig(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to backup config; %w", err)
	}

	// Read existing config
	config, err := m.readConfig()
	if err != nil {
		return fmt.Errorf("failed to read config; %w", err)
	}

	// Add MCP server to config
	if err := m.addServerToConfig(config); err != nil {
		if backupPath != "" {
			_ = RestoreBackup(backupPath, m.configPath)
		}
		return fmt.Errorf("failed to add MCP server; %w", err)
	}

	// Write updated config
	if err := m.writeConfig(config); err != nil {
		if backupPath != "" {
			_ = RestoreBackup(backupPath, m.configPath)
		}
		return fmt.Errorf("failed to write config; %w", err)
	}

	return nil
}

// Teardown removes the MCP integration.
func (m *MCPIntegration) Teardown(ctx context.Context) error {
	// Read existing config
	config, err := m.readConfig()
	if err != nil {
		return fmt.Errorf("failed to read config; %w", err)
	}

	// Backup before modifying
	backupPath, err := BackupConfig(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to backup config; %w", err)
	}

	// Remove MCP server from config
	if err := m.removeServerFromConfig(config); err != nil {
		if backupPath != "" {
			_ = RestoreBackup(backupPath, m.configPath)
		}
		return fmt.Errorf("failed to remove MCP server; %w", err)
	}

	// Write updated config
	if err := m.writeConfig(config); err != nil {
		if backupPath != "" {
			_ = RestoreBackup(backupPath, m.configPath)
		}
		return fmt.Errorf("failed to write config; %w", err)
	}

	return nil
}

// IsInstalled checks if the MCP integration is installed.
func (m *MCPIntegration) IsInstalled() (bool, error) {
	if !ConfigExists(m.configPath) {
		return false, nil
	}

	config, err := m.readConfig()
	if err != nil {
		return false, err
	}

	return m.hasMemorizerServer(config), nil
}

// Status returns the integration status.
func (m *MCPIntegration) Status() (*StatusInfo, error) {
	// Check if harness is installed
	binaryPath, found := FindBinary(m.binaryName)
	if !found {
		return &StatusInfo{
			Status:  StatusMissingHarness,
			Message: fmt.Sprintf("%s binary not found", m.binaryName),
		}, nil
	}

	// Check if config exists
	if !ConfigExists(m.configPath) {
		return &StatusInfo{
			Status:  StatusNotInstalled,
			Message: "Configuration file does not exist",
		}, nil
	}

	// Read config to check installation
	config, err := m.readConfig()
	if err != nil {
		return &StatusInfo{
			Status:  StatusError,
			Message: fmt.Sprintf("Failed to read config: %v", err),
		}, nil
	}

	if !m.hasMemorizerServer(config) {
		return &StatusInfo{
			Status:     StatusNotInstalled,
			Message:    fmt.Sprintf("Memorizer MCP server not found in %s", m.configPath),
			ConfigPath: expandPath(m.configPath),
		}, nil
	}

	// Get config file modification time as installed time
	var installedAt time.Time
	expandedPath := expandPath(m.configPath)
	if info, err := os.Stat(expandedPath); err == nil {
		installedAt = info.ModTime()
	}

	return &StatusInfo{
		Status:      StatusInstalled,
		Message:     fmt.Sprintf("MCP server installed via %s", binaryPath),
		ConfigPath:  expandedPath,
		InstalledAt: installedAt,
	}, nil
}

// readConfig reads the configuration file based on format.
func (m *MCPIntegration) readConfig() (map[string]any, error) {
	switch m.configFormat {
	case "json":
		config, err := ReadJSONConfig(m.configPath)
		if err != nil {
			return nil, err
		}
		return config.Content, nil
	case "toml":
		return m.readTOMLConfig()
	default:
		return nil, fmt.Errorf("unsupported config format: %s", m.configFormat)
	}
}

// writeConfig writes the configuration file based on format.
func (m *MCPIntegration) writeConfig(config map[string]any) error {
	switch m.configFormat {
	case "json":
		return WriteJSONConfig(m.configPath, config)
	case "toml":
		return m.writeTOMLConfig(config)
	default:
		return fmt.Errorf("unsupported config format: %s", m.configFormat)
	}
}

// readTOMLConfig reads a TOML configuration file.
func (m *MCPIntegration) readTOMLConfig() (map[string]any, error) {
	expandedPath := expandPath(m.configPath)

	data, err := os.ReadFile(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), nil
		}
		return nil, fmt.Errorf("failed to read TOML config; %w", err)
	}

	// Simple TOML parsing for our use case
	// For complex TOML, would need a full parser
	content := make(map[string]any)
	if len(data) > 0 {
		content["_raw"] = string(data)
	}

	return content, nil
}

// writeTOMLConfig writes a TOML configuration file.
func (m *MCPIntegration) writeTOMLConfig(config map[string]any) error {
	// For TOML files, we need to handle them specially
	// This is a simplified implementation
	return WriteJSONConfig(m.configPath, config)
}

// addServerToConfig adds the memorizer MCP server to the configuration.
func (m *MCPIntegration) addServerToConfig(config map[string]any) error {
	// Get or create MCP servers section
	serversSection, ok := GetMapSection(config, m.serverKey)
	if !ok {
		serversSection = make(map[string]any)
		config[m.serverKey] = serversSection
	}

	// Build server entry
	serverEntry := make(map[string]any)
	serverEntry["command"] = m.serverConfig.Command

	if len(m.serverConfig.Args) > 0 {
		serverEntry["args"] = m.serverConfig.Args
	}

	if len(m.serverConfig.Env) > 0 {
		serverEntry["env"] = m.serverConfig.Env
	}

	if m.serverConfig.Type != "" {
		serverEntry["type"] = m.serverConfig.Type
	}

	serversSection[m.serverName] = serverEntry

	return nil
}

// removeServerFromConfig removes the memorizer MCP server from the configuration.
func (m *MCPIntegration) removeServerFromConfig(config map[string]any) error {
	serversSection, ok := GetMapSection(config, m.serverKey)
	if !ok {
		return nil // No servers section, nothing to remove
	}

	delete(serversSection, m.serverName)

	// Clean up empty servers section
	if len(serversSection) == 0 {
		delete(config, m.serverKey)
	}

	return nil
}

// hasMemorizerServer checks if the memorizer MCP server is present in the config.
func (m *MCPIntegration) hasMemorizerServer(config map[string]any) bool {
	serversSection, ok := GetMapSection(config, m.serverKey)
	if !ok {
		return false
	}

	_, exists := serversSection[m.serverName]
	return exists
}
