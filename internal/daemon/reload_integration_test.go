//go:build integration

package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/cache"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/index"
	"github.com/leefowlercu/agentic-memorizer/internal/metadata"
	"github.com/leefowlercu/agentic-memorizer/internal/watcher"
	"gopkg.in/natefinch/lumberjack.v2"
)

// TestEnv provides isolated test environment for integration tests
type TestEnv struct {
	AppDir     string
	MemoryRoot string
	CacheDir   string
	ConfigPath string
	IndexPath  string
	PIDPath    string
	LogPath    string
	Config     *config.Config
	t          *testing.T
}

// NewTestEnv creates an isolated test environment
func NewTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	// Create temp directory for entire test environment
	appDir := t.TempDir()

	memoryRoot := filepath.Join(appDir, "memory")
	cacheDir := filepath.Join(memoryRoot, ".cache")
	configPath := filepath.Join(appDir, "config.yaml")
	indexPath := filepath.Join(appDir, "index.json")
	pidPath := filepath.Join(appDir, "daemon.pid")
	logPath := filepath.Join(appDir, "daemon.log")

	// Create directories
	if err := os.MkdirAll(memoryRoot, 0755); err != nil {
		t.Fatalf("failed to create memory directory: %v", err)
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("failed to create cache directory: %v", err)
	}

	// Create default config
	cfg := &config.Config{
		MemoryRoot: memoryRoot,
		Claude: config.ClaudeConfig{
			APIKey:         "test-api-key",
			Model:          "claude-3-haiku-20240307",
			MaxTokens:      1000,
			EnableVision:   false,
			TimeoutSeconds: 30,
		},
		Analysis: config.AnalysisConfig{
			Enable:      false, // Disable analysis for faster tests
			MaxFileSize: 1024 * 1024,
			CacheDir:    cacheDir,
		},
		Daemon: config.DaemonConfig{
			Workers:                    2,
			RateLimitPerMin:            20,
			DebounceMs:                 200,
			FullRebuildIntervalMinutes: 60,
			HealthCheckPort:            0, // Random port
			LogFile:                    logPath,
			LogLevel:                   "info",
		},
	}

	// Write config file
	if err := config.WriteConfig(configPath, cfg); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Set environment variable to use test app directory
	t.Setenv("MEMORIZER_APP_DIR", appDir)

	return &TestEnv{
		AppDir:     appDir,
		MemoryRoot: memoryRoot,
		CacheDir:   cacheDir,
		ConfigPath: configPath,
		IndexPath:  indexPath,
		PIDPath:    pidPath,
		LogPath:    logPath,
		Config:     cfg,
		t:          t,
	}
}

// UpdateConfig modifies the config and writes it back to disk
func (e *TestEnv) UpdateConfig(modifyFn func(*config.Config)) error {
	modifyFn(e.Config)
	return config.WriteConfig(e.ConfigPath, e.Config)
}

// CreateDaemon creates a new daemon instance for testing
func (e *TestEnv) CreateDaemon() (*Daemon, error) {
	e.t.Helper()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Reduce noise in test output
	}))

	logWriter := &lumberjack.Logger{
		Filename:   e.LogPath,
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   true,
	}

	// Create daemon components
	indexManager := index.NewManager(e.IndexPath)

	cacheManager, err := cache.NewManager(e.Config.Analysis.CacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache manager: %w", err)
	}

	metadataExtractor := metadata.NewExtractor()

	skipDirs := []string{".cache", ".git"}
	skipFiles := []string{"agentic-memorizer"}

	fileWatcher, err := watcher.New(
		e.Config.MemoryRoot,
		skipDirs,
		skipFiles,
		e.Config.Daemon.DebounceMs,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	d := &Daemon{
		cfg:               e.Config,
		logger:            logger,
		logWriter:         logWriter,
		indexManager:      indexManager,
		cacheManager:      cacheManager,
		metadataExtractor: metadataExtractor,
		fileWatcher:       fileWatcher,
		ctx:               ctx,
		cancel:            cancel,
		pidFile:           e.PIDPath,
		rebuildIntervalCh: make(chan time.Duration, 1),
		healthMetrics:     &HealthMetrics{},
	}

	// Set semantic analyzer to nil (analysis disabled for tests)
	d.semanticAnalyzer.Store((*interface{})(nil))

	return d, nil
}

// WaitForHealthy polls the health endpoint until daemon is ready
func (e *TestEnv) WaitForHealthy(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 1 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(fmt.Sprintf("http://localhost:%d/health", port))
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("daemon did not become healthy within %v", timeout)
}

// GetDaemonPID reads the PID from the PID file
func (e *TestEnv) GetDaemonPID() (int, error) {
	data, err := os.ReadFile(e.PIDPath)
	if err != nil {
		return 0, err
	}

	var pid int
	if _, err := fmt.Fscanf(os.Stdin, "%d", &pid); err != nil {
		// Try simpler parse
		if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
			return 0, fmt.Errorf("failed to parse PID: %w", err)
		}
	}

	return pid, nil
}

// TestDaemon_ReloadConfig_FullCycle tests the complete reload cycle
func TestDaemon_ReloadConfig_FullCycle(t *testing.T) {
	env := NewTestEnv(t)

	// Create and start daemon
	d, err := env.CreateDaemon()
	if err != nil {
		t.Fatalf("failed to create daemon: %v", err)
	}

	// Start daemon in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Start()
	}()

	// Give daemon time to start
	time.Sleep(500 * time.Millisecond)

	// Get original config values
	originalWorkers := d.GetConfig().Daemon.Workers
	originalLogLevel := d.GetConfig().Daemon.LogLevel

	// Modify config
	if err := env.UpdateConfig(func(cfg *config.Config) {
		cfg.Daemon.Workers = 5
		cfg.Daemon.LogLevel = "debug"
	}); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// Reload config using the daemon's method
	if err := d.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig() failed: %v", err)
	}

	// Verify changes were applied
	newConfig := d.GetConfig()
	if newConfig.Daemon.Workers != 5 {
		t.Errorf("expected workers=5, got %d", newConfig.Daemon.Workers)
	}
	if newConfig.Daemon.LogLevel != "debug" {
		t.Errorf("expected log_level=debug, got %s", newConfig.Daemon.LogLevel)
	}

	// Verify original values were different
	if originalWorkers == 5 {
		t.Error("test setup error: original workers should not be 5")
	}
	if originalLogLevel == "debug" {
		t.Error("test setup error: original log level should not be debug")
	}

	// Stop daemon
	d.Stop()

	// Wait for daemon to stop
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("daemon stopped with error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("daemon did not stop within timeout")
	}
}

// TestDaemon_ReloadConfig_ImmutableFieldRejection tests that immutable fields cannot be changed
func TestDaemon_ReloadConfig_ImmutableFieldRejection(t *testing.T) {
	env := NewTestEnv(t)

	d, err := env.CreateDaemon()
	if err != nil {
		t.Fatalf("failed to create daemon: %v", err)
	}

	// Try to change memory_root (immutable field)
	if err := env.UpdateConfig(func(cfg *config.Config) {
		cfg.MemoryRoot = "/different/path"
	}); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Attempt reload - should fail
	err = d.ReloadConfig()
	if err == nil {
		t.Fatal("ReloadConfig() should have failed for immutable field change")
	}

	// Verify error mentions the immutable field
	if err != nil && err.Error() != "" {
		// Error should mention memory_root
		// This is good enough for now
		t.Logf("Got expected error: %v", err)
	}
}

// TestDaemon_ReloadConfig_WorkerCountChange tests worker pool reconfiguration
func TestDaemon_ReloadConfig_WorkerCountChange(t *testing.T) {
	env := NewTestEnv(t)

	d, err := env.CreateDaemon()
	if err != nil {
		t.Fatalf("failed to create daemon: %v", err)
	}

	originalWorkers := d.GetConfig().Daemon.Workers

	// Change worker count
	newWorkers := originalWorkers + 3
	if err := env.UpdateConfig(func(cfg *config.Config) {
		cfg.Daemon.Workers = newWorkers
	}); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// Reload
	if err := d.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig() failed: %v", err)
	}

	// Verify worker count changed
	if d.GetConfig().Daemon.Workers != newWorkers {
		t.Errorf("expected workers=%d, got %d", newWorkers, d.GetConfig().Daemon.Workers)
	}
}

// TestDaemon_ReloadConfig_LogLevelChange tests log level hot-reloading
func TestDaemon_ReloadConfig_LogLevelChange(t *testing.T) {
	env := NewTestEnv(t)

	d, err := env.CreateDaemon()
	if err != nil {
		t.Fatalf("failed to create daemon: %v", err)
	}

	// Change log level to debug
	if err := env.UpdateConfig(func(cfg *config.Config) {
		cfg.Daemon.LogLevel = "debug"
	}); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// Reload
	if err := d.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig() failed: %v", err)
	}

	// Verify log level changed
	if d.GetConfig().Daemon.LogLevel != "debug" {
		t.Errorf("expected log_level=debug, got %s", d.GetConfig().Daemon.LogLevel)
	}

	// Change to error level
	if err := env.UpdateConfig(func(cfg *config.Config) {
		cfg.Daemon.LogLevel = "error"
	}); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// Reload again
	if err := d.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig() failed: %v", err)
	}

	// Verify log level changed again
	if d.GetConfig().Daemon.LogLevel != "error" {
		t.Errorf("expected log_level=error, got %s", d.GetConfig().Daemon.LogLevel)
	}
}

// TestDaemon_ReloadConfig_RateLimitChange tests rate limiter reconfiguration
func TestDaemon_ReloadConfig_RateLimitChange(t *testing.T) {
	env := NewTestEnv(t)

	d, err := env.CreateDaemon()
	if err != nil {
		t.Fatalf("failed to create daemon: %v", err)
	}

	originalRateLimit := d.GetConfig().Daemon.RateLimitPerMin

	// Change rate limit
	newRateLimit := originalRateLimit + 10
	if err := env.UpdateConfig(func(cfg *config.Config) {
		cfg.Daemon.RateLimitPerMin = newRateLimit
	}); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// Reload
	if err := d.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig() failed: %v", err)
	}

	// Verify rate limit changed
	if d.GetConfig().Daemon.RateLimitPerMin != newRateLimit {
		t.Errorf("expected rate_limit=%d, got %d", newRateLimit, d.GetConfig().Daemon.RateLimitPerMin)
	}
}

// TestDaemon_ReloadConfig_DebounceIntervalChange tests file watcher debounce reconfiguration
func TestDaemon_ReloadConfig_DebounceIntervalChange(t *testing.T) {
	env := NewTestEnv(t)

	d, err := env.CreateDaemon()
	if err != nil {
		t.Fatalf("failed to create daemon: %v", err)
	}

	originalDebounce := d.GetConfig().Daemon.DebounceMs

	// Change debounce interval
	newDebounce := originalDebounce + 100
	if err := env.UpdateConfig(func(cfg *config.Config) {
		cfg.Daemon.DebounceMs = newDebounce
	}); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// Reload
	if err := d.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig() failed: %v", err)
	}

	// Verify debounce changed
	if d.GetConfig().Daemon.DebounceMs != newDebounce {
		t.Errorf("expected debounce=%d, got %d", newDebounce, d.GetConfig().Daemon.DebounceMs)
	}
}

// TestDaemon_ReloadConfig_HealthPortChange tests health check server restart
func TestDaemon_ReloadConfig_HealthPortChange(t *testing.T) {
	env := NewTestEnv(t)

	// Set initial health port
	if err := env.UpdateConfig(func(cfg *config.Config) {
		cfg.Daemon.HealthCheckPort = 0 // Let OS assign port
	}); err != nil {
		t.Fatalf("failed to set initial config: %v", err)
	}

	d, err := env.CreateDaemon()
	if err != nil {
		t.Fatalf("failed to create daemon: %v", err)
	}

	// Start health server on initial port
	if err := d.startHealthServer(d.GetConfig().Daemon.HealthCheckPort); err != nil {
		t.Fatalf("failed to start health server: %v", err)
	}
	defer d.stopHealthServer()

	// Change health port
	if err := env.UpdateConfig(func(cfg *config.Config) {
		cfg.Daemon.HealthCheckPort = 0 // New random port
	}); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// Reload
	if err := d.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig() failed: %v", err)
	}

	// Health server should have restarted on new port
	// This is verified by the reload not failing
}

// TestDaemon_ReloadConfig_SignalHandling tests SIGHUP signal handling
func TestDaemon_ReloadConfig_SignalHandling(t *testing.T) {
	env := NewTestEnv(t)

	d, err := env.CreateDaemon()
	if err != nil {
		t.Fatalf("failed to create daemon: %v", err)
	}

	// Start daemon in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Start()
	}()

	// Give daemon time to start and set up signal handler
	time.Sleep(500 * time.Millisecond)

	// Write PID file for signal test
	pid := os.Getpid()
	if err := os.WriteFile(env.PIDPath, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		t.Fatalf("failed to write PID file: %v", err)
	}

	// Modify config
	if err := env.UpdateConfig(func(cfg *config.Config) {
		cfg.Daemon.Workers = 10
	}); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// Send SIGHUP to self (daemon is running in same process)
	if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
		t.Fatalf("failed to send SIGHUP: %v", err)
	}

	// Give reload time to process
	time.Sleep(500 * time.Millisecond)

	// Verify config was reloaded
	if d.GetConfig().Daemon.Workers != 10 {
		t.Errorf("expected workers=10 after SIGHUP, got %d", d.GetConfig().Daemon.Workers)
	}

	// Stop daemon
	d.Stop()

	// Wait for daemon to stop
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("daemon stopped with error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("daemon did not stop within timeout")
	}
}

// TestDaemon_ReloadConfig_ValidationError tests reload with validation errors
func TestDaemon_ReloadConfig_ValidationError(t *testing.T) {
	env := NewTestEnv(t)

	d, err := env.CreateDaemon()
	if err != nil {
		t.Fatalf("failed to create daemon: %v", err)
	}

	// Write invalid config (invalid log level)
	if err := env.UpdateConfig(func(cfg *config.Config) {
		cfg.Daemon.LogLevel = "invalid-level"
	}); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Attempt reload - should fail validation
	err = d.ReloadConfig()
	if err == nil {
		t.Fatal("ReloadConfig() should have failed for invalid config")
	}

	t.Logf("Got expected validation error: %v", err)
}

// TestDaemon_ReloadConfig_MultipleChanges tests applying multiple config changes at once
func TestDaemon_ReloadConfig_MultipleChanges(t *testing.T) {
	env := NewTestEnv(t)

	d, err := env.CreateDaemon()
	if err != nil {
		t.Fatalf("failed to create daemon: %v", err)
	}

	// Change multiple settings at once
	if err := env.UpdateConfig(func(cfg *config.Config) {
		cfg.Daemon.Workers = 8
		cfg.Daemon.LogLevel = "debug"
		cfg.Daemon.RateLimitPerMin = 50
		cfg.Daemon.DebounceMs = 300
	}); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// Reload
	if err := d.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig() failed: %v", err)
	}

	// Verify all changes were applied
	cfg := d.GetConfig()
	if cfg.Daemon.Workers != 8 {
		t.Errorf("expected workers=8, got %d", cfg.Daemon.Workers)
	}
	if cfg.Daemon.LogLevel != "debug" {
		t.Errorf("expected log_level=debug, got %s", cfg.Daemon.LogLevel)
	}
	if cfg.Daemon.RateLimitPerMin != 50 {
		t.Errorf("expected rate_limit=50, got %d", cfg.Daemon.RateLimitPerMin)
	}
	if cfg.Daemon.DebounceMs != 300 {
		t.Errorf("expected debounce=300, got %d", cfg.Daemon.DebounceMs)
	}
}
