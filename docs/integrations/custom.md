# Adding Custom Integrations

**Audience**: Developers
**Difficulty**: Intermediate
**Time**: 2-4 hours

## Overview

This guide shows you how to add automatic setup support for a new agent framework by implementing a custom integration adapter.

## Prerequisites

- Go 1.25.1 or later
- Understanding of the target framework's configuration format
- Familiarity with Go interfaces and packages

## Integration Interface

All integrations must implement this interface:

```go
type Integration interface {
    // Metadata
    GetName() string
    GetDescription() string
    GetVersion() string

    // Detection
    Detect() (bool, error)           // Can we find this framework?
    IsEnabled() (bool, error)         // Is it currently configured?

    // Lifecycle
    Setup(binaryPath string) error   // Configure the framework
    Update(binaryPath string) error  // Update existing config
    Remove() error                    // Remove configuration

    // Command Generation
    GetCommand(binaryPath string, format OutputFormat) string

    // Output Formatting
    FormatOutput(index *types.Index, format OutputFormat) (string, error)

    // Validation
    Validate() error                  // Check configuration health
    Reload(newConfig IntegrationConfig) error
}
```

## Step-by-Step Guide

### Step 1: Research the Framework

Before writing code, research your target framework:

**Questions to answer:**
1. Where does the framework store its configuration?
   - File path (e.g., `~/.framework/config.json`)
   - Format (JSON, YAML, TypeScript, etc.)

2. How does the framework support custom commands/tools?
   - Hooks system?
   - Tools/plugins array?
   - Startup scripts?

3. What permissions are needed?
   - Can we modify config files?
   - Are there APIs to use instead?

4. How does the framework execute commands?
   - Shell commands?
   - Internal APIs?
   - HTTP endpoints?

**Example: Continue.dev research findings**
- Config: `~/.continue/config.json` or `~/.continue/config.ts`
- Custom commands via `tools` array
- JSON format with name, description, command fields
- Supports shell command execution

### Step 2: Create Adapter Package

Create a new package for your adapter:

```bash
mkdir -p internal/integrations/adapters/yourframework
cd internal/integrations/adapters/yourframework
```

Create these files:
- `adapter.go` - Main integration implementation
- `config.go` - Configuration file management (optional)
- `output.go` - Output formatting (optional, if framework needs special wrapper)
- `register.go` - Auto-registration

### Step 3: Implement the Adapter

**File: `adapter.go`**

```go
package yourframework

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/leefowlercu/agentic-memorizer/internal/integrations"
    "github.com/leefowlercu/agentic-memorizer/internal/integrations/output"
    "github.com/leefowlercu/agentic-memorizer/pkg/types"
)

const (
    IntegrationName    = "yourframework"
    IntegrationVersion = "1.0.0"
)

type YourFrameworkAdapter struct {
    configPath string
}

func NewYourFrameworkAdapter() *YourFrameworkAdapter {
    home, _ := os.UserHomeDir()
    return &YourFrameworkAdapter{
        configPath: filepath.Join(home, ".yourframework", "config.json"),
    }
}

// GetName returns the integration name
func (a *YourFrameworkAdapter) GetName() string {
    return IntegrationName
}

// GetDescription returns a human-readable description
func (a *YourFrameworkAdapter) GetDescription() string {
    return "Your Framework integration with automatic setup"
}

// GetVersion returns the adapter version
func (a *YourFrameworkAdapter) GetVersion() string {
    return IntegrationVersion
}

// Detect checks if the framework is installed
func (a *YourFrameworkAdapter) Detect() (bool, error) {
    // Check if config file or directory exists
    _, err := os.Stat(a.configPath)
    if os.IsNotExist(err) {
        return false, nil
    }
    if err != nil {
        return false, fmt.Errorf("failed to check config: %w", err)
    }
    return true, nil
}

// IsEnabled checks if the integration is configured
func (a *YourFrameworkAdapter) IsEnabled() (bool, error) {
    // Load config and check if our command is present
    config, err := a.loadConfig()
    if err != nil {
        if os.IsNotExist(err) {
            return false, nil
        }
        return false, err
    }

    // Check if agentic-memorizer command is in the config
    for _, tool := range config.Tools {
        if tool.Name == "memory" || strings.Contains(tool.Command, "agentic-memorizer") {
            return true, nil
        }
    }

    return false, nil
}

// Setup configures the integration
func (a *YourFrameworkAdapter) Setup(binaryPath string) error {
    config, err := a.loadConfig()
    if err != nil && !os.IsNotExist(err) {
        return fmt.Errorf("failed to load config: %w", err)
    }

    if config == nil {
        // Create default config
        config = &Config{Tools: []Tool{}}
    }

    // Add or update our tool
    command := a.GetCommand(binaryPath, integrations.FormatMarkdown)
    tool := Tool{
        Name:        "memory",
        Description: "Access agentic memory index",
        Command:     command,
    }

    // Check if already exists
    found := false
    for i, t := range config.Tools {
        if t.Name == "memory" {
            config.Tools[i] = tool
            found = true
            break
        }
    }

    if !found {
        config.Tools = append(config.Tools, tool)
    }

    // Save config
    return a.saveConfig(config)
}

// Update updates the integration
func (a *YourFrameworkAdapter) Update(binaryPath string) error {
    return a.Setup(binaryPath)
}

// Remove removes the integration
func (a *YourFrameworkAdapter) Remove() error {
    config, err := a.loadConfig()
    if err != nil {
        return fmt.Errorf("failed to load config: %w", err)
    }

    // Remove our tool
    filtered := []Tool{}
    for _, tool := range config.Tools {
        if tool.Name != "memory" && !strings.Contains(tool.Command, "agentic-memorizer") {
            filtered = append(filtered, tool)
        }
    }

    config.Tools = filtered
    return a.saveConfig(config)
}

// GetCommand returns the command to execute
func (a *YourFrameworkAdapter) GetCommand(binaryPath string, format integrations.OutputFormat) string {
    return fmt.Sprintf("%s read --format %s", binaryPath, format)
}

// FormatOutput formats the index for the framework
func (a *YourFrameworkAdapter) FormatOutput(index *types.Index, format integrations.OutputFormat) (string, error) {
    // Most frameworks don't need special wrapping
    // Just use the output processors directly

    var processor output.OutputProcessor
    switch format {
    case integrations.FormatXML:
        processor = output.NewXMLProcessor()
    case integrations.FormatMarkdown:
        processor = output.NewMarkdownProcessor()
    case integrations.FormatJSON:
        processor = output.NewJSONProcessor()
    default:
        return "", fmt.Errorf("unsupported format: %s", format)
    }

    return processor.Format(index)
}

// Validate checks the configuration
func (a *YourFrameworkAdapter) Validate() error {
    // Check if config file exists
    if _, err := os.Stat(a.configPath); os.IsNotExist(err) {
        return fmt.Errorf("config file not found: %s", a.configPath)
    }

    // Check if our tool is configured
    enabled, err := a.IsEnabled()
    if err != nil {
        return fmt.Errorf("failed to check status: %w", err)
    }

    if !enabled {
        return fmt.Errorf("agentic-memorizer tool not found in config")
    }

    return nil
}

// Reload reloads configuration
func (a *YourFrameworkAdapter) Reload(newConfig integrations.IntegrationConfig) error {
    // Most integrations don't need special reload logic
    // Configuration is loaded fresh each time
    return nil
}
```

### Step 4: Implement Configuration Management

**File: `config.go`**

```go
package yourframework

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
)

// Config represents the framework's configuration
type Config struct {
    Tools []Tool `json:"tools"`
    // Add other fields as needed
}

// Tool represents a custom tool configuration
type Tool struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Command     string `json:"command"`
}

func (a *YourFrameworkAdapter) loadConfig() (*Config, error) {
    data, err := os.ReadFile(a.configPath)
    if err != nil {
        return nil, err
    }

    var config Config
    if err := json.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("failed to parse config: %w", err)
    }

    return &config, nil
}

func (a *YourFrameworkAdapter) saveConfig(config *Config) error {
    // Ensure directory exists
    dir := filepath.Dir(a.configPath)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("failed to create config directory: %w", err)
    }

    // Create backup if file exists
    if _, err := os.Stat(a.configPath); err == nil {
        backupPath := a.configPath + ".backup"
        if err := os.Rename(a.configPath, backupPath); err != nil {
            return fmt.Errorf("failed to create backup: %w", err)
        }
        defer os.Remove(backupPath)
    }

    // Marshal to JSON
    data, err := json.MarshalIndent(config, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal config: %w", err)
    }

    // Write file
    if err := os.WriteFile(a.configPath, data, 0644); err != nil {
        return fmt.Errorf("failed to write config: %w", err)
    }

    return nil
}
```

### Step 5: Register the Adapter

**File: `register.go`**

```go
package yourframework

import "github.com/leefowlercu/agentic-memorizer/internal/integrations"

// init registers the adapter with the global registry
func init() {
    integrations.GlobalRegistry().Register(NewYourFrameworkAdapter())
}
```

### Step 6: Import the Adapter

Add to `cmd/integrations/integrations.go`:

```go
import (
    // ... existing imports
    _ "github.com/leefowlercu/agentic-memorizer/internal/integrations/adapters/yourframework"
)
```

### Step 7: Test the Integration

```bash
# Build
go build

# Test detection
./agentic-memorizer integrations detect

# Test setup
./agentic-memorizer integrations setup yourframework

# Test validation
./agentic-memorizer integrations validate

# Test output
./agentic-memorizer read --format markdown

# Test removal
./agentic-memorizer integrations remove yourframework
```

## Advanced Topics

### Custom Output Wrapping

If your framework needs special output formatting (like Claude Code's SessionStart JSON), implement custom wrapper logic:

**File: `output.go`**

```go
package yourframework

import (
    "encoding/json"
    "fmt"

    "github.com/leefowlercu/agentic-memorizer/internal/integrations"
    "github.com/leefowlercu/agentic-memorizer/internal/integrations/output"
    "github.com/leefowlercu/agentic-memorizer/pkg/types"
)

// FrameworkOutput represents the framework-specific wrapper
type FrameworkOutput struct {
    Success bool   `json:"success"`
    Data    string `json:"data"`
}

func formatForFramework(index *types.Index, format integrations.OutputFormat) (string, error) {
    // Step 1: Generate formatted content
    var content string
    var err error

    switch format {
    case integrations.FormatXML:
        processor := output.NewXMLProcessor()
        content, err = processor.Format(index)
    case integrations.FormatMarkdown:
        processor := output.NewMarkdownProcessor()
        content, err = processor.Format(index)
    case integrations.FormatJSON:
        processor := output.NewJSONProcessor()
        content, err = processor.Format(index)
    default:
        return "", fmt.Errorf("unsupported format: %s", format)
    }

    if err != nil {
        return "", err
    }

    // Step 2: Wrap in framework-specific structure
    wrapper := FrameworkOutput{
        Success: true,
        Data:    content,
    }

    jsonBytes, err := json.MarshalIndent(wrapper, "", "  ")
    if err != nil {
        return "", fmt.Errorf("failed to marshal wrapper: %w", err)
    }

    return string(jsonBytes), nil
}

// Update FormatOutput in adapter.go to use this:
func (a *YourFrameworkAdapter) FormatOutput(index *types.Index, format integrations.OutputFormat) (string, error) {
    return formatForFramework(index, format)
}
```

### Complex Configuration

If the framework has complex configuration (YAML, TypeScript, multiple files):

1. Use appropriate parser library:
   - YAML: `gopkg.in/yaml.v3`
   - TOML: `github.com/BurntSushi/toml`
   - HCL: `github.com/hashicorp/hcl`

2. Preserve unknown fields when reading/writing

3. Validate configuration structure

4. Handle migration for breaking changes

### Error Handling Best Practices

```go
// Be specific about errors
func (a *Adapter) Setup(binaryPath string) error {
    if _, err := os.Stat(a.configPath); os.IsNotExist(err) {
        return fmt.Errorf("framework not installed: config file %s does not exist", a.configPath)
    }

    config, err := a.loadConfig()
    if err != nil {
        return fmt.Errorf("failed to load config from %s: %w", a.configPath, err)
    }

    if err := a.saveConfig(config); err != nil {
        return fmt.Errorf("failed to save config to %s: %w", a.configPath, err)
    }

    return nil
}

// Provide actionable error messages
func (a *Adapter) Validate() error {
    if !a.isConfigured() {
        return fmt.Errorf("not configured. Run: agentic-memorizer integrations setup %s", a.GetName())
    }
    return nil
}
```

## Testing

Create tests for your adapter:

**File: `adapter_test.go`**

```go
package yourframework

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/leefowlercu/agentic-memorizer/internal/integrations"
)

func TestDetect(t *testing.T) {
    adapter := NewYourFrameworkAdapter()

    // Test when config doesn't exist
    adapter.configPath = "/nonexistent/path"
    detected, err := adapter.Detect()
    if err != nil {
        t.Fatalf("Detect() error: %v", err)
    }
    if detected {
        t.Error("Expected Detect() to return false for nonexistent path")
    }

    // Test when config exists
    tmpDir := t.TempDir()
    adapter.configPath = filepath.Join(tmpDir, "config.json")
    os.WriteFile(adapter.configPath, []byte("{}"), 0644)

    detected, err = adapter.Detect()
    if err != nil {
        t.Fatalf("Detect() error: %v", err)
    }
    if !detected {
        t.Error("Expected Detect() to return true for existing config")
    }
}

func TestSetup(t *testing.T) {
    adapter := NewYourFrameworkAdapter()
    tmpDir := t.TempDir()
    adapter.configPath = filepath.Join(tmpDir, "config.json")

    err := adapter.Setup("/usr/bin/agentic-memorizer")
    if err != nil {
        t.Fatalf("Setup() error: %v", err)
    }

    // Verify config was created
    if _, err := os.Stat(adapter.configPath); os.IsNotExist(err) {
        t.Error("Setup() did not create config file")
    }

    // Verify IsEnabled returns true
    enabled, err := adapter.IsEnabled()
    if err != nil {
        t.Fatalf("IsEnabled() error: %v", err)
    }
    if !enabled {
        t.Error("Expected IsEnabled() to return true after Setup()")
    }
}
```

## Documentation

Document your integration:

**File: `docs/integrations/yourframework.md`**

Include:
1. Overview
2. Quick start guide
3. Manual configuration (as fallback)
4. Troubleshooting
5. Examples

## Submitting Your Integration

1. Ensure all tests pass: `go test ./...`
2. Run linters: `golangci-lint run`
3. Update README.md to mention your framework
4. Create pull request with:
   - Integration code
   - Tests
   - Documentation
   - Example configuration

## Examples

Study existing adapters:

- **Claude Code Adapter**: Full automatic setup with JSON wrapper
  - Location: `internal/integrations/adapters/claude/`
  - Features: Settings management, SessionStart JSON, validation

- **Generic Adapter**: Simple fallback adapter
  - Location: `internal/integrations/adapters/generic/`
  - Features: Manual instructions, no auto-setup

## Getting Help

- Open an issue: https://github.com/leefowlercu/agentic-memorizer/issues
- Check existing adapters for patterns
- Read the integration interface documentation

## References

- [Architecture Documentation](../architecture.md)
- [Integration Interface](../../internal/integrations/interface.go)
- [Claude Code Adapter](../../internal/integrations/adapters/claude/)
- [Generic Adapter](../../internal/integrations/adapters/generic/)
