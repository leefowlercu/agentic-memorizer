package integrations

import (
	"context"
	"fmt"
	"os"
	"time"
)

// HookConfig defines the hook configuration for setup.
type HookConfig struct {
	// HookType is the type of hook (e.g., "PreToolUse", "SessionStart").
	HookType string
	// Matcher defines which tools/events trigger the hook.
	Matcher string
	// Command is the shell command to execute.
	Command string
	// Timeout in milliseconds.
	Timeout int
}

// HookIntegration is the base type for hook-based integrations.
type HookIntegration struct {
	name        string
	harness     string
	description string
	binaryName  string
	configPath  string
	hooks       []HookConfig
	configKey   string // Key in the config file where hooks are stored
}

// NewHookIntegration creates a new hook-based integration.
func NewHookIntegration(
	name string,
	harness string,
	description string,
	binaryName string,
	configPath string,
	configKey string,
	hooks []HookConfig,
) *HookIntegration {
	return &HookIntegration{
		name:        name,
		harness:     harness,
		description: description,
		binaryName:  binaryName,
		configPath:  configPath,
		configKey:   configKey,
		hooks:       hooks,
	}
}

// Name returns the integration name.
func (h *HookIntegration) Name() string {
	return h.name
}

// Harness returns the target harness name.
func (h *HookIntegration) Harness() string {
	return h.harness
}

// Type returns the integration type.
func (h *HookIntegration) Type() IntegrationType {
	return IntegrationTypeHook
}

// Description returns the integration description.
func (h *HookIntegration) Description() string {
	return h.description
}

// Validate checks if the integration can be set up.
func (h *HookIntegration) Validate() error {
	// Check if harness binary exists
	if _, found := FindBinary(h.binaryName); !found {
		return fmt.Errorf("%s binary not found in PATH", h.binaryName)
	}

	return nil
}

// Setup installs the hook integration.
func (h *HookIntegration) Setup(ctx context.Context) error {
	if err := h.Validate(); err != nil {
		return err
	}

	// Backup existing config
	backupPath, err := BackupConfig(h.configPath)
	if err != nil {
		return fmt.Errorf("failed to backup config; %w", err)
	}

	// Read existing config
	config, err := ReadJSONConfig(h.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config; %w", err)
	}

	// Add hooks to config
	if err := h.addHooksToConfig(config.Content); err != nil {
		// Try to restore backup if we fail
		if backupPath != "" {
			_ = RestoreBackup(backupPath, h.configPath)
		}
		return fmt.Errorf("failed to add hooks; %w", err)
	}

	// Write updated config
	if err := WriteJSONConfig(h.configPath, config.Content); err != nil {
		// Try to restore backup if we fail
		if backupPath != "" {
			_ = RestoreBackup(backupPath, h.configPath)
		}
		return fmt.Errorf("failed to write config; %w", err)
	}

	return nil
}

// Teardown removes the hook integration.
func (h *HookIntegration) Teardown(ctx context.Context) error {
	// Read existing config
	config, err := ReadJSONConfig(h.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config; %w", err)
	}

	// Backup before modifying
	backupPath, err := BackupConfig(h.configPath)
	if err != nil {
		return fmt.Errorf("failed to backup config; %w", err)
	}

	// Remove hooks from config
	if err := h.removeHooksFromConfig(config.Content); err != nil {
		if backupPath != "" {
			_ = RestoreBackup(backupPath, h.configPath)
		}
		return fmt.Errorf("failed to remove hooks; %w", err)
	}

	// Write updated config
	if err := WriteJSONConfig(h.configPath, config.Content); err != nil {
		if backupPath != "" {
			_ = RestoreBackup(backupPath, h.configPath)
		}
		return fmt.Errorf("failed to write config; %w", err)
	}

	return nil
}

// IsInstalled checks if the hook integration is installed.
func (h *HookIntegration) IsInstalled() (bool, error) {
	if !ConfigExists(h.configPath) {
		return false, nil
	}

	config, err := ReadJSONConfig(h.configPath)
	if err != nil {
		return false, err
	}

	return h.hasMemorizerHooks(config.Content), nil
}

// Status returns the integration status.
func (h *HookIntegration) Status() (*StatusInfo, error) {
	// Check if harness is installed
	binaryPath, found := FindBinary(h.binaryName)
	if !found {
		return &StatusInfo{
			Status:  StatusMissingHarness,
			Message: fmt.Sprintf("%s binary not found", h.binaryName),
		}, nil
	}

	// Check if config exists
	if !ConfigExists(h.configPath) {
		return &StatusInfo{
			Status:  StatusNotInstalled,
			Message: "Configuration file does not exist",
		}, nil
	}

	// Read config to check installation
	config, err := ReadJSONConfig(h.configPath)
	if err != nil {
		return &StatusInfo{
			Status:  StatusError,
			Message: fmt.Sprintf("Failed to read config: %v", err),
		}, nil
	}

	if !h.hasMemorizerHooks(config.Content) {
		return &StatusInfo{
			Status:     StatusNotInstalled,
			Message:    fmt.Sprintf("Memorizer hooks not found in %s", h.configPath),
			ConfigPath: expandPath(h.configPath),
		}, nil
	}

	// Get config file modification time as installed time
	var installedAt time.Time
	expandedPath := expandPath(h.configPath)
	if info, err := os.Stat(expandedPath); err == nil {
		installedAt = info.ModTime()
	}

	return &StatusInfo{
		Status:      StatusInstalled,
		Message:     fmt.Sprintf("Hooks installed via %s", binaryPath),
		ConfigPath:  expandedPath,
		InstalledAt: installedAt,
	}, nil
}

// addHooksToConfig adds memorizer hooks to the configuration.
func (h *HookIntegration) addHooksToConfig(content map[string]any) error {
	// Get or create hooks section
	hooksSection, ok := GetMapSection(content, h.configKey)
	if !ok {
		hooksSection = make(map[string]any)
		content[h.configKey] = hooksSection
	}

	// Add each hook
	for _, hook := range h.hooks {
		hookEntry := map[string]any{
			"matcher":    hook.Matcher,
			"hooks":      []any{hook.Command},
		}
		if hook.Timeout > 0 {
			hookEntry["timeout"] = hook.Timeout
		}

		key := fmt.Sprintf("memorizer-%s", hook.HookType)
		hooksSection[key] = hookEntry
	}

	return nil
}

// removeHooksFromConfig removes memorizer hooks from the configuration.
func (h *HookIntegration) removeHooksFromConfig(content map[string]any) error {
	hooksSection, ok := GetMapSection(content, h.configKey)
	if !ok {
		return nil // No hooks section, nothing to remove
	}

	// Remove memorizer hooks
	for _, hook := range h.hooks {
		key := fmt.Sprintf("memorizer-%s", hook.HookType)
		delete(hooksSection, key)
	}

	// Clean up empty hooks section
	if len(hooksSection) == 0 {
		delete(content, h.configKey)
	}

	return nil
}

// hasMemorizerHooks checks if memorizer hooks are present in the config.
func (h *HookIntegration) hasMemorizerHooks(content map[string]any) bool {
	hooksSection, ok := GetMapSection(content, h.configKey)
	if !ok {
		return false
	}

	for _, hook := range h.hooks {
		key := fmt.Sprintf("memorizer-%s", hook.HookType)
		if _, exists := hooksSection[key]; exists {
			return true
		}
	}

	return false
}
