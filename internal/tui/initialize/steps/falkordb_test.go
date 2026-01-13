package steps

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
)

func TestFalkorDBStep_Title(t *testing.T) {
	step := NewFalkorDBStep()
	if step.Title() != "FalkorDB" {
		t.Errorf("expected title 'FalkorDB', got '%s'", step.Title())
	}
}

func TestFalkorDBStep_Init(t *testing.T) {
	cfg := config.NewDefaultConfig()
	step := NewFalkorDBStep()

	cmd := step.Init(&cfg)
	// Init should return nil (no async command needed initially)
	if cmd != nil {
		t.Error("expected nil command from Init")
	}
}

func TestFalkorDBStep_View(t *testing.T) {
	step := NewFalkorDBStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	view := step.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestFalkorDBStep_Validate_CustomHost(t *testing.T) {
	step := NewFalkorDBStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Select "Use existing instance" option
	step.radio.SetCursor(0) // Assuming first option after container options

	// Set custom host and port
	step.hostInput.SetValue("localhost")
	step.portInput.SetValue("6379")

	err := step.Validate()
	if err != nil {
		t.Errorf("expected no validation error, got %v", err)
	}
}

func TestFalkorDBStep_Validate_EmptyHost(t *testing.T) {
	step := NewFalkorDBStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Set phase to custom entry
	step.phase = phaseCustomEntry

	// Empty host should fail validation
	step.hostInput.SetValue("")
	step.portInput.SetValue("6379")

	err := step.Validate()
	if err == nil {
		t.Error("expected validation error for empty host")
	}
}

func TestFalkorDBStep_Apply(t *testing.T) {
	step := NewFalkorDBStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	step.hostInput.SetValue("myhost.example.com")
	step.portInput.SetValue("6380")
	step.phase = phaseCustomEntry

	err := step.Apply(&cfg)
	if err != nil {
		t.Errorf("expected no error from Apply, got %v", err)
	}

	if cfg.Graph.Host != "myhost.example.com" {
		t.Errorf("expected Graph.Host 'myhost.example.com', got '%s'", cfg.Graph.Host)
	}

	if cfg.Graph.Port != 6380 {
		t.Errorf("expected Graph.Port 6380, got %d", cfg.Graph.Port)
	}
}

func TestFalkorDBStep_Update_Navigation(t *testing.T) {
	step := NewFalkorDBStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Navigate down
	_, result := step.Update(tea.KeyMsg{Type: tea.KeyDown})
	if result != StepContinue {
		t.Errorf("expected StepContinue, got %v", result)
	}
}
