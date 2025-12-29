# Terminal User Interface

Interactive setup wizard built on Bubble Tea with multi-step navigation, reusable components, and consistent styling for guided configuration.

**Documented Version:** v0.13.0

**Last Updated:** 2025-12-29

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The Terminal User Interface (TUI) subsystem provides an interactive setup wizard for Agentic Memorizer. Rather than requiring users to edit configuration files directly, the wizard guides users through seven configuration steps: semantic provider selection, HTTP port configuration, FalkorDB setup, embeddings configuration, integration selection, confirmation, and startup options. The subsystem is built on Bubble Tea (bubbletea), Charm's framework for terminal applications.

The architecture separates concerns into three layers: the wizard orchestrator that manages step navigation and state, individual step implementations that handle specific configuration areas, and reusable components (radio groups, text inputs, checkboxes, progress indicators) shared across steps. A centralized styles package provides consistent visual theming using lipgloss.

Key capabilities include:

- **Multi-step wizard** - Seven configuration steps with forward/backward navigation
- **Bubble Tea integration** - Event-driven terminal UI using Charm's bubbletea library
- **Reusable components** - RadioGroup, TextInput, Checkbox, and Progress components
- **Consistent styling** - Centralized color palette and component styles via lipgloss
- **Environment detection** - Auto-detects API keys, running services, and available integrations
- **Configuration application** - Applies wizard choices to configuration struct progressively
- **Full-screen mode** - Uses alternate screen buffer for clean terminal experience

## Design Principles

### Step Interface Pattern

All wizard steps implement a common Step interface with six methods: Init for initialization with current config, Update for event handling, View for rendering, Title for progress display, Validate for input checking, and Apply for writing values to config. This interface enables the wizard orchestrator to treat all steps uniformly while allowing step-specific behavior.

### Event-Driven Architecture

The subsystem follows Bubble Tea's Elm-inspired architecture: an Init function returns initial commands, an Update function handles messages and returns new state plus commands, and a View function renders the current state. This pure functional approach makes state transitions predictable and testable.

### Component Composition

Steps compose reusable components rather than implementing UI primitives directly. A SemanticProviderStep contains a RadioGroup for provider selection and a TextInput for API key entry. This composition reduces duplication and ensures consistent behavior across the wizard.

### Progressive Configuration

Each step reads from and writes to a shared Config struct. Steps read current values during Init to pre-populate fields, and write values during Apply when advancing. This enables back navigation to show previously entered values and keeps configuration changes atomic per step.

### Centralized Styling

All colors, text styles, and visual indicators are defined in the styles package. Steps and components reference style constants rather than defining colors inline. This enables consistent theming and easy modification of the visual appearance.

### Environment-Aware Defaults

Steps detect environment state and adjust options accordingly. The FalkorDB step checks if Docker is available and if FalkorDB is already running. The semantic provider step checks for API keys in environment variables. This intelligence reduces user effort when services are already configured.

## Key Components

### WizardModel

The WizardModel struct is the main Bubble Tea model orchestrating the wizard. It holds the step list, current step index, config reference, and progress component. The Init, Update, and View methods implement the tea.Model interface. Navigation methods (nextStep, prevStep) handle step transitions with validation.

### WizardResult

The WizardResult struct contains the outcome of running the wizard: the final Config, a Confirmed boolean, a Cancelled boolean, any error, the StartupStep reference for post-wizard processing, and the list of selected integrations.

### Step Interface

The Step interface defines the contract for wizard steps with six methods. Init initializes with config and returns a tea.Cmd. Update handles messages and returns (tea.Cmd, StepResult). View returns the rendered string. Title returns the step name. Validate checks input validity. Apply writes values to config.

### StepResult Type

The StepResult type indicates what should happen after Update: StepContinue stays on current step, StepNext advances to next step, and StepPrev returns to previous step. Steps return this from Update to communicate navigation intent.

### SemanticProviderStep

The semantic provider step handles AI provider selection with three phases: provider selection (Claude, OpenAI, Gemini), model selection (provider-specific options), and API key entry (with environment variable detection). Displays model costs and rate limits to inform user choices.

### FalkorDBStep

The FalkorDB step configures the graph database connection. Options adapt based on environment: if Docker is available, offers to start FalkorDB automatically; if already running, shows current connection; always offers custom configuration. Can trigger Docker container startup within the wizard.

### EmbeddingsStep

The embeddings step configures vector embedding generation for similarity search. Explains the optional nature of embeddings and API key requirements. Options include enabling with OpenAI key or skipping embeddings entirely.

### IntegrationsStep

The integrations step presents available integrations (Claude Code, Gemini CLI, Codex CLI) with checkboxes for multi-selection. Shows detected integrations (those with CLI tools installed) and their configuration status.

### HTTPPortStep

The HTTP port step configures the daemon's health server port. Validates port number range (1024-65535) and provides sensible defaults. Uses a simple text input component.

### ConfirmStep

The confirm step displays a summary of all configuration choices before applying. Uses a radio group with Yes/No options. Shows key settings including provider, model, integrations, and storage locations.

### StartupStep

The startup step offers post-setup actions: start services now, start services and open config file, just open config file, or exit. Returns choices to the command handler for execution after the wizard completes.

### RadioGroup Component

The RadioGroup component manages single-selection from a list of options. Supports labels with optional descriptions. Handles up/down navigation with wraparound. Renders selection indicators and cursor.

### TextInput Component

The TextInput component wraps Bubble Tea's textinput.Model with project styling. Supports placeholders, width limits, and masked input for passwords. Methods for focus, blur, and value access.

### Checkbox Component

The Checkbox component manages multi-selection from a list of options. Similar to RadioGroup but allows multiple selections. Renders check marks and provides selected items accessor.

### Progress Component

The Progress component displays wizard progress as a step counter, dot indicators (filled/empty), and current step title. Updates as the wizard advances through steps.

### Styles Package

The styles package defines the visual theme: color palette (Primary, Secondary, Success, Warning, Error, Highlight, Muted), text styles (Title, Subtitle, Label, ErrorText, SuccessText), component styles (Focused, Unfocused, Selected, Cursor), layout styles (Container, StepContainer), and indicators (RadioSelected, CheckboxSelected, ProgressFilled, CursorIndicator).

## Integration Points

### Initialize Command

The `memorizer initialize` command invokes the TUI wizard via RunWizard. It passes initial configuration and receives WizardResult containing final config and user choices. The command handles post-wizard actions like starting services and writing the config file.

### Configuration Subsystem

The wizard reads and writes to the configuration subsystem's Config struct. Each step's Apply method updates specific config sections (Semantic, Graph, Embeddings, Daemon). The config is saved to YAML after wizard completion.

### Docker Subsystem

The FalkorDB step uses the Docker subsystem to check Docker availability, detect running containers, and start FalkorDB. The wizard can trigger container startup inline, waiting for readiness before proceeding.

### Integrations Subsystem

The integrations step queries the integrations registry to detect available integrations and their installation status. Selected integrations are returned in WizardResult for the command to configure after wizard completion.

### Semantic Providers

The semantic provider step presents options for Claude, OpenAI, and Gemini with their models and pricing. Provider-specific environment variables (ANTHROPIC_API_KEY, OPENAI_API_KEY, GOOGLE_API_KEY) are checked for auto-detection.

## Glossary

**Bubble Tea**
A Go framework for building terminal user interfaces based on the Elm architecture. Provides the Model interface, tea.Cmd for side effects, and tea.Msg for events.

**Component**
A reusable UI element (RadioGroup, TextInput, Checkbox, Progress) that can be composed into steps. Handles its own state and rendering.

**Lipgloss**
A Go library for styling terminal output with colors, borders, and padding. Used by the styles package for consistent theming.

**Model**
A Bubble Tea concept: a struct implementing Init, Update, and View methods that represents application state and behavior.

**Phase**
A sub-step within a step that handles a specific piece of configuration. For example, SemanticProviderStep has provider, model, and API key phases.

**Step**
A discrete configuration screen in the wizard implementing the Step interface. Each step handles one area of configuration (provider, database, integrations, etc.).

**StepResult**
An enum indicating what should happen after step Update: continue, advance, or go back. Returned from Update to communicate navigation intent.

**Wizard**
The complete initialization experience orchestrated by WizardModel, progressing through multiple steps to configure the application.
