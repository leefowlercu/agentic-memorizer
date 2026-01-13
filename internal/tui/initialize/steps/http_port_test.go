package steps

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
)

func TestHTTPPortStep_Title(t *testing.T) {
	step := NewHTTPPortStep()
	if step.Title() != "HTTP Port" {
		t.Errorf("expected title 'HTTP Port', got '%s'", step.Title())
	}
}

func TestHTTPPortStep_Init(t *testing.T) {
	cfg := config.NewDefaultConfig()
	step := NewHTTPPortStep()

	cmd := step.Init(&cfg)
	if cmd != nil {
		t.Error("expected nil command from Init")
	}

	// Should have default port
	if step.portInput.Value() != "7600" {
		t.Errorf("expected default port '7600', got '%s'", step.portInput.Value())
	}
}

func TestHTTPPortStep_Init_WithExistingConfig(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Daemon.HTTPPort = 8080

	step := NewHTTPPortStep()
	step.Init(&cfg)

	if step.portInput.Value() != "8080" {
		t.Errorf("expected port '8080' from config, got '%s'", step.portInput.Value())
	}
}

func TestHTTPPortStep_View(t *testing.T) {
	step := NewHTTPPortStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	view := step.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestHTTPPortStep_Validate_ValidPort(t *testing.T) {
	step := NewHTTPPortStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	step.portInput.SetValue("8080")

	err := step.Validate()
	if err != nil {
		t.Errorf("expected no validation error, got %v", err)
	}
}

func TestHTTPPortStep_Validate_InvalidPort_Zero(t *testing.T) {
	step := NewHTTPPortStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	step.portInput.SetValue("0")

	err := step.Validate()
	if err == nil {
		t.Error("expected validation error for port 0")
	}
}

func TestHTTPPortStep_Validate_InvalidPort_TooHigh(t *testing.T) {
	step := NewHTTPPortStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	step.portInput.SetValue("70000")

	err := step.Validate()
	if err == nil {
		t.Error("expected validation error for port 70000")
	}
}

func TestHTTPPortStep_Validate_InvalidPort_NonNumeric(t *testing.T) {
	step := NewHTTPPortStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	step.portInput.SetValue("abc")

	err := step.Validate()
	if err == nil {
		t.Error("expected validation error for non-numeric port")
	}
}

func TestHTTPPortStep_Validate_EmptyPort(t *testing.T) {
	step := NewHTTPPortStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	step.portInput.SetValue("")

	err := step.Validate()
	if err == nil {
		t.Error("expected validation error for empty port")
	}
}

func TestHTTPPortStep_Apply(t *testing.T) {
	step := NewHTTPPortStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	step.portInput.SetValue("9000")

	err := step.Apply(&cfg)
	if err != nil {
		t.Errorf("expected no error from Apply, got %v", err)
	}

	if cfg.Daemon.HTTPPort != 9000 {
		t.Errorf("expected Daemon.HTTPPort 9000, got %d", cfg.Daemon.HTTPPort)
	}
}

func TestHTTPPortStep_Update_Enter(t *testing.T) {
	step := NewHTTPPortStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Default port is valid, so Enter should advance
	_, result := step.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if result != StepNext {
		t.Errorf("expected StepNext with valid port, got %v", result)
	}
}

func TestHTTPPortStep_Update_Esc(t *testing.T) {
	step := NewHTTPPortStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	_, result := step.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if result != StepPrev {
		t.Errorf("expected StepPrev, got %v", result)
	}
}

func TestHTTPPortStep_Update_EnterWithInvalidPort(t *testing.T) {
	step := NewHTTPPortStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	step.portInput.SetValue("invalid")

	_, result := step.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if result != StepContinue {
		t.Errorf("expected StepContinue with invalid port, got %v", result)
	}
}
