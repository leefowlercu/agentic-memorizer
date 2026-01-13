# Configuration Subsystem Refactor

## Problem Statement

The configuration subsystem has a structural misalignment between two components:

1. **Config Package** (`internal/config/`) - Defines default values and expected configuration structure
2. **Initialize TUI** (`internal/tui/initialize/`) - Writes user-selected configuration during first-time setup

This misalignment causes the initialize command to write a config.yaml with a different structure than what the application expects to read. For example:
- Initialize writes `http.port`, but config expects `daemon.http_port`
- Initialize writes `semantic.provider`, but config expects `providers.default_semantic`
- Initialize writes `embeddings.enabled`, but config has no embeddings section

Additionally, there is no centralized mechanism for writing configuration to disk. The initialize workflow uses viper's `WriteConfigAs()` directly, bypassing any structural validation.

## Target Configuration Structure

```yaml
log_level: "info"
log_file: "~/.config/memorizer/memorizer.log"

daemon:
  http_port: 7600
  http_bind: "127.0.0.1"
  shutdown_timeout: 30
  pid_file: "~/.config/memorizer/daemon.pid"
  registry_path: "~/.config/memorizer/registry.db"
  metrics:
    collection_interval: 60

graph:
  host: "localhost"
  port: 6379
  name: "memorizer"
  password_env: "MEMORIZER_GRAPH_PASSWORD"
  max_retries: 3
  retry_delay_ms: 1000
  write_queue_size: 1000

semantic:
  provider: "anthropic"
  model: "claude-sonnet-4-5-20250929"
  rate_limit: 50
  api_key: ""
  api_key_env: "ANTHROPIC_API_KEY"

embeddings:
  enabled: true
  provider: "openai"
  model: "text-embedding-3-large"
  dimensions: 3072
  api_key: ""
  api_key_env: "OPENAI_API_KEY"
```

### Removed Sections

The following sections from the current config are being removed (no consumers in codebase):
- `database` - Moved to `daemon.registry_path`
- `handlers` - Unused placeholder
- `watcher` - Unused placeholder
- `cache` - Unused placeholder
- `providers` - Replaced by `semantic` and `embeddings` sections

## Typed Configuration Struct

### Structure

The config package will expose a fully typed Go struct hierarchy:

```go
type Config struct {
    LogLevel string        `yaml:"log_level" mapstructure:"log_level"`
    LogFile  string        `yaml:"log_file" mapstructure:"log_file"`
    Daemon   DaemonConfig  `yaml:"daemon" mapstructure:"daemon"`
    Graph    GraphConfig   `yaml:"graph" mapstructure:"graph"`
    Semantic SemanticConfig `yaml:"semantic" mapstructure:"semantic"`
    Embeddings EmbeddingsConfig `yaml:"embeddings" mapstructure:"embeddings"`
}

type DaemonConfig struct {
    HTTPPort        int           `yaml:"http_port" mapstructure:"http_port"`
    HTTPBind        string        `yaml:"http_bind" mapstructure:"http_bind"`
    ShutdownTimeout int           `yaml:"shutdown_timeout" mapstructure:"shutdown_timeout"`
    PIDFile         string        `yaml:"pid_file" mapstructure:"pid_file"`
    RegistryPath    string        `yaml:"registry_path" mapstructure:"registry_path"`
    Metrics         MetricsConfig `yaml:"metrics" mapstructure:"metrics"`
}

type MetricsConfig struct {
    CollectionInterval int `yaml:"collection_interval" mapstructure:"collection_interval"`
}

type GraphConfig struct {
    Host           string `yaml:"host" mapstructure:"host"`
    Port           int    `yaml:"port" mapstructure:"port"`
    Name           string `yaml:"name" mapstructure:"name"`
    PasswordEnv    string `yaml:"password_env" mapstructure:"password_env"`
    MaxRetries     int    `yaml:"max_retries" mapstructure:"max_retries"`
    RetryDelayMs   int    `yaml:"retry_delay_ms" mapstructure:"retry_delay_ms"`
    WriteQueueSize int    `yaml:"write_queue_size" mapstructure:"write_queue_size"`
}

type SemanticConfig struct {
    Provider  string  `yaml:"provider" mapstructure:"provider"`
    Model     string  `yaml:"model" mapstructure:"model"`
    RateLimit int     `yaml:"rate_limit" mapstructure:"rate_limit"`
    APIKey    *string `yaml:"api_key,omitempty" mapstructure:"api_key"`
    APIKeyEnv string  `yaml:"api_key_env" mapstructure:"api_key_env"`
}

type EmbeddingsConfig struct {
    Enabled    bool    `yaml:"enabled" mapstructure:"enabled"`
    Provider   string  `yaml:"provider" mapstructure:"provider"`
    Model      string  `yaml:"model" mapstructure:"model"`
    Dimensions int     `yaml:"dimensions" mapstructure:"dimensions"`
    APIKey     *string `yaml:"api_key,omitempty" mapstructure:"api_key"`
    APIKeyEnv  string  `yaml:"api_key_env" mapstructure:"api_key_env"`
}
```

### Optional Fields

Pointer types (`*string`) are used for optional/nullable values:
- `APIKey` fields use `*string` - nil means "use environment variable fallback"
- When nil, code should check the corresponding `APIKeyEnv` environment variable

### Struct Tags

Both `yaml` and `mapstructure` tags are required:
- `mapstructure` - Required for `viper.Unmarshal()` to populate the struct
- `yaml` - Required for direct `yaml.Marshal()` when writing config

## Default Values

### Constants

Exported constants provide individual default values for CLI help text and validation:

```go
const (
    DefaultLogLevel = "info"
    DefaultLogFile  = "~/.config/memorizer/memorizer.log"

    DefaultDaemonHTTPPort        = 7600
    DefaultDaemonHTTPBind        = "127.0.0.1"
    DefaultDaemonShutdownTimeout = 30
    DefaultDaemonPIDFile         = "~/.config/memorizer/daemon.pid"
    DefaultDaemonRegistryPath    = "~/.config/memorizer/registry.db"
    DefaultDaemonMetricsInterval = 60

    DefaultGraphHost           = "localhost"
    DefaultGraphPort           = 6379
    DefaultGraphName           = "memorizer"
    DefaultGraphPasswordEnv    = "MEMORIZER_GRAPH_PASSWORD"
    DefaultGraphMaxRetries     = 3
    DefaultGraphRetryDelayMs   = 1000
    DefaultGraphWriteQueueSize = 1000

    DefaultSemanticProvider  = "anthropic"
    DefaultSemanticModel     = "claude-sonnet-4-5-20250929"
    DefaultSemanticRateLimit = 50
    DefaultSemanticAPIKeyEnv = "ANTHROPIC_API_KEY"

    DefaultEmbeddingsEnabled    = true
    DefaultEmbeddingsProvider   = "openai"
    DefaultEmbeddingsModel      = "text-embedding-3-large"
    DefaultEmbeddingsDimensions = 3072
    DefaultEmbeddingsAPIKeyEnv  = "OPENAI_API_KEY"
)
```

### Factory Function

```go
func NewDefaultConfig() Config {
    return Config{
        LogLevel: DefaultLogLevel,
        LogFile:  DefaultLogFile,
        Daemon: DaemonConfig{
            HTTPPort:        DefaultDaemonHTTPPort,
            // ... uses constants internally
        },
        // ... all sections populated from constants
    }
}
```

## API Key Handling

### Storage Strategy

API keys can be stored directly in config.yaml. The initialize TUI reads from environment variables and writes the actual key value to the config file.

### Resolution Order

Code that needs API keys should:
1. Check `semantic.api_key` (or `embeddings.api_key`) in config
2. If nil/empty, check the environment variable specified in `api_key_env`

```go
func (c *SemanticConfig) ResolveAPIKey() string {
    if c.APIKey != nil && *c.APIKey != "" {
        return *c.APIKey
    }
    return os.Getenv(c.APIKeyEnv)
}
```

### Security

Config files containing API keys must be written with 0600 permissions (owner read/write only).

## Config Loading (Read Path)

### Flow

1. Viper reads config file from search paths
2. Viper merges environment variables (MEMORIZER_ prefix)
3. `viper.Unmarshal()` populates the typed Config struct
4. Consumers access configuration via the struct, not viper directly

### Validation

Configuration is validated on load. If validation fails:
- Startup fails fast with detailed error message
- No partial loading or default substitution for invalid values

### No Config File Behavior

If no config.yaml exists:
- Daemon startup fails with error directing user to run `memorizer initialize`
- The application requires explicit configuration due to external API costs

## Config Writing (Write Path)

### Implementation

Direct YAML marshaling of the Config struct (bypasses viper):

```go
func Write(cfg Config, path string) error {
    // Ensure directory exists with 0700 permissions
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0700); err != nil {
        return fmt.Errorf("failed to create config directory; %w", err)
    }

    data, err := yaml.Marshal(cfg)
    if err != nil {
        return fmt.Errorf("failed to marshal config; %w", err)
    }

    // Write with restricted permissions (API keys may be present)
    return os.WriteFile(path, data, 0600)
}
```

### Initialize Integration

The initialize TUI will:
1. Build a Config struct directly in memory
2. Call `config.Write(cfg, path)` to serialize
3. Write includes all fields with defaults (user sees complete config)

### Existing Config Handling

If `memorizer initialize` finds an existing config.yaml:
- Exit with error explaining config already exists
- `--force` flag allows overwriting existing config

## Hot Reload

### Reloadable Sections

Only certain sections support hot reload without daemon restart:
- `log_level`, `log_file` - Logging configuration
- `semantic.*` - Semantic provider settings
- `embeddings.*` - Embeddings provider settings

Non-reloadable sections (require daemon restart):
- `daemon.*` - Ports, bind addresses, paths
- `graph.*` - Database connection parameters

### Event Bus Integration

Config reload notifications use the existing event bus (`internal/events/`).

New event type:
```go
const ConfigReloaded EventType = "config.reloaded"
```

Payload structure:
```go
type ConfigReloadedPayload struct {
    Config          Config   // Full new config struct (atomic delivery)
    ChangedSections []string // e.g., ["logging", "semantic"]
}
```

### Reload Failure

If hot reload fails (invalid YAML, validation error):
1. Previous config is retained
2. `ConfigReloadFailed` event is published with error details
3. Daemon continues operating with previous config

```go
const ConfigReloadFailed EventType = "config.reload_failed"

type ConfigReloadFailedPayload struct {
    Error string
}
```

## CLI Commands

### Validate Command

New command: `memorizer config validate`

- Validates config.yaml syntax and structure
- Reports validation errors without starting daemon
- Does NOT check environment variables (config syntax only)

### Initialize Command

Updated behavior:
- Exits with error if config.yaml already exists
- `--force` flag overwrites existing config
- Writes full config with all defaults

## Provider Identifiers

Semantic and embeddings providers use technical identifiers matching SDK/API naming:
- `anthropic` (not "claude")
- `openai`
- `google` (not "gemini")

The initialize TUI displays user-friendly names but stores technical identifiers.

## Migration

### No Automatic Migration

There is no automatic migration from old config structures. Users with existing configs from before this refactor must:
1. Delete or rename existing config.yaml
2. Run `memorizer initialize` to create new config
3. Or use `memorizer initialize --force` to overwrite

### No Config Versioning

Config files do not include version fields. Schema changes are handled by requiring re-initialization rather than migration.

## Implementation Tasks

1. **Define typed Config struct** with all nested types and struct tags
2. **Add default constants** and `NewDefaultConfig()` factory
3. **Implement `config.Write()`** with directory creation and 0600 permissions
4. **Add `config.Validate()`** for structural validation
5. **Implement API key resolution** methods on SemanticConfig/EmbeddingsConfig
6. **Add config.reloaded and config.reload_failed event types** to events package
7. **Update hot reload** to publish events and respect reloadable section boundaries
8. **Add `memorizer config validate` command**
9. **Update `memorizer initialize`** to:
   - Check for existing config (exit with error unless --force)
   - Build Config struct directly
   - Use `config.Write()` for serialization
   - Update provider identifiers to technical names
10. **Update all config consumers** to use typed struct instead of `config.GetString()`
11. **Remove obsolete defaults** (handlers, watcher, cache, providers sections)
12. **Update tests** for new config structure
