package steps

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize/components"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// Phase represents the current sub-phase within the step
type providerPhase int

const (
	phaseProvider providerPhase = iota
	phaseModel
	phaseAPIKey
)

// ProviderInfo holds provider configuration details
type ProviderInfo struct {
	Name        string
	DisplayName string
	EnvVar      string
	KeyDetected bool
	Models      []ModelInfo
	DefaultRate int
}

// ModelInfo holds model configuration details
type ModelInfo struct {
	ID          string
	DisplayName string
	Description string
	InputCost   string
	OutputCost  string
}

// SemanticProviderStep handles semantic provider selection and configuration
type SemanticProviderStep struct {
	phase         providerPhase
	providers     []ProviderInfo
	providerRadio *components.RadioGroup
	modelRadio    *components.RadioGroup
	keyInput      *components.TextInput
	selectedProv  int
	selectedModel int
	focusIndex    int
	err           error
}

// NewSemanticProviderStep creates a new semantic provider configuration step
func NewSemanticProviderStep() *SemanticProviderStep {
	return &SemanticProviderStep{}
}

// Title returns the step title
func (s *SemanticProviderStep) Title() string {
	return "Semantic Provider"
}

// buildProviders creates the provider list with detection status
func (s *SemanticProviderStep) buildProviders() []ProviderInfo {
	return []ProviderInfo{
		{
			Name:        "claude",
			DisplayName: "Claude (Anthropic)",
			EnvVar:      config.ClaudeAPIKeyEnv,
			KeyDetected: os.Getenv(config.ClaudeAPIKeyEnv) != "",
			DefaultRate: config.DefaultClaudeRateLimit,
			Models: []ModelInfo{
				{ID: "claude-sonnet-4-5-20250929", DisplayName: "Claude Sonnet 4.5 (Recommended)", Description: "Fast, high-quality", InputCost: "$3", OutputCost: "$15"},
				{ID: "claude-opus-4-5-20251101", DisplayName: "Claude Opus 4.5", Description: "Best for coding/agents", InputCost: "$5", OutputCost: "$25"},
				{ID: "claude-haiku-4-5-20241022", DisplayName: "Claude Haiku 4.5", Description: "Fastest, real-time", InputCost: "$1", OutputCost: "$5"},
			},
		},
		{
			Name:        "openai",
			DisplayName: "OpenAI",
			EnvVar:      config.OpenAIAPIKeyEnv,
			KeyDetected: os.Getenv(config.OpenAIAPIKeyEnv) != "",
			DefaultRate: config.DefaultOpenAIRateLimit,
			Models: []ModelInfo{
				{ID: "gpt-5.2-chat-latest", DisplayName: "GPT-5.2 Chat (Recommended)", Description: "Fast, instant mode", InputCost: "$1.75", OutputCost: "$14"},
				{ID: "gpt-5.2", DisplayName: "GPT-5.2 Thinking", Description: "Reasoning mode", InputCost: "$1.75", OutputCost: "$14"},
				{ID: "gpt-4o", DisplayName: "GPT-4o", Description: "Previous gen, reliable", InputCost: "$2.50", OutputCost: "$10"},
			},
		},
		{
			Name:        "gemini",
			DisplayName: "Google Gemini",
			EnvVar:      config.GoogleAPIKeyEnv,
			KeyDetected: os.Getenv(config.GoogleAPIKeyEnv) != "",
			DefaultRate: config.DefaultGeminiRateLimit,
			Models: []ModelInfo{
				{ID: "gemini-2.5-flash", DisplayName: "Gemini 2.5 Flash (Recommended)", Description: "Fast, cost-effective", InputCost: "$0.15", OutputCost: "$0.60"},
				{ID: "gemini-2.5-pro", DisplayName: "Gemini 2.5 Pro", Description: "High-quality, multimodal", InputCost: "$1.25", OutputCost: "$10"},
				{ID: "gemini-3-flash", DisplayName: "Gemini 3 Flash", Description: "Latest, fast", InputCost: "$0.15", OutputCost: "$0.60"},
			},
		},
	}
}

// Init initializes the step
func (s *SemanticProviderStep) Init(cfg *config.Config) tea.Cmd {
	s.phase = phaseProvider
	s.providers = s.buildProviders()
	s.err = nil
	s.focusIndex = 0
	s.selectedProv = 0
	s.selectedModel = 0

	// Build provider options with API key detection status
	options := make([]components.RadioOption, len(s.providers)+1)
	for i, prov := range s.providers {
		status := "API Key: Not found"
		if prov.KeyDetected {
			status = "API Key: Detected"
		}
		options[i] = components.RadioOption{
			Label:       prov.DisplayName,
			Description: status,
		}
	}
	options[len(s.providers)] = components.RadioOption{
		Label:       "Skip (Disable semantic analysis)",
		Description: "Configure later",
	}

	s.providerRadio = components.NewRadioGroup(options)
	s.providerRadio.Focus()

	// Pre-select based on existing config or first detected key
	if cfg.Semantic.Provider != "" {
		for i, prov := range s.providers {
			if prov.Name == cfg.Semantic.Provider {
				s.providerRadio.SetSelected(i)
				s.selectedProv = i
				break
			}
		}
	} else {
		// Auto-select first provider with detected key
		for i, prov := range s.providers {
			if prov.KeyDetected {
				s.providerRadio.SetSelected(i)
				s.selectedProv = i
				break
			}
		}
	}

	// Initialize model radio (will be rebuilt when provider changes)
	s.initModelRadio(cfg)

	// Initialize API key input
	s.keyInput = components.NewTextInput("API Key").
		WithPlaceholder("Enter API key...").
		WithMasked().
		WithWidth(60)

	return nil
}

// initModelRadio initializes the model radio for the current provider
func (s *SemanticProviderStep) initModelRadio(cfg *config.Config) {
	if s.selectedProv >= len(s.providers) {
		return
	}

	prov := s.providers[s.selectedProv]
	options := make([]components.RadioOption, len(prov.Models))
	for i, model := range prov.Models {
		options[i] = components.RadioOption{
			Label:       model.DisplayName,
			Description: model.Description + " | " + model.InputCost + "/" + model.OutputCost + " per 1M tokens",
		}
	}

	s.modelRadio = components.NewRadioGroup(options)

	// Pre-select based on existing config
	if cfg.Semantic.Model != "" {
		for i, model := range prov.Models {
			if model.ID == cfg.Semantic.Model {
				s.modelRadio.SetSelected(i)
				s.selectedModel = i
				break
			}
		}
	}
}

// Update handles input
func (s *SemanticProviderStep) Update(msg tea.Msg) (tea.Cmd, StepResult) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return s.handleEnter()

		case "esc":
			return s.handleEsc()

		case "tab":
			if s.phase == phaseAPIKey {
				s.focusIndex = (s.focusIndex + 1) % 2
				if s.focusIndex == 0 {
					s.keyInput.Blur()
				} else {
					return s.keyInput.Focus(), StepContinue
				}
			}
			return nil, StepContinue

		case "shift+tab":
			if s.phase == phaseAPIKey && s.focusIndex == 1 {
				s.focusIndex = 0
				s.keyInput.Blur()
			}
			return nil, StepContinue
		}
	}

	// Delegate to current phase component
	switch s.phase {
	case phaseProvider:
		s.providerRadio.Update(msg)
		s.selectedProv = s.providerRadio.Selected()
	case phaseModel:
		s.modelRadio.Update(msg)
		s.selectedModel = s.modelRadio.Selected()
	case phaseAPIKey:
		if s.focusIndex == 1 {
			cmd := s.keyInput.Update(msg)
			return cmd, StepContinue
		}
	}

	return nil, StepContinue
}

// handleEnter processes enter key for current phase
func (s *SemanticProviderStep) handleEnter() (tea.Cmd, StepResult) {
	switch s.phase {
	case phaseProvider:
		s.selectedProv = s.providerRadio.Selected()

		// Skip selected
		if s.selectedProv >= len(s.providers) {
			return nil, StepNext
		}

		// Move to model selection
		s.phase = phaseModel
		s.initModelRadio(&config.DefaultConfig)
		s.modelRadio.Focus()
		return nil, StepContinue

	case phaseModel:
		s.selectedModel = s.modelRadio.Selected()
		prov := s.providers[s.selectedProv]

		// If API key detected, we're done
		if prov.KeyDetected {
			return nil, StepNext
		}

		// Move to API key input
		s.phase = phaseAPIKey
		s.focusIndex = 1
		return s.keyInput.Focus(), StepContinue

	case phaseAPIKey:
		if err := s.Validate(); err != nil {
			s.err = err
			return nil, StepContinue
		}
		s.err = nil
		return nil, StepNext
	}

	return nil, StepContinue
}

// handleEsc processes escape key for current phase
func (s *SemanticProviderStep) handleEsc() (tea.Cmd, StepResult) {
	switch s.phase {
	case phaseProvider:
		return nil, StepPrev

	case phaseModel:
		s.phase = phaseProvider
		s.providerRadio.Focus()
		return nil, StepContinue

	case phaseAPIKey:
		s.phase = phaseModel
		s.modelRadio.Focus()
		s.keyInput.Blur()
		s.focusIndex = 0
		return nil, StepContinue
	}

	return nil, StepPrev
}

// View renders the step
func (s *SemanticProviderStep) View() string {
	var b strings.Builder

	switch s.phase {
	case phaseProvider:
		b.WriteString(styles.Subtitle.Render("Select semantic analysis provider"))
		b.WriteString("\n\n")
		b.WriteString(s.providerRadio.View())
		b.WriteString("\n\n")
		b.WriteString(styles.HelpText.Render(NavigationHelp()))

	case phaseModel:
		prov := s.providers[s.selectedProv]
		b.WriteString(styles.Subtitle.Render("Select " + prov.DisplayName + " model"))
		b.WriteString("\n\n")
		b.WriteString(s.modelRadio.View())
		b.WriteString("\n\n")
		b.WriteString(styles.HelpText.Render("↑/↓: navigate  enter: next  esc: back to providers  ctrl+c: quit"))

	case phaseAPIKey:
		prov := s.providers[s.selectedProv]
		b.WriteString(styles.Subtitle.Render("Enter " + prov.DisplayName + " API Key"))
		b.WriteString("\n\n")
		b.WriteString(styles.MutedText.Render("No " + prov.EnvVar + " found. Please enter your API key:"))
		b.WriteString("\n\n")
		b.WriteString(s.keyInput.View())

		if s.err != nil {
			b.WriteString("\n\n")
			b.WriteString(styles.ErrorText.Render("Error: " + s.err.Error()))
		}

		b.WriteString("\n\n")
		b.WriteString(styles.HelpText.Render("enter: continue  esc: back to models  ctrl+c: quit"))
	}

	return b.String()
}

// Validate validates the step
func (s *SemanticProviderStep) Validate() error {
	// Skip selected - always valid
	if s.selectedProv >= len(s.providers) {
		return nil
	}

	// If in API key phase, require a key
	if s.phase == phaseAPIKey {
		prov := s.providers[s.selectedProv]
		if !prov.KeyDetected && s.keyInput.Value() == "" {
			return nil // Allow empty (can skip)
		}
	}

	return nil
}

// Apply applies the step values to config
func (s *SemanticProviderStep) Apply(cfg *config.Config) error {
	// Skip selected
	if s.selectedProv >= len(s.providers) {
		cfg.Semantic.Enabled = false
		cfg.Semantic.APIKey = ""
		cfg.Semantic.Provider = ""
		cfg.Semantic.Model = ""
		return nil
	}

	prov := s.providers[s.selectedProv]
	model := prov.Models[s.selectedModel]

	cfg.Semantic.Enabled = true
	cfg.Semantic.Provider = prov.Name
	cfg.Semantic.Model = model.ID
	cfg.Semantic.RateLimitPerMin = prov.DefaultRate

	// Set API key
	if prov.KeyDetected {
		cfg.Semantic.APIKey = os.Getenv(prov.EnvVar)
	} else {
		cfg.Semantic.APIKey = s.keyInput.Value()
	}

	return nil
}
