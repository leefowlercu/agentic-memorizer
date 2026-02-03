package steps

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
)

func TestSemanticProviderStep_Title(t *testing.T) {
	step := NewSemanticProviderStep()
	if step.Title() != "Semantic Provider" {
		t.Errorf("expected title 'Semantic Provider', got '%s'", step.Title())
	}
}

func TestSemanticProviderStep_Init(t *testing.T) {
	cfg := config.NewDefaultConfig()
	step := NewSemanticProviderStep()

	cmd := step.Init(&cfg)
	if cmd != nil {
		t.Error("expected nil command from Init")
	}
}

func TestSemanticProviderStep_View(t *testing.T) {
	step := NewSemanticProviderStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	view := step.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestSemanticProviderStep_Validate_NoAPIKey(t *testing.T) {
	step := NewSemanticProviderStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Clear any API keys
	os.Unsetenv("ANTHROPIC_API_KEY")

	// Select Anthropic provider and advance to model phase
	step.providerRadio.SetCursor(0) // Anthropic
	step.phase = phaseModel

	// Select a model and advance to key phase
	step.phase = phaseAPIKey
	step.keyInput.SetValue("")

	err := step.Validate()
	if err == nil {
		t.Error("expected validation error for missing API key")
	}
}

func TestSemanticProviderStep_Validate_WithAPIKey(t *testing.T) {
	step := NewSemanticProviderStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Set API key
	step.phase = phaseAPIKey
	step.keyInput.SetValue("sk-test-key-12345")

	err := step.Validate()
	if err != nil {
		t.Errorf("expected no validation error, got %v", err)
	}
}

func TestSemanticProviderStep_Apply(t *testing.T) {
	step := NewSemanticProviderStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Set provider to Anthropic (Claude models)
	step.providerRadio.SetCursor(0)
	step.phase = phaseAPIKey
	step.keyInput.SetValue("sk-test-key-12345")

	err := step.Apply(&cfg)
	if err != nil {
		t.Errorf("expected no error from Apply, got %v", err)
	}

	if cfg.Semantic.Provider != "anthropic" {
		t.Errorf("expected Semantic.Provider 'anthropic', got '%s'", cfg.Semantic.Provider)
	}

	if cfg.Semantic.APIKey == nil || *cfg.Semantic.APIKey != "sk-test-key-12345" {
		t.Errorf("expected Semantic.APIKey to be set")
	}
}

func TestSemanticProviderStep_Update_Navigation(t *testing.T) {
	step := NewSemanticProviderStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Navigate down
	_, result := step.Update(tea.KeyMsg{Type: tea.KeyDown})
	if result != StepContinue {
		t.Errorf("expected StepContinue, got %v", result)
	}
}

func TestSemanticProviderStep_ProviderSelection(t *testing.T) {
	step := NewSemanticProviderStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Enable semantic analysis
	_, result := step.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if result != StepContinue {
		t.Errorf("expected StepContinue after enable selection, got %v", result)
	}

	if step.phase != phaseProvider {
		t.Errorf("expected phase phaseProvider after enable selection, got %v", step.phase)
	}

	// Select Anthropic provider with Enter
	_, result = step.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if result != StepContinue {
		t.Errorf("expected StepContinue after provider selection, got %v", result)
	}

	if step.phase != phaseModel {
		t.Errorf("expected phase phaseModel after provider selection, got %v", step.phase)
	}
}

func TestSemanticProviderStep_APIKeyDetection(t *testing.T) {
	// Set API key in environment
	os.Setenv("ANTHROPIC_API_KEY", "test-key-123")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	step := NewSemanticProviderStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Verify API key was detected
	if !step.providers[0].KeyDetected {
		t.Error("expected Anthropic API key to be detected from environment")
	}
}

func TestSemanticProviderStep_OpenAIAPIKeyDetection(t *testing.T) {
	// Set OpenAI API key
	os.Setenv("OPENAI_API_KEY", "sk-openai-test")
	defer os.Unsetenv("OPENAI_API_KEY")

	step := NewSemanticProviderStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Find OpenAI provider (should be second)
	var openaiProvider *ProviderInfo
	for i := range step.providers {
		if step.providers[i].Name == "openai" {
			openaiProvider = &step.providers[i]
			break
		}
	}

	if openaiProvider == nil {
		t.Fatal("expected to find OpenAI provider")
	}

	if !openaiProvider.KeyDetected {
		t.Error("expected OpenAI API key to be detected from environment")
	}
}

func TestSemanticProviderStep_EscFromProviderPhase(t *testing.T) {
	step := NewSemanticProviderStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Move to provider phase
	step.phase = phaseProvider

	// Press Esc from provider phase - should go back to enable phase
	_, result := step.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if result != StepContinue {
		t.Errorf("expected StepContinue from Esc on provider phase, got %v", result)
	}

	if step.phase != phaseEnable {
		t.Errorf("expected phase phaseEnable after Esc, got %v", step.phase)
	}
}

func TestSemanticProviderStep_EscFromModelPhase(t *testing.T) {
	step := NewSemanticProviderStep()
	cfg := config.NewDefaultConfig()
	step.Init(&cfg)

	// Move to model phase
	step.phase = phaseModel

	// Press Esc - should go back to provider phase
	_, result := step.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if result != StepContinue {
		t.Errorf("expected StepContinue from Esc on model phase, got %v", result)
	}

	if step.phase != phaseProvider {
		t.Errorf("expected phase phaseProvider after Esc, got %v", step.phase)
	}
}

func TestSemanticProviderStep_Apply_SetsAPIKeyEnv(t *testing.T) {
	tests := []struct {
		name           string
		providerIdx    int
		expectedName   string
		expectedKeyEnv string
	}{
		{
			name:           "anthropic provider sets ANTHROPIC_API_KEY",
			providerIdx:    0,
			expectedName:   "anthropic",
			expectedKeyEnv: "ANTHROPIC_API_KEY",
		},
		{
			name:           "openai provider sets OPENAI_API_KEY",
			providerIdx:    1,
			expectedName:   "openai",
			expectedKeyEnv: "OPENAI_API_KEY",
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
			step := NewSemanticProviderStep()
			cfg := config.NewDefaultConfig()
			step.Init(&cfg)

			// Select provider and set up for Apply
			step.providerRadio.SetCursor(tt.providerIdx)
			step.selectedIdx = tt.providerIdx
			step.buildModelRadio()
			step.phase = phaseAPIKey
			step.keyInput.SetValue("test-api-key")

			err := step.Apply(&cfg)
			if err != nil {
				t.Fatalf("unexpected error from Apply: %v", err)
			}

			if cfg.Semantic.Provider != tt.expectedName {
				t.Errorf("expected Provider %q, got %q", tt.expectedName, cfg.Semantic.Provider)
			}

			if cfg.Semantic.APIKeyEnv != tt.expectedKeyEnv {
				t.Errorf("expected APIKeyEnv %q, got %q", tt.expectedKeyEnv, cfg.Semantic.APIKeyEnv)
			}
		})
	}
}
