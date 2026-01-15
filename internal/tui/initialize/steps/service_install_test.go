package steps

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/servicemanager"
)

func TestNewServiceInstallStep(t *testing.T) {
	step := NewServiceInstallStep()

	if step == nil {
		t.Fatal("NewServiceInstallStep() returned nil")
	}

	if step.Title() != "Start Daemon" {
		t.Errorf("Title() = %v, want 'Start Daemon'", step.Title())
	}

	if step.state != stateSelecting {
		t.Errorf("initial state = %v, want stateSelecting", step.state)
	}
}

func TestServiceInstallStep_Init(t *testing.T) {
	step := NewServiceInstallStep()
	cfg := config.NewDefaultConfig()

	cmd := step.Init(&cfg)

	if cmd != nil {
		t.Error("Init() should return nil cmd")
	}

	if step.state != stateSelecting {
		t.Errorf("state after Init() = %v, want stateSelecting", step.state)
	}

	if step.err != nil {
		t.Errorf("err after Init() = %v, want nil", step.err)
	}
}

func TestServiceInstallStep_Update_SelectNo(t *testing.T) {
	step := NewServiceInstallStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Move cursor to "No" option
	step.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Press Enter to select
	_, result := step.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if result != StepContinue {
		t.Errorf("Update() result = %v, want StepContinue", result)
	}

	if step.state != stateSkipped {
		t.Errorf("state after selecting No = %v, want stateSkipped", step.state)
	}
}

func TestServiceInstallStep_Update_EscGoesBack(t *testing.T) {
	step := NewServiceInstallStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	_, result := step.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if result != StepPrev {
		t.Errorf("Update(Esc) result = %v, want StepPrev", result)
	}
}

func TestServiceInstallStep_Update_Navigation(t *testing.T) {
	step := NewServiceInstallStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Initially cursor should be at 0 (Yes)
	if step.radio.Selected() != optionYes {
		t.Errorf("initial selection = %v, want %v", step.radio.Selected(), optionYes)
	}

	// Move down
	step.Update(tea.KeyMsg{Type: tea.KeyDown})

	if step.radio.Selected() != optionNo {
		t.Errorf("selection after down = %v, want %v", step.radio.Selected(), optionNo)
	}

	// Move up
	step.Update(tea.KeyMsg{Type: tea.KeyUp})

	if step.radio.Selected() != optionYes {
		t.Errorf("selection after up = %v, want %v", step.radio.Selected(), optionYes)
	}
}

func TestServiceInstallStep_Update_ProgressMessages(t *testing.T) {
	step := NewServiceInstallStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Simulate progress message
	step.Update(installProgressMsg{
		state:   stateInstalling,
		message: "Installing...",
	})

	if step.state != stateInstalling {
		t.Errorf("state = %v, want stateInstalling", step.state)
	}

	if step.progressMsg != "Installing..." {
		t.Errorf("progressMsg = %v, want 'Installing...'", step.progressMsg)
	}
}

func TestServiceInstallStep_Update_DoneMessage(t *testing.T) {
	step := NewServiceInstallStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Simulate done message
	step.Update(installDoneMsg{message: "Success!"})

	if step.state != stateDone {
		t.Errorf("state = %v, want stateDone", step.state)
	}

	if step.resultMsg != "Success!" {
		t.Errorf("resultMsg = %v, want 'Success!'", step.resultMsg)
	}
}

func TestServiceInstallStep_Update_ErrorMessage(t *testing.T) {
	step := NewServiceInstallStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	testErr := errors.New("test error")
	step.Update(installErrorMsg{
		err:     testErr,
		message: "Failed!",
	})

	if step.state != stateFailed {
		t.Errorf("state = %v, want stateFailed", step.state)
	}

	if step.err != testErr {
		t.Errorf("err = %v, want %v", step.err, testErr)
	}

	if step.resultMsg != "Failed!" {
		t.Errorf("resultMsg = %v, want 'Failed!'", step.resultMsg)
	}
}

func TestServiceInstallStep_Update_EnterAfterDone(t *testing.T) {
	step := NewServiceInstallStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Set state to done
	step.state = stateDone

	_, result := step.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if result != StepNext {
		t.Errorf("Update(Enter) in done state = %v, want StepNext", result)
	}
}

func TestServiceInstallStep_Update_EnterAfterFailed(t *testing.T) {
	step := NewServiceInstallStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Set state to failed
	step.state = stateFailed

	_, result := step.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if result != StepNext {
		t.Errorf("Update(Enter) in failed state = %v, want StepNext", result)
	}
}

func TestServiceInstallStep_Update_EnterAfterSkipped(t *testing.T) {
	step := NewServiceInstallStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Set state to skipped
	step.state = stateSkipped

	_, result := step.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if result != StepNext {
		t.Errorf("Update(Enter) in skipped state = %v, want StepNext", result)
	}
}

func TestServiceInstallStep_View_Selecting(t *testing.T) {
	step := NewServiceInstallStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	view := step.View()

	expectedContent := []string{
		"Start Daemon Service",
		"install and start",
		"Yes",
		"No",
	}

	for _, content := range expectedContent {
		if !strings.Contains(view, content) {
			t.Errorf("View() missing content: %s", content)
		}
	}
}

func TestServiceInstallStep_View_Progress(t *testing.T) {
	step := NewServiceInstallStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	step.state = stateInstalling
	step.progressMsg = "Working..."

	view := step.View()

	if !strings.Contains(view, "Installing service") {
		t.Error("View() in installing state should show 'Installing service'")
	}
}

func TestServiceInstallStep_View_Done(t *testing.T) {
	step := NewServiceInstallStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	step.state = stateDone
	step.resultMsg = "Daemon installed and running"

	view := step.View()

	if !strings.Contains(view, "Daemon installed") {
		t.Error("View() in done state should show result message")
	}

	if !strings.Contains(view, "Service Information") {
		t.Error("View() in done state should show service info")
	}
}

func TestServiceInstallStep_View_Failed(t *testing.T) {
	step := NewServiceInstallStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	step.state = stateFailed
	step.resultMsg = "Installation failed"
	step.err = errors.New("test error")

	view := step.View()

	if !strings.Contains(view, "Installation failed") {
		t.Error("View() in failed state should show result message")
	}

	if !strings.Contains(view, "Troubleshooting") {
		t.Error("View() in failed state should show troubleshooting")
	}

	if !strings.Contains(view, "Configuration was saved") {
		t.Error("View() in failed state should indicate config was saved")
	}
}

func TestServiceInstallStep_View_Skipped(t *testing.T) {
	step := NewServiceInstallStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	step.state = stateSkipped

	view := step.View()

	if !strings.Contains(view, "skipped") {
		t.Error("View() in skipped state should mention skipped")
	}

	if !strings.Contains(view, "memorizer daemon start") {
		t.Error("View() in skipped state should show manual start command")
	}
}

func TestServiceInstallStep_Validate(t *testing.T) {
	step := NewServiceInstallStep()

	err := step.Validate()

	if err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestServiceInstallStep_Apply(t *testing.T) {
	// Create temp dir for config file
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	step := NewServiceInstallStep()
	cfg := config.NewDefaultConfig()

	// Init sets the cfg field on the step
	step.Init(&cfg)

	err := step.Apply(&cfg)

	if err != nil {
		t.Errorf("Apply() = %v, want nil", err)
	}
}

// mockDaemonManager is a test double for servicemanager.DaemonManager.
type mockDaemonManager struct {
	installCalled    bool
	startCalled      bool
	stopCalled       bool
	statusCalled     bool
	isInstalledValue bool
	installErr       error
	startErr         error
	statusValue      servicemanager.DaemonStatus
}

func (m *mockDaemonManager) Install(ctx context.Context) error {
	m.installCalled = true
	return m.installErr
}

func (m *mockDaemonManager) Uninstall(ctx context.Context) error {
	return nil
}

func (m *mockDaemonManager) StartDaemon(ctx context.Context) error {
	m.startCalled = true
	return m.startErr
}

func (m *mockDaemonManager) StopDaemon(ctx context.Context) error {
	m.stopCalled = true
	return nil
}

func (m *mockDaemonManager) Restart(ctx context.Context) error {
	return nil
}

func (m *mockDaemonManager) Status(ctx context.Context) (servicemanager.DaemonStatus, error) {
	m.statusCalled = true
	return m.statusValue, nil
}

func (m *mockDaemonManager) IsInstalled() (bool, error) {
	return m.isInstalledValue, nil
}

func TestServiceInstallStep_SetManagerFactory(t *testing.T) {
	step := NewServiceInstallStep()

	mockMgr := &mockDaemonManager{}
	step.SetManagerFactory(func() (servicemanager.DaemonManager, error) {
		return mockMgr, nil
	})

	// Verify factory was set by calling it
	if step.managerFactory == nil {
		t.Error("SetManagerFactory() did not set factory")
	}

	mgr, err := step.managerFactory()
	if err != nil {
		t.Errorf("factory returned error: %v", err)
	}
	if mgr != mockMgr {
		t.Error("factory did not return expected manager")
	}
}
