package initialize

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"
)

// mockStep is a simple step for testing.
type mockStep struct {
	title     string
	viewText  string
	validated bool
	applied   bool
}

func (m *mockStep) Init(cfg *viper.Viper) tea.Cmd { return nil }

func (m *mockStep) Update(msg tea.Msg) (tea.Cmd, StepResult) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.Type == tea.KeyEnter {
			return nil, StepNext
		}
		if keyMsg.Type == tea.KeyEsc {
			return nil, StepPrev
		}
	}
	return nil, StepContinue
}

func (m *mockStep) View() string      { return m.viewText }
func (m *mockStep) Title() string     { return m.title }
func (m *mockStep) Validate() error   { m.validated = true; return nil }
func (m *mockStep) Apply(*viper.Viper) error { m.applied = true; return nil }

func TestWizardModel_Init(t *testing.T) {
	cfg := viper.New()
	steps := []Step{
		&mockStep{title: "Step 1", viewText: "First step"},
		&mockStep{title: "Step 2", viewText: "Second step"},
	}

	wizard := NewWizard(cfg, steps)

	if wizard.currentStep != 0 {
		t.Errorf("expected currentStep 0, got %d", wizard.currentStep)
	}

	if len(wizard.steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(wizard.steps))
	}
}

func TestWizardModel_View(t *testing.T) {
	cfg := viper.New()
	steps := []Step{
		&mockStep{title: "Step 1", viewText: "First step content"},
	}

	wizard := NewWizard(cfg, steps)
	view := wizard.View()

	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestWizardModel_Navigation_Next(t *testing.T) {
	cfg := viper.New()
	step1 := &mockStep{title: "Step 1", viewText: "First"}
	step2 := &mockStep{title: "Step 2", viewText: "Second"}
	steps := []Step{step1, step2}

	wizard := NewWizard(cfg, steps)

	// Press Enter to go to next step
	model, _ := wizard.Update(tea.KeyMsg{Type: tea.KeyEnter})
	wizard = model.(WizardModel)

	if wizard.currentStep != 1 {
		t.Errorf("expected currentStep 1 after Enter, got %d", wizard.currentStep)
	}

	// Step 1 should have been validated and applied
	if !step1.validated {
		t.Error("step 1 should have been validated")
	}
	if !step1.applied {
		t.Error("step 1 should have been applied")
	}
}

func TestWizardModel_Navigation_Prev(t *testing.T) {
	cfg := viper.New()
	steps := []Step{
		&mockStep{title: "Step 1", viewText: "First"},
		&mockStep{title: "Step 2", viewText: "Second"},
	}

	wizard := NewWizard(cfg, steps)

	// Go to step 2
	model, _ := wizard.Update(tea.KeyMsg{Type: tea.KeyEnter})
	wizard = model.(WizardModel)

	if wizard.currentStep != 1 {
		t.Errorf("expected currentStep 1, got %d", wizard.currentStep)
	}

	// Press Esc to go back
	model, _ = wizard.Update(tea.KeyMsg{Type: tea.KeyEsc})
	wizard = model.(WizardModel)

	if wizard.currentStep != 0 {
		t.Errorf("expected currentStep 0 after Esc, got %d", wizard.currentStep)
	}
}

func TestWizardModel_Cancel(t *testing.T) {
	cfg := viper.New()
	steps := []Step{
		&mockStep{title: "Step 1", viewText: "First"},
	}

	wizard := NewWizard(cfg, steps)

	// Press Ctrl+C to cancel
	model, cmd := wizard.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	wizard = model.(WizardModel)

	if !wizard.cancelled {
		t.Error("expected wizard to be cancelled")
	}

	// Should return quit command
	if cmd == nil {
		t.Error("expected quit command to be returned")
	}
}

func TestWizardResult(t *testing.T) {
	result := WizardResult{
		Confirmed: true,
		Cancelled: false,
		Err:       nil,
	}

	if !result.Confirmed {
		t.Error("expected Confirmed to be true")
	}

	if result.Cancelled {
		t.Error("expected Cancelled to be false")
	}

	if result.Err != nil {
		t.Error("expected Err to be nil")
	}
}
