package steps

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/initialize/components"
	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// embeddingsPhase represents the current sub-phase within the step
type embeddingsPhase int

const (
	embeddingsPhaseProvider embeddingsPhase = iota
	embeddingsPhaseModel
	embeddingsPhaseAPIKey
)

// EmbeddingsProviderInfo holds embeddings provider configuration details
type EmbeddingsProviderInfo struct {
	Name        string
	DisplayName string
	EnvVar      string
	KeyDetected bool
	Models      []EmbeddingsModelInfo
}

// EmbeddingsModelInfo holds embeddings model configuration details
type EmbeddingsModelInfo struct {
	ID          string
	DisplayName string
	Description string
	Dimensions  int
}

// EmbeddingsStep handles embeddings provider selection and configuration
type EmbeddingsStep struct {
	phase         embeddingsPhase
	providers     []EmbeddingsProviderInfo
	providerRadio *components.RadioGroup
	modelRadio    *components.RadioGroup
	keyInput      *components.TextInput
	selectedProv  int
	selectedModel int
	focusIndex    int
	err           error
}

// NewEmbeddingsStep creates a new embeddings configuration step
func NewEmbeddingsStep() *EmbeddingsStep {
	return &EmbeddingsStep{}
}

// Title returns the step title
func (s *EmbeddingsStep) Title() string {
	return "Embeddings"
}

// buildProviders creates the embeddings provider list with detection status
func (s *EmbeddingsStep) buildProviders() []EmbeddingsProviderInfo {
	return []EmbeddingsProviderInfo{
		{
			Name:        "openai",
			DisplayName: "OpenAI",
			EnvVar:      config.OpenAIAPIKeyEnv,
			KeyDetected: os.Getenv(config.OpenAIAPIKeyEnv) != "",
			Models: []EmbeddingsModelInfo{
				{ID: "text-embedding-3-small", DisplayName: "text-embedding-3-small (Recommended)", Description: "Fast, cost-effective", Dimensions: 1536},
				{ID: "text-embedding-3-large", DisplayName: "text-embedding-3-large", Description: "Highest quality", Dimensions: 3072},
				{ID: "text-embedding-ada-002", DisplayName: "text-embedding-ada-002", Description: "Legacy model", Dimensions: 1536},
			},
		},
		{
			Name:        "voyage",
			DisplayName: "Voyage AI",
			EnvVar:      config.VoyageAPIKeyEnv,
			KeyDetected: os.Getenv(config.VoyageAPIKeyEnv) != "",
			Models: []EmbeddingsModelInfo{
				{ID: "voyage-3", DisplayName: "voyage-3 (Recommended)", Description: "Best overall performance", Dimensions: 1024},
				{ID: "voyage-3-lite", DisplayName: "voyage-3-lite", Description: "Fast, cost-effective", Dimensions: 512},
				{ID: "voyage-code-3", DisplayName: "voyage-code-3", Description: "Optimized for code", Dimensions: 1024},
			},
		},
		{
			Name:        "gemini",
			DisplayName: "Google Gemini",
			EnvVar:      config.GoogleAPIKeyEnv,
			KeyDetected: os.Getenv(config.GoogleAPIKeyEnv) != "",
			Models: []EmbeddingsModelInfo{
				{ID: "text-embedding-004", DisplayName: "text-embedding-004 (Recommended)", Description: "Latest model", Dimensions: 768},
				{ID: "embedding-001", DisplayName: "embedding-001", Description: "Legacy model", Dimensions: 768},
			},
		},
	}
}

// Init initializes the step
func (s *EmbeddingsStep) Init(cfg *config.Config) tea.Cmd {
	s.phase = embeddingsPhaseProvider
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
		Label:       "Skip (Disable embeddings)",
		Description: "No vector similarity search",
	}

	s.providerRadio = components.NewRadioGroup(options)
	s.providerRadio.Focus()

	// Pre-select based on existing config or first detected key
	if cfg.Embeddings.Provider != "" {
		for i, prov := range s.providers {
			if prov.Name == cfg.Embeddings.Provider {
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
func (s *EmbeddingsStep) initModelRadio(cfg *config.Config) {
	if s.selectedProv >= len(s.providers) {
		return
	}

	prov := s.providers[s.selectedProv]
	options := make([]components.RadioOption, len(prov.Models))
	for i, model := range prov.Models {
		options[i] = components.RadioOption{
			Label:       model.DisplayName,
			Description: model.Description + " | " + formatDimensions(model.Dimensions),
		}
	}

	s.modelRadio = components.NewRadioGroup(options)

	// Pre-select based on existing config
	if cfg.Embeddings.Model != "" {
		for i, model := range prov.Models {
			if model.ID == cfg.Embeddings.Model {
				s.modelRadio.SetSelected(i)
				s.selectedModel = i
				break
			}
		}
	}
}

// formatDimensions formats dimension count for display
func formatDimensions(dim int) string {
	switch {
	case dim >= 1000:
		return fmt.Sprintf("%dk dimensions", dim/1000)
	default:
		return fmt.Sprintf("%d dimensions", dim)
	}
}

// Update handles input
func (s *EmbeddingsStep) Update(msg tea.Msg) (tea.Cmd, StepResult) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return s.handleEnter()

		case "esc":
			return s.handleEsc()

		case "tab":
			if s.phase == embeddingsPhaseAPIKey {
				s.focusIndex = (s.focusIndex + 1) % 2
				if s.focusIndex == 0 {
					s.keyInput.Blur()
				} else {
					return s.keyInput.Focus(), StepContinue
				}
			}
			return nil, StepContinue

		case "shift+tab":
			if s.phase == embeddingsPhaseAPIKey && s.focusIndex == 1 {
				s.focusIndex = 0
				s.keyInput.Blur()
			}
			return nil, StepContinue
		}
	}

	// Delegate to current phase component
	switch s.phase {
	case embeddingsPhaseProvider:
		s.providerRadio.Update(msg)
		s.selectedProv = s.providerRadio.Selected()
	case embeddingsPhaseModel:
		s.modelRadio.Update(msg)
		s.selectedModel = s.modelRadio.Selected()
	case embeddingsPhaseAPIKey:
		if s.focusIndex == 1 {
			cmd := s.keyInput.Update(msg)
			return cmd, StepContinue
		}
	}

	return nil, StepContinue
}

// handleEnter processes enter key for current phase
func (s *EmbeddingsStep) handleEnter() (tea.Cmd, StepResult) {
	switch s.phase {
	case embeddingsPhaseProvider:
		s.selectedProv = s.providerRadio.Selected()

		// Skip selected
		if s.selectedProv >= len(s.providers) {
			return nil, StepNext
		}

		// Move to model selection
		s.phase = embeddingsPhaseModel
		s.initModelRadio(&config.DefaultConfig)
		s.modelRadio.Focus()
		return nil, StepContinue

	case embeddingsPhaseModel:
		s.selectedModel = s.modelRadio.Selected()
		prov := s.providers[s.selectedProv]

		// If API key detected, we're done
		if prov.KeyDetected {
			return nil, StepNext
		}

		// Move to API key input
		s.phase = embeddingsPhaseAPIKey
		s.focusIndex = 1
		return s.keyInput.Focus(), StepContinue

	case embeddingsPhaseAPIKey:
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
func (s *EmbeddingsStep) handleEsc() (tea.Cmd, StepResult) {
	switch s.phase {
	case embeddingsPhaseProvider:
		return nil, StepPrev

	case embeddingsPhaseModel:
		s.phase = embeddingsPhaseProvider
		s.providerRadio.Focus()
		return nil, StepContinue

	case embeddingsPhaseAPIKey:
		s.phase = embeddingsPhaseModel
		s.modelRadio.Focus()
		s.keyInput.Blur()
		s.focusIndex = 0
		return nil, StepContinue
	}

	return nil, StepPrev
}

// View renders the step
func (s *EmbeddingsStep) View() string {
	var b strings.Builder

	switch s.phase {
	case embeddingsPhaseProvider:
		b.WriteString(styles.Subtitle.Render("Select embeddings provider for vector similarity search"))
		b.WriteString("\n\n")
		b.WriteString(s.providerRadio.View())
		b.WriteString("\n\n")
		b.WriteString(styles.HelpText.Render(NavigationHelp()))

	case embeddingsPhaseModel:
		prov := s.providers[s.selectedProv]
		b.WriteString(styles.Subtitle.Render("Select " + prov.DisplayName + " embedding model"))
		b.WriteString("\n\n")
		b.WriteString(s.modelRadio.View())
		b.WriteString("\n\n")
		b.WriteString(styles.HelpText.Render("↑/↓: navigate  enter: next  esc: back to providers  ctrl+c: quit"))

	case embeddingsPhaseAPIKey:
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
func (s *EmbeddingsStep) Validate() error {
	// Skip selected - always valid
	if s.selectedProv >= len(s.providers) {
		return nil
	}

	// If in API key phase, require a key
	if s.phase == embeddingsPhaseAPIKey {
		prov := s.providers[s.selectedProv]
		if !prov.KeyDetected && s.keyInput.Value() == "" {
			return nil // Allow empty (can skip)
		}
	}

	return nil
}

// Apply applies the step values to config
func (s *EmbeddingsStep) Apply(cfg *config.Config) error {
	// Skip selected
	if s.selectedProv >= len(s.providers) {
		cfg.Embeddings.Enabled = false
		cfg.Embeddings.APIKey = ""
		cfg.Embeddings.Provider = ""
		cfg.Embeddings.Model = ""
		cfg.Embeddings.Dimensions = 0
		return nil
	}

	prov := s.providers[s.selectedProv]
	model := prov.Models[s.selectedModel]

	cfg.Embeddings.Enabled = true
	cfg.Embeddings.Provider = prov.Name
	cfg.Embeddings.Model = model.ID
	cfg.Embeddings.Dimensions = model.Dimensions

	// Set API key
	if prov.KeyDetected {
		cfg.Embeddings.APIKey = os.Getenv(prov.EnvVar)
	} else {
		cfg.Embeddings.APIKey = s.keyInput.Value()
	}

	return nil
}
