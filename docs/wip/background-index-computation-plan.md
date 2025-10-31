# Background Index Precomputation - Implementation Plan

## Executive Summary

This document outlines a comprehensive implementation plan for moving heavy metadata extraction and semantic analysis operations out of the SessionStart critical path by introducing a background daemon/watcher system. The goal is to achieve predictable, near-instantaneous startup times (<50ms) regardless of the number of new or modified files in the memory directory.

**Architecture:**
- **Read command** (`agentic-memorizer read`): Fast read-only operation that formats and outputs the computed index
- **Daemon commands** (`agentic-memorizer daemon start/stop/...`): Background process management for maintaining the computed index
- **Init command** (`agentic-memorizer init`): Configuration initialization (unchanged)

**Key Benefits:**
- Predictable startup latency (target: <50ms, even with many file changes)
- Better user experience (no blocking during Claude Code startup)
- Richer incremental updates (daemon can track file changes continuously)
- Reduced API costs (smarter batching and rate limiting)
- Clean separation of concerns (indexing vs. formatting)
- Foundation for future enhancements (real-time indexing, prioritization)

**Complexity:** High
**Breaking Changes:** Yes - indexing moved to daemon, new `read` command for output (hooks must be updated)

---

## Current State Analysis

### Current Architecture

The agentic-memorizer currently operates in a **synchronous, on-demand** model:

```
SessionStart Hook Triggered
    ↓
Walk File System (walker.Walk)
    ↓
For Each File:
    • Extract Metadata (internal/metadata)
    • Compute File Hash (SHA256)
    • Check Cache (internal/cache)
    • If cache miss or stale:
        ◦ Perform Semantic Analysis (Claude API call)
        ◦ Cache Results
    ↓
Build Index Structure (types.Index)
    ↓
Format Output (XML/Markdown)
    ↓
Return to Hook
```

### Performance Characteristics

**Warm Start** (all files cached):
- File system walk: ~50-100ms
- Cache lookups: ~50-100ms
- Index formatting: ~20-50ms
- **Total: ~120-250ms**

**Cold Start** (10 new files):
- File system walk: ~50-100ms
- Semantic analysis: ~1-1.5s per file × 10 = 10-15s
- Cache writes: ~50ms
- Index formatting: ~20-50ms
- **Total: ~10-15s**

**Mixed Scenario** (100 files, 5 new):
- File system walk: ~100-200ms
- Cache hits: ~100ms
- Semantic analysis: ~1-1.5s per file × 5 = 5-7.5s
- **Total: ~5.5-8s**

### Problem Statement

The current architecture has several limitations:

1. **Unpredictable Startup Latency**: Startup time varies from 200ms to 15+ seconds depending on cache state
2. **Blocking SessionStart**: All work happens in the critical startup path, delaying Claude Code availability
3. **Poor User Experience**: Users experience delays when new files are added or modified
4. **Inefficient Resource Usage**: No batching or rate limiting of API calls
5. **No Incremental Updates**: Full rebuild on every session start, even if nothing changed
6. **Reactive Processing**: Only processes files when Claude Code starts, not when files change

---

## Proposed Solution Architecture

### High-Level Design

Introduce a **background daemon** that continuously monitors the memory directory and maintains a **computed index**. The SessionStart hook becomes a simple read operation from the computed index.

```
┌─────────────────────────────────────────────────────┐
│              Background Daemon                      │
│                                                     │
│  ┌────────────┐      ┌─────────────┐                │
│  │   Watch    │────▶ │    Queue    │                │
│  │  Memory    │      │  (Buffered) │                │
│  │    Dir     │      └──────┬──────┘                │
│  └────────────┘             │                       │
│                             ▼                       │
│                    ┌─────────────────┐              │
│                    │   Worker Pool   │              │
│                    │  (Configurable) │              │
│                    └────────┬────────┘              │
│                             │                       │
│                             ▼                       │
│                  ┌─────────────────────┐            │
│                  │    Computed Index   │            │
│                  │   (Atomic Writes)   │            │
│                  └─────────────────────┘            │
└─────────────────────────────────────────────────────┘
                             │
                             │ Read (Fast)
                             ▼
                   ┌──────────────────┐
                   │  SessionStart    │
                   │      Hook        │
                   │  (<50ms target)  │
                   └──────────────────┘
```

### Architecture Components

#### 1. Background Daemon (`daemon` package)

**Responsibilities:**
- File system watching (using `fsnotify`)
- Change event batching and debouncing
- Worker pool management
- Index rebuilding and atomic updates
- Graceful shutdown
- Health monitoring

**Key Features:**
- Configurable debounce period (e.g., 500ms) to batch rapid changes
- Respects existing `analysis.parallel` configuration
- Rate limiting for API calls
- Crash recovery and restart capability
- PID file management
- Signal handling (SIGTERM, SIGINT, SIGHUP for reload)

#### 2. Computed Index Storage (`index` package)

**Format:** JSON file with atomic writes

**Location:** `~/.agentic-memorizer/index.json`

**Structure:**
```json
{
  "version": "1.0",
  "generated_at": "2025-10-30T14:30:22Z",
  "daemon_version": "0.5.0",
  "index": {
    "generated": "2025-10-30T14:30:22Z",
    "root": "/Users/username/.agentic-memorizer/memory",
    "entries": [...],
    "stats": {...}
  },
  "metadata": {
    "build_duration_ms": 1250,
    "files_processed": 15,
    "cache_hits": 12,
    "api_calls": 3
  }
}
```

**Atomic Write Strategy:**
- Write to temp file: `index.json.tmp`
- Sync to disk
- Atomic rename to `index.json`
- Ensures readers always see complete, valid index

#### 3. CLI Fast Path (`cmd/read.go` - read command)

**Command:** `agentic-memorizer read`

**Behavior:**

The read command is simple and fast - it only reads the computed index, formats it, and outputs it. No on-demand indexing.

```go
func runMemorizer(cmd *cobra.Command, args []string) error {
    cfg, err := config.GetConfig()
    if err != nil {
        return fmt.Errorf("failed to load config; %w", err)
    }

    // Load computed index
    indexPath := filepath.Join(cfg.MemoryRoot, "..", "index.json")
    index, err := loadComputedIndex(indexPath)

    if err != nil {
        // Index doesn't exist - create empty index with warning
        index = createEmptyIndex(cfg.MemoryRoot)

        if cfg.Output.WrapJSON {
            // Warning will be in systemMessage
            return formatAndOutputWithWarning(index, cfg, "Index not found. Start daemon to build index.")
        } else {
            // Output warning to stdout
            fmt.Fprintf(os.Stderr, "Warning: Index not found. Start daemon with 'agentic-memorizer daemon start' to build index.\n\n")
            return formatAndOutput(index, cfg)
        }
    }

    // Format and return the computed index
    return formatAndOutput(index, cfg)
}
```

**Performance Target:** <50ms end-to-end

---

## Technical Design

### Component 1: File System Watcher

**Package:** `internal/watcher`

**Implementation:**

```go
package watcher

import (
    "github.com/fsnotify/fsnotify"
    "time"
)

type Watcher struct {
    fsWatcher     *fsnotify.Watcher
    eventQueue    chan FileEvent
    debounceTimer *time.Timer
    debouncePeriod time.Duration
    skipDirs      []string
    skipFiles     []string
}

type FileEvent struct {
    Path      string
    Operation Operation
    Timestamp time.Time
}

type Operation int

const (
    Create Operation = iota
    Modify
    Delete
    Rename
)

func NewWatcher(root string, debouncePeriod time.Duration, skipDirs, skipFiles []string) (*Watcher, error)
func (w *Watcher) Start() error
func (w *Watcher) Stop() error
func (w *Watcher) Events() <-chan []FileEvent  // Returns batched events
```

**Key Features:**
- Recursive directory watching
- Event debouncing (configurable, default 500ms)
- Batched event delivery
- Automatic re-watch on new directories
- Skip hidden files and configured exclusions

**Debouncing Strategy:**
```
File Modified Event Received
    ↓
Add to Pending Events Map (deduplicated by path)
    ↓
Reset Debounce Timer (500ms)
    ↓
[Wait for timer or more events]
    ↓
Timer Expires
    ↓
Flush Batched Events to Queue
```

### Component 2: Background Daemon

**Package:** `internal/daemon`

**Main Structure:**

```go
package daemon

type Daemon struct {
    config            *config.Config
    watcher           *watcher.Watcher
    workerPool        *WorkerPool
    indexManager      *index.Manager
    cacheManager      *cache.Manager
    metadataExtractor *metadata.Extractor
    semanticAnalyzer  *semantic.Analyzer
    shutdown          chan struct{}
    pidFile           string
}

type Config struct {
    DebouncePeriod    time.Duration
    WorkerCount       int
    RateLimitPerMin   int
    FullRebuildInterval time.Duration  // Periodic full rebuild (default: 1 hour)
    HealthCheckPort   int
}

func NewDaemon(cfg *config.Config, daemonCfg *Config) (*Daemon, error)
func (d *Daemon) Start() error
func (d *Daemon) Stop() error
func (d *Daemon) Reload() error  // Reload configuration
```

**Worker Pool:**

```go
type WorkerPool struct {
    jobs         chan Job
    results      chan Result
    workerCount  int
    rateLimiter  *rate.Limiter
}

type Job struct {
    FilePath  string
    Operation watcher.Operation
}

type Result struct {
    Entry *types.IndexEntry
    Error error
}

func NewWorkerPool(count int, rateLimit int) *WorkerPool
func (wp *WorkerPool) Start()
func (wp *WorkerPool) Stop()
func (wp *WorkerPool) Submit(job Job)
func (wp *WorkerPool) Results() <-chan Result
```

**Processing Flow:**

```
Batched File Events Received
    ↓
For Each Event:
    Create Job (FilePath + Operation)
    ↓
Submit to Worker Pool Queue
    ↓
Worker Picks Up Job
    ↓
Rate Limiter (respects API limits)
    ↓
Extract Metadata
    ↓
Check Cache (by file hash)
    ↓
If Cache Miss/Stale:
    Semantic Analysis (Claude API)
    Update Cache
    ↓
Send Result to Results Channel
    ↓
Index Manager Receives Results
    ↓
Update In-Memory Index (atomic)
    ↓
Write Computed Index (atomic file write)
```

### Component 3: Index Manager

**Package:** `internal/index`

**Structure:**

```go
package index

type Manager struct {
    currentIndex  *types.Index
    indexPath     string
    indexLock     sync.RWMutex
}

type ComputedIndex struct {
    Version        string          `json:"version"`
    GeneratedAt    time.Time       `json:"generated_at"`
    DaemonVersion  string          `json:"daemon_version"`
    Index          *types.Index    `json:"index"`
    Metadata       BuildMetadata   `json:"metadata"`
}

type BuildMetadata struct {
    BuildDurationMs int   `json:"build_duration_ms"`
    FilesProcessed  int   `json:"files_processed"`
    CacheHits       int   `json:"cache_hits"`
    APICalls        int   `json:"api_calls"`
}

func NewManager(indexPath string) *Manager
func (m *Manager) Load() error
func (m *Manager) Update(entries []types.IndexEntry) error
func (m *Manager) UpdateSingle(entry types.IndexEntry) error
func (m *Manager) RemoveFile(path string) error
func (m *Manager) WriteAtomic() error
func (m *Manager) GetCurrent() *types.Index
```

**Atomic Write Implementation:**

```go
func (m *Manager) WriteAtomic() error {
    m.indexLock.RLock()
    defer m.indexLock.RUnlock()

    computed := ComputedIndex{
        Version:       "1.0",
        GeneratedAt:   time.Now(),
        DaemonVersion: version.Version,
        Index:         m.currentIndex,
        Metadata:      m.buildMetadata,
    }

    // Write to temp file
    tmpPath := m.indexPath + ".tmp"
    data, err := json.MarshalIndent(computed, "", "  ")
    if err != nil {
        return err
    }

    if err := os.WriteFile(tmpPath, data, 0644); err != nil {
        return err
    }

    // Sync to disk
    if f, err := os.Open(tmpPath); err == nil {
        f.Sync()
        f.Close()
    }

    // Atomic rename
    return os.Rename(tmpPath, m.indexPath)
}
```

### Component 4: Daemon Management

**Daemon Subcommands:**

```bash
# Start daemon (foreground)
agentic-memorizer daemon start

# Start daemon (background)
agentic-memorizer daemon start --detach

# Stop daemon
agentic-memorizer daemon stop

# Restart daemon
agentic-memorizer daemon restart

# Status check
agentic-memorizer daemon status

# Trigger immediate full rebuild
agentic-memorizer daemon rebuild

# Show daemon logs
agentic-memorizer daemon logs [--follow]
```

**PID File Management:**

Location: `~/.agentic-memorizer/daemon.pid`

```go
type PIDFile struct {
    path string
}

func NewPIDFile(path string) *PIDFile
func (p *PIDFile) Write(pid int) error
func (p *PIDFile) Read() (int, error)
func (p *PIDFile) Remove() error
func (p *PIDFile) IsRunning() bool
```

**Signal Handling:**

```go
func (d *Daemon) setupSignalHandling() {
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

    go func() {
        for sig := range sigChan {
            switch sig {
            case syscall.SIGTERM, syscall.SIGINT:
                d.Stop()
            case syscall.SIGHUP:
                d.Reload()
            }
        }
    }()
}
```

### Component 5: Read Command Implementation

**Implementation in `cmd/read/read.go`:**

The read command (`agentic-memorizer read`) is simple - read computed index, format, output. No fallback logic.

```go
func runMemorizer(cmd *cobra.Command, args []string) error {
    cfg, err := config.GetConfig()
    if err != nil {
        return fmt.Errorf("failed to load config; %w", err)
    }

    // Load computed index
    indexPath := getIndexPath(cfg)
    indexMgr := index.NewManager(indexPath)
    computed, err := indexMgr.LoadComputed()

    if err != nil {
        // Index doesn't exist - create empty index
        if cfg.Output.Verbose {
            fmt.Fprintf(os.Stderr, "Index file not found at %s\n", indexPath)
        }

        emptyIndex := createEmptyIndex(cfg.MemoryRoot)
        warningMsg := "Memory index not available. Start daemon with 'agentic-memorizer daemon start' to build index."

        return formatAndOutput(emptyIndex, cfg, warningMsg)
    }

    // Success - format and output the computed index
    if cfg.Output.Verbose {
        fmt.Fprintf(os.Stderr, "Using computed index (age: %s, %d files)\n",
            time.Since(computed.GeneratedAt),
            computed.Index.Stats.TotalFiles)
    }

    return formatAndOutput(computed.Index, cfg, "")
}

// Helper function that handles both normal and warning cases
func formatAndOutput(index *types.Index, cfg *config.Config, warningMsg string) error {
    formatter := output.NewFormatter(cfg.Output.Verbose, cfg.Output.ShowRecentDays)

    var content string
    switch cfg.Output.Format {
    case "xml":
        content = formatter.FormatXML(index)
    default:
        content = formatter.FormatMarkdown(index)
    }

    if cfg.Output.WrapJSON {
        systemMsg := ""
        if warningMsg != "" {
            systemMsg = warningMsg
        } else {
            systemMsg = formatter.GenerateSystemMessage(index)
        }
        jsonOutput, err := formatter.WrapJSONWithMessage(content, index, systemMsg)
        if err != nil {
            return fmt.Errorf("failed to wrap in JSON; %w", err)
        }
        fmt.Println(jsonOutput)
    } else {
        if warningMsg != "" {
            fmt.Fprintf(os.Stderr, "Warning: %s\n\n", warningMsg)
        }
        fmt.Print(content)
    }

    return nil
}

func createEmptyIndex(memoryRoot string) *types.Index {
    return &types.Index{
        Generated: time.Now(),
        Root:      memoryRoot,
        Entries:   []types.IndexEntry{},
        Stats: types.IndexStats{
            TotalFiles:    0,
            TotalSize:     0,
            AnalyzedFiles: 0,
            CachedFiles:   0,
            ErrorFiles:    0,
        },
    }
}

func getIndexPath(cfg *config.Config) string {
    // Index stored at ~/.agentic-memorizer/index.json
    configDir := filepath.Dir(cfg.MemoryRoot)
    return filepath.Join(configDir, "index.json")
}
```

**Flags:**

The read command uses only the existing flags (`--format`, `--wrap-json`, `--verbose`). No special flags needed.

---

## Configuration Changes

### New Configuration Section

Add to `config.yaml.example`:

```yaml
# Background Daemon Configuration
daemon:
  # Enable daemon mode
  enabled: false

  # Debounce period for file changes (milliseconds)
  debounce_ms: 500

  # Number of worker threads for processing
  workers: 3

  # Rate limit for API calls (per minute)
  rate_limit_per_min: 20

  # Full rebuild interval (even if no changes detected)
  full_rebuild_interval_minutes: 60

  # Health check HTTP server port (0 to disable)
  health_check_port: 0

  # Log file location
  log_file: ~/.agentic-memorizer/daemon.log

  # Log level (debug, info, warn, error)
  log_level: info
```

### Config Struct Updates

```go
// internal/config/types.go

type Config struct {
    MemoryRoot string         `mapstructure:"memory_root" yaml:"memory_root"`
    CacheDir   string         `mapstructure:"cache_dir" yaml:"cache_dir"`
    Claude     ClaudeConfig   `mapstructure:"claude" yaml:"claude"`
    Output     OutputConfig   `mapstructure:"output" yaml:"output"`
    Analysis   AnalysisConfig `mapstructure:"analysis" yaml:"analysis"`
    Daemon     DaemonConfig   `mapstructure:"daemon" yaml:"daemon"`  // NEW
}

type DaemonConfig struct {
    Enabled                    bool   `mapstructure:"enabled" yaml:"enabled"`
    DebounceMs                 int    `mapstructure:"debounce_ms" yaml:"debounce_ms"`
    Workers                    int    `mapstructure:"workers" yaml:"workers"`
    RateLimitPerMin            int    `mapstructure:"rate_limit_per_min" yaml:"rate_limit_per_min"`
    FullRebuildIntervalMinutes int    `mapstructure:"full_rebuild_interval_minutes" yaml:"full_rebuild_interval_minutes"`
    HealthCheckPort            int    `mapstructure:"health_check_port" yaml:"health_check_port"`
    LogFile                    string `mapstructure:"log_file" yaml:"log_file"`
    LogLevel                   string `mapstructure:"log_level" yaml:"log_level"`
}
```

---

## Implementation Phases

### Phase 1: Foundation

**Goals:**
- Set up basic daemon infrastructure
- Implement computed index storage
- No file watching yet (manual trigger only)

**Tasks:**

1. **Create Index Manager** (`internal/index`)
   - Implement `ComputedIndex` struct
   - Implement atomic write functionality
   - Add load/save methods
   - Write unit tests

2. **Add Daemon Config** (`internal/config`)
   - Add `DaemonConfig` struct
   - Update config loading
   - Add defaults
   - Update `config.yaml.example`

3. **Create Basic Daemon** (`internal/daemon`)
   - Implement `Daemon` struct
   - Add start/stop methods
   - PID file management
   - Signal handling (SIGTERM, SIGINT)

4. **Add Daemon Commands** (`cmd/daemon/`)
   - `daemon start` subcommand (foreground only)
   - `daemon stop` subcommand
   - `daemon status` subcommand
   - `daemon restart` subcommand
   - `daemon rebuild` subcommand
   - `daemon logs` subcommand

5. **Add Read Command** (`cmd/read/read.go`)
   - Implement read command to load computed index
   - Implement empty index creation for missing index.json
   - Add warning message handling

**Deliverables:**
- Manual daemon start/stop working
- Read command can read computed index
- Empty index with warning when index.json missing

**Testing:**
```bash
# Start daemon manually (foreground)
agentic-memorizer daemon start

# In another terminal, trigger index build
# (daemon does full scan once at startup)

# Test read command
time agentic-memorizer read  # Should be fast

# Stop daemon
agentic-memorizer daemon stop
```

### Phase 2: File Watching

**Goals:**
- Implement file system watching
- Event debouncing and batching
- Incremental index updates

**Tasks:**

1. **Create Watcher** (`internal/watcher`)
   - Implement file system watching with `fsnotify`
   - Event debouncing logic
   - Recursive directory watching
   - Skip patterns (hidden files, excluded dirs)

2. **Integrate Watcher with Daemon**
   - Start watcher on daemon start
   - Process batched events
   - Trigger index updates

3. **Incremental Updates** (`internal/index`)
   - `UpdateSingle()` method for single file changes
   - `RemoveFile()` method for deletions
   - Efficient in-memory index updates

4. **Testing & Debugging**
   - Test create/modify/delete events
   - Test rapid changes (debouncing)
   - Test directory creation
   - Stress test with many files

**Deliverables:**
- Daemon reacts to file changes
- Incremental updates working
- Debouncing prevents excessive rebuilds

**Testing:**
```bash
# Start daemon
agentic-memorizer daemon start --detach

# Add a file
echo "test" > ~/.agentic-memorizer/memory/test.txt

# Wait for debounce (500ms)
sleep 1

# Check index updated
agentic-memorizer read | grep test.txt

# Modify file multiple times rapidly
for i in {1..10}; do
  echo "update $i" >> ~/.agentic-memorizer/memory/test.txt
done

# Should only trigger one rebuild after debounce
```

### Phase 3: Worker Pool & Rate Limiting

**Goals:**
- Implement worker pool for parallel processing
- Add rate limiting for API calls
- Respect existing `analysis.parallel` config

**Tasks:**

1. **Create Worker Pool** (`internal/daemon`)
   - Implement job queue
   - Worker goroutines
   - Result aggregation
   - Graceful shutdown

2. **Add Rate Limiter**
   - Use `golang.org/x/time/rate`
   - Configure from `daemon.rate_limit_per_min`
   - Apply before Claude API calls

3. **Optimize Processing**
   - Batch file processing
   - Prioritize recent files
   - Skip unchanged files (hash check)

4. **Performance Testing**
   - Benchmark with 100+ files
   - Test rate limiting effectiveness
   - Measure API call patterns

**Deliverables:**
- Parallel processing working
- Rate limiting prevents API throttling
- Efficient resource usage

### Phase 4: Polish & Production Readiness

**Goals:**
- Background daemon mode
- Logging and monitoring
- Error recovery
- Documentation

**Tasks:**

1. **Daemonization** (`cmd/daemon/`)
   - `--detach` flag for background mode
   - Proper process daemonization
   - Log file output (not stdout/stderr)

2. **Logging** (`internal/daemon`)
   - Structured logging with `log/slog`
   - Configurable log levels
   - Log rotation (daily or size-based, using external library like `lumberjack`)
   - Log to configured file

3. **Error Recovery**
   - Retry failed API calls (with backoff)
   - Recover from crashes
   - Handle network failures
   - Corrupt index recovery

4. **Health Monitoring**
   - Optional HTTP health check endpoint
   - Metrics: uptime, files processed, API calls, errors
   - Status command shows health

5. **Documentation**
   - Update README.md
   - Document daemon commands
   - Add troubleshooting guide
   - Update configuration examples
   - Create migration guide for existing users
   - Update all hooks in documentation to use `agentic-memorizer read`
   - Add launchd plist example
   - Add systemd service example

6. **Init Command Updates**
   - `agentic-memorizer init --with-daemon`
   - Offer to start daemon
   - Update hook setup message

7. **Testing**
   - Integration tests
   - Daemon lifecycle tests
   - Crash recovery tests
   - Performance benchmarks

**Deliverables:**
- Production-ready daemon
- Comprehensive documentation
- Automated tests
- User-friendly setup

---

## Implementation Checklist

Use this checklist to track progress during implementation.

### Phase 1: Foundation
- [x] Create `internal/index/` package
- [x] Create `computed.go` with `ComputedIndex` struct
- [x] Implement atomic write functionality in `manager.go` (temp file + rename)
- [x] Add index load/save methods to `manager.go`
- [x] Write unit tests in `manager_test.go`
- [x] Add `DaemonConfig` struct to `internal/config/types.go`
- [x] Update config loading in `internal/config/config.go`
- [x] Add daemon defaults to config
- [x] Update `config.yaml.example` with daemon section
- [x] Create `internal/daemon/` package
- [x] Implement `Daemon` struct
- [x] Add daemon start/stop methods
- [x] Create `pid.go` for PID file management
- [x] Create `signals.go` for signal handling (SIGTERM, SIGINT, SIGUSR1)
- [x] Create `internal/version/` package
- [x] Add version information for daemon tracking
- [x] Create `cmd/daemon/` package
- [x] Implement `daemon start` subcommand (foreground)
- [x] Implement `daemon stop` subcommand
- [x] Implement `daemon status` subcommand
- [x] Implement `daemon restart` subcommand
- [x] Implement `daemon rebuild` subcommand
- [x] Implement `daemon logs` subcommand
- [x] Create `cmd/read/` package
- [x] Implement read command to load computed index
- [x] Implement empty index creation for missing index.json
- [x] Add warning message handling
- [x] Test: Manual daemon start/stop works
- [x] Test: Read command loads index successfully
- [x] Test: Empty index with warning when missing

### Phase 2: File Watching
- [x] Create `internal/watcher/` package
- [x] Implement file system watching with fsnotify
- [x] Implement event debouncing logic
- [x] Implement recursive directory watching
- [x] Add skip patterns (hidden files, excluded dirs)
- [x] Write watcher unit tests
- [x] Integrate watcher with daemon startup
- [x] Implement batched event processing
- [x] Trigger index updates from file events
- [x] Add `UpdateSingle()` method to index manager (completed in Phase 1)
- [x] Add `RemoveFile()` method to index manager (completed in Phase 1)
- [x] Optimize in-memory index updates
- [x] Test: Create file triggers index update
- [x] Test: Modify file triggers index update
- [x] Test: Delete file triggers index update
- [x] Test: Rapid changes are properly debounced
- [x] Test: Directory creation is handled
- [ ] Test: Stress test with many files (deferred to Phase 3)

### Phase 3: Worker Pool & Rate Limiting
- [x] Create `worker_pool.go` in `internal/daemon/`
- [x] Implement job queue
- [x] Implement worker goroutines
- [x] Implement result aggregation
- [x] Add graceful shutdown for workers
- [x] Add `golang.org/x/time/rate` dependency
- [x] Implement rate limiter
- [x] Configure rate limit from `daemon.rate_limit_per_min`
- [x] Apply rate limiting before Claude API calls
- [x] Implement batch file processing
- [x] Add file prioritization (recent files first)
- [x] Implement hash-based change detection (already in Phase 1/2)
- [x] Benchmark with 100+ files (tested with 20 files)
- [x] Test rate limiting effectiveness
- [x] Measure and optimize API call patterns
- [x] Test: Parallel processing works correctly
- [x] Test: Rate limiting prevents throttling
- [x] Verify: Resource usage is efficient

### Phase 4: Polish & Production Readiness

**Already Complete (from Phases 1-3):**
- ✓ Configure log file output (not stdout/stderr)
- ✓ Implement structured logging with log/slog
- ✓ Add configurable log levels
- ✓ Configure logging to daemon.log_file
- ✓ Status command shows daemon, index, and config info

**Remaining Tasks:**
- [x] Add `gopkg.in/natefinch/lumberjack.v2` dependency
- [x] Implement log rotation (daily or size-based) using lumberjack
- [x] Implement retry logic for failed API calls
- [x] Add exponential backoff for retries
- [x] Implement crash recovery (load last good index on startup)
- [x] Handle network failures gracefully (via retry logic)
- [x] Add corrupt index recovery (validates index on load)
- [x] Create `health.go` for health monitoring
- [x] Add optional HTTP health check endpoint
- [x] Implement health metrics (uptime, files, API calls, errors)
- [x] Enhance status command to show health check URL
- [x] Update README.md with daemon usage section
- [x] Document all daemon commands with examples
- [x] Add troubleshooting guide
- [x] Update configuration examples (completed in earlier phases)
- [ ] Create migration guide for existing users
- [ ] Update all hooks in documentation to use `agentic-memorizer read`
- [x] Add launchd plist example (macOS) - `examples/com.agentic-memorizer.daemon.plist`
- [x] Add systemd service example (Linux) - `examples/agentic-memorizer.service`
- [x] Add `--with-daemon` flag to init command
- [x] Prompt to start daemon during init
- [x] Update hook setup message (now recommends daemon mode)
- [ ] Write integration tests
- [ ] Write daemon lifecycle tests
- [ ] Write crash recovery tests
- [ ] Create performance benchmarks

---

## Code Changes Summary

### New Files

```
internal/watcher/
  watcher.go          - File system watching
  watcher_test.go     - Tests

internal/daemon/
  daemon.go           - Main daemon logic
  daemon_test.go      - Tests
  worker_pool.go      - Worker pool implementation
  pid.go              - PID file management
  signals.go          - Signal handling
  health.go           - Health monitoring and metrics endpoint

internal/index/
  manager.go          - Index manager
  manager_test.go     - Tests
  computed.go         - Computed index structures

cmd/daemon/
  daemon.go           - Daemon parent command
  start.go            - Start subcommand
  stop.go             - Stop subcommand
  status.go           - Status subcommand
  restart.go          - Restart subcommand
  rebuild.go          - Rebuild subcommand
  logs.go             - Logs subcommand

cmd/read/
  read.go             - Read command

internal/version/
  version.go          - Version information
```

### Modified Files

```
cmd/root.go
  - Add daemon subcommand
  - Add read command
  - Update command structure

cmd/init/init.go
  - Add --with-daemon flag
  - Offer to start daemon during initialization
  - Update hook setup messages to reference new command structure

internal/config/types.go
  - Add DaemonConfig struct

internal/config/config.go
  - Add daemon config defaults
  - Add daemon config loading

internal/config/constants.go
  - Add daemon-related constants

config.yaml.example
  - Add daemon configuration section

README.md
  - Document daemon usage
  - Update installation instructions
  - Add troubleshooting for daemon

Makefile
  - Add daemon build targets (optional)

go.mod
  - Add fsnotify dependency
  - Add rate limiting dependency
  - Add lumberjack dependency for log rotation
```

### Dependencies

Add to `go.mod`:

```go
require (
    github.com/fsnotify/fsnotify v1.7.0
    golang.org/x/time v0.5.0
    gopkg.in/natefinch/lumberjack.v2 v2.2.1  // For log rotation
)
```

---

## Testing Strategy

### Unit Tests

**Coverage Target:** >85%

**Key Test Areas:**

1. **Index Manager** (`internal/index/manager_test.go`)
   - Atomic writes
   - Concurrent reads during writes
   - Load/save operations
   - Incremental updates
   - Error handling

2. **Watcher** (`internal/watcher/watcher_test.go`)
   - Event detection (create, modify, delete)
   - Debouncing logic
   - Recursive watching
   - Skip patterns

3. **Worker Pool** (`internal/daemon/worker_pool_test.go`)
   - Job processing
   - Rate limiting
   - Graceful shutdown
   - Error handling

4. **PID File** (`internal/daemon/pid_test.go`)
   - Write/read/remove
   - Process existence checking
   - Concurrent access

### Integration Tests

**Test Scenarios:**

1. **End-to-End Daemon Lifecycle**
   ```go
   func TestDaemonLifecycle(t *testing.T) {
       // Start daemon
       // Add file to memory dir
       // Wait for processing
       // Verify computed index updated
       // Check CLI fast path works
       // Stop daemon
   }
   ```

2. **File Change Handling**
   ```go
   func TestFileChangeProcessing(t *testing.T) {
       // Start daemon
       // Create file
       // Modify file
       // Delete file
       // Verify index reflects all changes
   }
   ```

3. **Concurrent Operations**
   ```go
   func TestConcurrentChanges(t *testing.T) {
       // Start daemon
       // Trigger many file changes simultaneously
       // Verify all processed correctly
       // Verify no race conditions
   }
   ```

4. **Recovery from Failures**
   ```go
   func TestRecovery(t *testing.T) {
       // Start daemon
       // Simulate API failure
       // Verify retry logic
       // Verify eventual consistency
   }
   ```

### Performance Tests

**Benchmarks:**

```go
func BenchmarkComputedIndexLoad(b *testing.B) {
    // Benchmark loading computed index
}

func BenchmarkIncrementalUpdate(b *testing.B) {
    // Benchmark single file update
}

func BenchmarkCLIFastPath(b *testing.B) {
    // Benchmark full CLI execution with computed index
}
```

**Load Tests:**

- 1000 files in memory directory
- 100 concurrent file changes
- API rate limiting under load

### Manual Testing Checklist

- [ ] Daemon starts successfully
- [ ] Daemon detaches properly in background mode
- [ ] PID file created and valid
- [ ] Initial index build completes
- [ ] File changes trigger updates
- [ ] Rapid changes debounced correctly
- [ ] CLI fast path < 50ms
- [ ] Empty index warning shown when index.json missing
- [ ] Daemon stops cleanly (SIGTERM)
- [ ] Daemon reloads config (SIGHUP)
- [ ] Status command accurate
- [ ] Logs written correctly
- [ ] Rate limiting prevents throttling
- [ ] Works with no internet (cached only)
- [ ] Recovers from daemon crash
- [ ] Multiple sessions concurrent with daemon

---

## Migration Path

### Backward Compatibility

**Design Principle:** Simple, predictable behavior.

**Important Note:** This is a significant architectural change from the current version. Indexing is now handled by a background daemon, and output is via the `read` command.

**Behavior Matrix:**

| Scenario | Behavior | Notes |
|----------|----------|-------|
| Daemon running | `read` command outputs index, fast | Normal operation |
| Daemon not running | `read` command outputs empty index with warning | User prompted to start daemon |
| Index file missing | `read` command creates empty index with warning | Graceful degradation |
| Config without daemon section | Daemon won't auto-start | User must start manually |
| First time use | Must run `daemon start` first | Clear setup instructions |

### User Upgrade Path

**New Users:**

```bash
# Install
go install github.com/leefowlercu/agentic-memorizer@latest

# Initialize configuration
agentic-memorizer init --setup-daemon

# Start daemon (builds initial index)
agentic-memorizer daemon start --detach

# Wait a few seconds for initial indexing

# Test (should see index output)
agentic-memorizer read
```

**Existing Users Upgrading:**

```bash
# Update to new version
go install github.com/leefowlercu/agentic-memorizer@latest

# Start the daemon (required for indexing)
agentic-memorizer daemon start --detach

# Wait for initial index build

# Use read command to see index (fast)
agentic-memorizer read
```

**Migration Notes:**
- The `read` command must be used to output the index (no longer root command)
- The daemon must be running for the index to stay up-to-date
- First run will show a warning until daemon builds the index
- Update hooks in `~/.claude/settings.json` to use `agentic-memorizer read` instead of just `agentic-memorizer`

### System Integration

**macOS (launchd):**

Create: `~/Library/LaunchAgents/com.agentic-memorizer.daemon.plist`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.agentic-memorizer.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Users/username/.local/bin/agentic-memorizer</string>
        <string>daemon</string>
        <string>start</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/Users/username/.agentic-memorizer/daemon.log</string>
    <key>StandardErrorPath</key>
    <string>/Users/username/.agentic-memorizer/daemon.err</string>
</dict>
</plist>
```

**Linux (systemd):**

Create: `~/.config/systemd/user/agentic-memorizer.service`

```ini
[Unit]
Description=Agentic Memorizer Background Daemon
After=network.target

[Service]
Type=simple
ExecStart=%h/.local/bin/agentic-memorizer daemon start
Restart=always
RestartSec=10

[Install]
WantedBy=default.target
```

**Helper Commands:**

Add to `init` command:

```bash
# macOS
agentic-memorizer init --setup-daemon --autostart-launchd

# Linux
agentic-memorizer init --setup-daemon --autostart-systemd
```

---

## Performance Considerations

### Target Metrics

| Metric | Target | Current (Traditional) | Improvement |
|--------|--------|----------------------|-------------|
| Warm startup | <50ms | 120-250ms | 2.4-5x faster |
| Cold startup (10 files) | <50ms | 10-15s | 200-300x faster |
| File change → index update | <2s | N/A (on next start) | Real-time |
| Memory usage (daemon) | <100MB | N/A | Acceptable |
| CPU usage (idle) | <1% | N/A | Minimal |

### Optimization Strategies

1. **Index Format**
   - JSON for simplicity (phase 1)
   - Consider MessagePack/Protocol Buffers for speed (future)
   - mmap for large indexes (future)

2. **Caching Strategy**
   - Keep in-memory index in daemon
   - Lazy load formatted output
   - Cache formatted XML/Markdown (optional)

3. **API Efficiency**
   - Batch similar file types
   - Reuse Claude client connections
   - Implement exponential backoff for retries

4. **File System Watching**
   - Use native OS events (inotify on Linux, FSEvents on macOS)
   - Debounce aggressively (500ms default)
   - Skip non-interesting events early

### Resource Management

**Memory:**
- Index size: ~1KB per file × 1000 files = ~1MB
- Daemon overhead: ~50MB
- Worker pool: ~20MB per worker
- **Total: ~100-150MB** (acceptable)

**CPU:**
- Idle: <1% (just file watching)
- Processing: Burst to 100% during analysis, then idle
- Rate limited to prevent sustained high usage

**Disk I/O:**
- Write computed index: Once per batch (~500ms after changes)
- Sequential writes (append to log)
- No excessive disk activity

**Network:**
- API calls: Rate limited to configured value (default: 20/min)
- Respects Anthropic API rate limits
- Exponential backoff on errors

---

## Risk Analysis

### Technical Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Daemon crashes | Medium | Auto-restart via systemd/launchd, PID monitoring |
| Computed index corruption | Medium | Atomic writes, validation on load, automatic rebuild |
| File watcher misses events | Low | Periodic full scans (every hour), validate consistency |
| Race conditions in index updates | Medium | Proper locking (RWMutex), atomic operations, thorough testing |
| Memory leaks in long-running daemon | Medium | Profiling, testing, periodic metrics, memory limits |
| API rate limit exceeded | Low | Built-in rate limiting, exponential backoff |

### User Experience Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Daemon not running (user forgot to start) | Medium | Empty index with clear warning message, instructions to start daemon |
| Users expect on-demand indexing | Medium | Clear documentation about daemon requirement, helpful error messages |
| Breaking change from current behavior | High | Clear upgrade guide, migration documentation |
| Setup complexity | Medium | Simple `--setup-daemon` flag, good defaults, clear first-run experience |

### Operational Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Increased support burden | Medium | Comprehensive documentation, troubleshooting guide |
| Daemon management complexity | Medium | Simple commands, system integration helpers |
| Debugging issues harder | Medium | Structured logging, verbose mode, health endpoint |

---

## Future Enhancements

### Phase 5+ (Post-MVP)

1. **Smart Prioritization**
   - Prioritize recently accessed files
   - Analyze frequently modified files first
   - Deprioritize large/old files

2. **Advanced Caching**
   - Distributed cache (Redis/Memcached)
   - Shared cache across multiple machines
   - Cloud-backed cache

3. **Rich Metadata**
   - Extract more signals (page counts, thumbnails, audio duration)
   - OCR for images with text
   - Code analysis (imports, functions, classes)

4. **Web UI Dashboard**
   - View indexed files
   - Trigger manual re-analysis
   - Monitor daemon health
   - View API usage/costs

5. **Incremental Semantic Analysis**
   - Diff-based analysis for text files
   - Only re-analyze changed sections
   - Reduce API costs for large files

6. **Plugin System**
   - Custom metadata extractors
   - Custom analyzers
   - Integration with other tools

7. **Multi-Memory Support**
   - Multiple memory directories
   - Per-project memories
   - Shared vs private memories

8. **Cloud Sync**
   - Sync memory and cache across devices
   - Collaborative memory directories
   - Conflict resolution

---

## Success Metrics

### Primary Metrics

1. **Startup Latency**
   - **Target:** <50ms with daemon
   - **Measurement:** Time from hook trigger to output complete
   - **Success:** 95th percentile <50ms

2. **User Adoption**
   - **Target:** 50% of users enable daemon
   - **Measurement:** Telemetry (opt-in) or survey
   - **Success:** Positive feedback, feature requests

3. **Reliability**
   - **Target:** 99.9% uptime (daemon runs without intervention)
   - **Measurement:** Crash reports, restart count
   - **Success:** <1 crash per 1000 daemon-hours

### Secondary Metrics

1. **API Cost Reduction**
   - **Target:** 30% reduction vs traditional (better batching)
   - **Measurement:** API calls per file change
   - **Success:** Measurable savings for heavy users

2. **Code Quality**
   - **Target:** >85% test coverage
   - **Measurement:** go test -cover
   - **Success:** Passes all tests, clean code reviews

3. **User Satisfaction**
   - **Target:** >80% satisfaction
   - **Measurement:** User feedback, issues filed
   - **Success:** More positive than negative feedback

---

## Conclusion

The Background Index Precomputation feature represents a significant architectural improvement to the agentic-memorizer system. By moving heavy operations out of the critical SessionStart path, we achieve:

1. **Predictable Performance:** <50ms startup regardless of file changes
2. **Better UX:** No blocking during Claude Code startup
3. **Real-time Updates:** Index reflects file changes within seconds
4. **Cost Efficiency:** Better rate limiting and batching
5. **Scalability:** Handles larger memory directories gracefully

The implementation is designed to be **simple**, **predictable**, and **production-ready**. The separation of concerns is clear:
- **Daemon subcommands** (`daemon start/stop/status/...`) = Background daemon control
- **Read command** = Fast read and format of computed index
- **Init command** = Configuration initialization (unchanged)

**Key Architectural Decisions:**
- No on-demand indexing in the read command (simplicity)
- Daemon must be running for index updates (clear responsibility)
- Empty index with warnings when daemon not started (graceful degradation)
- Clean separation between indexing and formatting logic
- Organized command structure (daemon operations grouped under `daemon` subcommand)

This foundation also enables **future enhancements** like smart prioritization, advanced caching, and richer metadata extraction.

**Recommendation:** Proceed with implementation following the phased approach outlined above.
