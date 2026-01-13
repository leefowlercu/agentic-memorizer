package steps

import (
	"errors"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/viper"

	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize/components"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// Provider selection phases.
type providerPhase int

const (
	phaseProvider providerPhase = iota
	phaseModel
	phaseAPIKey
)

// ModelInfo describes an LLM model.
type ModelInfo struct {
	ID          string
	DisplayName string
	Description string
	InputCost   float64 // per 1M tokens
	OutputCost  float64 // per 1M tokens
}

// ProviderInfo describes an LLM provider.
type ProviderInfo struct {
	Name        string
	DisplayName string
	EnvVar      string
	KeyDetected bool
	Models      []ModelInfo
	DefaultRate int // requests per minute
}

// SemanticProviderStep handles semantic analysis provider configuration.
type SemanticProviderStep struct {
	BaseStep

	phase         providerPhase
	providers     []ProviderInfo
	providerRadio components.RadioGroup
	modelRadio    components.RadioGroup
	keyInput      components.TextInput
	selectedIdx   int
}

// NewSemanticProviderStep creates a new semantic provider configuration step.
func NewSemanticProviderStep() *SemanticProviderStep {
	return &SemanticProviderStep{
		BaseStep: NewBaseStep("Semantic Provider"),
		keyInput: components.NewTextInput("API Key:", ""),
	}
}

// buildProviders returns the available LLM providers.
func buildProviders() []ProviderInfo {
	return []ProviderInfo{
		{
			Name:        "claude",
			DisplayName: "Claude (Anthropic)",
			EnvVar:      "ANTHROPIC_API_KEY",
			DefaultRate: 50,
			Models: []ModelInfo{
				{ID: "claude-sonnet-4-5-20250929", DisplayName: "Claude Sonnet 4.5", Description: "Latest balanced model, 1M context", InputCost: 3.0, OutputCost: 15.0},
				{ID: "claude-opus-4-5-20251101", DisplayName: "Claude Opus 4.5", Description: "Most capable model", InputCost: 15.0, OutputCost: 75.0},
				{ID: "claude-haiku-4-5-20251015", DisplayName: "Claude Haiku 4.5", Description: "Fast and cost-effective", InputCost: 1.0, OutputCost: 5.0},
			},
		},
		{
			Name:        "openai",
			DisplayName: "OpenAI",
			EnvVar:      "OPENAI_API_KEY",
			DefaultRate: 60,
			Models: []ModelInfo{
				{ID: "gpt-5.2", DisplayName: "GPT-5.2", Description: "Flagship model, 400K context", InputCost: 2.5, OutputCost: 10.0},
				{ID: "gpt-5.2-pro", DisplayName: "GPT-5.2 Pro", Description: "Enhanced reasoning", InputCost: 5.0, OutputCost: 20.0},
				{ID: "gpt-5-mini", DisplayName: "GPT-5 Mini", Description: "Fast and affordable", InputCost: 0.5, OutputCost: 2.0},
			},
		},
		{
			Name:        "gemini",
			DisplayName: "Gemini (Google)",
			EnvVar:      "GOOGLE_API_KEY",
			DefaultRate: 60,
			Models: []ModelInfo{
				{ID: "gemini-3-pro-preview", DisplayName: "Gemini 3 Pro", Description: "Most advanced reasoning, 1M context", InputCost: 2.0, OutputCost: 12.0},
				{ID: "gemini-3-flash-preview", DisplayName: "Gemini 3 Flash", Description: "Fast frontier model", InputCost: 0.1, OutputCost: 0.4},
				{ID: "gemini-2.5-flash", DisplayName: "Gemini 2.5 Flash", Description: "Stable fast model", InputCost: 0.075, OutputCost: 0.3},
			},
		},
	}
}

// Init initializes the step with provider detection.
func (s *SemanticProviderStep) Init(cfg *viper.Viper) tea.Cmd {
	s.providers = buildProviders()

	// Detect API keys from environment
	for i := range s.providers {
		s.providers[i].KeyDetected = os.Getenv(s.providers[i].EnvVar) != ""
	}

	// Build provider radio options
	var options []components.RadioOption
	for _, p := range s.providers {
		label := p.DisplayName
		if p.KeyDetected {
			label += " (API Key detected)"
		}
		options = append(options, components.RadioOption{
			Label:       label,
			Value:       p.Name,
			Description: "Environment variable: " + p.EnvVar,
		})
	}

	s.providerRadio = components.NewRadioGroup(options)
	s.phase = phaseProvider

	// Pre-fill from existing config
	if provider := cfg.GetString("semantic.provider"); provider != "" {
		for i, p := range s.providers {
			if p.Name == provider {
				s.providerRadio.SetCursor(i)
				s.selectedIdx = i
				break
			}
		}
	}

	return nil
}

// Update handles input and phase transitions.
func (s *SemanticProviderStep) Update(msg tea.Msg) (tea.Cmd, StepResult) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, StepContinue
	}

	switch s.phase {
	case phaseProvider:
		return s.handleProviderPhase(keyMsg)
	case phaseModel:
		return s.handleModelPhase(keyMsg)
	case phaseAPIKey:
		return s.handleAPIKeyPhase(keyMsg)
	}

	return nil, StepContinue
}

func (s *SemanticProviderStep) handleProviderPhase(msg tea.KeyMsg) (tea.Cmd, StepResult) {
	switch msg.Type {
	case tea.KeyEnter:
		s.selectedIdx = s.providerRadio.Cursor()
		s.buildModelRadio()
		s.phase = phaseModel
		return nil, StepContinue

	case tea.KeyEsc:
		return nil, StepPrev

	default:
		s.providerRadio, _ = s.providerRadio.Update(msg)
		return nil, StepContinue
	}
}

func (s *SemanticProviderStep) handleModelPhase(msg tea.KeyMsg) (tea.Cmd, StepResult) {
	switch msg.Type {
	case tea.KeyEnter:
		provider := s.providers[s.selectedIdx]
		if provider.KeyDetected {
			// API key already in environment, skip to next step
			return nil, StepNext
		}
		s.phase = phaseAPIKey
		s.keyInput.SetMasked(true)
		s.keyInput.Focus()
		return nil, StepContinue

	case tea.KeyEsc:
		s.phase = phaseProvider
		return nil, StepContinue

	default:
		s.modelRadio, _ = s.modelRadio.Update(msg)
		return nil, StepContinue
	}
}

func (s *SemanticProviderStep) handleAPIKeyPhase(msg tea.KeyMsg) (tea.Cmd, StepResult) {
	switch msg.Type {
	case tea.KeyEnter:
		if err := s.Validate(); err == nil {
			return nil, StepNext
		}
		return nil, StepContinue

	case tea.KeyEsc:
		s.phase = phaseModel
		s.keyInput.Blur()
		return nil, StepContinue

	default:
		s.keyInput, _ = s.keyInput.Update(msg)
		return nil, StepContinue
	}
}

func (s *SemanticProviderStep) buildModelRadio() {
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
func (s *SemanticProviderStep) View() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Secondary).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("Semantic Analysis Provider"))
	b.WriteString("\n\n")

	switch s.phase {
	case phaseProvider:
		b.WriteString(s.viewProviderPhase())
	case phaseModel:
		b.WriteString(s.viewModelPhase())
	case phaseAPIKey:
		b.WriteString(s.viewAPIKeyPhase())
	}

	b.WriteString("\n")

	// Navigation help
	b.WriteString("\n")
	if s.phase == phaseAPIKey {
		b.WriteString(NavigationHelpWithInput())
	} else {
		b.WriteString(NavigationHelp())
	}

	return b.String()
}

func (s *SemanticProviderStep) viewProviderPhase() string {
	var b strings.Builder

	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	b.WriteString(mutedStyle.Render("Select your LLM provider for semantic analysis:"))
	b.WriteString("\n\n")

	b.WriteString(s.providerRadio.View())

	return b.String()
}

func (s *SemanticProviderStep) viewModelPhase() string {
	var b strings.Builder

	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	provider := s.providers[s.selectedIdx]

	b.WriteString(mutedStyle.Render("Select a model for " + provider.DisplayName + ":"))
	b.WriteString("\n\n")

	b.WriteString(s.modelRadio.View())

	// Show API key status
	b.WriteString("\n")
	b.WriteString(FormatKeyStatus(provider.KeyDetected))

	return b.String()
}

func (s *SemanticProviderStep) viewAPIKeyPhase() string {
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
func (s *SemanticProviderStep) Validate() error {
	if s.phase == phaseAPIKey {
		key := strings.TrimSpace(s.keyInput.Value())
		if key == "" {
			return errors.New("API key is required")
		}
	}

	return nil
}

// Apply writes the semantic provider configuration.
func (s *SemanticProviderStep) Apply(cfg *viper.Viper) error {
	provider := s.providers[s.selectedIdx]
	model := provider.Models[s.modelRadio.Cursor()]

	cfg.Set("semantic.provider", provider.Name)
	cfg.Set("semantic.model", model.ID)
	cfg.Set("semantic.rate_limit", provider.DefaultRate)

	// Store API key if provided
	if s.phase == phaseAPIKey {
		key := strings.TrimSpace(s.keyInput.Value())
		if key != "" {
			cfg.Set("semantic.api_key", key)
		}
	} else if provider.KeyDetected {
		// Store key from environment
		cfg.Set("semantic.api_key", os.Getenv(provider.EnvVar))
	}

	return nil
}
