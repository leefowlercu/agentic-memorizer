# Configuration Subsystem Refactor - Implementation Plan

## Overview

This plan implements a comprehensive refactor of the configuration subsystem to:
1. Add type-safe configuration structs with compile-time safety
2. Align the initialize TUI output with the config package's expected structure
3. Centralize config writing with proper validation and permissions
4. Integrate hot reload notifications with the event bus
5. Update all config consumers to use the typed interface

**Note**: This application is pre-release with no users. Backwards compatibility is not maintained. Old config files, deprecated constants, and migration paths are not supported. Users must re-run `memorizer initialize` after this refactor.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Phase 1: Typed Config Foundation](#phase-1-typed-config-foundation)
- [Phase 2: Config Reading and Validation](#phase-2-config-reading-and-validation)
- [Phase 3: Config Writing](#phase-3-config-writing)
- [Phase 4: Event Bus Integration](#phase-4-event-bus-integration)
- [Phase 5: Consumer Migration](#phase-5-consumer-migration)
- [Phase 6: Initialize TUI Update](#phase-6-initialize-tui-update)
- [Phase 7: CLI Commands](#phase-7-cli-commands)
- [Phase 8: Cleanup and Testing](#phase-8-cleanup-and-testing)
- [Files to Modify/Create](#files-to-modifycreate)
- [Testing Strategy](#testing-strategy)
- [Potential Risks & Mitigations](#potential-risks--mitigations)
- [Implementation Checklist](#implementation-checklist)

## Prerequisites

Before starting implementation:

1. **Backup any existing config.yaml** - The refactor changes the schema
2. **Ensure clean git state** - Commit or stash any pending changes
3. **Review SPEC.md** - Familiarize with all requirements and decisions
4. **Add gopkg.in/yaml.v3 dependency** - Required for direct YAML marshaling

```bash
go get gopkg.in/yaml.v3
```

## Phase 1: Typed Config Foundation

**Objective**: Define the typed configuration structs and default constants.

### Steps

1.1. Create `internal/config/types.go` with all configuration structs:
   - `Config` (root struct)
   - `DaemonConfig` with nested `MetricsConfig`
   - `GraphConfig`
   - `SemanticConfig`
   - `EmbeddingsConfig`
   - Include both `yaml` and `mapstructure` struct tags
   - Use `*string` for optional API key fields

1.2. Update `internal/config/defaults.go`:
   - Replace existing constants with new naming convention (`DefaultDaemonHTTPPort`, etc.)
   - Add constants for new sections (semantic, embeddings)
   - Implement `NewDefaultConfig() Config` factory function

1.3. Add API key resolution methods:
   - `(c *SemanticConfig) ResolveAPIKey() string`
   - `(c *EmbeddingsConfig) ResolveAPIKey() string`

### Expected Outcome

- New `types.go` file with complete struct definitions
- Updated `defaults.go` with new constants and factory function
- All structs compile with correct tags
- Unit tests for `NewDefaultConfig()` and `ResolveAPIKey()` methods

### Verification

```bash
go build ./internal/config/...
go test ./internal/config/... -run TestNewDefaultConfig
go test ./internal/config/... -run TestResolveAPIKey
```

## Phase 2: Config Reading and Validation

**Objective**: Implement typed config loading with validation.

### Steps

2.1. Add `internal/config/load.go`:
   - `Load() (*Config, error)` - Load and unmarshal config into typed struct
   - `LoadFromPath(path string) (*Config, error)` - Load from specific path
   - Internal `unmarshalConfig(v *viper.Viper) (*Config, error)`

2.2. Add `internal/config/validate.go`:
   - `Validate(cfg *Config) error` - Validate config structure
   - Validation rules:
     - Port numbers in valid range (1-65535)
     - Required strings not empty
     - Provider names are recognized
   - Return detailed error messages for each validation failure

2.3. Update `internal/config/config.go`:
   - Add package-level `var currentConfig *Config`
   - Modify `Init()` to call `Load()` and store result
   - Add `Get() *Config` accessor for typed config
   - Add `MustGet() *Config` that panics if config not initialized
   - Remove `GetString()`, `GetInt()`, `GetBool()`, `GetPath()` accessors

2.4. Handle missing config file:
   - In `Init()`, if config file not found AND not in "initialize" context:
     - Return error directing user to run `memorizer initialize`
   - Add `InitWithDefaults() error` for contexts that allow defaults-only

### Expected Outcome

- Typed config loading via `config.Get()`
- Validation on load with clear error messages
- No string-based accessors (all consumers use typed access)

### Verification

```bash
go test ./internal/config/... -run TestLoad
go test ./internal/config/... -run TestValidate
go test ./internal/config/... -run TestInit
```

## Phase 3: Config Writing

**Objective**: Implement centralized config writing with proper permissions.

### Steps

3.1. Add `internal/config/write.go`:
   - `Write(cfg *Config, path string) error`
   - Create directory with 0700 permissions if missing
   - Marshal struct to YAML using `gopkg.in/yaml.v3`
   - Write file with 0600 permissions
   - Add file header comment with generation timestamp

3.2. Add `internal/config/path.go` (refactor from config.go):
   - `DefaultConfigPath() string` - Returns `~/.config/memorizer/config.yaml`
   - `ConfigDir() string` - Returns `~/.config/memorizer/`
   - `EnsureConfigDir() error` - Creates config directory
   - `ConfigExists() bool` - Checks if config file exists

3.3. Add tests for write functionality:
   - Test directory creation
   - Test file permissions (0600)
   - Test YAML output format
   - Test round-trip (write then read)

### Expected Outcome

- `config.Write()` function creates properly formatted config files
- File permissions are correct (0600 for file, 0700 for directory)
- YAML output is clean and readable

### Verification

```bash
go test ./internal/config/... -run TestWrite
go test ./internal/config/... -run TestConfigExists
```

## Phase 4: Event Bus Integration

**Objective**: Add config reload events to the event bus.

### Steps

4.1. Update `internal/events/events.go`:
   - Add `ConfigReloaded EventType = "config.reloaded"`
   - Add `ConfigReloadFailed EventType = "config.reload_failed"`

4.2. Create `internal/config/events.go`:
   - Define `ConfigReloadedPayload` struct
   - Define `ConfigReloadFailedPayload` struct
   - Helper function `detectChangedSections(old, new *Config) []string`

4.3. Update `internal/config/signals.go`:
   - Modify `Reload()` to:
     - Store previous config
     - Attempt reload into new config
     - On success: publish `ConfigReloaded` event with changed sections
     - On failure: publish `ConfigReloadFailed` event, retain previous config
   - Add `SetEventBus(bus events.Bus)` to inject event bus dependency

4.4. Add reloadable section logic:
   - Define which sections are reloadable: logging, semantic, embeddings
   - Log warning if non-reloadable sections changed (daemon, graph)

### Expected Outcome

- Hot reload publishes events via event bus
- Changed sections are detected and reported
- Failed reloads retain previous config and publish failure event

### Verification

```bash
go test ./internal/config/... -run TestReload
go test ./internal/config/... -run TestDetectChangedSections
```

## Phase 5: Consumer Migration

**Objective**: Update all config consumers to use typed struct.

### Steps

5.1. Update daemon commands (`cmd/daemon/subcommands/`):
   - `start.go`: Replace `config.GetInt("daemon.http_port")` with `config.Get().Daemon.HTTPPort`
   - `stop.go`: Replace `config.GetPath("daemon.pid_file")` with `config.Get().Daemon.PIDFile`
   - `status.go`: Same pattern
   - `rebuild.go`: Same pattern

5.2. Update file commands (`cmd/remember/`, `cmd/forget/`, `cmd/list/`):
   - Replace `config.GetPath("database.registry_path")` with `config.Get().Daemon.RegistryPath`

5.3. Update config commands (`cmd/config/subcommands/`):
   - `show.go`: Update to display typed config
   - `edit.go`: Use `config.DefaultConfigPath()`
   - `reset.go`: Use `config.DefaultConfigPath()`

5.4. Update internal packages:
   - `internal/daemon/orchestrator.go`: Use typed config
   - `internal/testutil/testutil.go`: Update test helper for new structure

5.5. Update root command (`cmd/root.go`):
   - Use typed config for log file path

### Expected Outcome

- All consumers use `config.Get().Section.Field` pattern
- No more string-based config access in application code
- Compile-time safety for config field names

### Verification

```bash
go build ./...
go test ./...
```

## Phase 6: Initialize TUI Update

**Objective**: Update the initialize workflow to use the new config system.

### Steps

6.1. Update `cmd/initialize/initialize.go`:
   - Add `--force` flag to allow overwriting existing config
   - Check `config.ConfigExists()` before running wizard
   - Exit with error if config exists (unless --force)
   - Replace `writeConfig()` to use `config.Write()`

6.2. Update TUI wizard to build Config struct:
   - Modify `internal/tui/initialize/wizard.go`:
     - Change from viper instance to `*config.Config` parameter
     - Steps modify the Config struct directly
   - Update Step interface if needed

6.3. Update wizard steps (`internal/tui/initialize/steps/`):
   - `falkordb.go`: Set `cfg.Graph.Host`, `cfg.Graph.Port`
   - `semantic_provider.go`:
     - Set `cfg.Semantic.Provider`, `cfg.Semantic.Model`, etc.
     - Use technical identifiers (anthropic, openai, google)
   - `embeddings.go`:
     - Set `cfg.Embeddings.*` fields
     - Use technical identifiers
   - `http_port.go`: Set `cfg.Daemon.HTTPPort`
   - `confirm.go`: Display summary from Config struct

6.4. Update step interface:
   - Change `Init(*viper.Viper) tea.Cmd` to `Init(*config.Config) tea.Cmd`
   - Change `Apply(*viper.Viper) error` to `Apply(*config.Config) error`
   - Update all step implementations

6.5. Write full config with defaults:
   - After wizard completes, merge user selections with `NewDefaultConfig()`
   - Call `config.Write(cfg, config.DefaultConfigPath())`

### Expected Outcome

- Initialize creates config.yaml with correct structure
- User-selected values merged with defaults
- Full config written (not just user-modified fields)
- `--force` flag works correctly

### Verification

```bash
# Test without existing config
rm -f ~/.config/memorizer/config.yaml
go run . initialize
cat ~/.config/memorizer/config.yaml

# Test with existing config (should fail)
go run . initialize
# Expected: error message

# Test with --force
go run . initialize --force
```

## Phase 7: CLI Commands

**Objective**: Add the config validate command.

### Steps

7.1. Create `cmd/config/subcommands/validate.go`:
   - Command: `memorizer config validate`
   - Load config from default path (or `--config` flag)
   - Run `config.Validate()`
   - Print success message or validation errors
   - Exit code 0 on success, 1 on validation failure

7.2. Update `cmd/config/config.go`:
   - Add `subcommands.ValidateCmd` to parent command

7.3. Add integration tests for validate command

### Expected Outcome

- `memorizer config validate` command works
- Reports detailed validation errors
- Exits with appropriate codes

### Verification

```bash
# Test with valid config
go run . config validate

# Test with invalid config (manually corrupt config.yaml)
go run . config validate
# Expected: validation errors
```

## Phase 8: Cleanup and Testing

**Objective**: Remove deprecated code and ensure comprehensive test coverage.

### Steps

8.1. Remove obsolete defaults from `internal/config/defaults.go`:
   - Remove `database.*` keys (moved to daemon.registry_path)
   - Remove `handlers.*` keys
   - Remove `watcher.*` keys
   - Remove `cache.*` keys
   - Remove `providers.*` keys
   - Remove old `setDefaults()` function

8.2. Remove string-based accessors:
   - Remove `GetString()`, `GetInt()`, `GetBool()`, `GetPath()` from config package
   - All consumers must use typed `Get().Section.Field` pattern

8.3. Update test files:
   - `internal/config/config_test.go`: Update for new structure
   - `internal/config/defaults_test.go`: Test new constants and factory
   - `internal/testutil/testutil.go`: Update TestEnv for new keys
   - Add integration tests for full config lifecycle

8.4. Update CLAUDE.md:
   - Document new config structure
   - Update any references to old config keys

8.5. Manual testing:
   - Run full initialize workflow
   - Start daemon with new config
   - Test hot reload with SIGHUP
   - Verify all commands work

### Expected Outcome

- No obsolete code remains
- All tests pass
- Full integration testing complete

### Verification

```bash
go test ./...
go build ./...

# Full workflow test
rm -f ~/.config/memorizer/config.yaml
go run . initialize
go run . daemon start &
go run . daemon status
kill -HUP $(cat ~/.config/memorizer/daemon.pid)
go run . daemon stop
```

## Files to Modify/Create

### Phase 1: Types and Defaults
| File | Action |
|------|--------|
| `internal/config/types.go` | **Create** - Typed config structs |
| `internal/config/defaults.go` | **Modify** - New constants, factory |
| `internal/config/types_test.go` | **Create** - Tests for types |

### Phase 2: Loading and Validation
| File | Action |
|------|--------|
| `internal/config/load.go` | **Create** - Load functions |
| `internal/config/validate.go` | **Create** - Validation logic |
| `internal/config/config.go` | **Modify** - Add typed accessors |
| `internal/config/load_test.go` | **Create** - Load tests |
| `internal/config/validate_test.go` | **Create** - Validation tests |

### Phase 3: Writing
| File | Action |
|------|--------|
| `internal/config/write.go` | **Create** - Write functions |
| `internal/config/path.go` | **Create** - Path helpers |
| `internal/config/write_test.go` | **Create** - Write tests |

### Phase 4: Events
| File | Action |
|------|--------|
| `internal/events/events.go` | **Modify** - Add event types |
| `internal/config/events.go` | **Create** - Event payloads |
| `internal/config/signals.go` | **Modify** - Event publishing |

### Phase 5: Consumer Migration
| File | Action |
|------|--------|
| `cmd/daemon/subcommands/start.go` | **Modify** |
| `cmd/daemon/subcommands/stop.go` | **Modify** |
| `cmd/daemon/subcommands/status.go` | **Modify** |
| `cmd/daemon/subcommands/rebuild.go` | **Modify** |
| `cmd/remember/remember.go` | **Modify** |
| `cmd/forget/forget.go` | **Modify** |
| `cmd/list/list.go` | **Modify** |
| `cmd/config/subcommands/show.go` | **Modify** |
| `cmd/config/subcommands/edit.go` | **Modify** |
| `cmd/config/subcommands/reset.go` | **Modify** |
| `cmd/root.go` | **Modify** |
| `internal/daemon/orchestrator.go` | **Modify** |
| `internal/testutil/testutil.go` | **Modify** |

### Phase 6: Initialize TUI
| File | Action |
|------|--------|
| `cmd/initialize/initialize.go` | **Modify** |
| `internal/tui/initialize/wizard.go` | **Modify** |
| `internal/tui/initialize/steps/step.go` | **Modify** - Interface change |
| `internal/tui/initialize/steps/falkordb.go` | **Modify** |
| `internal/tui/initialize/steps/semantic_provider.go` | **Modify** |
| `internal/tui/initialize/steps/embeddings.go` | **Modify** |
| `internal/tui/initialize/steps/http_port.go` | **Modify** |
| `internal/tui/initialize/steps/confirm.go` | **Modify** |

### Phase 7: CLI Commands
| File | Action |
|------|--------|
| `cmd/config/subcommands/validate.go` | **Create** |
| `cmd/config/config.go` | **Modify** |

### Phase 8: Cleanup
| File | Action |
|------|--------|
| `internal/config/defaults.go` | **Modify** - Remove obsolete |
| `internal/config/config_test.go` | **Modify** |
| `internal/testutil/testutil.go` | **Modify** |
| `CLAUDE.md` | **Modify** |

## Testing Strategy

### Unit Tests

Each phase adds unit tests for new functionality:
- **Phase 1**: `TestNewDefaultConfig`, `TestResolveAPIKey`
- **Phase 2**: `TestLoad`, `TestValidate`, `TestInit`
- **Phase 3**: `TestWrite`, `TestConfigExists`, `TestRoundTrip`
- **Phase 4**: `TestReload`, `TestDetectChangedSections`, `TestReloadEvents`
- **Phase 7**: `TestValidateCommand`

### Integration Tests

After Phase 6:
- Full initialize workflow test
- Config write/read round-trip
- Daemon startup with new config

### Manual Testing Checklist

- [ ] `memorizer initialize` creates valid config
- [ ] `memorizer initialize --force` overwrites existing
- [ ] `memorizer daemon start` works with new config
- [ ] `memorizer daemon status` shows correct values
- [ ] `memorizer config show` displays typed config
- [ ] `memorizer config validate` validates config
- [ ] Hot reload (SIGHUP) publishes events
- [ ] Environment variable overrides work

## Potential Risks & Mitigations

### Risk 1: Breaking Existing Configs
**Impact**: Low - Pre-release application with no users
**Mitigation**:
- Initialize command fails fast if old config detected
- `--force` flag allows explicit overwrite
- Users re-run `memorizer initialize` after refactor

### Risk 2: Consumer Migration Errors
**Impact**: Medium - Typos in field names cause runtime errors
**Mitigation**:
- Typed struct provides compile-time safety
- Comprehensive tests for each consumer
- Phase-by-phase verification

### Risk 3: Event Bus Dependency Injection
**Impact**: Low - Complexity in wiring event bus to config package
**Mitigation**:
- Use SetEventBus() injection pattern
- Default to no-op if event bus not set
- Test with and without event bus

### Risk 4: YAML Marshal Differences
**Impact**: Low - gopkg.in/yaml.v3 may format differently than viper
**Mitigation**:
- Test YAML output format
- Verify round-trip compatibility
- Add header comment for clarity

### Risk 5: TUI Wizard Interface Change
**Impact**: Medium - Changing Step interface affects all steps
**Mitigation**:
- Change interface in single commit
- Update all steps atomically
- Test each step individually

## Implementation Checklist

### Phase 1: Typed Config Foundation
- [x] Create `internal/config/types.go` with Config struct
- [x] Create `internal/config/types.go` with DaemonConfig struct
- [x] Create `internal/config/types.go` with MetricsConfig struct
- [x] Create `internal/config/types.go` with GraphConfig struct
- [x] Create `internal/config/types.go` with SemanticConfig struct
- [x] Create `internal/config/types.go` with EmbeddingsConfig struct
- [x] Add yaml and mapstructure struct tags to all fields
- [x] Update `internal/config/defaults.go` with new constant names
- [x] Add constants for semantic section
- [x] Add constants for embeddings section
- [x] Implement `NewDefaultConfig()` factory function
- [x] Implement `SemanticConfig.ResolveAPIKey()` method
- [x] Implement `EmbeddingsConfig.ResolveAPIKey()` method
- [x] Create `internal/config/types_test.go`
- [x] Test `NewDefaultConfig()` returns expected values
- [x] Test `ResolveAPIKey()` methods
- [x] Run `go build ./internal/config/...`
- [x] Run `go test ./internal/config/...`

### Phase 2: Config Reading and Validation
- [x] Create `internal/config/load.go`
- [x] Implement `Load() (*Config, error)`
- [x] Implement `LoadFromPath(path string) (*Config, error)`
- [x] Create `internal/config/validate.go`
- [x] Implement `Validate(cfg *Config) error`
- [x] Add port range validation
- [x] Add required field validation
- [x] Add provider name validation
- [x] Update `internal/config/config.go` with `currentConfig` variable
- [x] Add `Get() *Config` accessor
- [x] Add `MustGet() *Config` accessor
- [x] Modify `Init()` to use typed loading
- [x] Add `InitWithDefaults() error` for default-only contexts
- [x] Handle missing config file with helpful error
- [x] Create `internal/config/load_test.go`
- [x] Create `internal/config/validate_test.go`
- [x] Run `go test ./internal/config/...`

### Phase 3: Config Writing
- [x] Add `gopkg.in/yaml.v3` dependency
- [x] Create `internal/config/write.go`
- [x] Implement `Write(cfg *Config, path string) error`
- [x] Add directory creation with 0700 permissions
- [x] Add file writing with 0600 permissions
- [x] Add header comment with timestamp
- [x] Create `internal/config/path.go`
- [x] Implement `DefaultConfigPath() string`
- [x] Implement `ConfigDir() string`
- [x] Implement `EnsureConfigDir() error`
- [x] Implement `ConfigExists() bool`
- [x] Create `internal/config/write_test.go`
- [x] Test directory creation
- [x] Test file permissions
- [x] Test YAML format
- [x] Test write/read round-trip
- [x] Run `go test ./internal/config/...`

### Phase 4: Event Bus Integration
- [x] Update `internal/events/events.go` with ConfigReloaded event type
- [x] Update `internal/events/events.go` with ConfigReloadFailed event type
- [x] Create `internal/config/events.go`
- [x] Define `ConfigReloadedPayload` struct
- [x] Define `ConfigReloadFailedPayload` struct
- [x] Implement `detectChangedSections(old, new *Config) []string`
- [x] Update `internal/config/signals.go`
- [x] Add `SetEventBus(bus events.Bus)` function
- [x] Modify `Reload()` to publish ConfigReloaded on success
- [x] Modify `Reload()` to publish ConfigReloadFailed on failure
- [x] Retain previous config on reload failure
- [x] Add warning log for non-reloadable section changes
- [x] Create/update tests for reload events
- [x] Run `go test ./internal/config/...`

### Phase 5: Consumer Migration
- [x] Update `cmd/daemon/subcommands/start.go`
- [x] Update `cmd/daemon/subcommands/stop.go`
- [x] Update `cmd/daemon/subcommands/status.go`
- [x] Update `cmd/daemon/subcommands/rebuild.go`
- [x] Update `cmd/remember/remember.go`
- [x] Update `cmd/forget/forget.go`
- [x] Update `cmd/list/list.go`
- [x] Update `cmd/config/subcommands/show.go` (no changes needed - uses GetConfigPath(), GetAllSettings())
- [x] Update `cmd/config/subcommands/edit.go` (no changes needed - uses GetConfigPath(), EnsureConfigDir())
- [x] Update `cmd/config/subcommands/reset.go` (no changes needed - uses GetConfigPath())
- [x] Update `cmd/root.go`
- [x] Update `internal/daemon/orchestrator.go`
- [x] Update `internal/testutil/testutil.go`
- [x] Run `go build ./...`
- [x] Run `go test ./...`

### Phase 6: Initialize TUI Update
- [x] Update `cmd/initialize/initialize.go` - Add `--force` flag (already existed)
- [x] Update `cmd/initialize/initialize.go` - Check ConfigExists()
- [x] Update `cmd/initialize/initialize.go` - Use config.Write()
- [x] Update Step interface in `internal/tui/initialize/steps/step.go`
- [x] Update `internal/tui/initialize/wizard.go` for Config struct
- [x] Update `internal/tui/initialize/steps/falkordb.go`
- [x] Update `internal/tui/initialize/steps/semantic_provider.go`
- [x] Update `internal/tui/initialize/steps/embeddings.go`
- [x] Update `internal/tui/initialize/steps/http_port.go`
- [x] Update `internal/tui/initialize/steps/confirm.go`
- [x] Use technical provider identifiers (claude, openai, gemini - already used)
- [ ] Test initialize without existing config
- [ ] Test initialize with existing config (should fail)
- [ ] Test initialize with --force flag
- [ ] Verify generated config.yaml structure

### Phase 7: CLI Commands
- [x] Create `cmd/config/subcommands/validate.go`
- [x] Implement validate command
- [x] Add ValidateCmd to `cmd/config/config.go`
- [ ] Test validate with valid config
- [ ] Test validate with invalid config
- [ ] Verify exit codes

### Phase 8: Cleanup and Testing
- [x] Remove `database.*` defaults (not applicable - never existed)
- [x] Remove `handlers.*` defaults (not applicable - never existed)
- [x] Remove `watcher.*` defaults (not applicable - never existed)
- [x] Remove `cache.*` defaults (not applicable - never existed)
- [x] Remove `providers.*` defaults (not applicable - never existed)
- [x] Remove old `setDefaults()` function (retained - still used by Init())
- [x] Remove string-based accessors (`GetString`, `GetInt`, `GetBool`, `GetPath`)
- [x] Update `internal/config/config_test.go`
- [x] Update `internal/testutil/testutil.go` (done in Phase 5)
- [x] Update CLAUDE.md with new config structure
- [x] Run full test suite: `go test ./...`
- [ ] Manual test: full initialize workflow
- [ ] Manual test: daemon start/stop
- [ ] Manual test: hot reload with SIGHUP
- [ ] Manual test: all CLI commands
