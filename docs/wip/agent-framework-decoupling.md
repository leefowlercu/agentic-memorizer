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

## Platform Support

**Supported Platforms**: macOS and Linux (Unix-like systems)

This implementation targets Unix-like systems with the following assumptions:

- **Signal handling**: SIGHUP (reload config), SIGINT/SIGTERM (graceful shutdown)
- **File paths**: Using `~/` expansion for user home directories
- **File permissions**: Standard Unix permissions (0600 for sensitive files, 0644 for configs)
- **Daemon management**: PID files in `~/.agentic-memorizer/daemon.pid`
- **Shell integration**: Assumes bash/zsh compatible shells
- **Path separators**: Forward slashes (`/`)

### Future Work: Windows Support

Windows support will be addressed in a future release and will require:

- **Signal alternative**: Named pipes, file watching, or HTTP endpoint for reload (no SIGHUP on Windows)
- **Path handling**: `%USERPROFILE%` instead of `~/`, backslash path separators
- **Service integration**: Windows service manager instead of Unix daemon model
- **File locking**: Windows-compatible file locking mechanisms
- **Integration paths**: Windows-specific default paths for Claude Code, Continue, etc.

**Note**: All examples, paths, and commands in this document assume macOS/Linux unless otherwise noted.

---

## Architecture Analysis

### Current Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     CORE PIPELINE (GOOD)                        │
│  ┌──────┐   ┌──────────┐   ┌──────────┐   ┌───────┐             │
│  │Daemon│──>│ Metadata │──>│ Semantic │──>│ Index │             │
│  │      │   │ Extract  │   │ Analysis │   │ Mgr   │             │
│  └──────┘   └──────────┘   └──────────┘   └───────┘             │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│              TIGHTLY COUPLED INTEGRATION (PROBLEM)              │
│                                                                 │
│  ┌──────────────────────┐         ┌──────────────────────┐      │
│  │   hooks/manager.go   │         │  output/formatter.go │      │
│  │                      │         │                      │      │
│  │ - ~/.claude/         │         │ - WrapJSON()         │      │
│  │   settings.json      │────────>│ - SessionStart       │      │
│  │ - 4 hardcoded        │         │   specific format    │      │
│  │   matchers           │         │ - Hardcoded fields   │      │
│  └──────────────────────┘         └──────────────────────┘      │
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
│                     CORE PIPELINE (UNCHANGED)                   │
│  ┌──────┐   ┌──────────┐   ┌──────────┐   ┌───────┐             │
│  │Daemon│──>│ Metadata │──>│ Semantic │──>│ Index │             │
│  │      │   │ Extract  │   │ Analysis │   │ Mgr   │             │
│  └──────┘   └──────────┘   └──────────┘   └───────┘             │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│              ABSTRACTION LAYER (NEW)                            │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │            Integration Registry & Manager                │   │
│  │  - Discovers available integrations                      │   │
│  │  - Manages lifecycle (setup/update/remove)               │   │
│  │  - Configuration per integration                         │   │
│  └──────────────────────────────────────────────────────────┘   │
│                          │                                      │
│            ┌─────────────┴─────────────┬──────────────┐         │
│            ▼                           ▼              ▼         │
│  ┌──────────────────┐     ┌──────────────────┐   ┌────────┐     │
│  │  ClaudeAdapter   │     │ ContinueAdapter  │   │  ...   │     │
│  │                  │     │                  │   │        │     │
│  │ Interface:       │     │ Interface:       │   │        │     │
│  │ - GetName()      │     │ - GetName()      │   │        │     │
│  │ - Detect()       │     │ - Detect()       │   │        │     │
│  │ - Setup()        │     │ - Setup()        │   │        │     │
│  │ - GetCommand()   │     │ - GetCommand()   │   │        │     │
│  │ - FormatOutput() │     │ - FormatOutput() │   │        │     │
│  └──────────────────┘     └──────────────────┘   └────────┘     │
│            │                           │              │         │
│            ▼                           ▼              ▼         │
│  ┌──────────────┐         ┌──────────────┐   ┌──────────┐       │
│  │ Claude Code  │         │   Continue   │   │  Cline   │       │
│  │.claude/      │         │.continue/    │   │ .cline/  │       │
│  │settings.json │         │config.json   │   │config.ts │       │
│  └──────────────┘         └──────────────┘   └──────────┘       │
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

    // Configuration Management
    Reload(newConfig IntegrationConfig) error  // Apply config changes without full teardown
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

**Configuration Management**:

- **Source of Truth**: `~/.agentic-memorizer/config.yaml` is the single source of truth
- **Modification Methods**:
  1. **CLI Commands** (Preferred): `integrations update`, `integrations enable/disable`
     - Validates changes before applying
     - Updates config file atomically
     - Applies changes to running daemon (if running)
  2. **Manual Editing** (Advanced): Direct YAML editing + `daemon reload`
     - User edits config file
     - Runs `integrations validate` to check for errors
     - Runs `daemon reload` to apply changes
     - Invalid config preserves current daemon state

- **Reload Behavior**:
  - Daemon loads config once at startup (in-memory)
  - Manual config changes require explicit reload (no file watching)
  - `daemon reload` command validates + applies changes without restart
  - SIGHUP signal triggers same reload mechanism
  - Failed reload preserves current state (no crash)

### 5. Clean Implementation

Start from scratch without legacy constraints:
- No deprecated flags or compatibility shims
- Direct implementation of integration pattern
- Modern, idiomatic Go code following current best practices

### 6. Configuration Change Handling

**Philosophy**: Similar to Vault Enterprise's dual-configuration model, agentic-memorizer supports both **CLI-managed** and **manual configuration editing** with explicit reload.

#### Configuration Change Model

**Hybrid Approach**:
1. **CLI Commands** (Preferred): Safe, validated, guided changes
   - `agentic-memorizer integrations setup <integration>`
   - `agentic-memorizer integrations update <integration> [flags]`
   - `agentic-memorizer integrations enable/disable <integration>`
   - Changes are validated and applied atomically

2. **Manual Editing** (Advanced Users): Direct YAML modification
   - Edit `~/.agentic-memorizer/config.yaml` directly
   - Run `agentic-memorizer integrations validate` to check syntax
   - Run `agentic-memorizer daemon reload` to apply changes
   - All-or-nothing: invalid config keeps current state

#### Configuration State Transitions

| User Action | Config Change | Integration Method Called |
|-------------|---------------|---------------------------|
| Enable integration | `enabled: false` → `true` | `Setup(binaryPath)` |
| Disable integration | `enabled: true` → `false` | `Remove()` |
| Change output format | `output_format: xml` → `markdown` | `Reload(newConfig)` |
| Change settings | `matchers: [...]` modified | `Reload(newConfig)` |
| Binary path change | Command path updated | `Update(binaryPath)` |

#### Reload Behavior

**Daemon Running**:
```bash
# User edits config.yaml manually
vim ~/.agentic-memorizer/config.yaml

# Validate config syntax and logic
agentic-memorizer integrations validate
# → Reports: "Config valid. 1 integration will be reloaded: claude-code"

# Apply changes to running daemon
agentic-memorizer daemon reload
# → Validates config
# → Calls Reload() on changed integrations
# → Returns success or rollback on failure
```

**Daemon Not Running**:
- Changes take effect on next `daemon start`
- No reload needed

**Validation Requirements**:
- Config file must be valid YAML
- Integration types must exist in registry
- Integration-specific settings must be valid (per `Validate()`)
- Atomic: all changes apply or none do

#### Error Handling

**Invalid Config on Reload**:
```bash
$ agentic-memorizer daemon reload
Error: Invalid configuration in ~/.agentic-memorizer/config.yaml:
  - Line 15: Unknown integration type "invalid-type"
  - claude-code.output_format: must be xml, markdown, or json (got "yaml")

Current configuration retained. Fix errors and try again.
```

**Graceful Degradation**:
- If one integration fails to reload, disable it (don't crash daemon)
- Log detailed error for debugging
- Continue serving other integrations

---

## Error Handling Framework

**Philosophy**: Comprehensive, actionable error handling with clear user guidance and automatic recovery where possible.

### 7.1 Error Type Hierarchy

All errors in the integration system follow a structured hierarchy for consistent handling:

```go
// Error categories for classification
type ErrorCategory string

const (
    ErrCategoryConfig      ErrorCategory = "config"       // Configuration errors
    ErrCategoryIntegration ErrorCategory = "integration"  // Integration-specific errors
    ErrCategoryDaemon      ErrorCategory = "daemon"       // Daemon lifecycle errors
    ErrCategoryIO          ErrorCategory = "io"           // File system errors
    ErrCategoryValidation  ErrorCategory = "validation"   // Validation errors
    ErrCategoryNetwork     ErrorCategory = "network"      // Network/API errors
)

// Error severity levels
type ErrorSeverity string

const (
    SeverityFatal    ErrorSeverity = "fatal"    // Unrecoverable, daemon must stop
    SeverityCritical ErrorSeverity = "critical" // Recoverable but disables functionality
    SeverityWarning  ErrorSeverity = "warning"  // Degraded but functional
    SeverityInfo     ErrorSeverity = "info"     // Informational only
)

// Structured error type
type IntegrationError struct {
    Category   ErrorCategory
    Severity   ErrorSeverity
    Integration string        // Which integration (if applicable)
    Operation   string        // What operation failed
    Err         error         // Underlying error
    Retryable   bool          // Can this be retried?
    Suggestion  string        // Actionable suggestion for user
}
```

**Specific Error Types**:

```go
// Configuration errors
var (
    ErrConfigNotFound       = errors.New("configuration file not found")
    ErrConfigInvalid        = errors.New("configuration file invalid")
    ErrConfigCorrupted      = errors.New("configuration file corrupted")
    ErrConfigLocked         = errors.New("configuration file locked")
)

// Integration errors
var (
    ErrIntegrationNotFound    = errors.New("integration not found in registry")
    ErrIntegrationNotDetected = errors.New("integration framework not detected on system")
    ErrIntegrationDisabled    = errors.New("integration is disabled")
    ErrIntegrationFailed      = errors.New("integration operation failed")
    ErrIntegrationConflict    = errors.New("integration configuration conflict")
)

// Binary errors
var (
    ErrBinaryNotFound       = errors.New("binary not found")
    ErrBinaryNotExecutable  = errors.New("binary not executable")
    ErrBinaryInvalid        = errors.New("binary path invalid")
)

// Settings file errors
var (
    ErrSettingsNotFound     = errors.New("settings file not found")
    ErrSettingsCorrupted    = errors.New("settings file corrupted or invalid JSON")
    ErrSettingsPermission   = errors.New("insufficient permissions for settings file")
    ErrSettingsLocked       = errors.New("settings file locked by another process")
)

// State errors
var (
    ErrStateInconsistent    = errors.New("integration state inconsistent")
    ErrStateCorrupted       = errors.New("state file corrupted")
    ErrStateRollbackFailed  = errors.New("state rollback failed")
)
```

### 7.2 Retry Policies

Different error types have different retry strategies:

| Error Type | Retry Count | Backoff Strategy | Max Wait |
|------------|-------------|------------------|----------|
| **Transient I/O** | 3 | Exponential (1s, 2s, 4s) | 7s |
| **File Locked** | 5 | Linear (500ms intervals) | 2.5s |
| **Network/API** | 5 | Exponential (2s, 4s, 8s, 16s, 32s) | 62s |
| **Config Read** | 2 | Linear (1s interval) | 2s |
| **Validation** | 0 | No retry (permanent) | N/A |
| **Integration Setup** | 2 | Linear (2s interval) | 4s |

**Implementation**:

```go
func RetryWithBackoff(operation func() error, policy RetryPolicy) error {
    var lastErr error

    for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
        if attempt > 0 {
            wait := policy.CalculateWait(attempt)
            time.Sleep(wait)
        }

        err := operation()
        if err == nil {
            return nil // Success
        }

        lastErr = err

        // Don't retry permanent errors
        if !isRetryable(err) {
            return err
        }
    }

    return fmt.Errorf("operation failed after %d retries: %w", policy.MaxRetries, lastErr)
}
```

### 7.3 Rollback Procedures

When operations fail, automatic rollback ensures system consistency:

#### Integration Setup Rollback

**Failure Point** → **Rollback Actions**:

1. **During external config write** (e.g., settings.json):
   - Restore settings file from backup
   - Remove partially written hooks
   - Mark integration as setup_failed

2. **During integration config write** (agentic-memorizer config.yaml):
   - Don't persist integration config
   - Mark integration as disabled
   - Log setup failure

3. **During validation**:
   - No rollback needed (no changes made yet)
   - Return validation errors to user

**Rollback Implementation**:

```go
func (a *ClaudeCodeAdapter) Setup(binaryPath string) error {
    // Create backup before making changes
    backup, err := a.backupSettings()
    if err != nil {
        return fmt.Errorf("failed to backup settings: %w", err)
    }
    defer backup.Cleanup() // Remove backup on success

    // Perform setup operations
    if err := a.setupSessionStartHooks(binaryPath); err != nil {
        // Rollback: restore from backup
        if restoreErr := backup.Restore(); restoreErr != nil {
            return fmt.Errorf("setup failed and rollback failed: %v (original error: %w)", restoreErr, err)
        }
        return fmt.Errorf("setup failed, changes rolled back: %w", err)
    }

    return nil
}
```

#### Config Reload Rollback

**On reload failure**:
- Keep in-memory config unchanged
- Don't persist new config to disk
- Log detailed error with line numbers
- Return error to user with actionable message

**No partial application**: Either all integrations reload successfully, or none do (atomic operation).

#### Integration Removal Rollback

**If Remove() fails**:
1. Retry operation once (after 2s delay)
2. If still fails: Mark integration state as "removal_failed"
3. Log detailed error for manual intervention
4. Keep integration in config but disabled
5. Health check will report failed removal

**Manual recovery command**:
```bash
agentic-memorizer integrations recover claude-code
```

### 7.4 Error Messages and User Guidance

All error messages follow this format:

```
❌ {Operation} failed: {Brief Description}

Details:
  Integration: {integration_name}
  Error: {specific_error}
  Location: {file:line or component}

Suggestion:
  {Actionable steps to fix the issue}

For more help: agentic-memorizer integrations health
```

**Examples**:

```bash
# Example 1: Config validation error
❌ Configuration reload failed: Invalid integration settings

Details:
  Integration: claude-code
  Error: output_format "yaml" is not supported
  Location: ~/.agentic-memorizer/config.yaml:15

Suggestion:
  Change output_format to one of: xml, markdown, json

  Valid configuration:
    integrations:
      claude-code:
        output_format: "xml"  # or "markdown" or "json"

For more help: agentic-memorizer integrations health claude-code
```

```bash
# Example 2: Binary not found
❌ Integration setup failed: Binary not found

Details:
  Integration: claude-code
  Error: binary path does not exist
  Location: /usr/local/bin/agentic-memorizer

Suggestion:
  1. Check if agentic-memorizer is installed:
     which agentic-memorizer

  2. If installed, update config with correct path:
     agentic-memorizer integrations update claude-code --binary-path $(which agentic-memorizer)

  3. If not installed, install agentic-memorizer first

For more help: agentic-memorizer --help
```

```bash
# Example 3: Settings file corrupted
❌ Integration health check failed: Settings file corrupted

Details:
  Integration: claude-code
  Error: invalid JSON in ~/.claude/settings.json at line 42
  Location: ~/.claude/settings.json

Suggestion:
  1. A backup was created: ~/.claude/settings.json.backup.2025-10-31

  2. Fix the JSON syntax error at line 42, or

  3. Restore from backup:
     cp ~/.claude/settings.json.backup.2025-10-31 ~/.claude/settings.json

  4. Re-run setup:
     agentic-memorizer integrations setup claude-code

For more help: agentic-memorizer integrations health claude-code
```

### 7.5 Error Handling Per Operation

#### Integration.Setup()
- **On validation error**: Return immediately with detailed validation message
- **On external config error**: Backup, attempt write, rollback on failure
- **On success**: Persist integration config, mark as enabled

#### Integration.Reload()
- **On config diff detection**: Calculate what changed
- **On validation error**: Keep current config, log error
- **On apply error**: Attempt rollback to previous integration state
- **On success**: Update in-memory config

#### Integration.Remove()
- **On remove error**: Retry once after 2s
- **On persistent failure**: Mark as "removal_failed", require manual intervention
- **On success**: Remove from config, clean up external files

#### Daemon Reload
- **On config read error**: Keep running with current config
- **On validation error**: Return errors, don't apply any changes
- **On partial integration failure**: Disable failed integrations, continue with successful ones
- **On success**: All integrations reloaded, config updated

### 7.6 Crash Recovery

**On daemon crash during operations**:

1. **Detection**: On next daemon start, check for `.in-progress` marker files
2. **Analysis**: Determine which operation was in-progress
3. **Recovery**:
   - **During reload**: Restore from backup if exists, use previous config
   - **During setup**: Clean up partial setup, mark integration as failed
   - **During remove**: Complete removal or mark as removal_failed

**Implementation**:

```go
func (d *Daemon) Start() error {
    // Check for crash recovery needed
    if markers, err := d.findInProgressMarkers(); err == nil && len(markers) > 0 {
        d.logger.Warn("Detected incomplete operations from previous run")
        for _, marker := range markers {
            if err := d.recoverOperation(marker); err != nil {
                d.logger.Error("Recovery failed", "operation", marker.Operation, "error", err)
            }
        }
    }

    // Continue normal startup...
}
```

### 7.7 Logging Strategy

**Log Levels**:
- **ERROR**: Operation failures, rollbacks, unrecoverable errors
- **WARN**: Retryable errors, degraded functionality, failed health checks
- **INFO**: Successful operations, state changes, configuration reloads
- **DEBUG**: Detailed operation traces, validation steps, cache operations

**Structured Logging Fields**:
```go
logger.Error("integration setup failed",
    "integration", "claude-code",
    "operation", "setup",
    "error", err,
    "retry_count", retryCount,
    "rollback_status", "success",
)
```

### 7.8 Health Check Integration

Errors trigger health check state changes:

| Error Severity | Health Check Status | Action |
|----------------|---------------------|--------|
| **Fatal** | unhealthy, disabled | Disable integration, notify user |
| **Critical** | degraded | Mark degraded, continue operation |
| **Warning** | healthy with warnings | Log warning, no action |
| **Info** | healthy | Log info, no action |

**Health check includes**:
- Last error timestamp
- Error count in last hour
- Current state (healthy, degraded, failed)
- Suggested recovery actions

---

## Multi-Phase Implementation Plan

### Phase 1: Foundation & Abstraction Layer

**Goal**: Create the core abstraction layer for integration support.

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
- Implement XML formatting logic as `XMLProcessor`
- Implement Markdown formatting logic as `MarkdownProcessor`
- **NEW**: Create JSON output processor to render index as JSON (this is a new output format)
  - Note: The index is stored on disk as JSON, but we don't currently have a JSON output format
  - This will be a human-readable/agent-readable JSON representation of the index
  - Different from the storage format (may include pretty-printing, filtering, etc.)
- Create modular processors that work independently of integration wrappers
- Remove old `internal/output/formatter.go` code - clean implementation only

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
    Enabled: []string{},  // Empty by default - user chooses during init
    Configs: map[string]IntegrationConfig{
        // Configs added during interactive setup
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
- [ ] Output processors implemented and tested
- [ ] Configuration schema extended
- [ ] All new code passes tests and follows Go style guide

---

### Phase 2: Claude Code Adapter

**Goal**: Implement Claude Code adapter for SessionStart hook integration.

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
    // Fresh implementation of SessionStart hook setup
    // Read settings, add hooks for matchers, write settings
    return a.setupSessionStartHooks(binaryPath)
}

func (a *ClaudeCodeAdapter) GetCommand(binaryPath string, format OutputFormat) string {
    return fmt.Sprintf("%s read --format %s --integration claude-code", binaryPath, format)
}

func (a *ClaudeCodeAdapter) FormatOutput(index *types.Index, format OutputFormat) (string, error) {
    // Fresh implementation of SessionStart JSON wrapper
    // Use output processor for content, wrap in SessionStart envelope
    return formatSessionStartJSON(index, format)
}
```

**Implementation Notes**:
- Implement fresh SessionStart hook logic
- Follow Go best practices and current idioms
- Clean, well-documented code

#### 2.2 Create Settings Manager for Claude Code

**New File**: `internal/integrations/adapters/claude/settings.go`

Implement fresh settings management:
- `ReadSettings()` - parse `~/.claude/settings.json`
- `WriteSettings()` - atomic write with backup
- `SetupSessionStartHooks()` - add hooks for 4 matchers (startup, resume, clear, compact)

Use modern JSON handling, clean implementation.

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
    // Fresh implementation of SessionStart JSON wrapper

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

#### 2.6 Read Command Integration

**Direct Implementation**:
```go
// cmd/read/read.go
var integrationName string

func init() {
    readCmd.Flags().StringVar(&integrationName, "integration", "",
        "Integration to format output for (claude-code, continue, cline, etc.)")
}

func run() error {
    // Direct implementation - no legacy compatibility layer
    if integrationName != "" {
        return outputForIntegration(integrationName, computed.Index, format)
    }

    // Default: plain formatted output
    return outputDefault(computed.Index, format)
}
```

**Examples**:
```bash
# Claude Code SessionStart hook
agentic-memorizer read --integration claude-code

# Continue.dev
agentic-memorizer read --format markdown --integration continue

# Plain output (no integration wrapper)
agentic-memorizer read --format xml
```

#### 2.7 Integration Testing

**Test Scenarios**:
1. Fresh install with `agentic-memorizer init`
2. Fresh install on system with multiple frameworks
3. `agentic-memorizer read --integration claude-code` produces valid SessionStart JSON
4. Claude Code SessionStart hooks trigger correctly
5. Output format matches SessionStart JSON specification
6. All 4 matchers work (startup, resume, clear, compact)

#### 2.8 Configuration Reload

**New File**: `internal/integrations/adapters/claude/reload.go`

Implement configuration reload logic for Claude Code adapter:

```go
func (a *ClaudeCodeAdapter) Reload(newConfig IntegrationConfig) error {
    // Determine what changed
    changes := a.detectChanges(newConfig)

    if changes.OutputFormatChanged {
        // Output format change doesn't affect hooks, just runtime behavior
        a.outputFormat = newConfig.OutputFormat
    }

    if changes.MatchersChanged {
        // Matchers changed - update hooks in settings.json
        return a.updateMatchers(newConfig.Settings["matchers"].([]string))
    }

    if changes.SettingsPathChanged {
        // Settings path changed - re-setup from scratch
        return a.Setup(a.binaryPath)
    }

    return nil
}

func (a *ClaudeCodeAdapter) detectChanges(newConfig IntegrationConfig) ConfigChanges {
    // Compare current config with new config
    // Return struct indicating what changed
    // This avoids unnecessary updates
}

func (a *ClaudeCodeAdapter) updateMatchers(newMatchers []string) error {
    settings, err := a.ReadSettings()
    if err != nil {
        return err
    }

    // Remove old hooks for removed matchers
    // Add new hooks for added matchers
    // Update hooks for existing matchers

    return a.WriteSettings(settings)
}
```

**Key Features**:
- Detect which configuration values changed
- Update only what's necessary (don't rebuild everything)
- Preserve other hooks in settings.json (don't clobber unrelated hooks)
- Atomic updates (all-or-nothing)
- Return detailed errors if reload fails

**Deliverables**:
- [ ] Claude Code adapter fully implemented
- [ ] Settings management implemented
- [ ] Output formatting implemented
- [ ] Configuration reload logic implemented
- [ ] Read command updated with `--integration` flag
- [ ] Init command uses integration registry
- [ ] All tests pass and follow Go style guide
- [ ] Integration tests verify SessionStart hooks work correctly

---

### Phase 3: Additional Integrations

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

var validateCmd = &cobra.Command{
    Use:   "validate",
    Short: "Validate configuration file",
    Run:   runValidate,
}

var updateCmd = &cobra.Command{
    Use:   "update [integration]",
    Short: "Update integration settings",
    Args:  cobra.ExactArgs(1),
    Run:   runUpdate,
}

var enableCmd = &cobra.Command{
    Use:   "enable [integration]",
    Short: "Enable an integration",
    Args:  cobra.ExactArgs(1),
    Run:   runEnable,
}

var disableCmd = &cobra.Command{
    Use:   "disable [integration]",
    Short: "Disable an integration",
    Args:  cobra.ExactArgs(1),
    Run:   runDisable,
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

func runValidate(cmd *cobra.Command, args []string) {
    // Load config file
    cfg, err := config.LoadConfig()
    if err != nil {
        fmt.Printf("❌ Config file error: %v\n", err)
        os.Exit(1)
    }

    // Validate each integration configuration
    registry := integrations.GlobalRegistry()
    errors := []string{}

    for name, intConfig := range cfg.Integrations.Configs {
        integration, err := registry.Get(name)
        if err != nil {
            errors = append(errors, fmt.Sprintf("%s: integration type not found", name))
            continue
        }

        if err := integration.Validate(); err != nil {
            errors = append(errors, fmt.Sprintf("%s: %v", name, err))
        }
    }

    if len(errors) > 0 {
        fmt.Println("❌ Configuration validation failed:")
        for _, e := range errors {
            fmt.Printf("  - %s\n", e)
        }
        os.Exit(1)
    }

    fmt.Println("✓ Configuration is valid")
}

func runUpdate(cmd *cobra.Command, args []string) {
    integrationName := args[0]

    // Get flags (--output-format, --enabled, etc.)
    outputFormat, _ := cmd.Flags().GetString("output-format")

    // Load config
    cfg, _ := config.GetConfig()
    intConfig := cfg.Integrations.Configs[integrationName]

    // Update config values
    if outputFormat != "" {
        intConfig.OutputFormat = outputFormat
    }

    // Save config
    cfg.Integrations.Configs[integrationName] = intConfig
    config.WriteConfig(cfg)

    // Reload integration if daemon is running
    if isDaemonRunning() {
        reloadIntegration(integrationName, intConfig)
    }

    fmt.Printf("✓ %s integration updated\n", integrationName)
}

func runEnable(cmd *cobra.Command, args []string) {
    integrationName := args[0]

    cfg, _ := config.GetConfig()
    intConfig := cfg.Integrations.Configs[integrationName]
    intConfig.Enabled = true
    cfg.Integrations.Configs[integrationName] = intConfig

    // Add to enabled list if not present
    if !contains(cfg.Integrations.Enabled, integrationName) {
        cfg.Integrations.Enabled = append(cfg.Integrations.Enabled, integrationName)
    }

    config.WriteConfig(cfg)

    // Setup integration if daemon is running
    if isDaemonRunning() {
        registry := integrations.GlobalRegistry()
        integration, _ := registry.Get(integrationName)
        binaryPath, _ := findBinaryPath()
        integration.Setup(binaryPath)
    }

    fmt.Printf("✓ %s integration enabled\n", integrationName)
}

func runDisable(cmd *cobra.Command, args []string) {
    integrationName := args[0]

    cfg, _ := config.GetConfig()
    intConfig := cfg.Integrations.Configs[integrationName]
    intConfig.Enabled = false
    cfg.Integrations.Configs[integrationName] = intConfig

    // Remove from enabled list
    cfg.Integrations.Enabled = removeString(cfg.Integrations.Enabled, integrationName)

    config.WriteConfig(cfg)

    // Remove integration if daemon is running
    if isDaemonRunning() {
        registry := integrations.GlobalRegistry()
        integration, _ := registry.Get(integrationName)
        integration.Remove()
    }

    fmt.Printf("✓ %s integration disabled\n", integrationName)
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
- [ ] Integration management commands working (list, detect, setup, remove, validate, update, enable, disable)
- [ ] Reload() method implemented for all adapters
- [ ] Documentation for each integration
- [ ] Tests for all new adapters

---

### Phase 4: Documentation & User Experience

**Goal**: Comprehensive documentation and excellent user experience for new implementation.

#### 4.1 Core Documentation

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

3. **`docs/architecture.md`** - Architecture documentation
   - Adapter pattern explanation
   - Integration interface specification
   - Output format documentation
   - How to create custom adapters

4. **`CHANGELOG.md`** - Document new features
   - Integration system implementation
   - New features and capabilities
   - Usage examples

#### 4.2 Configuration Examples

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

#### 4.3 Configuration Change Workflows

Document both CLI and manual editing workflows for users.

**Workflow 1: CLI-Based Configuration (Recommended)**

```bash
# List all available integrations
agentic-memorizer integrations list
# Output:
# Available Integrations:
#   - claude-code: Claude Code SessionStart hook integration (configured)
#   - continue: Continue.dev custom tool integration (not configured)
#   - cline: Cline integration (not configured)

# Enable a new integration
agentic-memorizer integrations setup continue
# Output:
# Setting up Continue.dev integration...
# ✓ Continue.dev integration configured

# Update integration settings
agentic-memorizer integrations update claude-code --output-format markdown
# Output:
# ✓ claude-code integration updated
# Reloading daemon configuration...
# ✓ Configuration reloaded successfully

# Disable an integration
agentic-memorizer integrations disable cline
# Output:
# ✓ cline integration disabled

# Check integration health
agentic-memorizer integrations health
# Output:
# Checking integration health...
#
# ✓ claude-code: healthy
# ✓ continue: healthy
```

**Workflow 2: Manual Configuration Editing (Advanced)**

```bash
# Step 1: Edit configuration file
vim ~/.agentic-memorizer/config.yaml

# Make changes to integrations section:
#   - Change claude-code output_format from xml to markdown
#   - Enable cline integration
#   - Add new matcher to claude-code

# Step 2: Validate configuration before applying
agentic-memorizer integrations validate
# Output:
# ✓ Configuration is valid
# Changes detected:
#   - claude-code: output_format changed (xml → markdown)
#   - claude-code: matchers changed (4 → 5)
#   - cline: will be enabled

# Step 3: Apply changes to running daemon
agentic-memorizer daemon reload
# Output:
# Reloading daemon configuration...
# ✓ claude-code: configuration reloaded
# ✓ cline: integration enabled
# ✓ Configuration reloaded successfully

# Alternative: If validation fails
agentic-memorizer integrations validate
# Output:
# ❌ Configuration validation failed:
#   - claude-code.output_format: must be xml, markdown, or json (got "yaml")
#   - cline: integration type not found in registry
#
# Fix errors and try again.

# After fixing errors, validate again
agentic-memorizer integrations validate
# Output:
# ✓ Configuration is valid

# Then reload
agentic-memorizer daemon reload
# Output:
# ✓ Configuration reloaded successfully
```

**Workflow 3: Using SIGHUP Signal (Unix)**

```bash
# Edit configuration file
vim ~/.agentic-memorizer/config.yaml

# Send SIGHUP signal to daemon (triggers reload)
kill -HUP $(cat ~/.agentic-memorizer/daemon.pid)

# Check daemon logs to verify reload
tail ~/.agentic-memorizer/logs/daemon.log
# Output:
# [INFO] Received reload signal
# [INFO] Loading new configuration
# [INFO] Validating configuration
# [INFO] Reloading integrations
# [INFO] claude-code: configuration reloaded
# [INFO] Configuration reload complete
```

**Error Handling Examples**:

```bash
# Scenario 1: Invalid YAML syntax
agentic-memorizer daemon reload
# Output:
# ❌ Failed to load config: yaml: line 15: could not find expected ':'
# Current configuration retained.

# Scenario 2: Invalid integration settings
agentic-memorizer integrations validate
# Output:
# ❌ Configuration validation failed:
#   - claude-code: settings_path does not exist: /invalid/path/settings.json
#   - continue.output_format: must be xml, markdown, or json (got "html")

# Scenario 3: Integration reload failure (daemon continues with other integrations)
agentic-memorizer daemon reload
# Output:
# Reloading daemon configuration...
# ✓ claude-code: configuration reloaded
# ⚠ continue: reload failed: config.json not found
#   → continue integration disabled
# ✓ cline: configuration reloaded
# ⚠ Configuration reload completed with warnings
```

**Comparison: CLI vs Manual Editing**

| Aspect | CLI Commands | Manual Editing + Reload |
|--------|--------------|-------------------------|
| **Safety** | Validates before applying | User must remember to validate |
| **Ease of Use** | Guided, flags for options | Requires knowledge of schema |
| **Flexibility** | Limited to defined flags | Full control over all settings |
| **Best For** | Common changes, beginners | Complex changes, power users |
| **Example Use Case** | Change output format, enable/disable | Bulk changes, custom settings |

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
- [ ] README.md updated
- [ ] Integration guides created for all supported frameworks
- [ ] Architecture documentation complete
- [ ] Configuration examples provided
- [ ] CLI help text improved
- [ ] Testing documentation created
- [ ] Contributing guide with integration instructions
- [ ] CHANGELOG updated

---

### Phase 5: Polish & Release

**Goal**: Prepare for production release with monitoring, error handling, and user experience improvements.

#### 5.1 Enhanced Error Handling

**Implementation Note**: This phase implements the comprehensive Error Handling Framework defined in **Section 7: Error Handling Framework**.

**Key Implementation Tasks**:

1. **Create Error Type Hierarchy** (`internal/integrations/errors.go`):
   - Implement all error types from Section 7.1
   - Add `ErrorCategory` and `ErrorSeverity` enums
   - Create `IntegrationError` struct with all required fields

2. **Implement Retry Policies** (`internal/integrations/retry.go`):
   - Create `RetryPolicy` type with backoff strategies
   - Implement `RetryWithBackoff()` function
   - Add retry policies for each error type (see Section 7.2 table)
   - Implement `isRetryable()` to determine if error supports retry

3. **Implement Rollback Mechanisms**:
   - Add backup functionality to all adapters (see Section 7.3)
   - Implement transaction-style operations for Setup/Reload/Remove
   - Add `.in-progress` marker files for crash recovery
   - Implement rollback procedures for each integration adapter

4. **Implement Structured Error Messages**:
   - Follow error message format from Section 7.4
   - Include actionable suggestions in all errors
   - Add context (file:line, integration name, operation)
   - Reference health check commands in error output

5. **Implement Crash Recovery** (`internal/daemon/recovery.go`):
   - Detect incomplete operations on daemon start (Section 7.6)
   - Implement recovery procedures for each operation type
   - Add recovery markers and cleanup logic

6. **Integrate with Health Checks**:
   - Map error severity to health check status (Section 7.8)
   - Track error counts and timestamps
   - Update health status based on errors

**Testing Requirements**:
- Test all error types with appropriate messages
- Test retry policies with various backoff strategies
- Test rollback procedures (force failures at each step)
- Test crash recovery (kill daemon mid-operation)
- Verify error messages match format specification
- Test error severity mapping to health checks

**Example Implementation**:

```go
// internal/integrations/errors.go - Implement Section 7.1

package integrations

import "errors"

type ErrorCategory string

const (
    ErrCategoryConfig      ErrorCategory = "config"
    ErrCategoryIntegration ErrorCategory = "integration"
    ErrCategoryDaemon      ErrorCategory = "daemon"
    ErrCategoryIO          ErrorCategory = "io"
    ErrCategoryValidation  ErrorCategory = "validation"
    ErrCategoryNetwork     ErrorCategory = "network"
)

type ErrorSeverity string

const (
    SeverityFatal    ErrorSeverity = "fatal"
    SeverityCritical ErrorSeverity = "critical"
    SeverityWarning  ErrorSeverity = "warning"
    SeverityInfo     ErrorSeverity = "info"
)

type IntegrationError struct {
    Category    ErrorCategory
    Severity    ErrorSeverity
    Integration string
    Operation   string
    Err         error
    Retryable   bool
    Suggestion  string
}

func (e *IntegrationError) Error() string {
    return fmt.Sprintf("❌ %s failed: %s\n\nDetails:\n  Integration: %s\n  Error: %v\n\nSuggestion:\n  %s",
        e.Operation, e.Err.Error(), e.Integration, e.Err, e.Suggestion)
}

func (e *IntegrationError) Unwrap() error {
    return e.Err
}

// All error types from Section 7.1
var (
    ErrConfigNotFound       = errors.New("configuration file not found")
    ErrConfigInvalid        = errors.New("configuration file invalid")
    ErrConfigCorrupted      = errors.New("configuration file corrupted")
    ErrConfigLocked         = errors.New("configuration file locked")
    ErrIntegrationNotFound  = errors.New("integration not found in registry")
    ErrIntegrationNotDetected = errors.New("integration framework not detected on system")
    ErrIntegrationDisabled  = errors.New("integration is disabled")
    ErrIntegrationFailed    = errors.New("integration operation failed")
    ErrIntegrationConflict  = errors.New("integration configuration conflict")
    ErrBinaryNotFound       = errors.New("binary not found")
    ErrBinaryNotExecutable  = errors.New("binary not executable")
    ErrBinaryInvalid        = errors.New("binary path invalid")
    ErrSettingsNotFound     = errors.New("settings file not found")
    ErrSettingsCorrupted    = errors.New("settings file corrupted or invalid JSON")
    ErrSettingsPermission   = errors.New("insufficient permissions for settings file")
    ErrSettingsLocked       = errors.New("settings file locked by another process")
    ErrStateInconsistent    = errors.New("integration state inconsistent")
    ErrStateCorrupted       = errors.New("state file corrupted")
    ErrStateRollbackFailed  = errors.New("state rollback failed")
)
```

**Reference**: See **Section 7: Error Handling Framework** for complete specifications of:
- Error type hierarchy (7.1)
- Retry policies (7.2)
- Rollback procedures (7.3)
- Error message format (7.4)
- Error handling per operation (7.5)
- Crash recovery (7.6)
- Logging strategy (7.7)
- Health check integration (7.8)

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
3. Switching between integrations
4. Running multiple integrations simultaneously
5. Daemon restart with integrations active
6. Error recovery (invalid config, missing binary, etc.)
7. All output formats (XML, Markdown, JSON) work correctly
8. Integration-specific wrappers work correctly
9. **NEW**: Configuration reload without daemon restart
10. **NEW**: Config validation catches errors before applying
11. **NEW**: Invalid config reload preserves current state

#### 5.9 Daemon Reload Mechanism

**Goal**: Implement configuration reload without daemon restart, similar to Vault Enterprise's SIGHUP mechanism.

**New Command**: `cmd/daemon/reload.go`

```go
var reloadCmd = &cobra.Command{
    Use:   "reload",
    Short: "Reload daemon configuration without restart",
    Long: `Reload the daemon's configuration file and apply changes to integrations.

This command validates the new configuration before applying it. If validation
fails, the current configuration is retained and the daemon continues running.

Changes applied:
- Integration enable/disable state
- Integration settings (output format, matchers, etc.)
- Other configuration values

Note: Some settings may require a full daemon restart (e.g., memory_root path).`,
    Run: runReload,
}

func runReload(cmd *cobra.Command, args []string) {
    // Check if daemon is running
    if !isDaemonRunning() {
        fmt.Println("❌ Daemon is not running")
        fmt.Println("Start the daemon with: agentic-memorizer daemon start")
        os.Exit(1)
    }

    // Load new configuration from disk
    newConfig, err := config.LoadConfig()
    if err != nil {
        fmt.Printf("❌ Failed to load config: %v\n", err)
        fmt.Println("Current configuration retained.")
        os.Exit(1)
    }

    // Validate new configuration
    if err := validateConfig(newConfig); err != nil {
        fmt.Printf("❌ Configuration validation failed: %v\n", err)
        fmt.Println("Current configuration retained.")
        os.Exit(1)
    }

    // Send reload signal to daemon
    if err := signalDaemonReload(); err != nil {
        fmt.Printf("❌ Failed to signal daemon: %v\n", err)
        os.Exit(1)
    }

    fmt.Println("✓ Configuration reloaded successfully")
}
```

**Daemon Reload Handler**:

```go
// internal/daemon/reload.go

func (d *Daemon) HandleReload() error {
    d.logger.Info("Received reload signal")

    // Load new config
    newConfig, err := config.LoadConfig()
    if err != nil {
        d.logger.Error("Failed to load new config", "error", err)
        return err
    }

    // Validate new config
    if err := d.validateConfig(newConfig); err != nil {
        d.logger.Error("Invalid config", "error", err)
        return err
    }

    // Apply integration changes
    if err := d.reloadIntegrations(newConfig.Integrations); err != nil {
        d.logger.Error("Failed to reload integrations", "error", err)
        // Don't return error - partial reload is acceptable
    }

    // Update in-memory config
    d.config = newConfig

    d.logger.Info("Configuration reload complete")
    return nil
}

func (d *Daemon) reloadIntegrations(newIntegrations IntegrationsConfig) error {
    registry := integrations.GlobalRegistry()
    currentIntegrations := d.config.Integrations
    errors := []error{}

    // Compare old and new integrations
    for name, newIntConfig := range newIntegrations.Configs {
        oldIntConfig, existed := currentIntegrations.Configs[name]

        // New integration enabled
        if !existed && newIntConfig.Enabled {
            if err := d.enableIntegration(name, newIntConfig); err != nil {
                errors = append(errors, fmt.Errorf("%s: enable failed: %w", name, err))
            }
            continue
        }

        // Existing integration disabled
        if existed && !newIntConfig.Enabled && oldIntConfig.Enabled {
            if err := d.disableIntegration(name); err != nil {
                errors = append(errors, fmt.Errorf("%s: disable failed: %w", name, err))
            }
            continue
        }

        // Integration settings changed
        if existed && configChanged(oldIntConfig, newIntConfig) {
            integration, _ := registry.Get(name)
            if err := integration.Reload(newIntConfig); err != nil {
                errors = append(errors, fmt.Errorf("%s: reload failed: %w", name, err))
                // Disable failed integration
                d.disableIntegration(name)
            }
        }
    }

    // Handle removed integrations
    for name := range currentIntegrations.Configs {
        if _, exists := newIntegrations.Configs[name]; !exists {
            d.disableIntegration(name)
        }
    }

    if len(errors) > 0 {
        return fmt.Errorf("integration reload errors: %v", errors)
    }

    return nil
}

func (d *Daemon) enableIntegration(name string, config IntegrationConfig) error {
    registry := integrations.GlobalRegistry()
    integration, err := registry.Get(name)
    if err != nil {
        return err
    }

    binaryPath, _ := findBinaryPath()
    if err := integration.Setup(binaryPath); err != nil {
        return err
    }

    d.logger.Info("Integration enabled", "integration", name)
    return nil
}

func (d *Daemon) disableIntegration(name string) error {
    registry := integrations.GlobalRegistry()
    integration, err := registry.Get(name)
    if err != nil {
        return err
    }

    if err := integration.Remove(); err != nil {
        return err
    }

    d.logger.Info("Integration disabled", "integration", name)
    return nil
}

func configChanged(old, new IntegrationConfig) bool {
    return old.OutputFormat != new.OutputFormat ||
           old.Type != new.Type ||
           !reflect.DeepEqual(old.Settings, new.Settings)
}
```

**Signal Handling**:

```go
// cmd/daemon/start.go (existing file, add signal handler)

func (d *Daemon) Run(ctx context.Context) error {
    // ... existing daemon setup ...

    // Setup signal handling
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

    for {
        select {
        case sig := <-sigChan:
            switch sig {
            case syscall.SIGHUP:
                // Reload configuration
                if err := d.HandleReload(); err != nil {
                    d.logger.Error("Reload failed", "error", err)
                }
            case syscall.SIGINT, syscall.SIGTERM:
                // Graceful shutdown
                return d.Shutdown()
            }
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}
```

**Configuration Validation Framework**:

#### Comprehensive Validation Rules

All configuration values are validated before being applied to the daemon. Validation occurs in multiple phases:

**Phase 1: YAML Syntax Validation**
- Valid YAML structure
- Proper indentation
- Quoted strings where required
- No duplicate keys

**Phase 2: Schema Validation**
- All required fields present
- Field types correct (string, int, bool, array, map)
- No unknown fields (warn only, don't error)

**Phase 3: Value Validation**
- Values within acceptable ranges
- Enums match allowed values
- Paths exist and are accessible

**Phase 4: Cross-Field Validation**
- Dependencies between fields satisfied
- No conflicting settings
- Logical consistency checks

**Phase 5: Integration-Specific Validation**
- Integration types exist in registry
- Integration-specific settings valid
- External resources accessible (settings files, binaries)

#### Core Configuration Validation Rules

| Field | Type | Required | Validation Rules | Default |
|-------|------|----------|------------------|---------|
| `memory_root` | string | Yes | Must exist, must be directory, must be readable, absolute path, not symlink to system dirs (/etc, /usr, /bin) | N/A |
| `claude.api_key` | string | Conditional | Required if `analysis.enabled=true`, non-empty, valid format (starts with `sk-ant-`) | "" |
| `claude.model` | string | No | Must be valid Claude model name | "claude-3-5-sonnet-20241022" |
| `claude.max_tokens` | int | No | Must be > 0 and ≤ 8192 | 2000 |
| `output.format` | string | No | One of: "xml", "markdown", "json" | "xml" |
| `analysis.enabled` | bool | No | N/A | true |
| `analysis.rate_limit_per_min` | int | No | Must be > 0 and ≤ 10000 | 5 |
| `daemon.workers` | int | No | Must be > 0 and ≤ 100 | 3 |
| `daemon.file_watch_enabled` | bool | No | N/A | true |
| `daemon.debounce_ms` | int | No | Must be ≥ 100 and ≤ 60000 | 2000 |

#### Integration Configuration Validation Rules

**Common Fields (All Integrations)**:

| Field | Type | Required | Validation Rules |
|-------|------|----------|------------------|
| `type` | string | Yes | Must match integration name, must exist in registry |
| `enabled` | bool | No | N/A |
| `output_format` | string | No | One of: "xml", "markdown", "json" |

**Claude Code Integration (`integrations.claude-code`)**:

| Field | Type | Required | Validation Rules |
|-------|------|----------|------------------|
| `settings.settings_path` | string | No | If specified: must exist, must be readable/writable, must be valid JSON, must contain valid Claude settings structure |
| `settings.matchers` | array[string] | No | Each element must be valid matcher name: "startup", "resume", "clear", "compact", "create_project" |

**Default**: `~/.claude/settings.json`

**Validation checks**:
- Settings file exists and is accessible
- File is valid JSON
- File is readable and writable by current user
- File permissions are safe (not world-writable)

**Continue.dev Integration (`integrations.continue`)**:

| Field | Type | Required | Validation Rules |
|-------|------|----------|------------------|
| `settings.config_path` | string | No | If specified: must exist, must be readable/writable, must be valid JSON or TypeScript config |

**Default**: `~/.continue/config.json` or `~/.continue/config.ts`

**Generic Integration (`integrations.generic`)**:

| Field | Type | Required | Validation Rules |
|-------|------|----------|------------------|
| `settings.name` | string | Yes | Non-empty, alphanumeric with hyphens |

#### Validation Implementation

```go
// internal/config/validate.go (new file)

package config

import (
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "strings"
)

// ValidateConfig performs comprehensive validation of configuration
func ValidateConfig(cfg *Config) error {
    validator := NewValidator()

    // Phase 1: YAML syntax (already validated by unmarshaling)

    // Phase 2: Schema validation
    validator.ValidateSchema(cfg)

    // Phase 3: Value validation
    validator.ValidateValues(cfg)

    // Phase 4: Cross-field validation
    validator.ValidateCrossFields(cfg)

    // Phase 5: Integration-specific validation
    validator.ValidateIntegrations(cfg)

    return validator.Errors()
}

type Validator struct {
    errors []ValidationError
}

type ValidationError struct {
    Field      string
    Value      interface{}
    Rule       string
    Message    string
    Suggestion string
}

func (v *Validator) ValidateSchema(cfg *Config) {
    // Required fields
    if cfg.MemoryRoot == "" {
        v.AddError("memory_root", "", "required", "memory_root is required", "Add memory_root: /path/to/memory")
    }
}

func (v *Validator) ValidateValues(cfg *Config) {
    // Memory root validation
    if cfg.MemoryRoot != "" {
        if !filepath.IsAbs(cfg.MemoryRoot) {
            v.AddError("memory_root", cfg.MemoryRoot, "absolute_path",
                "memory_root must be an absolute path",
                "Use absolute path like /Users/name/memory instead of ~/memory")
        }

        info, err := os.Stat(cfg.MemoryRoot)
        if err != nil {
            v.AddError("memory_root", cfg.MemoryRoot, "exists",
                fmt.Sprintf("memory_root does not exist: %v", err),
                "Create the directory: mkdir -p " + cfg.MemoryRoot)
        } else if !info.IsDir() {
            v.AddError("memory_root", cfg.MemoryRoot, "is_directory",
                "memory_root must be a directory",
                "Specify a directory path, not a file")
        }

        // Check for dangerous system directories
        dangerousPaths := []string{"/", "/etc", "/usr", "/bin", "/sbin", "/System"}
        for _, dangerous := range dangerousPaths {
            if strings.HasPrefix(cfg.MemoryRoot, dangerous) {
                v.AddError("memory_root", cfg.MemoryRoot, "safe_path",
                    "memory_root cannot be a system directory",
                    "Use a user directory like ~/agentic-memory")
            }
        }
    }

    // Claude API key validation
    if cfg.Analysis.Enabled && cfg.Claude.APIKey == "" {
        v.AddError("claude.api_key", "", "required_when_analysis_enabled",
            "claude.api_key is required when analysis.enabled=true",
            "Add API key or set analysis.enabled=false")
    }

    if cfg.Claude.APIKey != "" {
        if !strings.HasPrefix(cfg.Claude.APIKey, "sk-ant-") {
            v.AddError("claude.api_key", cfg.Claude.APIKey, "format",
                "claude.api_key must start with 'sk-ant-'",
                "Check your API key from https://console.anthropic.com/")
        }
    }

    // Model validation
    validModels := []string{
        "claude-3-5-sonnet-20241022",
        "claude-3-5-haiku-20241022",
        "claude-3-opus-20240229",
        "claude-3-sonnet-20240229",
        "claude-3-haiku-20240307",
    }
    if !contains(validModels, cfg.Claude.Model) {
        v.AddError("claude.model", cfg.Claude.Model, "valid_model",
            "claude.model must be a valid Claude model",
            fmt.Sprintf("Use one of: %s", strings.Join(validModels, ", ")))
    }

    // Numeric range validations
    if cfg.Claude.MaxTokens <= 0 || cfg.Claude.MaxTokens > 8192 {
        v.AddError("claude.max_tokens", cfg.Claude.MaxTokens, "range",
            "claude.max_tokens must be > 0 and ≤ 8192",
            "Set to a reasonable value like 2000")
    }

    if cfg.Analysis.RateLimitPerMin <= 0 || cfg.Analysis.RateLimitPerMin > 10000 {
        v.AddError("analysis.rate_limit_per_min", cfg.Analysis.RateLimitPerMin, "range",
            "analysis.rate_limit_per_min must be > 0 and ≤ 10000",
            "Set to a reasonable value like 5")
    }

    if cfg.Daemon.Workers <= 0 || cfg.Daemon.Workers > 100 {
        v.AddError("daemon.workers", cfg.Daemon.Workers, "range",
            "daemon.workers must be > 0 and ≤ 100",
            "Set to a reasonable value like 3")
    }

    if cfg.Daemon.DebounceMs < 100 || cfg.Daemon.DebounceMs > 60000 {
        v.AddError("daemon.debounce_ms", cfg.Daemon.DebounceMs, "range",
            "daemon.debounce_ms must be ≥ 100 and ≤ 60000",
            "Set to a reasonable value like 2000 (2 seconds)")
    }

    // Output format validation
    validFormats := []string{"xml", "markdown", "json"}
    if !contains(validFormats, cfg.Output.Format) {
        v.AddError("output.format", cfg.Output.Format, "valid_format",
            "output.format must be xml, markdown, or json",
            fmt.Sprintf("Change to one of: %s", strings.Join(validFormats, ", ")))
    }
}

func (v *Validator) ValidateCrossFields(cfg *Config) {
    // If file watching is disabled, workers should be 0 or 1
    if !cfg.Daemon.FileWatchEnabled && cfg.Daemon.Workers > 1 {
        v.AddError("daemon.workers", cfg.Daemon.Workers, "cross_field",
            "daemon.workers should be 1 when file_watch_enabled=false",
            "Set daemon.workers=1 or enable file watching")
    }
}

func (v *Validator) ValidateIntegrations(cfg *Config) {
    registry := integrations.GlobalRegistry()

    for name, intConfig := range cfg.Integrations.Configs {
        // Type must exist in registry
        integration, err := registry.Get(intConfig.Type)
        if err != nil {
            v.AddError(fmt.Sprintf("integrations.%s.type", name), intConfig.Type,
                "integration_exists",
                fmt.Sprintf("integration type '%s' not found in registry", intConfig.Type),
                "Use a valid integration type: claude-code, continue, cline, aider, generic")
            continue
        }

        // Output format validation
        if intConfig.OutputFormat != "" {
            validFormats := []string{"xml", "markdown", "json"}
            if !contains(validFormats, intConfig.OutputFormat) {
                v.AddError(fmt.Sprintf("integrations.%s.output_format", name),
                    intConfig.OutputFormat, "valid_format",
                    "output_format must be xml, markdown, or json",
                    "Change to: xml, markdown, or json")
            }
        }

        // Integration-specific validation
        if err := integration.Validate(); err != nil {
            v.AddError(fmt.Sprintf("integrations.%s", name), nil,
                "integration_validation",
                fmt.Sprintf("integration validation failed: %v", err),
                "Check integration-specific settings")
        }

        // Claude Code specific validation
        if intConfig.Type == "claude-code" {
            v.validateClaudeCodeIntegration(name, intConfig)
        }

        // Continue.dev specific validation
        if intConfig.Type == "continue" {
            v.validateContinueIntegration(name, intConfig)
        }
    }
}

func (v *Validator) validateClaudeCodeIntegration(name string, cfg IntegrationConfig) {
    // Settings path validation
    settingsPath, ok := cfg.Settings["settings_path"].(string)
    if !ok || settingsPath == "" {
        settingsPath = filepath.Join(os.Getenv("HOME"), ".claude", "settings.json")
    }

    // Check file exists
    info, err := os.Stat(settingsPath)
    if err != nil {
        v.AddError(fmt.Sprintf("integrations.%s.settings.settings_path", name),
            settingsPath, "exists",
            fmt.Sprintf("settings file not found: %v", err),
            "Ensure Claude Code is installed and has created settings.json")
        return
    }

    // Check it's a file
    if info.IsDir() {
        v.AddError(fmt.Sprintf("integrations.%s.settings.settings_path", name),
            settingsPath, "is_file",
            "settings_path must be a file, not a directory",
            "Specify the full path to settings.json")
        return
    }

    // Check file permissions
    if info.Mode().Perm()&0200 == 0 {
        v.AddError(fmt.Sprintf("integrations.%s.settings.settings_path", name),
            settingsPath, "writable",
            "settings file is not writable",
            fmt.Sprintf("Fix permissions: chmod 644 %s", settingsPath))
    }

    // Check file is valid JSON
    if err := validateJSONFile(settingsPath); err != nil {
        v.AddError(fmt.Sprintf("integrations.%s.settings.settings_path", name),
            settingsPath, "valid_json",
            fmt.Sprintf("settings file is not valid JSON: %v", err),
            "Fix JSON syntax errors in settings.json")
    }

    // Validate matchers
    if matchersRaw, ok := cfg.Settings["matchers"]; ok {
        matchers, ok := matchersRaw.([]interface{})
        if !ok {
            v.AddError(fmt.Sprintf("integrations.%s.settings.matchers", name),
                matchersRaw, "type",
                "matchers must be an array of strings",
                "Use: matchers: [\"startup\", \"resume\", \"clear\", \"compact\"]")
            return
        }

        validMatchers := []string{"startup", "resume", "clear", "compact", "create_project"}
        for i, m := range matchers {
            matcher, ok := m.(string)
            if !ok {
                v.AddError(fmt.Sprintf("integrations.%s.settings.matchers[%d]", name, i),
                    m, "type",
                    "matcher must be a string",
                    "Use string values for matchers")
                continue
            }

            if !contains(validMatchers, matcher) {
                v.AddError(fmt.Sprintf("integrations.%s.settings.matchers[%d]", name, i),
                    matcher, "valid_matcher",
                    fmt.Sprintf("'%s' is not a valid matcher", matcher),
                    fmt.Sprintf("Use one of: %s", strings.Join(validMatchers, ", ")))
            }
        }
    }
}

func (v *Validator) validateContinueIntegration(name string, cfg IntegrationConfig) {
    // Config path validation
    configPath, ok := cfg.Settings["config_path"].(string)
    if !ok || configPath == "" {
        // Try both .json and .ts
        jsonPath := filepath.Join(os.Getenv("HOME"), ".continue", "config.json")
        tsPath := filepath.Join(os.Getenv("HOME"), ".continue", "config.ts")

        if _, err := os.Stat(jsonPath); err == nil {
            return // JSON config exists
        }
        if _, err := os.Stat(tsPath); err == nil {
            return // TS config exists
        }

        v.AddError(fmt.Sprintf("integrations.%s.settings.config_path", name),
            "", "exists",
            "Continue.dev config not found (checked config.json and config.ts)",
            "Ensure Continue.dev is installed")
        return
    }

    // Check specified path exists
    if _, err := os.Stat(configPath); err != nil {
        v.AddError(fmt.Sprintf("integrations.%s.settings.config_path", name),
            configPath, "exists",
            fmt.Sprintf("config file not found: %v", err),
            "Ensure Continue.dev is installed and config path is correct")
    }
}

func (v *Validator) AddError(field string, value interface{}, rule string, message string, suggestion string) {
    v.errors = append(v.errors, ValidationError{
        Field:      field,
        Value:      value,
        Rule:       rule,
        Message:    message,
        Suggestion: suggestion,
    })
}

func (v *Validator) Errors() error {
    if len(v.errors) == 0 {
        return nil
    }

    var sb strings.Builder
    sb.WriteString("Configuration validation failed:\n\n")

    for i, err := range v.errors {
        sb.WriteString(fmt.Sprintf("%d. Field: %s\n", i+1, err.Field))
        if err.Value != nil {
            sb.WriteString(fmt.Sprintf("   Value: %v\n", err.Value))
        }
        sb.WriteString(fmt.Sprintf("   Error: %s\n", err.Message))
        if err.Suggestion != "" {
            sb.WriteString(fmt.Sprintf("   Suggestion: %s\n", err.Suggestion))
        }
        sb.WriteString("\n")
    }

    return errors.New(sb.String())
}

func validateJSONFile(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return err
    }

    var js map[string]interface{}
    return json.Unmarshal(data, &js)
}

func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item {
            return true
        }
    }
    return false
}
```

#### Validation Error Messages

All validation errors follow the structured format defined in the Error Handling Framework (Section 7.4) with:
- **Field**: Which configuration field failed validation
- **Value**: The invalid value (if applicable)
- **Error**: Clear description of what's wrong
- **Suggestion**: Actionable steps to fix

#### Validation Testing

**Test Cases**:
1. Empty config (should fail required fields)
2. Invalid memory_root path (should fail exists check)
3. System directory as memory_root (should fail safe_path check)
4. Missing API key when analysis enabled (should fail conditional required)
5. Invalid model name (should fail valid_model check)
6. Out of range numeric values (should fail range checks)
7. Invalid output format (should fail valid_format check)
8. Non-existent integration type (should fail integration_exists check)
9. Invalid Claude Code settings path (should fail exists/valid_json checks)
10. Invalid matchers array (should fail valid_matcher checks)

**Deliverables**:
- [ ] Enhanced error handling implemented
- [ ] Health check system added
- [ ] Logging and observability improved
- [ ] Interactive setup wizard complete
- [ ] Performance optimizations applied
- [ ] Security review completed and issues addressed
- [ ] Version compatibility checking added
- [ ] **NEW**: Daemon reload mechanism implemented (`daemon reload` command)
- [ ] **NEW**: SIGHUP signal handling for config reload
- [ ] **NEW**: Configuration validation framework (`internal/config/validate.go`)
- [ ] **NEW**: Atomic config updates with rollback on failure
- [ ] Full test matrix executed
- [ ] All E2E scenarios passing (including config reload scenarios)
- [ ] Release notes drafted
- [ ] Version bumped to 1.0.0 or 0.5.0 (breaking changes)

---

## Implementation Checklist

Use this checklist to track progress through the implementation plan.

### Phase 1: Foundation & Abstraction Layer ✓

#### Core Abstractions
- [x] Create `internal/integrations/` package structure
- [x] Define `Integration` interface in `interface.go`
- [x] Implement `Registry` in `registry.go`
- [x] Add thread-safe registry operations (register, get, list, detect)
- [x] Create `types.go` with shared types (`OutputFormat`, `IntegrationConfig`, etc.)
- [x] Add utility functions in `utils.go`

#### Output Processors
- [x] Create `internal/integrations/output/` package
- [x] Define `OutputProcessor` interface
- [x] Implement XML formatting as `XMLProcessor`
- [x] Implement Markdown formatting as `MarkdownProcessor`
- [x] **NEW**: Implement `JSONProcessor` to render index as JSON (this is a new output format, not the storage format)
- [x] Test all processors independently
- [x] Remove old `internal/output/formatter.go` code (COMPLETED - removed during legacy cleanup)

#### Configuration Schema
- [x] Add `IntegrationsConfig` to `internal/config/types.go`
- [x] Add `IntegrationConfig` struct
- [x] Update `DefaultConfig` in `constants.go` (empty integrations by default)
- [x] Test configuration loading with new schema
- [x] Validate configuration structure

#### Testing Infrastructure
- [x] Create mock integration for testing (inline in registry_test.go)
- [x] Write `registry_test.go` (registration, lookup, concurrency) - 12 tests
- [x] Write `output_test.go` (all output processors) - 8 tests
- [ ] Write `config_test.go` (configuration loading) (deferred - config loads successfully via build test)
- [x] All new tests pass and follow Go style guide
- [x] Document testing utilities (via inline comments)

### Phase 2: Claude Code Adapter ✓

#### Adapter Implementation
- [x] Create `internal/integrations/adapters/claude/` package
- [x] Implement `ClaudeCodeAdapter` struct
- [x] Implement `GetName()`, `GetDescription()`, `GetVersion()`
- [x] Implement `Detect()` - check for `~/.claude/settings.json`
- [x] Implement `IsEnabled()` - verify hooks are configured
- [x] Implement fresh settings management

#### Settings Management
- [x] Create `settings.go` in Claude adapter
- [x] Implement `ReadSettings()` for Claude settings.json
- [x] Implement `WriteSettings()` with atomic updates
- [x] Implement `SetupSessionStartHooks()` logic
- [x] Support all 4 matchers (startup, resume, clear, compact)
- [x] Test settings read/write/update operations

#### Output Formatting
- [x] Create `output.go` in Claude adapter
- [x] Define `SessionStartOutput` struct
- [x] Define `HookSpecificOutput` struct
- [x] Implement SessionStart JSON wrapper
- [x] Implement `FormatOutput()` adapter method
- [x] Test JSON wrapping produces valid SessionStart format

#### Command Updates
- [x] Add `--integration <name>` flag to `cmd/read/read.go`
- [x] Implement `outputForIntegration()` function
- [x] Update help text with integration examples

#### Init Command Updates
- [x] Update `cmd/init/init.go` to use integration registry
- [x] Implement integration detection and setup flow
- [x] Prompt user for integration selection (if multiple detected)
- [x] Support `--setup-integrations` flag for automated setup

#### Legacy Code Removal
- [x] Remove `internal/output/formatter.go` (491 lines)
- [x] Remove `internal/output/formatter_test.go` (364 lines)
- [x] Remove `internal/hooks/manager.go` (178 lines)
- [x] Remove `internal/hooks/manager_test.go` (584 lines)
- [x] Remove `internal/hooks/types.go` (17 lines)
- [x] Remove `--wrap-json` flag from read command
- [x] Remove `WrapJSON` from config types
- [x] Update README.md with new integration system
- [x] Update config.yaml.example

#### Validation & Testing
- [x] All tests pass (20 tests in integrations and output packages)
- [x] Build succeeds with no errors
- [x] Test `--integration claude-code` produces valid SessionStart JSON
- [x] Verify Claude adapter generates correct commands (no `--wrap-json`)
- [x] Validate GetCommand() generates `read --format xml --integration claude-code`
- [ ] Test fresh install with Claude Code (requires manual testing)
- [ ] Test fresh install on system with multiple frameworks (deferred to Phase 3)
- [ ] Verify SessionStart hooks trigger correctly (requires manual testing)
- [ ] Test all 4 matchers (startup, resume, clear, compact) (requires manual testing)

### Phase 3: Additional Integrations ✓

#### Generic Adapter (COMPLETED)
- [x] Create `internal/integrations/adapters/generic/` package
- [x] Implement `GenericAdapter` for unsupported frameworks
- [x] `Setup()` returns helpful error with manual instructions
- [x] `FormatOutput()` returns plain format without wrapping
- [x] Register generic adapters for Continue, Cline, Aider, Cursor, Custom
- [x] Test generic adapter with manual setup flow
- [x] Document generic adapter usage in README

#### Management Commands (COMPLETED)
- [x] Create `cmd/integrations/` package
- [x] Implement `integrations list` command
- [x] Implement `integrations detect` command
- [x] Implement `integrations setup <name>` command
- [x] Implement `integrations remove <name>` command
- [x] Implement `integrations validate` command
- [x] Add help text and examples for all commands
- [x] Wire into root command
- [x] Test all commands with Claude adapter
- [x] Update README with integrations commands documentation

#### Continue.dev Adapter (DEFERRED - Generic Adapter Sufficient)
- [x] Generic adapter registered as "continue"
- [ ] Research Continue.dev configuration format (deferred)
- [ ] Create dedicated `internal/integrations/adapters/continue/` package (deferred)
- [ ] Implement `ContinueAdapter` struct (deferred)
- [ ] Implement detection (`~/.continue/config.json` or `.ts`) (deferred)
- [ ] Create `config.go` for Continue config management (deferred)
- [ ] Implement `Setup()` - add memory tool to tools array (deferred)
- [ ] Test with real Continue.dev installation (deferred)

**Note**: Generic adapter provides manual setup instructions for Continue.dev. A dedicated adapter can be implemented when there's demand for automatic setup.

#### Cline Adapter (DEFERRED - Generic Adapter Sufficient)
- [x] Generic adapter registered as "cline"
- [ ] Research Cline configuration mechanism (deferred)
- [ ] Create dedicated adapter package (deferred)

**Note**: Generic adapter provides manual setup instructions for Cline.

#### Aider Adapter (DEFERRED - Generic Adapter Sufficient)
- [x] Generic adapter registered as "aider"
- [ ] Research Aider configuration (deferred)
- [ ] Create dedicated adapter package (deferred)

**Note**: Generic adapter provides manual setup instructions for Aider.

#### Cursor AI Adapter (ADDED - Generic)
- [x] Generic adapter registered as "cursor"

**Note**: Generic adapter provides manual setup instructions for Cursor AI.

#### Integration Registry (COMPLETED)
- [x] Register Claude Code adapter (via register.go)
- [x] Register generic adapters (Continue, Cline, Aider, Cursor, Custom)
- [x] Lazy initialization via blank imports
- [x] Thread-safe global registry

### Phase 4: Documentation & User Experience ✓

#### Core Documentation (COMPLETED)
- [x] Update `README.md` with multi-framework support
- [x] Add "Supported Integrations" section to README
- [x] Add "Managing Integrations" section with full command documentation
- [x] Update integration features list
- [x] Create `docs/architecture.md` - Comprehensive system design documentation
- [x] Document adapter pattern and registry pattern
- [x] Document integration interface specification
- [x] Document output format options (XML, Markdown, JSON)
- [x] Document data flow and component architecture

#### Integration Guides (COMPLETED)
- [x] Create `docs/integrations/` directory
- [x] Write `claude-code.md` guide - Complete automatic setup guide
- [x] Write `generic.md` guide - Covers Continue, Cline, Aider, Cursor with manual setup instructions
- [x] Write `custom.md` guide - Developer guide for adding new integrations
- [x] Include comprehensive examples in all guides
- [x] Add troubleshooting sections
- [x] Add best practices and common patterns

**Note**: Dedicated guides for Continue, Cline, and Aider not needed - `generic.md` covers all manual setup frameworks with framework-specific instructions.

#### Configuration Examples (COMPLETED)
- [x] Create `examples/config-basic.yaml` - Basic configuration
- [x] Create `examples/config-with-integrations.yaml` - Production config with daemon enabled
- [x] Create `examples/README.md` - Documentation for all examples
- [x] Document each configuration option with comments
- [x] Include usage scenarios and customization examples

#### Help Text & CLI UX (COMPLETED)
- [x] Update `read` command help text with improved Long description and Examples
- [x] Init command help text already comprehensive
- [x] Integration commands have detailed help and examples
- [x] Error messages in integration commands provide actionable guidance

#### Contributing & Testing Docs (COMPLETED)
- [x] Create `docs/CONTRIBUTING.md` - Comprehensive contribution guide
- [x] Add detailed section on adding new integrations
- [x] Document testing approach and examples
- [x] Document coding standards and conventions
- [x] Add PR process and commit message guidelines
- [ ] Create `docs/testing.md` (DEFERRED - covered in CONTRIBUTING.md)
- [ ] Document E2E testing approach (DEFERRED - not yet implemented)

#### Changelog (DEFERRED)
- [ ] Update `CHANGELOG.md` (deferred to release)
- [ ] Document new features (deferred to release)
- [ ] Bump version appropriately (deferred to release)

### Phase 5: Polish & Release ✓

#### Error Handling (Implements Section 7: Error Handling Framework) - DEFERRED
- [ ] Create `internal/integrations/errors.go` (DEFERRED - basic error handling in place)
- [ ] Implement complete error type hierarchy (Section 7.1) (DEFERRED)
  - [ ] Add `ErrorCategory` enum (config, integration, daemon, io, validation, network)
  - [ ] Add `ErrorSeverity` enum (fatal, critical, warning, info)
  - [ ] Define `IntegrationError` struct with all fields
  - [ ] Define all specific error types (20+ error vars)
- [ ] Create `internal/integrations/retry.go`
  - [ ] Implement `RetryPolicy` type
  - [ ] Implement `RetryWithBackoff()` function
  - [ ] Add retry policies for each error type (Section 7.2 table)
  - [ ] Implement `isRetryable()` helper
- [ ] Implement rollback mechanisms (Section 7.3)
  - [ ] Add backup functionality to all adapters
  - [ ] Implement transaction-style operations
  - [ ] Add `.in-progress` marker files
  - [ ] Implement rollback procedures for Setup/Reload/Remove
- [ ] Implement structured error messages (Section 7.4)
  - [ ] Follow error message template format
  - [ ] Include actionable suggestions in all errors
  - [ ] Add context (file:line, integration, operation)
  - [ ] Reference health check commands
- [ ] Implement crash recovery (Section 7.6)
  - [ ] Create `internal/daemon/recovery.go`
  - [ ] Detect incomplete operations on daemon start
  - [ ] Implement recovery procedures for each operation
  - [ ] Add recovery markers and cleanup
- [ ] Integrate errors with health checks (Section 7.8)
  - [ ] Map error severity to health status
  - [ ] Track error counts and timestamps
  - [ ] Update health status based on errors
- [ ] Test all error scenarios
  - [ ] Test all error types with appropriate messages
  - [ ] Test retry policies with backoff
  - [ ] Test rollback procedures (forced failures)
  - [ ] Test crash recovery (kill daemon mid-operation)
  - [ ] Verify error message format compliance

#### Configuration Validation (Implements Section 5.9: Config Validation Framework) - COMPLETED
- [x] Create `internal/config/validate.go`
- [x] Implement `ValidateConfig()` with comprehensive validation (simplified from 5-phase)
  - [x] YAML syntax validation (handled by viper)
  - [x] Schema validation (required fields, types)
  - [x] Value validation (ranges, enums, paths)
  - [x] Path safety validation (traversal protection)
- [x] Implement `Validator` type with error accumulation
- [x] Implement core config validation rules
  - [x] Validate memory_root (exists, is directory, safe path, etc.)
  - [x] Validate claude.api_key (conditional validation)
  - [x] Validate claude.model (presence check)
  - [x] Validate numeric ranges (max_tokens, rate_limit, workers, debounce, timeout)
  - [x] Validate output_format enum
  - [x] Validate daemon configuration (log_level enum, health_check_port range)
  - [x] Validate analysis configuration (max_file_size, parallel workers)
- [x] Implement integration validation rules
  - [x] Common fields (type, output_format)
  - [x] `ValidateIntegrationConfig()` function
- [x] Implement validation helper functions
  - [x] `contains()` for slice checks
  - [x] `SafePath()` for path safety validation
  - [x] `ValidateBinaryPath()` with permission checking
  - [x] `ExpandHome()` for path expansion
- [x] Implement structured validation errors
  - [x] `ValidationError` struct with field/value/rule/message/suggestion
  - [x] Format validation errors as comprehensive reports with actionable suggestions
- [x] Create `cmd/config/config.go` with validate subcommand
- [x] Test validation with actual config file

#### Health Checks - COMPLETED
- [x] Health checks implemented using existing `Validate()` and `Detect()` methods
- [x] Implement health checks for Claude Code adapter (via Validate method)
- [x] Implement health checks for generic adapters (via Validate method)
- [x] Create health check command in `cmd/integrations/integrations.go`
- [x] Test health checks with configured and unconfigured integrations
- [ ] Add `Health()` method to `Integration` interface (DEFERRED - existing methods sufficient)
- [ ] Define `HealthStatus` struct (DEFERRED - existing methods sufficient)

#### Logging & Observability - COMPLETED
- [x] Integration operations have comprehensive stdout/stderr output
- [x] Log setup/update/remove operations (via fmt.Printf)
- [x] All commands provide detailed progress and result information
- [x] Error messages include actionable guidance
- [ ] Create `internal/integrations/logger.go` (DEFERRED - CLI output sufficient for commands)
- [ ] Add metrics tracking (DEFERRED - not needed for initial release)
- [ ] Integrate with daemon logging (DEFERRED - daemon has separate logging)

#### Interactive Setup Wizard - DEFERRED
- [ ] Create `cmd/init/wizard.go` (DEFERRED - init command has interactive prompts)
- [ ] Implement interactive prompts for directory selection (DEFERRED)
- [x] Implement integration detection display (in init command)
- [x] Implement prompts for integration setup (in init command)
- [x] Add confirmation steps (in init command)
- [ ] Implement multi-select for integration setup (DEFERRED - auto-setup all detected)
- [ ] Make wizard default for `agentic-memorizer init` (DEFERRED)
- [x] Add flags for non-interactive mode (--skip-integrations, --skip-daemon)

#### Performance Optimization - COMPLETED
- [x] Performance testing completed and documented in `docs/PERFORMANCE.md`
- [x] Read command: <10ms (target <50ms) ✓ EXCELLENT
- [x] Build time: 0.75s ✓ EXCELLENT
- [x] Test suite: 0.23s ✓ EXCELLENT
- [x] Integration operations: <10ms ✓ EXCELLENT
- [ ] Implement lazy loading for integrations (DEFERRED - not needed, already fast)
- [ ] Add output caching (DEFERRED - read already <10ms)
- [ ] Optimize registry lookups (DEFERRED - already optimized)

#### Security Review - COMPLETED
- [x] Review all file path handling for traversal vulnerabilities (completed, documented in `docs/SECURITY.md`)
- [x] Validate binary paths before execution (ValidateBinaryPath() implemented)
- [x] Review JSON/YAML parsing (using standard library - safe)
- [x] Check file permissions on config files (appropriate permissions verified)
- [x] Sanitize user input in commands (path validation in place)
- [x] Review for arbitrary code execution risks (no risks identified)
- [x] Document security considerations (`docs/SECURITY.md` created)

#### Version Compatibility - DEFERRED
- [ ] Add version constraint system (DEFERRED - future enhancement)
- [ ] Implement version compatibility checking (DEFERRED)
- [ ] Add `GetVersionConstraint()` to integrations (DEFERRED)
- [ ] Check compatibility on setup/update (DEFERRED)
- [ ] Document version compatibility in docs (DEFERRED)

#### Final Testing - COMPLETED
- [x] Run full unit test suite (20 tests pass)
- [x] Test all output formats (XML, Markdown, JSON) - all working
- [x] Test integration commands (list, detect, setup, remove, validate, health) - all working
- [x] Test config validate command - working
- [x] Test read command - <10ms performance ✓
- [x] Test daemon status command - working
- [x] Verify build completes successfully
- [ ] Create test matrix (all integrations × all operations) (DEFERRED)
- [ ] Test fresh install on clean system (DEFERRED - manual testing)
- [ ] Test switching between integrations (DEFERRED)
- [ ] Test daemon restart with integrations active (DEFERRED)
- [ ] Performance testing under load (DEFERRED - basic performance verified)

#### Release Preparation - PARTIALLY COMPLETED
- [x] Update version in `internal/version/version.go` to 0.6.0
- [ ] Draft release notes (DEFERRED to release)
- [ ] Tag release in git (DEFERRED to release)
- [ ] Build binaries for all platforms (DEFERRED to release)
- [ ] Test binaries on fresh systems (DEFERRED to release)
- [ ] Update package manager configs (DEFERRED to release)
- [ ] Prepare announcement post/tweet (DEFERRED to release)

---

## Risk Assessment & Mitigation

### High Risk Items

#### 1. Integration Complexity

**Risk**: Supporting multiple frameworks increases maintenance burden.

**Mitigation**:
- Strong abstraction layer (adapter pattern)
- Comprehensive testing for each integration
- Clear interface contracts
- Documentation for adding new integrations
- Community can contribute adapters

### Medium Risk Items

#### 2. Performance Regression

**Risk**: Additional abstraction layers could slow down reads.

**Mitigation**:
- Benchmark implementation
- Lazy loading for integrations
- Output caching
- Profile and optimize hot paths
- Target: <50ms for read command

#### 3. Documentation Maintenance

**Risk**: Multiple integrations = multiple docs to maintain.

**Mitigation**:
- Modular documentation structure
- Template for new integration docs
- Automated documentation generation where possible
- Community contributions for framework-specific docs

### Low Risk Items

#### 4. Version Compatibility

**Risk**: Different agentic-memorizer versions with different integrations.

**Mitigation**:
- Version constraints per integration
- Clear compatibility matrix in docs
- Fail fast with helpful errors

---

## Success Criteria

### Functional Requirements
- [ ] Claude Code integration works correctly with SessionStart hooks
- [ ] At least 2 additional frameworks supported (Continue, Cline/Aider)
- [ ] Generic adapter works for unsupported frameworks
- [ ] All integrations can be enabled simultaneously
- [ ] Health checks verify integration status
- [ ] All output formats (XML, Markdown, JSON) work correctly

### Non-Functional Requirements
- [ ] Read command performance: <50ms
- [ ] Clean, idiomatic Go code following style guide
- [ ] 100% test coverage for integration layer
- [ ] Documentation complete for all supported integrations

### User Experience
- [ ] `agentic-memorizer init` detects and sets up integrations automatically
- [ ] Clear error messages with actionable guidance
- [ ] `--help` text explains integration options clearly
- [ ] New users can add integrations easily
- [ ] Configuration is straightforward and well-documented

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

## Implementation Summary

| Phase | Key Deliverables |
|-------|------------------|
| Phase 1: Foundation | Interface, Registry, Output Processors, Config Schema |
| Phase 2: Claude Adapter | Claude Code adapter implementation |
| Phase 3: Additional Integrations | Continue, Cline, Aider adapters, management commands |
| Phase 4: Documentation & UX | Comprehensive documentation, examples |
| Phase 5: Polish & Release | Error handling, health checks, testing, release |

---

## Conclusion

This implementation transforms agentic-memorizer into a **framework-agnostic agentic memory system**. The adapter pattern provides clean abstraction, making it easy to support multiple agent frameworks while maintaining the robust core pipeline.

The phased approach ensures:
1. **Clean implementation** following Go best practices
2. **Extensibility** for future integrations
3. **Maintainability** through strong abstractions
4. **Excellent user experience** with automatic detection and setup

By the end of this implementation, agentic-memorizer will be positioned as the **go-to memory solution** for AI agent frameworks (Claude Code, Continue, Cline, Aider, and more).

---

**Next Steps**: Review this plan, adjust timeline/scope as needed, and begin Phase 1 implementation.
