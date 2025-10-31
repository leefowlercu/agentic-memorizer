# Agent Framework Decoupling Implementation Plan

**Version**: 1.0
**Date**: 2025-10-31
**Status**: Draft

## Executive Summary

This document outlines a comprehensive plan to decouple `agentic-memorizer` from Claude Code's SessionStart hook integration, transforming it into a **framework-agnostic memory system** that can integrate with multiple AI agent platforms (Claude Code, Continue, Cline, Aider, custom frameworks, etc.).

### Current State

The system is tightly coupled to Claude Code through:
- Direct manipulation of `~/.claude/settings.json` (`internal/hooks/manager.go`)
- Hardcoded SessionStart hook output format (`internal/output/formatter.go:265-284`)
- Claude Code-specific hook response wrapper (the `--wrap-json` flag wraps formatted output in SessionStart JSON envelope)
- Assumed hook lifecycle and event names (`startup`, `resume`, `clear`, `compact`)
- Only two output formats supported: XML and Markdown (no JSON format option)

### Target State

A pluggable architecture where:
- **Core Pipeline** remains unchanged (daemon → metadata → semantic → index)
- **Integration Layer** is abstracted via adapter pattern
- **Output Formats** are pluggable and framework-specific
- **Hook Management** is generalized to support any configuration mechanism
- **Claude Code** becomes one of many supported integrations

---

## Architecture Analysis

### Current Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     CORE PIPELINE (GOOD)                         │
│  ┌──────┐   ┌──────────┐   ┌──────────┐   ┌───────┐            │
│  │Daemon│──>│ Metadata │──>│ Semantic │──>│ Index │            │
│  │      │   │ Extract  │   │ Analysis │   │ Mgr   │            │
│  └──────┘   └──────────┘   └──────────┘   └───────┘            │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│              TIGHTLY COUPLED INTEGRATION (PROBLEM)               │
│                                                                  │
│  ┌──────────────────────┐         ┌──────────────────────┐     │
│  │   hooks/manager.go   │         │  output/formatter.go │     │
│  │                      │         │                      │     │
│  │ - ~/.claude/         │         │ - WrapJSON()         │     │
│  │   settings.json      │────────>│ - SessionStart       │     │
│  │ - 4 hardcoded        │         │   specific format    │     │
│  │   matchers           │         │ - Hardcoded fields   │     │
│  └──────────────────────┘         └──────────────────────┘     │
│           │                                  │                  │
│           └──────────────┬───────────────────┘                  │
│                          ▼                                      │
│                  ┌──────────────┐                               │
│                  │ Claude Code  │                               │
│                  │ SessionStart │                               │
│                  │     Hook     │                               │
│                  └──────────────┘                               │
└─────────────────────────────────────────────────────────────────┘
```

### Target Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     CORE PIPELINE (UNCHANGED)                    │
│  ┌──────┐   ┌──────────┐   ┌──────────┐   ┌───────┐            │
│  │Daemon│──>│ Metadata │──>│ Semantic │──>│ Index │            │
│  │      │   │ Extract  │   │ Analysis │   │ Mgr   │            │
│  └──────┘   └──────────┘   └──────────┘   └───────┘            │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│              ABSTRACTION LAYER (NEW)                             │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │            Integration Registry & Manager                 │  │
│  │  - Discovers available integrations                       │  │
│  │  - Manages lifecycle (setup/update/remove)                │  │
│  │  - Configuration per integration                          │  │
│  └──────────────────────────────────────────────────────────┘  │
│                          │                                      │
│            ┌─────────────┴─────────────┬──────────────┐        │
│            ▼                           ▼              ▼        │
│  ┌──────────────────┐     ┌──────────────────┐   ┌────────┐   │
│  │  ClaudeAdapter   │     │ ContinueAdapter  │   │  ...   │   │
│  │                  │     │                  │   │        │   │
│  │ Interface:       │     │ Interface:       │   │        │   │
│  │ - GetName()      │     │ - GetName()      │   │        │   │
│  │ - Detect()       │     │ - Detect()       │   │        │   │
│  │ - Setup()        │     │ - Setup()        │   │        │   │
│  │ - GetCommand()   │     │ - GetCommand()   │   │        │   │
│  │ - FormatOutput() │     │ - FormatOutput() │   │        │   │
│  └──────────────────┘     └──────────────────┘   └────────┘   │
│            │                           │              │        │
│            ▼                           ▼              ▼        │
│  ┌──────────────┐         ┌──────────────┐   ┌──────────┐    │
│  │ Claude Code  │         │   Continue   │   │  Cline   │    │
│  │.claude/      │         │.continue/    │   │ .cline/  │    │
│  │settings.json │         │config.json   │   │config.ts │    │
│  └──────────────┘         └──────────────┘   └──────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

---

## Key Design Principles

### 1. Adapter Pattern for Integrations

Each agent framework gets its own adapter implementing a common interface:

```go
type Integration interface {
    // Metadata
    GetName() string
    GetDescription() string
    GetVersion() string

    // Detection
    Detect() (bool, error)                    // Can we find this framework?
    IsEnabled() (bool, error)                 // Is it currently configured?

    // Lifecycle
    Setup(binaryPath string) error            // Configure the framework
    Update(binaryPath string) error           // Update existing config
    Remove() error                             // Remove configuration

    // Command Generation
    GetCommand(binaryPath string, format OutputFormat) string

    // Output Formatting
    FormatOutput(index *types.Index, format OutputFormat) (string, error)

    // Validation
    Validate() error                           // Check configuration health
}
```

### 2. Registry Pattern for Discovery

A central registry manages all available integrations:

```go
type Registry struct {
    integrations map[string]Integration
    mu           sync.RWMutex
}

func (r *Registry) Register(integration Integration)
func (r *Registry) Get(name string) (Integration, error)
func (r *Registry) List() []Integration
func (r *Registry) DetectAvailable() []Integration  // Find installed frameworks
```

### 3. Output Format Abstraction

**Important Distinction**: Separate concerns of **output formatting** vs **integration wrapping**:

- **Output Format** = How the index is rendered (XML, Markdown, JSON)
  - Currently: XML and Markdown only
  - Proposed: Add JSON as a third format option
- **Integration Wrapper** = Framework-specific envelope around the formatted content
  - Currently: `--wrap-json` wraps XML/Markdown in Claude Code SessionStart JSON response
  - Proposed: Each integration adapter handles its own wrapping logic

**New Output Format System**:

```go
type OutputFormat string

const (
    FormatXML      OutputFormat = "xml"       // Existing
    FormatMarkdown OutputFormat = "markdown"  // Existing
    FormatJSON     OutputFormat = "json"      // NEW - renders index as JSON
)

// OutputProcessor renders the index in a specific format
type OutputProcessor interface {
    Format(index *types.Index) (string, error)
}

// Integration adapters handle framework-specific wrapping
// (e.g., Claude Code's SessionStart JSON envelope)
```

**Example Flow**:
```
Index → OutputProcessor.Format() → XML/Markdown/JSON string
                                         ↓
                    Integration.FormatOutput() → Framework-specific wrapper
                                         ↓
                            Final output (e.g., SessionStart JSON with XML inside)
```

### 4. Configuration Schema

Unified configuration for all integrations:

```yaml
integrations:
  enabled: ["claude-code", "continue"]

  claude-code:
    type: "claude-code"
    enabled: true
    settings_path: "~/.claude/settings.json"
    output_format: "xml"
    matchers: ["startup", "resume", "clear", "compact"]

  continue:
    type: "continue"
    enabled: true
    settings_path: "~/.continue/config.json"
    output_format: "markdown"

  cline:
    type: "cline"
    enabled: false
    settings_path: "~/.cline/config.ts"
```

### 5. Backward Compatibility

Ensure existing users aren't disrupted:
- Old `--wrap-json` flag remains but becomes deprecated
- `agentic-memorizer init --setup-hooks` still works (detects Claude Code)
- Migration path provided for existing installations

---

## Multi-Phase Implementation Plan

### Phase 1: Foundation & Abstraction Layer (Weeks 1-2)

**Goal**: Create the core abstraction layer without breaking existing functionality.

#### 1.1 Create Integration Interface Package

**New Package**: `internal/integrations/`

```
internal/integrations/
├── interface.go          # Integration interface definition
├── registry.go           # Registry implementation
├── types.go              # Shared types (OutputFormat, etc.)
└── utils.go              # Common utilities
```

**Key Files**:
- `interface.go`: Define `Integration` interface
- `registry.go`: Implement registry with thread-safe operations
- `types.go`: Define `OutputFormat`, `IntegrationConfig`, etc.

#### 1.2 Create Base Output Processor

**New Package**: `internal/integrations/output/`

```
internal/integrations/output/
├── interface.go          # OutputProcessor interface
├── xml.go                # XMLProcessor (extract from formatter.go)
├── markdown.go           # MarkdownProcessor (extract from formatter.go)
└── json.go               # JSONProcessor (NEW - renders index as JSON)
```

**Actions**:
- Extract XML formatting logic from `internal/output/formatter.go` (existing format)
- Extract Markdown formatting logic (existing format)
- **NEW**: Create JSON output processor to render index as JSON (this is a new output format)
  - Note: The index is stored on disk as JSON, but we don't currently have a JSON output format
  - This will be a human-readable/agent-readable JSON representation of the index
  - Different from the storage format (may include pretty-printing, filtering, etc.)
- Create modular processors that work independently of integration wrappers
- Keep original `Formatter` as wrapper for backward compatibility

#### 1.3 Update Configuration Schema

**Modify**: `internal/config/types.go` and `internal/config/constants.go`

**Add New Section**:
```go
type Config struct {
    // ... existing fields ...

    Integrations IntegrationsConfig `mapstructure:"integrations" yaml:"integrations"`
}

type IntegrationsConfig struct {
    Enabled []string                       `mapstructure:"enabled" yaml:"enabled"`
    Configs map[string]IntegrationConfig   `mapstructure:"configs" yaml:"configs"`
}

type IntegrationConfig struct {
    Type         string                 `mapstructure:"type" yaml:"type"`
    Enabled      bool                   `mapstructure:"enabled" yaml:"enabled"`
    OutputFormat string                 `mapstructure:"output_format" yaml:"output_format"`
    Settings     map[string]interface{} `mapstructure:"settings" yaml:"settings"`
}
```

**Default Config**:
```go
DefaultIntegrationsConfig: IntegrationsConfig{
    Enabled: []string{"claude-code"},  // Backward compatible
    Configs: map[string]IntegrationConfig{
        "claude-code": {
            Type:         "claude-code",
            Enabled:      true,
            OutputFormat: "xml",
            Settings: map[string]interface{}{
                "settings_path": "~/.claude/settings.json",
                "matchers":      []string{"startup", "resume", "clear", "compact"},
            },
        },
    },
}
```

#### 1.4 Testing Infrastructure

**New Package**: `internal/integrations/testing/`

```go
// Mock integration for testing
type MockIntegration struct {
    name      string
    detected  bool
    setupErr  error
}

func (m *MockIntegration) GetName() string { return m.name }
func (m *MockIntegration) Detect() (bool, error) { return m.detected, nil }
// ... implement interface
```

**Write Tests**:
- `registry_test.go`: Test registration, lookup, concurrency
- `output_test.go`: Test all output processors
- `config_test.go`: Test configuration loading

**Deliverables**:
- [ ] Integration interface defined
- [ ] Registry implementation complete with tests
- [ ] Output processors extracted and tested
- [ ] Configuration schema extended
- [ ] All existing tests still pass
- [ ] No breaking changes to public API

---

### Phase 2: Claude Code Adapter (Weeks 2-3)

**Goal**: Migrate existing Claude Code integration to the new adapter pattern.

#### 2.1 Create Claude Code Adapter

**New File**: `internal/integrations/adapters/claude/adapter.go`

```go
package claude

type ClaudeCodeAdapter struct {
    settingsPath string
    matchers     []string
    binaryPath   string
}

func NewClaudeCodeAdapter() *ClaudeCodeAdapter {
    return &ClaudeCodeAdapter{
        settingsPath: getDefaultSettingsPath(),
        matchers:     []string{"startup", "resume", "clear", "compact"},
    }
}

func (a *ClaudeCodeAdapter) GetName() string {
    return "claude-code"
}

func (a *ClaudeCodeAdapter) Detect() (bool, error) {
    // Check if ~/.claude/settings.json exists
    settingsPath := a.settingsPath
    _, err := os.Stat(settingsPath)
    return err == nil, nil
}

func (a *ClaudeCodeAdapter) Setup(binaryPath string) error {
    // Port logic from internal/hooks/manager.go
    // But make it part of the adapter
    return a.setupSessionStartHooks(binaryPath)
}

func (a *ClaudeCodeAdapter) GetCommand(binaryPath string, format OutputFormat) string {
    return fmt.Sprintf("%s read --format %s --integration claude-code", binaryPath, format)
}

func (a *ClaudeCodeAdapter) FormatOutput(index *types.Index, format OutputFormat) (string, error) {
    // Port WrapJSON logic here
    // This is Claude Code-specific formatting
    return formatSessionStartJSON(index, format)
}
```

**Key Migration**:
- Move `internal/hooks/manager.go` logic into adapter
- Keep as much existing code as possible
- Adapt to new interface requirements

#### 2.2 Create Settings Manager for Claude Code

**New File**: `internal/integrations/adapters/claude/settings.go`

Port existing settings management:
- `ReadSettings()` from `hooks/manager.go`
- `WriteSettings()` from `hooks/manager.go`
- `SetupSessionStartHooks()` logic

Keep JSON structure handling exactly as-is for backward compatibility.

#### 2.3 Create Output Wrapper for Claude Code

**New File**: `internal/integrations/adapters/claude/output.go`

```go
type SessionStartOutput struct {
    Continue       bool                    `json:"continue"`
    SuppressOutput bool                    `json:"suppressOutput"`
    SystemMessage  string                  `json:"systemMessage"`
    HookSpecific   *HookSpecificOutput     `json:"hookSpecificOutput,omitempty"`
}

type HookSpecificOutput struct {
    HookEventName     string `json:"hookEventName"`
    AdditionalContext string `json:"additionalContext"`  // Contains formatted content (XML/Markdown/JSON)
}

func formatSessionStartJSON(index *types.Index, format OutputFormat) (string, error) {
    // Port from internal/output/formatter.go:265-284

    // Step 1: Generate formatted content using appropriate output processor
    var content string
    switch format {
    case FormatXML:
        processor := output.NewXMLProcessor()
        content, _ = processor.Format(index)
    case FormatMarkdown:
        processor := output.NewMarkdownProcessor()
        content, _ = processor.Format(index)
    case FormatJSON:
        processor := output.NewJSONProcessor()
        content, _ = processor.Format(index)
    }

    // Step 2: Wrap the formatted content in SessionStart JSON envelope
    // The content (XML/Markdown/JSON) goes into hookSpecificOutput.additionalContext
    wrapper := SessionStartOutput{
        Continue:       true,
        SuppressOutput: true,
        SystemMessage:  generateSystemMessage(index),
        HookSpecific: &HookSpecificOutput{
            HookEventName:     "SessionStart",
            AdditionalContext: content,  // This is the formatted index (still as string)
        },
    }

    return json.Marshal(wrapper)
}
```

#### 2.4 Update Read Command

**Modify**: `cmd/read/read.go`

**Add New Flag**:
```go
var integrationName string

func init() {
    readCmd.Flags().StringVar(&integrationName, "integration", "",
        "Integration to format output for (claude-code, continue, etc.)")
}
```

**Update Execution Logic**:
```go
func run() error {
    cfg, err := config.GetConfig()
    if err != nil {
        return err
    }

    // Load index
    indexManager := index.NewManager(cfg.GetIndexPath())
    computed, err := indexManager.LoadComputed()
    if err != nil {
        return err
    }

    // Determine output format
    format := determineFormat(cfg)

    // Check if integration-specific output requested
    if integrationName != "" {
        return outputForIntegration(integrationName, computed.Index, format)
    }

    // Default: backward compatible output
    return outputDefault(computed.Index, format, cfg)
}

func outputForIntegration(name string, idx *types.Index, format OutputFormat) error {
    registry := integrations.GlobalRegistry()
    integration, err := registry.Get(name)
    if err != nil {
        return fmt.Errorf("integration %s not found: %w", name, err)
    }

    output, err := integration.FormatOutput(idx, format)
    if err != nil {
        return err
    }

    fmt.Println(output)
    return nil
}
```

#### 2.5 Update Init Command

**Modify**: `cmd/init/init.go`

**Replace Hook Setup Logic**:
```go
// Old: Direct call to hooks.SetupSessionStartHooks()
if setupHooks {
    err := hooks.SetupSessionStartHooks(binaryPath)
    if err != nil {
        return err
    }
}

// New: Use integration registry
if setupHooks {
    registry := integrations.GlobalRegistry()

    // Detect available integrations
    available := registry.DetectAvailable()

    if len(available) == 0 {
        fmt.Println("No compatible agent frameworks detected")
        return nil
    }

    // Setup detected integrations (or prompt user)
    for _, integration := range available {
        fmt.Printf("Setting up %s integration...\n", integration.GetName())
        err := integration.Setup(binaryPath)
        if err != nil {
            fmt.Printf("Warning: Failed to setup %s: %v\n", integration.GetName(), err)
            continue
        }
        fmt.Printf("✓ %s integration configured\n", integration.GetName())
    }
}
```

#### 2.6 Deprecation Strategy

**Keep Existing `--wrap-json` Flag** (backward compatibility):

**What `--wrap-json` Actually Does**:
- It does NOT change the output format (still XML or Markdown based on `--format`)
- It wraps the formatted content in a Claude Code SessionStart hook JSON response envelope
- The JSON envelope contains fields: `continue`, `suppressOutput`, `systemMessage`, `hookSpecificOutput`
- The actual index content goes in `hookSpecificOutput.additionalContext` (still as XML/Markdown)

**Deprecation Implementation**:
```go
// cmd/read/read.go
var wrapJSON bool  // Keep for backward compatibility

func init() {
    readCmd.Flags().BoolVar(&wrapJSON, "wrap-json", false,
        "[DEPRECATED] Use --integration claude-code instead. "+
        "Wraps formatted output (XML/Markdown) in Claude Code SessionStart hook JSON envelope.")
}

func run() error {
    // ... load index ...

    // Handle deprecated flag
    if wrapJSON {
        fmt.Fprintln(os.Stderr, "Warning: --wrap-json is deprecated. Use --integration claude-code instead.")
        integrationName = "claude-code"
    }

    // Continue with normal flow
}
```

**Examples**:
```bash
# Old way (still works but deprecated)
agentic-memorizer read --format xml --wrap-json
# → Outputs: {"continue": true, "hookSpecificOutput": {"additionalContext": "<xml>...</xml>"}}

# New way
agentic-memorizer read --format xml --integration claude-code
# → Outputs: Same JSON envelope with XML inside
```

#### 2.7 Integration Testing

**Test Scenarios**:
1. Fresh install with `agentic-memorizer init --setup-hooks`
2. Existing installation upgrade (hooks should be migrated)
3. `agentic-memorizer read --wrap-json` (deprecated path)
4. `agentic-memorizer read --integration claude-code` (new path)
5. Claude Code SessionStart hooks still work
6. Output format is identical to before

**Deliverables**:
- [ ] Claude Code adapter fully implemented
- [ ] Settings management ported to adapter
- [ ] Output formatting ported to adapter
- [ ] Read command updated with `--integration` flag
- [ ] Init command uses integration registry
- [ ] Backward compatibility maintained (`--wrap-json` works)
- [ ] All existing Claude Code tests pass
- [ ] Integration tests verify SessionStart hooks work

---

### Phase 3: Additional Integrations (Weeks 4-5)

**Goal**: Implement adapters for other popular agent frameworks to validate the abstraction.

#### 3.1 Continue.dev Adapter

**Research Continue.dev Integration**:
- Configuration location: `~/.continue/config.json` or `~/.continue/config.ts`
- Hook/plugin system: Custom tools via `tools` array
- Command invocation: Shell commands in tool definitions

**New File**: `internal/integrations/adapters/continue/adapter.go`

```go
package continue

type ContinueAdapter struct {
    configPath string
}

func NewContinueAdapter() *ContinueAdapter {
    return &ContinueAdapter{
        configPath: getDefaultConfigPath(), // ~/.continue/config.json
    }
}

func (a *ContinueAdapter) GetName() string {
    return "continue"
}

func (a *ContinueAdapter) Detect() (bool, error) {
    _, err := os.Stat(a.configPath)
    return err == nil, nil
}

func (a *ContinueAdapter) Setup(binaryPath string) error {
    // Read config.json
    // Add custom tool to tools array
    // Tool definition:
    // {
    //   "name": "memory",
    //   "description": "Access agentic memory index",
    //   "command": "agentic-memorizer read --format markdown --integration continue"
    // }
    return a.setupMemoryTool(binaryPath)
}

func (a *ContinueAdapter) FormatOutput(index *types.Index, format OutputFormat) (string, error) {
    // Continue doesn't need JSON wrapping
    // Just return plain markdown or XML
    processor := getOutputProcessor(format)
    return processor.Format(index)
}
```

**Continue-Specific Settings Manager**:
```go
// internal/integrations/adapters/continue/config.go

type ContinueConfig struct {
    Tools []Tool `json:"tools"`
    // ... other Continue config fields
}

type Tool struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Command     string `json:"command"`
}

func (a *ContinueAdapter) setupMemoryTool(binaryPath string) error {
    config, err := a.readConfig()
    if err != nil {
        return err
    }

    // Check if memory tool already exists
    for i, tool := range config.Tools {
        if tool.Name == "memory" {
            // Update existing
            config.Tools[i].Command = fmt.Sprintf("%s read --format markdown --integration continue", binaryPath)
            return a.writeConfig(config)
        }
    }

    // Add new tool
    config.Tools = append(config.Tools, Tool{
        Name:        "memory",
        Description: "Access agentic memory index with semantic understanding of all files",
        Command:     fmt.Sprintf("%s read --format markdown --integration continue", binaryPath),
    })

    return a.writeConfig(config)
}
```

#### 3.2 Cline Adapter

**Research Cline Integration**:
- Configuration: VS Code extension settings or `.cline/` directory
- Integration mechanism: Custom tools or startup commands

**New File**: `internal/integrations/adapters/cline/adapter.go`

Similar structure to Continue adapter, adapted for Cline's specific configuration format.

#### 3.3 Aider Adapter

**Research Aider Integration**:
- Aider uses `.aider.conf.yml` for configuration
- May support custom commands or startup scripts

**New File**: `internal/integrations/adapters/aider/adapter.go`

#### 3.4 Generic Adapter (Fallback)

**For frameworks without specific integration**:

**New File**: `internal/integrations/adapters/generic/adapter.go`

```go
type GenericAdapter struct {
    name         string
    commandPath  string
    outputFormat OutputFormat
}

func NewGenericAdapter(name, commandPath string, format OutputFormat) *GenericAdapter {
    return &GenericAdapter{
        name:         name,
        commandPath:  commandPath,
        outputFormat: format,
    }
}

func (a *GenericAdapter) GetName() string {
    return a.name
}

func (a *GenericAdapter) GetCommand(binaryPath string, format OutputFormat) string {
    return fmt.Sprintf("%s read --format %s", binaryPath, format)
}

func (a *GenericAdapter) FormatOutput(index *types.Index, format OutputFormat) (string, error) {
    // Just return formatted content without wrapping
    processor := getOutputProcessor(format)
    return processor.Format(index)
}

// Setup() returns error "manual setup required"
func (a *GenericAdapter) Setup(binaryPath string) error {
    cmd := a.GetCommand(binaryPath, a.outputFormat)
    return fmt.Errorf("automatic setup not supported for %s. Please add this command to your configuration: %s", a.name, cmd)
}
```

#### 3.5 Integration Registry Initialization

**Modify**: `main.go` or create `internal/integrations/init.go`

```go
func init() {
    registry := integrations.GlobalRegistry()

    // Register all adapters
    registry.Register(claude.NewClaudeCodeAdapter())
    registry.Register(continue.NewContinueAdapter())
    registry.Register(cline.NewClineAdapter())
    registry.Register(aider.NewAiderAdapter())
}
```

#### 3.6 Integration Management Commands

**New Command Group**: `cmd/integrations/`

```go
// cmd/integrations/integrations.go
var integrationsCmd = &cobra.Command{
    Use:   "integrations",
    Short: "Manage agent framework integrations",
}

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List available integrations",
    Run:   runList,
}

var detectCmd = &cobra.Command{
    Use:   "detect",
    Short: "Detect installed agent frameworks",
    Run:   runDetect,
}

var setupCmd = &cobra.Command{
    Use:   "setup [integration]",
    Short: "Setup a specific integration",
    Args:  cobra.ExactArgs(1),
    Run:   runSetup,
}

var removeCmd = &cobra.Command{
    Use:   "remove [integration]",
    Short: "Remove an integration",
    Args:  cobra.ExactArgs(1),
    Run:   runRemove,
}

func runList(cmd *cobra.Command, args []string) {
    registry := integrations.GlobalRegistry()
    all := registry.List()

    fmt.Println("Available Integrations:")
    for _, integration := range all {
        status := "not configured"
        if enabled, _ := integration.IsEnabled(); enabled {
            status = "configured"
        }
        fmt.Printf("  - %s: %s (%s)\n",
            integration.GetName(),
            integration.GetDescription(),
            status)
    }
}

func runDetect(cmd *cobra.Command, args []string) {
    registry := integrations.GlobalRegistry()
    detected := registry.DetectAvailable()

    fmt.Println("Detected Agent Frameworks:")
    for _, integration := range detected {
        fmt.Printf("  ✓ %s\n", integration.GetName())
    }
}
```

**Wire into Root Command**:
```go
// cmd/root.go
func init() {
    rootCmd.AddCommand(integrationsCmd)
}
```

**Deliverables**:
- [ ] Continue.dev adapter implemented
- [ ] Cline adapter implemented
- [ ] Aider adapter implemented (or Generic adapter documented)
- [ ] Generic adapter for unsupported frameworks
- [ ] All adapters registered in registry
- [ ] Integration management commands working
- [ ] Documentation for each integration
- [ ] Tests for all new adapters

---

### Phase 4: Migration & Documentation (Week 6)

**Goal**: Ensure smooth migration for existing users and comprehensive documentation.

#### 4.1 Migration Tool

**New Command**: `cmd/migrate/migrate.go`

```go
var migrateCmd = &cobra.Command{
    Use:   "migrate",
    Short: "Migrate from legacy hook setup to new integration system",
    Long:  `Analyzes existing configuration and migrates to the new integration-based system.`,
    Run:   runMigrate,
}

func runMigrate(cmd *cobra.Command, args []string) {
    fmt.Println("Analyzing existing configuration...")

    // Check if using old hook setup
    hasOldHooks := checkLegacyClaudeHooks()

    if !hasOldHooks {
        fmt.Println("No legacy configuration detected. Nothing to migrate.")
        return
    }

    fmt.Println("Legacy Claude Code hooks detected.")
    fmt.Println("Migrating to new integration system...")

    // Update config.yaml to enable claude-code integration
    err := updateConfigForIntegrations()
    if err != nil {
        fmt.Printf("Error updating config: %v\n", err)
        return
    }

    // Verify hooks still work with new command format
    binaryPath, _ := hooks.FindBinaryPath()
    registry := integrations.GlobalRegistry()
    claudeAdapter, _ := registry.Get("claude-code")

    // Update hook commands to use new format
    err = claudeAdapter.Update(binaryPath)
    if err != nil {
        fmt.Printf("Error updating hooks: %v\n", err)
        return
    }

    fmt.Println("✓ Migration complete!")
    fmt.Println("\nYour Claude Code hooks now use the new integration system.")
    fmt.Println("Run 'agentic-memorizer integrations list' to see available integrations.")
}
```

#### 4.2 Update Documentation

**Files to Create/Update**:

1. **`README.md`** - Update to mention multi-framework support
   - Add "Supported Integrations" section
   - Update installation instructions
   - Show examples for different frameworks

2. **`docs/integrations/`** - New directory
   - `claude-code.md` - Claude Code integration guide
   - `continue.md` - Continue.dev integration guide
   - `cline.md` - Cline integration guide
   - `custom.md` - Guide for adding custom integrations

3. **`docs/migration-guide.md`** - For existing users
   - What's changing
   - Automatic migration steps
   - Manual migration if needed
   - Troubleshooting

4. **`docs/architecture.md`** - Architecture documentation
   - Adapter pattern explanation
   - Integration interface specification
   - Output format documentation
   - How to create custom adapters

5. **`CHANGELOG.md`** - Document breaking changes
   - Deprecation notices
   - New features
   - Migration guide reference

#### 4.3 Configuration Examples

**Create Example Configs**:

```yaml
# examples/config-multi-integration.yaml
# Example configuration with multiple integrations enabled

integrations:
  enabled: ["claude-code", "continue"]

  claude-code:
    type: "claude-code"
    enabled: true
    output_format: "xml"
    settings:
      settings_path: "~/.claude/settings.json"
      matchers: ["startup", "resume", "clear", "compact"]

  continue:
    type: "continue"
    enabled: true
    output_format: "markdown"
    settings:
      config_path: "~/.continue/config.json"
```

#### 4.4 Update Help Text

**Improve CLI Help**:
```go
// cmd/read/read.go
var readCmd = &cobra.Command{
    Use:   "read",
    Short: "Read the precomputed memory index",
    Long: `Read and display the precomputed memory index.

This command loads the index from disk and formats it for consumption by
AI agent frameworks. The index contains metadata and semantic analysis
for all files in your memory directory.

Output Formats (--format):
  The --format flag controls how the index content is rendered:
  - xml: Structured XML format (default, best for Claude Code)
  - markdown: Human-readable markdown format (good for Continue, Aider)
  - json: JSON format (NEW - renders index as JSON for custom integrations)

Integration Wrappers (--integration):
  The --integration flag wraps the formatted content for specific frameworks:
  - claude-code: Wraps in Claude Code SessionStart hook JSON envelope
  - continue: Returns plain output (no wrapper needed)
  - cline: Cline-specific wrapper (if needed)
  - aider: Returns plain output (no wrapper needed)

  Note: Integration wrappers do NOT change the format. They wrap the
  already-formatted content in framework-specific envelopes.

Examples:
  # Default output (XML format, no wrapper)
  agentic-memorizer read

  # Claude Code SessionStart hook (XML wrapped in JSON envelope)
  agentic-memorizer read --integration claude-code
  # → Output: {"continue": true, "hookSpecificOutput": {"additionalContext": "<xml>..."}}

  # Claude Code with Markdown format (Markdown wrapped in JSON envelope)
  agentic-memorizer read --format markdown --integration claude-code
  # → Output: {"continue": true, "hookSpecificOutput": {"additionalContext": "# Markdown..."}}

  # Continue.dev (Markdown, no wrapper)
  agentic-memorizer read --format markdown --integration continue
  # → Output: # Markdown index content...

  # New JSON format (no wrapper)
  agentic-memorizer read --format json
  # → Output: {"generated": "...", "entries": [...]}

  # DEPRECATED (but still works) - equivalent to --integration claude-code
  agentic-memorizer read --format xml --wrap-json
`,
    Run: runRead,
}
```

#### 4.5 Testing Documentation

**Create Testing Guide**: `docs/testing.md`

Document how to test each integration:
- Unit tests for adapters
- Integration tests for each framework
- End-to-end tests with real agent frameworks
- Mock testing for CI/CD

#### 4.6 Contributing Guide

**Create**: `docs/CONTRIBUTING.md`

Section on adding new integrations:
```markdown
## Adding a New Integration

To add support for a new agent framework:

1. **Research the framework's integration mechanism**
   - How does it accept external tools/commands?
   - Where are configuration files stored?
   - What format does it expect for output?

2. **Create an adapter**
   - Create `internal/integrations/adapters/{framework}/adapter.go`
   - Implement the `Integration` interface
   - Add framework-specific configuration handling

3. **Implement configuration management**
   - Read framework's config file format
   - Safely modify configuration
   - Preserve existing settings

4. **Add output formatting if needed**
   - Use existing output processors (XML, Markdown, JSON)
   - Create custom wrapper if framework requires it

5. **Write tests**
   - Unit tests for adapter methods
   - Integration tests with mock configs
   - End-to-end tests (optional)

6. **Document the integration**
   - Create `docs/integrations/{framework}.md`
   - Add examples
   - Include troubleshooting tips

7. **Register the adapter**
   - Add to `internal/integrations/init.go`
   - Update README.md with supported frameworks list
```

**Deliverables**:
- [ ] Migration tool implemented and tested
- [ ] README.md updated
- [ ] Integration guides created for all supported frameworks
- [ ] Migration guide written
- [ ] Architecture documentation complete
- [ ] Configuration examples provided
- [ ] CLI help text improved
- [ ] Testing documentation created
- [ ] Contributing guide with integration instructions
- [ ] CHANGELOG updated

---

### Phase 5: Polish & Release (Week 7)

**Goal**: Prepare for production release with monitoring, error handling, and user experience improvements.

#### 5.1 Enhanced Error Handling

**Add Integration-Specific Error Types**:
```go
// internal/integrations/errors.go

type IntegrationError struct {
    Integration string
    Operation   string
    Err         error
}

func (e *IntegrationError) Error() string {
    return fmt.Sprintf("%s integration %s failed: %v", e.Integration, e.Operation, e.Err)
}

func (e *IntegrationError) Unwrap() error {
    return e.Err
}

// Specific error types
var (
    ErrIntegrationNotFound    = errors.New("integration not found")
    ErrIntegrationNotDetected = errors.New("integration not detected on system")
    ErrConfigNotFound         = errors.New("configuration file not found")
    ErrConfigInvalid          = errors.New("configuration file invalid")
    ErrSetupFailed            = errors.New("setup failed")
)
```

**Improve Error Messages**:
```go
// Instead of: "failed to setup hooks"
// Use: "Failed to setup Claude Code integration: ~/.claude/settings.json not found.
//       Run 'agentic-memorizer init' to create initial configuration."
```

#### 5.2 Integration Health Checks

**Add to Integration Interface**:
```go
type Integration interface {
    // ... existing methods ...

    // Health check
    Health() (*HealthStatus, error)
}

type HealthStatus struct {
    Healthy     bool
    Message     string
    LastChecked time.Time
    Issues      []string
}
```

**Implement Health Command**:
```go
// cmd/integrations/health.go

var healthCmd = &cobra.Command{
    Use:   "health",
    Short: "Check health of configured integrations",
    Run:   runHealth,
}

func runHealth(cmd *cobra.Command, args []string) {
    cfg, _ := config.GetConfig()
    registry := integrations.GlobalRegistry()

    fmt.Println("Checking integration health...\n")

    for _, name := range cfg.Integrations.Enabled {
        integration, err := registry.Get(name)
        if err != nil {
            fmt.Printf("❌ %s: not registered\n", name)
            continue
        }

        status, err := integration.Health()
        if err != nil {
            fmt.Printf("❌ %s: health check failed: %v\n", name, err)
            continue
        }

        if status.Healthy {
            fmt.Printf("✓ %s: healthy\n", name)
        } else {
            fmt.Printf("⚠ %s: %s\n", name, status.Message)
            for _, issue := range status.Issues {
                fmt.Printf("  - %s\n", issue)
            }
        }
    }
}
```

#### 5.3 Logging & Observability

**Add Integration Event Logging**:
```go
// internal/integrations/logger.go

type Logger struct {
    slog.Logger
}

func (l *Logger) LogSetup(integration string, success bool, err error) {
    if success {
        l.Info("integration setup successful",
            "integration", integration)
    } else {
        l.Error("integration setup failed",
            "integration", integration,
            "error", err)
    }
}

func (l *Logger) LogOutput(integration string, format OutputFormat, size int) {
    l.Debug("formatted output for integration",
        "integration", integration,
        "format", format,
        "bytes", size)
}
```

**Add Metrics**:
```go
// Track which integrations are used
type Metrics struct {
    IntegrationCalls map[string]int
    mu               sync.Mutex
}

func (m *Metrics) RecordCall(integration string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.IntegrationCalls[integration]++
}
```

#### 5.4 Interactive Setup Wizard

**Enhance Init Command**:
```go
// cmd/init/wizard.go

func runInteractiveSetup() error {
    fmt.Println("🔧 Agentic Memorizer Setup Wizard\n")

    // Step 1: Memory directory
    memoryRoot := promptForDirectory(
        "Where should your memory files be stored?",
        "~/.agentic-memorizer/memory",
    )

    // Step 2: Detect integrations
    fmt.Println("\n🔍 Detecting agent frameworks...")
    registry := integrations.GlobalRegistry()
    detected := registry.DetectAvailable()

    if len(detected) == 0 {
        fmt.Println("No agent frameworks detected on your system.")
        fmt.Println("You can manually configure integrations later.")
        return finishSetup(memoryRoot, nil)
    }

    fmt.Printf("\nFound %d agent framework(s):\n", len(detected))
    for i, integration := range detected {
        fmt.Printf("  %d. %s - %s\n", i+1, integration.GetName(), integration.GetDescription())
    }

    // Step 3: Choose integrations to setup
    selected := promptForIntegrations(detected)

    // Step 4: Setup each selected integration
    binaryPath, _ := findBinaryPath()
    for _, integration := range selected {
        fmt.Printf("\n📝 Setting up %s...\n", integration.GetName())
        err := integration.Setup(binaryPath)
        if err != nil {
            fmt.Printf("⚠ Warning: %v\n", err)
            continue
        }
        fmt.Printf("✓ %s configured successfully\n", integration.GetName())
    }

    // Step 5: Start daemon
    startDaemon := promptYesNo("\nStart background daemon now?", true)
    if startDaemon {
        fmt.Println("\n🚀 Starting daemon...")
        return startDaemonProcess()
    }

    fmt.Println("\n✓ Setup complete!")
    fmt.Println("\nNext steps:")
    fmt.Println("  1. Start the daemon: agentic-memorizer daemon start")
    fmt.Println("  2. Add files to your memory directory: " + memoryRoot)
    fmt.Println("  3. Check status: agentic-memorizer daemon status")

    return nil
}
```

#### 5.5 Performance Optimization

**Lazy Loading for Registry**:
```go
// Don't initialize all adapters at startup
// Only initialize when needed

type LazyIntegration struct {
    name    string
    factory func() Integration
    once    sync.Once
    inst    Integration
}

func (l *LazyIntegration) Get() Integration {
    l.once.Do(func() {
        l.inst = l.factory()
    })
    return l.inst
}
```

**Output Caching**:
```go
// Cache formatted output per integration
// Invalidate when index changes

type OutputCache struct {
    cache map[string]*CachedOutput
    mu    sync.RWMutex
}

type CachedOutput struct {
    Output      string
    IndexHash   string
    GeneratedAt time.Time
}
```

#### 5.6 Security Review

**Review**:
- [ ] Input validation for all configuration files
- [ ] Path traversal prevention in file operations
- [ ] Safe JSON/YAML parsing (use standard library)
- [ ] No arbitrary code execution
- [ ] Secure file permissions on config files
- [ ] Validate binary paths before execution
- [ ] Sanitize user input in commands

**Add Validation**:
```go
func validateBinaryPath(path string) error {
    // Ensure path is absolute
    if !filepath.IsAbs(path) {
        return fmt.Errorf("binary path must be absolute: %s", path)
    }

    // Ensure file exists and is executable
    info, err := os.Stat(path)
    if err != nil {
        return fmt.Errorf("binary not found: %w", err)
    }

    if info.IsDir() {
        return fmt.Errorf("binary path is a directory: %s", path)
    }

    // Check executable bit (Unix)
    if info.Mode()&0111 == 0 {
        return fmt.Errorf("binary is not executable: %s", path)
    }

    return nil
}
```

#### 5.7 Version Compatibility

**Add Version Checks**:
```go
// internal/integrations/version.go

type VersionConstraint struct {
    MinVersion string
    MaxVersion string
}

func CheckVersionCompatibility(integration Integration) error {
    constraint := integration.GetVersionConstraint()
    currentVersion := version.Get()

    if !isVersionCompatible(currentVersion, constraint) {
        return fmt.Errorf(
            "agentic-memorizer version %s is not compatible with %s (requires %s-%s)",
            currentVersion,
            integration.GetName(),
            constraint.MinVersion,
            constraint.MaxVersion,
        )
    }

    return nil
}
```

#### 5.8 Final Testing

**Test Matrix**:
```
| Integration | Auto-Detect | Setup | Update | Remove | Output | Health |
|-------------|-------------|-------|--------|--------|--------|--------|
| Claude Code |     ✓       |   ✓   |   ✓    |   ✓    |   ✓    |   ✓    |
| Continue    |     ✓       |   ✓   |   ✓    |   ✓    |   ✓    |   ✓    |
| Cline       |     ✓       |   ✓   |   ✓    |   ✓    |   ✓    |   ✓    |
| Generic     |     -       |   ✓   |   -    |   -    |   ✓    |   ✓    |
```

**E2E Test Scenarios**:
1. Fresh install on clean system with Claude Code
2. Fresh install on system with multiple frameworks
3. Upgrade from legacy version (pre-decoupling)
4. Migration from `--wrap-json` to `--integration`
5. Switching between integrations
6. Running multiple integrations simultaneously
7. Daemon restart with integrations active
8. Error recovery (invalid config, missing binary, etc.)

**Deliverables**:
- [ ] Enhanced error handling implemented
- [ ] Health check system added
- [ ] Logging and observability improved
- [ ] Interactive setup wizard complete
- [ ] Performance optimizations applied
- [ ] Security review completed and issues addressed
- [ ] Version compatibility checking added
- [ ] Full test matrix executed
- [ ] All E2E scenarios passing
- [ ] Release notes drafted
- [ ] Version bumped to 1.0.0 or 0.5.0 (breaking changes)

---

## Implementation Checklist

Use this checklist to track progress through the implementation plan.

### Phase 1: Foundation & Abstraction Layer ✓

#### Core Abstractions
- [ ] Create `internal/integrations/` package structure
- [ ] Define `Integration` interface in `interface.go`
- [ ] Implement `Registry` in `registry.go`
- [ ] Add thread-safe registry operations (register, get, list, detect)
- [ ] Create `types.go` with shared types (`OutputFormat`, `IntegrationConfig`, etc.)
- [ ] Add utility functions in `utils.go`

#### Output Processors
- [ ] Create `internal/integrations/output/` package
- [ ] Define `OutputProcessor` interface
- [ ] Extract XML formatting to `XMLProcessor` (from `internal/output/formatter.go`)
- [ ] Extract Markdown formatting to `MarkdownProcessor`
- [ ] **NEW**: Implement `JSONProcessor` to render index as JSON (this is a new output format, not the storage format)
- [ ] Test all processors independently
- [ ] Keep `internal/output/formatter.go` as backward-compatible wrapper

#### Configuration Schema
- [ ] Add `IntegrationsConfig` to `internal/config/types.go`
- [ ] Add `IntegrationConfig` struct
- [ ] Update `DefaultConfig` in `constants.go` with Claude Code defaults
- [ ] Test configuration loading with new schema
- [ ] Ensure backward compatibility (old configs still work)

#### Testing Infrastructure
- [ ] Create `internal/integrations/testing/` package
- [ ] Implement `MockIntegration` for testing
- [ ] Write `registry_test.go` (registration, lookup, concurrency)
- [ ] Write `output_test.go` (all output processors)
- [ ] Write `config_test.go` (configuration loading)
- [ ] Ensure all existing tests still pass
- [ ] Document testing utilities

### Phase 2: Claude Code Adapter ✓

#### Adapter Implementation
- [ ] Create `internal/integrations/adapters/claude/` package
- [ ] Implement `ClaudeCodeAdapter` struct
- [ ] Implement `GetName()`, `GetDescription()`, `GetVersion()`
- [ ] Implement `Detect()` - check for `~/.claude/settings.json`
- [ ] Implement `IsEnabled()` - verify hooks are configured
- [ ] Port settings management from `internal/hooks/manager.go`

#### Settings Management
- [ ] Create `settings.go` in Claude adapter
- [ ] Port `ReadSettings()` from hooks manager
- [ ] Port `WriteSettings()` from hooks manager
- [ ] Port `SetupSessionStartHooks()` logic
- [ ] Preserve all 4 matchers (startup, resume, clear, compact)
- [ ] Test settings read/write/update operations

#### Output Formatting
- [ ] Create `output.go` in Claude adapter
- [ ] Define `SessionStartOutput` struct
- [ ] Define `HookSpecificOutput` struct
- [ ] Port `WrapJSON()` logic from `internal/output/formatter.go`
- [ ] Implement `FormatOutput()` adapter method
- [ ] Test JSON wrapping produces identical output to legacy

#### Command Updates
- [ ] Add `--integration <name>` flag to `cmd/read/read.go`
- [ ] Implement `outputForIntegration()` function
- [ ] Keep `--wrap-json` flag with deprecation warning
- [ ] Map `--wrap-json` to `--integration claude-code` internally
- [ ] Update help text with integration examples

#### Init Command Updates
- [ ] Update `cmd/init/init.go` to use integration registry
- [ ] Replace direct `hooks.SetupSessionStartHooks()` call
- [ ] Add integration detection and setup flow
- [ ] Prompt user for integration selection (if multiple detected)
- [ ] Support `--integration <name>` flag for non-interactive setup

#### Validation & Testing
- [ ] Test fresh install with Claude Code
- [ ] Test upgrade from legacy hook setup
- [ ] Test `--wrap-json` deprecated path still works
- [ ] Test `--integration claude-code` new path
- [ ] Verify SessionStart hooks trigger correctly
- [ ] Compare output format (byte-for-byte identical to legacy)
- [ ] Test all 4 matchers (startup, resume, clear, compact)

### Phase 3: Additional Integrations ✓

#### Continue.dev Adapter
- [ ] Research Continue.dev configuration format
- [ ] Create `internal/integrations/adapters/continue/` package
- [ ] Implement `ContinueAdapter` struct
- [ ] Implement detection (`~/.continue/config.json` or `.ts`)
- [ ] Create `config.go` for Continue config management
- [ ] Implement `Setup()` - add memory tool to tools array
- [ ] Implement `FormatOutput()` - plain markdown (no wrapping)
- [ ] Test with real Continue.dev installation
- [ ] Document Continue.dev integration

#### Cline Adapter
- [ ] Research Cline configuration mechanism
- [ ] Create `internal/integrations/adapters/cline/` package
- [ ] Implement `ClineAdapter` struct
- [ ] Implement detection
- [ ] Implement configuration management
- [ ] Implement `Setup()` based on Cline's integration points
- [ ] Implement `FormatOutput()`
- [ ] Test with real Cline installation
- [ ] Document Cline integration

#### Aider Adapter
- [ ] Research Aider configuration (`.aider.conf.yml`)
- [ ] Create `internal/integrations/adapters/aider/` package
- [ ] Implement `AiderAdapter` struct
- [ ] Implement detection and setup
- [ ] Test with Aider
- [ ] Document Aider integration

#### Generic Adapter
- [ ] Create `internal/integrations/adapters/generic/` package
- [ ] Implement `GenericAdapter` for unsupported frameworks
- [ ] `Setup()` returns helpful error with manual instructions
- [ ] `FormatOutput()` returns plain format without wrapping
- [ ] Document how to use generic adapter

#### Integration Registry
- [ ] Create `internal/integrations/init.go` or modify `main.go`
- [ ] Register Claude Code adapter
- [ ] Register Continue adapter
- [ ] Register Cline adapter
- [ ] Register Aider adapter
- [ ] Ensure lazy initialization for performance

#### Management Commands
- [ ] Create `cmd/integrations/` package
- [ ] Implement `integrations list` command
- [ ] Implement `integrations detect` command
- [ ] Implement `integrations setup <name>` command
- [ ] Implement `integrations remove <name>` command
- [ ] Add help text and examples for all commands
- [ ] Wire into root command

### Phase 4: Migration & Documentation ✓

#### Migration Tool
- [ ] Create `cmd/migrate/migrate.go`
- [ ] Implement legacy hook detection
- [ ] Implement config.yaml migration
- [ ] Implement hook command format update
- [ ] Test migration with various legacy setups
- [ ] Add rollback capability if migration fails
- [ ] Document migration process

#### Core Documentation
- [ ] Update `README.md` with multi-framework support
- [ ] Add "Supported Integrations" section to README
- [ ] Update installation instructions
- [ ] Add examples for each framework
- [ ] Create `docs/architecture.md`
- [ ] Document adapter pattern
- [ ] Document integration interface specification
- [ ] Document output format options

#### Integration Guides
- [ ] Create `docs/integrations/` directory
- [ ] Write `claude-code.md` guide
- [ ] Write `continue.md` guide
- [ ] Write `cline.md` guide
- [ ] Write `aider.md` guide (or note as generic)
- [ ] Write `custom.md` guide for adding new integrations
- [ ] Include screenshots/examples in guides

#### Migration & Maintenance Docs
- [ ] Create `docs/migration-guide.md`
- [ ] Document what's changing
- [ ] Document automatic migration steps
- [ ] Document manual migration fallback
- [ ] Add troubleshooting section
- [ ] Create rollback instructions

#### Examples & Config
- [ ] Create `examples/config-multi-integration.yaml`
- [ ] Create example for Claude Code only
- [ ] Create example for Continue only
- [ ] Create example for multiple integrations
- [ ] Document each configuration option

#### Help Text & CLI UX
- [ ] Update `read` command help text
- [ ] Update `init` command help text
- [ ] Add integration examples to help
- [ ] Improve error messages with actionable guidance
- [ ] Add deprecation warnings for `--wrap-json`

#### Contributing & Testing Docs
- [ ] Create `docs/CONTRIBUTING.md`
- [ ] Add section on adding new integrations
- [ ] Create `docs/testing.md`
- [ ] Document unit testing approach
- [ ] Document integration testing approach
- [ ] Document E2E testing approach

#### Changelog
- [ ] Update `CHANGELOG.md`
- [ ] Document breaking changes
- [ ] Document new features
- [ ] Document deprecations
- [ ] Add migration guide reference
- [ ] Bump version appropriately (1.0.0 or 0.5.0)

### Phase 5: Polish & Release ✓

#### Error Handling
- [ ] Create `internal/integrations/errors.go`
- [ ] Define `IntegrationError` type
- [ ] Define specific error types (NotFound, NotDetected, etc.)
- [ ] Update all adapter error returns to use typed errors
- [ ] Improve error messages with actionable guidance
- [ ] Add context to errors (which integration, which operation)

#### Health Checks
- [ ] Add `Health()` method to `Integration` interface
- [ ] Define `HealthStatus` struct
- [ ] Implement health checks for Claude Code adapter
- [ ] Implement health checks for other adapters
- [ ] Create `cmd/integrations/health.go` command
- [ ] Test health checks in various failure scenarios

#### Logging & Observability
- [ ] Create `internal/integrations/logger.go`
- [ ] Add structured logging for integration events
- [ ] Log setup/update/remove operations
- [ ] Log output formatting calls
- [ ] Add metrics tracking (integration usage counts)
- [ ] Integrate with existing daemon logging

#### Interactive Setup Wizard
- [ ] Create `cmd/init/wizard.go`
- [ ] Implement interactive prompts for directory selection
- [ ] Implement integration detection display
- [ ] Implement multi-select for integration setup
- [ ] Add confirmation steps
- [ ] Make wizard default for `agentic-memorizer init`
- [ ] Add `--non-interactive` flag for scripting

#### Performance Optimization
- [ ] Implement lazy loading for integrations
- [ ] Add output caching (invalidate on index change)
- [ ] Profile read command with integrations
- [ ] Optimize registry lookups
- [ ] Benchmark before/after performance

#### Security Review
- [ ] Review all file path handling for traversal vulnerabilities
- [ ] Validate binary paths before execution
- [ ] Review JSON/YAML parsing (use safe parsers)
- [ ] Check file permissions on config files
- [ ] Sanitize user input in commands
- [ ] Review for arbitrary code execution risks
- [ ] Document security considerations

#### Version Compatibility
- [ ] Add version constraint system
- [ ] Implement version compatibility checking
- [ ] Add `GetVersionConstraint()` to integrations
- [ ] Check compatibility on setup/update
- [ ] Document version compatibility in docs

#### Final Testing
- [ ] Create test matrix (all integrations × all operations)
- [ ] Test fresh install on clean system (Claude Code only)
- [ ] Test fresh install with multiple frameworks
- [ ] Test upgrade from legacy (pre-decoupling) version
- [ ] Test migration from `--wrap-json` to `--integration`
- [ ] Test switching between integrations
- [ ] Test multiple simultaneous integrations
- [ ] Test daemon restart with integrations active
- [ ] Test error recovery scenarios
- [ ] Run full E2E test suite
- [ ] Performance testing under load

#### Release Preparation
- [ ] Draft release notes
- [ ] Update version in `internal/version/version.go`
- [ ] Tag release in git
- [ ] Build binaries for all platforms
- [ ] Test binaries on fresh systems
- [ ] Update package manager configs (if applicable)
- [ ] Prepare announcement post/tweet

---

## Risk Assessment & Mitigation

### High Risk Items

#### 1. Breaking Existing Claude Code Users

**Risk**: Users rely on current hook setup; changes could break their workflow.

**Mitigation**:
- Keep `--wrap-json` flag working (deprecated but functional)
- Auto-migration tool for upgrading
- Extensive backward compatibility testing
- Clear migration guide
- Gradual deprecation timeline (6+ months before removal)

#### 2. Integration Complexity

**Risk**: Supporting multiple frameworks increases maintenance burden.

**Mitigation**:
- Strong abstraction layer (adapter pattern)
- Comprehensive testing for each integration
- Clear interface contracts
- Documentation for adding new integrations
- Community can contribute adapters

#### 3. Configuration Migration Failures

**Risk**: Auto-migration could corrupt user settings.

**Mitigation**:
- Always backup original config before migration
- Atomic file operations (temp + rename)
- Rollback capability
- Validation after migration
- Dry-run mode for migration

### Medium Risk Items

#### 4. Performance Regression

**Risk**: Additional abstraction layers could slow down reads.

**Mitigation**:
- Benchmark before/after
- Lazy loading for integrations
- Output caching
- Profile and optimize hot paths
- Target: <50ms for read command (same as current)

#### 5. Documentation Maintenance

**Risk**: Multiple integrations = multiple docs to maintain.

**Mitigation**:
- Modular documentation structure
- Template for new integration docs
- Automated documentation generation where possible
- Community contributions for framework-specific docs

### Low Risk Items

#### 6. Version Compatibility

**Risk**: Different agentic-memorizer versions with different integrations.

**Mitigation**:
- Version constraints per integration
- Clear compatibility matrix in docs
- Fail fast with helpful errors

---

## Success Criteria

### Functional Requirements
- [ ] All existing Claude Code functionality works identically
- [ ] At least 2 additional frameworks supported (Continue, Cline/Aider)
- [ ] Generic adapter works for unsupported frameworks
- [ ] Migration from legacy setup is automatic and reliable
- [ ] All integrations can be enabled simultaneously
- [ ] Health checks verify integration status

### Non-Functional Requirements
- [ ] Read command performance: <50ms (same as current)
- [ ] No breaking changes without migration path
- [ ] Backward compatibility maintained for 1+ versions
- [ ] 100% test coverage for integration layer
- [ ] Documentation complete for all supported integrations

### User Experience
- [ ] `agentic-memorizer init` detects and sets up integrations automatically
- [ ] Clear error messages with actionable guidance
- [ ] `--help` text explains integration options clearly
- [ ] Migration is transparent (user barely notices)
- [ ] New users can add integrations easily

---

## Post-Implementation Tasks

### After v1.0 Release

1. **Monitor Adoption**
   - Track which integrations are most used
   - Gather user feedback
   - Monitor issue reports

2. **Community Integrations**
   - Accept PRs for new adapters
   - Create integration adapter template
   - Maintain compatibility matrix

3. **Optimization**
   - Profile real-world usage
   - Optimize based on telemetry
   - Cache improvements

4. **Future Integrations**
   - Open Interpreter
   - AutoGPT
   - LangChain agents
   - Custom enterprise agents

5. **Advanced Features**
   - Per-integration index filtering
   - Integration-specific semantic analysis prompts
   - Multi-index support (different memory dirs per integration)
   - Remote integration support (API-based)

---

## Timeline Summary

| Phase | Duration | Key Deliverables |
|-------|----------|------------------|
| Phase 1: Foundation | 2 weeks | Interface, Registry, Output Processors, Config Schema |
| Phase 2: Claude Adapter | 1 week | Claude Code adapter, backward compatibility |
| Phase 3: Additional Integrations | 2 weeks | Continue, Cline, Aider adapters, management commands |
| Phase 4: Migration & Docs | 1 week | Migration tool, comprehensive documentation |
| Phase 5: Polish & Release | 1 week | Error handling, health checks, testing, release |
| **Total** | **7 weeks** | Production-ready v1.0 release |

---

## Conclusion

This decoupling effort transforms agentic-memorizer from a Claude Code-specific tool into a **framework-agnostic agentic memory system**. The adapter pattern provides clean abstraction, making it easy to add new integrations while maintaining the robust core pipeline.

The phased approach ensures:
1. **No disruption** to existing users
2. **Clear migration path** from legacy to new system
3. **Extensibility** for future integrations
4. **Maintainability** through strong abstractions

By the end of this implementation, agentic-memorizer will be positioned as the **go-to memory solution** for AI agent frameworks, not just Claude Code.

---

**Next Steps**: Review this plan, adjust timeline/scope as needed, and begin Phase 1 implementation.
