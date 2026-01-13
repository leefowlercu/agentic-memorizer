package steps

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
)

func TestConfirmStep_Title(t *testing.T) {
	step := NewConfirmStep()
	if step.Title() != "Confirm" {
		t.Errorf("expected title 'Confirm', got '%s'", step.Title())
	}
}

func TestConfirmStep_Init(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Graph.Host = "localhost"
	cfg.Graph.Port = 6379
	cfg.Semantic.Provider = "claude"
	cfg.Semantic.Model = "claude-sonnet-4"
	cfg.Daemon.HTTPPort = 7600

	step := NewConfirmStep()
	cmd := step.Init(&cfg)

	if cmd != nil {
		t.Error("expected nil command from Init")
	}
}

func TestConfirmStep_View(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Graph.Host = "localhost"
	cfg.Graph.Port = 6379
	cfg.Semantic.Provider = "claude"
	cfg.Semantic.Model = "claude-sonnet-4"
	cfg.Embeddings.Enabled = true
	cfg.Embeddings.Provider = "openai"
	cfg.Daemon.HTTPPort = 7600

	step := NewConfirmStep()
	step.Init(&cfg)

	view := step.View()
	if view == "" {
		t.Error("expected non-empty view")
	}

	// Check that config summary is shown
	if len(view) < 100 {
		t.Error("expected view to contain configuration summary")
	}
}

func TestConfirmStep_View_EmbeddingsDisabled(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Graph.Host = "localhost"
	cfg.Graph.Port = 6379
	cfg.Semantic.Provider = "openai"
	cfg.Semantic.Model = "gpt-4o"
	cfg.Embeddings.Enabled = false
	cfg.Daemon.HTTPPort = 8080

	step := NewConfirmStep()
	step.Init(&cfg)

	view := step.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestConfirmStep_Validate(t *testing.T) {
	step := NewConfirmStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Validate should always pass for confirm step
	err := step.Validate()
	if err != nil {
		t.Errorf("expected no validation error, got %v", err)
	}
}

func TestConfirmStep_Apply(t *testing.T) {
	step := NewConfirmStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Apply should be no-op for confirm step
	err := step.Apply(&cfg)
	if err != nil {
		t.Errorf("expected no error from Apply, got %v", err)
	}
}

func TestConfirmStep_Update_Enter(t *testing.T) {
	step := NewConfirmStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	_, result := step.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if result != StepNext {
		t.Errorf("expected StepNext on Enter, got %v", result)
	}
}

func TestConfirmStep_Update_Esc(t *testing.T) {
	step := NewConfirmStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	_, result := step.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if result != StepPrev {
		t.Errorf("expected StepPrev on Esc, got %v", result)
	}
}

func TestConfirmStep_Update_Navigation(t *testing.T) {
	step := NewConfirmStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Other keys should continue
	_, result := step.Update(tea.KeyMsg{Type: tea.KeyDown})
	if result != StepContinue {
		t.Errorf("expected StepContinue, got %v", result)
	}
}
