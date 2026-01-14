package steps

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
)

func TestEmbeddingsStep_Title(t *testing.T) {
	step := NewEmbeddingsStep()
	if step.Title() != "Embeddings" {
		t.Errorf("expected title 'Embeddings', got '%s'", step.Title())
	}
}

func TestEmbeddingsStep_Init(t *testing.T) {
	cfg := config.NewDefaultConfig()
	step := NewEmbeddingsStep()

	cmd := step.Init(&cfg)
	if cmd != nil {
		t.Error("expected nil command from Init")
	}
}

func TestEmbeddingsStep_View(t *testing.T) {
	step := NewEmbeddingsStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	view := step.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestEmbeddingsStep_DisableEmbeddings(t *testing.T) {
	step := NewEmbeddingsStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Select "Disable" option (second option)
	step.enableRadio.SetCursor(1)
	step.phase = embPhaseEnable

	// Press Enter to confirm disable
	_, result := step.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if result != StepNext {
		t.Errorf("expected StepNext after disabling, got %v", result)
	}
}

func TestEmbeddingsStep_EnableEmbeddings(t *testing.T) {
	step := NewEmbeddingsStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Select "Enable" option (first option)
	step.enableRadio.SetCursor(0)
	step.phase = embPhaseEnable

	// Press Enter to enable
	_, result := step.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if result != StepContinue {
		t.Errorf("expected StepContinue after enabling, got %v", result)
	}

	if step.phase != embPhaseProvider {
		t.Errorf("expected phase embPhaseProvider after enabling, got %v", step.phase)
	}
}

func TestEmbeddingsStep_Validate_Disabled(t *testing.T) {
	step := NewEmbeddingsStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Disable embeddings
	step.enabled = false

	err := step.Validate()
	if err != nil {
		t.Errorf("expected no validation error when disabled, got %v", err)
	}
}

func TestEmbeddingsStep_Validate_NoAPIKey(t *testing.T) {
	step := NewEmbeddingsStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Enable embeddings
	step.enabled = true
	step.phase = embPhaseAPIKey
	step.keyInput.SetValue("")

	err := step.Validate()
	if err == nil {
		t.Error("expected validation error for missing API key")
	}
}

func TestEmbeddingsStep_Apply_Disabled(t *testing.T) {
	step := NewEmbeddingsStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	step.enabled = false

	err := step.Apply(&cfg)
	if err != nil {
		t.Errorf("expected no error from Apply, got %v", err)
	}

	if cfg.Embeddings.Enabled != false {
		t.Error("expected Embeddings.Enabled to be false")
	}
}

func TestEmbeddingsStep_Apply_Enabled(t *testing.T) {
	step := NewEmbeddingsStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	step.enabled = true
	step.providerRadio.SetCursor(0) // OpenAI
	step.phase = embPhaseAPIKey
	step.keyInput.SetValue("test-key-12345")

	err := step.Apply(&cfg)
	if err != nil {
		t.Errorf("expected no error from Apply, got %v", err)
	}

	if cfg.Embeddings.Enabled != true {
		t.Error("expected Embeddings.Enabled to be true")
	}

	if cfg.Embeddings.Provider != "openai" {
		t.Errorf("expected Embeddings.Provider 'openai', got '%s'", cfg.Embeddings.Provider)
	}
}

func TestEmbeddingsStep_Update_Navigation(t *testing.T) {
	step := NewEmbeddingsStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Navigate down
	_, result := step.Update(tea.KeyMsg{Type: tea.KeyDown})
	if result != StepContinue {
		t.Errorf("expected StepContinue, got %v", result)
	}
}

func TestEmbeddingsStep_APIKeyDetection(t *testing.T) {
	// Set OpenAI API key in environment
	os.Setenv("OPENAI_API_KEY", "test-key-123")
	defer os.Unsetenv("OPENAI_API_KEY")

	step := NewEmbeddingsStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Verify API key was detected for OpenAI
	if !step.providers[0].KeyDetected {
		t.Error("expected OpenAI API key to be detected from environment")
	}
}

func TestEmbeddingsStep_EscFromEnablePhase(t *testing.T) {
	step := NewEmbeddingsStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Press Esc from enable phase - should go to previous wizard step
	_, result := step.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if result != StepPrev {
		t.Errorf("expected StepPrev from Esc on enable phase, got %v", result)
	}
}

func TestEmbeddingsStep_EscFromProviderPhase(t *testing.T) {
	step := NewEmbeddingsStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Move to provider phase
	step.phase = embPhaseProvider

	// Press Esc - should go back to enable phase
	_, result := step.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if result != StepContinue {
		t.Errorf("expected StepContinue from Esc on provider phase, got %v", result)
	}

	if step.phase != embPhaseEnable {
		t.Errorf("expected phase embPhaseEnable after Esc, got %v", step.phase)
	}
}

func TestEmbeddingsStep_Apply_SetsAPIKeyEnv(t *testing.T) {
	tests := []struct {
		name           string
		providerIdx    int
		expectedName   string
		expectedKeyEnv string
	}{
		{
			name:           "openai provider sets OPENAI_API_KEY",
			providerIdx:    0,
			expectedName:   "openai",
			expectedKeyEnv: "OPENAI_API_KEY",
		},
		{
			name:           "voyage provider sets VOYAGE_API_KEY",
			providerIdx:    1,
			expectedName:   "voyage",
			expectedKeyEnv: "VOYAGE_API_KEY",
		},
		{
			name:           "google provider sets GOOGLE_API_KEY",
			providerIdx:    2,
			expectedName:   "google",
			expectedKeyEnv: "GOOGLE_API_KEY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := NewEmbeddingsStep()
			cfg := config.NewDefaultConfig()
			step.Init(&cfg)

			// Enable embeddings and select provider
			step.enabled = true
			step.providerRadio.SetCursor(tt.providerIdx)
			step.selectedIdx = tt.providerIdx
			step.buildModelRadio()
			step.phase = embPhaseAPIKey
			step.keyInput.SetValue("test-api-key")

			err := step.Apply(&cfg)
			if err != nil {
				t.Fatalf("unexpected error from Apply: %v", err)
			}

			if cfg.Embeddings.Provider != tt.expectedName {
				t.Errorf("expected Provider %q, got %q", tt.expectedName, cfg.Embeddings.Provider)
			}

			if cfg.Embeddings.APIKeyEnv != tt.expectedKeyEnv {
				t.Errorf("expected APIKeyEnv %q, got %q", tt.expectedKeyEnv, cfg.Embeddings.APIKeyEnv)
			}
		})
	}
}
