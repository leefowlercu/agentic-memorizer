package steps

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"
)

func TestConfirmStep_Title(t *testing.T) {
	step := NewConfirmStep()
	if step.Title() != "Confirm" {
		t.Errorf("expected title 'Confirm', got '%s'", step.Title())
	}
}

func TestConfirmStep_Init(t *testing.T) {
	cfg := viper.New()
	cfg.Set("graph.host", "localhost")
	cfg.Set("graph.port", 6379)
	cfg.Set("semantic.provider", "claude")
	cfg.Set("semantic.model", "claude-sonnet-4")
	cfg.Set("http.port", 7600)

	step := NewConfirmStep()
	cmd := step.Init(cfg)

	if cmd != nil {
		t.Error("expected nil command from Init")
	}
}

func TestConfirmStep_View(t *testing.T) {
	cfg := viper.New()
	cfg.Set("graph.host", "localhost")
	cfg.Set("graph.port", 6379)
	cfg.Set("semantic.provider", "claude")
	cfg.Set("semantic.model", "claude-sonnet-4")
	cfg.Set("embeddings.enabled", true)
	cfg.Set("embeddings.provider", "openai")
	cfg.Set("http.port", 7600)

	step := NewConfirmStep()
	step.Init(cfg)

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
	cfg := viper.New()
	cfg.Set("graph.host", "localhost")
	cfg.Set("graph.port", 6379)
	cfg.Set("semantic.provider", "openai")
	cfg.Set("semantic.model", "gpt-4o")
	cfg.Set("embeddings.enabled", false)
	cfg.Set("http.port", 8080)

	step := NewConfirmStep()
	step.Init(cfg)

	view := step.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestConfirmStep_Validate(t *testing.T) {
	step := NewConfirmStep()
	cfg := viper.New()
	step.Init(cfg)

	// Validate should always pass for confirm step
	err := step.Validate()
	if err != nil {
		t.Errorf("expected no validation error, got %v", err)
	}
}

func TestConfirmStep_Apply(t *testing.T) {
	step := NewConfirmStep()
	cfg := viper.New()
	step.Init(cfg)

	// Apply should be no-op for confirm step
	err := step.Apply(cfg)
	if err != nil {
		t.Errorf("expected no error from Apply, got %v", err)
	}
}

func TestConfirmStep_Update_Enter(t *testing.T) {
	step := NewConfirmStep()
	cfg := viper.New()
	step.Init(cfg)

	_, result := step.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if result != StepNext {
		t.Errorf("expected StepNext on Enter, got %v", result)
	}
}

func TestConfirmStep_Update_Esc(t *testing.T) {
	step := NewConfirmStep()
	cfg := viper.New()
	step.Init(cfg)

	_, result := step.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if result != StepPrev {
		t.Errorf("expected StepPrev on Esc, got %v", result)
	}
}

func TestConfirmStep_Update_Navigation(t *testing.T) {
	step := NewConfirmStep()
	cfg := viper.New()
	step.Init(cfg)

	// Other keys should continue
	_, result := step.Update(tea.KeyMsg{Type: tea.KeyDown})
	if result != StepContinue {
		t.Errorf("expected StepContinue, got %v", result)
	}
}
