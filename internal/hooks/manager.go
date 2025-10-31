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

func ReadSettings(path string) (*Settings, map[string]any, error) {
	fullSettings := make(map[string]any)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Settings{
			Hooks: make(map[string][]HookEvent),
		}, fullSettings, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read settings file; %w", err)
	}

	if err := json.Unmarshal(data, &fullSettings); err != nil {
		return nil, nil, fmt.Errorf("failed to parse settings JSON; %w", err)
	}

	var settings Settings
	if hooksData, ok := fullSettings["hooks"]; ok {
		hooksJSON, err := json.Marshal(hooksData)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal hooks; %w", err)
		}
		var hooksMap map[string][]HookEvent
		if err := json.Unmarshal(hooksJSON, &hooksMap); err != nil {
			return nil, nil, fmt.Errorf("failed to parse hooks; %w", err)
		}
		settings.Hooks = hooksMap
	}

	if settings.Hooks == nil {
		settings.Hooks = make(map[string][]HookEvent)
	}

	return &settings, fullSettings, nil
}

func WriteSettings(path string, settings *Settings, fullSettings map[string]any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create settings directory; %w", err)
	}

	fullSettings["hooks"] = settings.Hooks

	data, err := json.MarshalIndent(fullSettings, "", "  ")
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

	settings, fullSettings, err := ReadSettings(settingsPath)
	if err != nil {
		return nil, nil, err
	}

	command := fmt.Sprintf("%s read --format xml --wrap-json", binaryPath)

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
			for i := range sessionStartEvents[matcherIdx].Hooks {
				if strings.Contains(sessionStartEvents[matcherIdx].Hooks[i].Command, "agentic-memorizer") {
					hookExists = true
					if sessionStartEvents[matcherIdx].Hooks[i].Command != command {
						sessionStartEvents[matcherIdx].Hooks[i].Command = command
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

	if err := WriteSettings(settingsPath, settings, fullSettings); err != nil {
		return nil, nil, err
	}

	return settings, updated, nil
}
