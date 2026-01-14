package steps

import (
	"errors"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize/components"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// Embeddings step phases.
type embeddingsPhase int

const (
	embPhaseEnable embeddingsPhase = iota
	embPhaseProvider
	embPhaseModel
	embPhaseAPIKey
)

// EmbeddingsModelInfo describes an embeddings model.
type EmbeddingsModelInfo struct {
	ID          string
	DisplayName string
	Description string
	Dimensions  int
}

// EmbeddingsProviderInfo describes an embeddings provider.
type EmbeddingsProviderInfo struct {
	Name        string
	DisplayName string
	EnvVar      string
	KeyDetected bool
	Models      []EmbeddingsModelInfo
}

// EmbeddingsStep handles embeddings provider configuration.
type EmbeddingsStep struct {
	BaseStep

	phase         embeddingsPhase
	enabled       bool
	providers     []EmbeddingsProviderInfo
	enableRadio   components.RadioGroup
	providerRadio components.RadioGroup
	modelRadio    components.RadioGroup
	keyInput      components.TextInput
	selectedIdx   int
}

// NewEmbeddingsStep creates a new embeddings configuration step.
func NewEmbeddingsStep() *EmbeddingsStep {
	return &EmbeddingsStep{
		BaseStep: NewBaseStep("Embeddings"),
		keyInput: components.NewTextInput("API Key:", ""),
	}
}

// buildEmbeddingsProviders returns the available embeddings providers.
func buildEmbeddingsProviders() []EmbeddingsProviderInfo {
	return []EmbeddingsProviderInfo{
		{
			Name:        "openai",
			DisplayName: "OpenAI",
			EnvVar:      "OPENAI_API_KEY",
			Models: []EmbeddingsModelInfo{
				{ID: "text-embedding-3-large", DisplayName: "Embedding 3 Large", Description: "Highest quality, 3072 dimensions", Dimensions: 3072},
				{ID: "text-embedding-3-small", DisplayName: "Embedding 3 Small", Description: "Cost-effective, 1536 dimensions", Dimensions: 1536},
			},
		},
		{
			Name:        "voyage",
			DisplayName: "Voyage AI",
			EnvVar:      "VOYAGE_API_KEY",
			Models: []EmbeddingsModelInfo{
				{ID: "voyage-3-large", DisplayName: "Voyage 3 Large", Description: "State-of-the-art, 1024 dimensions", Dimensions: 1024},
				{ID: "voyage-3.5", DisplayName: "Voyage 3.5", Description: "Latest general-purpose", Dimensions: 1024},
				{ID: "voyage-code-3", DisplayName: "Voyage Code 3", Description: "Optimized for code", Dimensions: 1024},
			},
		},
		{
			Name:        "google",
			DisplayName: "Google (Gemini)",
			EnvVar:      "GOOGLE_API_KEY",
			Models: []EmbeddingsModelInfo{
				{ID: "gemini-embedding-001", DisplayName: "Gemini Embedding", Description: "Latest model, 3072 dimensions", Dimensions: 3072},
				{ID: "text-embedding-004", DisplayName: "Text Embedding 004", Description: "Previous model, 768 dimensions", Dimensions: 768},
			},
		},
	}
}

// Init initializes the step.
func (s *EmbeddingsStep) Init(cfg *config.Config) tea.Cmd {
	s.providers = buildEmbeddingsProviders()

	// Detect API keys from environment
	for i := range s.providers {
		s.providers[i].KeyDetected = os.Getenv(s.providers[i].EnvVar) != ""
	}

	// Build enable/disable radio
	s.enableRadio = components.NewRadioGroup([]components.RadioOption{
		{Label: "Enable vector embeddings", Value: "enable", Description: "Enables semantic search via vector similarity"},
		{Label: "Disable vector embeddings", Value: "disable", Description: "Skip embeddings configuration"},
	})

	// Build provider radio
	var providerOptions []components.RadioOption
	for _, p := range s.providers {
		label := p.DisplayName
		if p.KeyDetected {
			label += " (API Key detected)"
		}
		providerOptions = append(providerOptions, components.RadioOption{
			Label:       label,
			Value:       p.Name,
			Description: "Environment variable: " + p.EnvVar,
		})
	}
	s.providerRadio = components.NewRadioGroup(providerOptions)

	s.phase = embPhaseEnable
	s.enabled = false

	// Pre-fill from existing config
	if cfg.Embeddings.Enabled {
		s.enabled = true
		s.enableRadio.SetCursor(0)
		if cfg.Embeddings.Provider != "" {
			for i, p := range s.providers {
				if p.Name == cfg.Embeddings.Provider {
					s.providerRadio.SetCursor(i)
					s.selectedIdx = i
					break
				}
			}
		}
	}

	return nil
}

// Update handles input and phase transitions.
func (s *EmbeddingsStep) Update(msg tea.Msg) (tea.Cmd, StepResult) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, StepContinue
	}

	switch s.phase {
	case embPhaseEnable:
		return s.handleEnablePhase(keyMsg)
	case embPhaseProvider:
		return s.handleProviderPhase(keyMsg)
	case embPhaseModel:
		return s.handleModelPhase(keyMsg)
	case embPhaseAPIKey:
		return s.handleAPIKeyPhase(keyMsg)
	}

	return nil, StepContinue
}

func (s *EmbeddingsStep) handleEnablePhase(msg tea.KeyMsg) (tea.Cmd, StepResult) {
	switch msg.Type {
	case tea.KeyEnter:
		if s.enableRadio.Selected() == "disable" {
			s.enabled = false
			return nil, StepNext
		}
		s.enabled = true
		s.phase = embPhaseProvider
		return nil, StepContinue

	case tea.KeyEsc:
		return nil, StepPrev

	default:
		s.enableRadio, _ = s.enableRadio.Update(msg)
		return nil, StepContinue
	}
}

func (s *EmbeddingsStep) handleProviderPhase(msg tea.KeyMsg) (tea.Cmd, StepResult) {
	switch msg.Type {
	case tea.KeyEnter:
		s.selectedIdx = s.providerRadio.Cursor()
		s.buildModelRadio()
		s.phase = embPhaseModel
		return nil, StepContinue

	case tea.KeyEsc:
		s.phase = embPhaseEnable
		return nil, StepContinue

	default:
		s.providerRadio, _ = s.providerRadio.Update(msg)
		return nil, StepContinue
	}
}

func (s *EmbeddingsStep) handleModelPhase(msg tea.KeyMsg) (tea.Cmd, StepResult) {
	switch msg.Type {
	case tea.KeyEnter:
		provider := s.providers[s.selectedIdx]
		if provider.KeyDetected {
			return nil, StepNext
		}
		s.phase = embPhaseAPIKey
		s.keyInput.SetMasked(true)
		s.keyInput.Focus()
		return nil, StepContinue

	case tea.KeyEsc:
		s.phase = embPhaseProvider
		return nil, StepContinue

	default:
		s.modelRadio, _ = s.modelRadio.Update(msg)
		return nil, StepContinue
	}
}

func (s *EmbeddingsStep) handleAPIKeyPhase(msg tea.KeyMsg) (tea.Cmd, StepResult) {
	switch msg.Type {
	case tea.KeyEnter:
		if err := s.Validate(); err == nil {
			return nil, StepNext
		}
		return nil, StepContinue

	case tea.KeyEsc:
		s.phase = embPhaseModel
		s.keyInput.Blur()
		return nil, StepContinue

	default:
		s.keyInput, _ = s.keyInput.Update(msg)
		return nil, StepContinue
	}
}

func (s *EmbeddingsStep) buildModelRadio() {
	provider := s.providers[s.selectedIdx]
	var options []components.RadioOption

	for _, m := range provider.Models {
		options = append(options, components.RadioOption{
			Label:       m.DisplayName,
			Value:       m.ID,
			Description: m.Description,
		})
	}

	s.modelRadio = components.NewRadioGroup(options)
}

// View renders the step UI.
func (s *EmbeddingsStep) View() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Secondary).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("Vector Embeddings"))
	b.WriteString("\n\n")

	switch s.phase {
	case embPhaseEnable:
		b.WriteString(s.viewEnablePhase())
	case embPhaseProvider:
		b.WriteString(s.viewProviderPhase())
	case embPhaseModel:
		b.WriteString(s.viewModelPhase())
	case embPhaseAPIKey:
		b.WriteString(s.viewAPIKeyPhase())
	}

	b.WriteString("\n")

	// Navigation help
	b.WriteString("\n")
	if s.phase == embPhaseAPIKey {
		b.WriteString(NavigationHelpWithInput())
	} else {
		b.WriteString(NavigationHelp())
	}

	return b.String()
}

func (s *EmbeddingsStep) viewEnablePhase() string {
	var b strings.Builder

	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	b.WriteString(mutedStyle.Render("Vector embeddings enable semantic search capabilities:"))
	b.WriteString("\n\n")

	b.WriteString(s.enableRadio.View())

	return b.String()
}

func (s *EmbeddingsStep) viewProviderPhase() string {
	var b strings.Builder

	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	b.WriteString(mutedStyle.Render("Select your embeddings provider:"))
	b.WriteString("\n\n")

	b.WriteString(s.providerRadio.View())

	return b.String()
}

func (s *EmbeddingsStep) viewModelPhase() string {
	var b strings.Builder

	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	provider := s.providers[s.selectedIdx]

	b.WriteString(mutedStyle.Render("Select an embeddings model for " + provider.DisplayName + ":"))
	b.WriteString("\n\n")

	b.WriteString(s.modelRadio.View())

	// Show API key status
	b.WriteString("\n")
	b.WriteString(FormatKeyStatus(provider.KeyDetected))

	return b.String()
}

func (s *EmbeddingsStep) viewAPIKeyPhase() string {
	var b strings.Builder

	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	provider := s.providers[s.selectedIdx]

	b.WriteString(mutedStyle.Render("Enter your API key for " + provider.DisplayName + ":"))
	b.WriteString("\n\n")

	b.WriteString(s.keyInput.View())
	b.WriteString("\n\n")

	b.WriteString(mutedStyle.Render("Tip: Set " + provider.EnvVar + " environment variable to auto-detect."))

	return b.String()
}

// Validate checks the step configuration.
func (s *EmbeddingsStep) Validate() error {
	if !s.enabled {
		return nil
	}

	if s.phase == embPhaseAPIKey {
		key := strings.TrimSpace(s.keyInput.Value())
		if key == "" {
			return errors.New("API key is required")
		}
	}

	return nil
}

// Apply writes the embeddings configuration.
func (s *EmbeddingsStep) Apply(cfg *config.Config) error {
	cfg.Embeddings.Enabled = s.enabled

	if !s.enabled {
		return nil
	}

	provider := s.providers[s.selectedIdx]
	model := provider.Models[s.modelRadio.Cursor()]

	cfg.Embeddings.Provider = provider.Name
	cfg.Embeddings.Model = model.ID
	cfg.Embeddings.Dimensions = model.Dimensions
	cfg.Embeddings.APIKeyEnv = provider.EnvVar

	// Store API key if provided
	if s.phase == embPhaseAPIKey {
		key := strings.TrimSpace(s.keyInput.Value())
		if key != "" {
			cfg.Embeddings.APIKey = &key
		}
	} else if provider.KeyDetected {
		key := os.Getenv(provider.EnvVar)
		cfg.Embeddings.APIKey = &key
	}

	return nil
}
