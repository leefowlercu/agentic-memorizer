package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	sessionStartEvent = "SessionStart"
)

var sessionStartMatchers = []string{"startup", "resume", "clear", "compact"}

// GetClaudeSettingsPath can be overridden in tests
var GetClaudeSettingsPath = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory; %w", err)
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

func FindBinaryPath() (string, error) {
	execPath, err := os.Executable()
	if err == nil {
		if filepath.Base(execPath) == "agentic-memorizer" {
			return execPath, nil
		}
	}

	home, err := os.UserHomeDir()
	if err == nil {
		commonPath := filepath.Join(home, ".local", "bin", "agentic-memorizer")
		if _, err := os.Stat(commonPath); err == nil {
			return commonPath, nil
		}
	}

	pathBinary, err := exec.LookPath("agentic-memorizer")
	if err == nil {
		return pathBinary, nil
	}

	return "", fmt.Errorf("could not locate agentic-memorizer binary")
}

func ReadSettings(path string) (*Settings, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Settings{
			Hooks: make(map[string][]HookEvent),
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read settings file; %w", err)
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("failed to parse settings JSON; %w", err)
	}

	if settings.Hooks == nil {
		settings.Hooks = make(map[string][]HookEvent)
	}

	return &settings, nil
}

func WriteSettings(path string, settings *Settings) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create settings directory; %w", err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings; %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings file; %w", err)
	}

	return nil
}

func SetupSessionStartHooks(binaryPath string) (*Settings, []string, error) {
	settingsPath, err := GetClaudeSettingsPath()
	if err != nil {
		return nil, nil, err
	}

	settings, err := ReadSettings(settingsPath)
	if err != nil {
		return nil, nil, err
	}

	command := fmt.Sprintf("%s --format json", binaryPath)

	var updated []string

	sessionStartEvents := settings.Hooks[sessionStartEvent]
	if sessionStartEvents == nil {
		sessionStartEvents = []HookEvent{}
	}

	for _, matcher := range sessionStartMatchers {
		matcherIdx := -1
		for i, event := range sessionStartEvents {
			if event.Matcher == matcher {
				matcherIdx = i
				break
			}
		}

		hookExists := false
		if matcherIdx >= 0 {
			for _, hook := range sessionStartEvents[matcherIdx].Hooks {
				if strings.Contains(hook.Command, "agentic-memorizer") {
					hookExists = true
					if hook.Command != command {
						hook.Command = command
						updated = append(updated, matcher)
					}
					break
				}
			}
		}

		if !hookExists {
			newHook := Hook{
				Type:    "command",
				Command: command,
			}

			if matcherIdx >= 0 {
				sessionStartEvents[matcherIdx].Hooks = append(sessionStartEvents[matcherIdx].Hooks, newHook)
			} else {
				sessionStartEvents = append(sessionStartEvents, HookEvent{
					Matcher: matcher,
					Hooks:   []Hook{newHook},
				})
			}
			updated = append(updated, matcher)
		}
	}

	settings.Hooks[sessionStartEvent] = sessionStartEvents

	if err := WriteSettings(settingsPath, settings); err != nil {
		return nil, nil, err
	}

	return settings, updated, nil
}
