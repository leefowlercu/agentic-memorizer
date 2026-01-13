package steps

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"
)

func TestFalkorDBStep_Title(t *testing.T) {
	step := NewFalkorDBStep()
	if step.Title() != "FalkorDB" {
		t.Errorf("expected title 'FalkorDB', got '%s'", step.Title())
	}
}

func TestFalkorDBStep_Init(t *testing.T) {
	cfg := viper.New()
	step := NewFalkorDBStep()

	cmd := step.Init(cfg)
	// Init should return nil (no async command needed initially)
	if cmd != nil {
		t.Error("expected nil command from Init")
	}
}

func TestFalkorDBStep_View(t *testing.T) {
	step := NewFalkorDBStep()
	cfg := viper.New()
	step.Init(cfg)

	view := step.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestFalkorDBStep_Validate_CustomHost(t *testing.T) {
	step := NewFalkorDBStep()
	cfg := viper.New()
	step.Init(cfg)

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
	cfg := viper.New()
	step.Init(cfg)

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
	cfg := viper.New()
	step.Init(cfg)

	step.hostInput.SetValue("myhost.example.com")
	step.portInput.SetValue("6380")
	step.phase = phaseCustomEntry

	err := step.Apply(cfg)
	if err != nil {
		t.Errorf("expected no error from Apply, got %v", err)
	}

	if cfg.GetString("graph.host") != "myhost.example.com" {
		t.Errorf("expected graph.host 'myhost.example.com', got '%s'", cfg.GetString("graph.host"))
	}

	if cfg.GetInt("graph.port") != 6380 {
		t.Errorf("expected graph.port 6380, got %d", cfg.GetInt("graph.port"))
	}
}

func TestFalkorDBStep_Update_Navigation(t *testing.T) {
	step := NewFalkorDBStep()
	cfg := viper.New()
	step.Init(cfg)

	// Navigate down
	_, result := step.Update(tea.KeyMsg{Type: tea.KeyDown})
	if result != StepContinue {
		t.Errorf("expected StepContinue, got %v", result)
	}
}
