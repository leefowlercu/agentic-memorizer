package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Settings represents the Claude Code settings structure
type Settings struct {
	Hooks map[string][]HookEvent `json:"hooks,omitempty"`
}

// HookEvent represents a hook event configuration
type HookEvent struct {
	Matcher string `json:"matcher,omitempty"`
	Hooks   []Hook `json:"hooks"`
}

// Hook represents a single hook command
type Hook struct {
	Type    string  `json:"type"`
	Command string  `json:"command"`
	Timeout float64 `json:"timeout,omitempty"`
}

// readSettings reads the Claude Code settings file
// Returns the parsed settings, the full settings map (for preserving other fields), and any error
func readSettings(path string) (*Settings, map[string]any, error) {
	fullSettings := make(map[string]any)

	// If file doesn't exist, return empty settings
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Settings{
			Hooks: make(map[string][]HookEvent),
		}, fullSettings, nil
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read settings file: %w", err)
	}

	// Parse into full settings map to preserve all fields
	if err := json.Unmarshal(data, &fullSettings); err != nil {
		return nil, nil, fmt.Errorf("failed to parse settings JSON: %w", err)
	}

	// Extract hooks if they exist
	var settings Settings
	if hooksData, ok := fullSettings["hooks"]; ok {
		hooksJSON, err := json.Marshal(hooksData)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal hooks: %w", err)
		}
		var hooksMap map[string][]HookEvent
		if err := json.Unmarshal(hooksJSON, &hooksMap); err != nil {
			return nil, nil, fmt.Errorf("failed to parse hooks: %w", err)
		}
		settings.Hooks = hooksMap
	}

	if settings.Hooks == nil {
		settings.Hooks = make(map[string][]HookEvent)
	}

	return &settings, fullSettings, nil
}

// writeSettings writes the Claude Code settings file
// Performs atomic write with backup
func writeSettings(path string, settings *Settings, fullSettings map[string]any) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create settings directory: %w", err)
	}

	// Update hooks in full settings
	fullSettings["hooks"] = settings.Hooks

	// Marshal to JSON with pretty printing
	data, err := json.MarshalIndent(fullSettings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	// Create backup if file exists
	if _, err := os.Stat(path); err == nil {
		backupPath := path + ".backup"
		if err := os.Rename(path, backupPath); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
		// Remove backup on success
		defer os.Remove(backupPath)
	}

	// Write file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}
