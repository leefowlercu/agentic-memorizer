//go:build !integration

package daemon

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/daemon/api"
	_ "github.com/leefowlercu/agentic-memorizer/internal/semantic/providers/claude" // Register Claude provider
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
	"gopkg.in/natefinch/lumberjack.v2"
)

// newTestDaemon creates a minimal daemon instance for testing
func newTestDaemon() *Daemon {
	ctx, cancel := context.WithCancel(context.Background())

	cfg := &config.Config{
		MemoryRoot: "/test/memory",
		Semantic:   config.SemanticConfig{CacheDir: "/test/cache"},
		Daemon: config.DaemonConfig{
			LogFile:  "/test/daemon.log",
			LogLevel: "info",
			Workers:  3,
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	logWriter := &lumberjack.Logger{
		Filename: "/dev/null",
	}

	healthMetrics := NewHealthMetrics()
	sseHub := api.NewSSEHub(logger)
	httpServer := api.NewHTTPServer(sseHub, healthMetrics, nil, "/test/memory", logger)

	d := &Daemon{
		cfg:               cfg,
		logger:            logger,
		logWriter:         logWriter,
		ctx:               ctx,
		cancel:            cancel,
		rebuildIntervalCh: make(chan time.Duration, 1),
		healthMetrics:     healthMetrics,
		sseHub:            sseHub,
		httpServer:        httpServer,
	}

	// Don't set semantic analyzer to test nil handling

	return d
}

// TestDaemon_ConfigLocking tests concurrent config access
func TestDaemon_ConfigLocking(t *testing.T) {
	t.Run("concurrent reads", func(t *testing.T) {
		d := newTestDaemon()
		defer d.cancel()

		const numReaders = 100
		var wg sync.WaitGroup
		results := make(chan *config.Config, numReaders)

		// Launch concurrent readers
		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				cfg := d.GetConfig()
				results <- cfg
			}()
		}

		// Wait for all reads
		wg.Wait()
		close(results)

		// All readers should get the same config pointer
		var firstCfg *config.Config
		count := 0
		for cfg := range results {
			if firstCfg == nil {
				firstCfg = cfg
			}
			if cfg != firstCfg {
				t.Error("GetConfig() returned different config pointers")
			}
			count++
		}

		if count != numReaders {
			t.Errorf("Expected %d results, got %d", numReaders, count)
		}
	})

	t.Run("read during write", func(t *testing.T) {
		d := newTestDaemon()
		defer d.cancel()

		const numReaders = 50
		const numWriters = 10
		var wg sync.WaitGroup

		// Start concurrent readers
		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					cfg := d.GetConfig()
					if cfg == nil {
						t.Error("GetConfig() returned nil")
					}
					time.Sleep(time.Microsecond)
				}
			}()
		}

		// Start concurrent writers
		for i := 0; i < numWriters; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 5; j++ {
					newCfg := &config.Config{
						MemoryRoot: fmt.Sprintf("/test/memory-%d", id),
					}
					d.SetConfig(newCfg)
					time.Sleep(time.Microsecond)
				}
			}(i)
		}

		// Wait with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out - possible deadlock")
		}
	})

	t.Run("config pointer consistency", func(t *testing.T) {
		d := newTestDaemon()
		defer d.cancel()

		cfg1 := d.GetConfig()
		cfg2 := d.GetConfig()

		if cfg1 != cfg2 {
			t.Error("Multiple GetConfig() calls should return same pointer")
		}

		newCfg := &config.Config{MemoryRoot: "/new/path"}
		d.SetConfig(newCfg)

		cfg3 := d.GetConfig()
		if cfg3 != newCfg {
			t.Error("GetConfig() should return newly set config")
		}
		if cfg3 == cfg1 {
			t.Error("GetConfig() should return different pointer after SetConfig()")
		}
	})
}

// mockProvider is a simple mock semantic provider for testing
type mockProvider struct {
	name  string
	model string
}

func (m *mockProvider) Analyze(ctx context.Context, metadata *types.FileMetadata) (*types.SemanticAnalysis, error) {
	return nil, nil
}
func (m *mockProvider) Name() string             { return m.name }
func (m *mockProvider) Model() string            { return m.model }
func (m *mockProvider) SupportsVision() bool     { return false }
func (m *mockProvider) SupportsDocuments() bool  { return false }

// TestDaemon_SemanticProviderSwap tests atomic provider replacement
func TestDaemon_SemanticProviderSwap(t *testing.T) {
	t.Run("concurrent access", func(t *testing.T) {
		d := newTestDaemon()
		defer d.cancel()

		// Set initial provider
		provider1 := &mockProvider{name: "test", model: "test-model"}
		d.SetSemanticProvider(provider1)

		const numReaders = 100
		var wg sync.WaitGroup
		errors := make(chan error, numReaders)

		// Launch concurrent readers
		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					p := d.GetSemanticProvider()
					if p == nil {
						errors <- fmt.Errorf("GetSemanticProvider() returned nil")
						return
					}
					time.Sleep(time.Microsecond)
				}
			}()
		}

		// Wait for reads
		wg.Wait()
		close(errors)

		for err := range errors {
			t.Error(err)
		}
	})

	t.Run("swap during use", func(t *testing.T) {
		d := newTestDaemon()
		defer d.cancel()

		// Set initial provider
		provider1 := &mockProvider{name: "test", model: "test-model"}
		d.SetSemanticProvider(provider1)

		const numReaders = 50
		const numWriters = 10
		var wg sync.WaitGroup

		// Concurrent readers
		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					p := d.GetSemanticProvider()
					if p == nil {
						t.Error("GetSemanticProvider() returned nil during swap")
					}
					time.Sleep(time.Microsecond)
				}
			}()
		}

		// Concurrent writers
		for i := 0; i < numWriters; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					provider := &mockProvider{name: fmt.Sprintf("test-%d", id), model: "test-model"}
					d.SetSemanticProvider(provider)
					time.Sleep(time.Microsecond)
				}
			}(i)
		}

		// Wait with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out")
		}
	})

	t.Run("nil handling", func(t *testing.T) {
		d := newTestDaemon()
		defer d.cancel()

		// Initially nil (not set yet)
		if d.GetSemanticProvider() != nil {
			t.Error("Expected nil provider initially")
		}

		// Set to non-nil
		provider := &mockProvider{name: "test", model: "test-model"}
		d.SetSemanticProvider(provider)

		if d.GetSemanticProvider() == nil {
			t.Error("Expected non-nil provider after setting")
		}

		// Note: atomic.Value doesn't support storing nil once a value is set.
		// To "disable" semantic analysis, the config disables it rather than
		// clearing the provider. The provider stays set but isn't used.
	})
}

// TestDaemon_LoggerReplacement tests logger swapping
func TestDaemon_LoggerReplacement(t *testing.T) {
	t.Run("concurrent access", func(t *testing.T) {
		d := newTestDaemon()
		defer d.cancel()

		const numReaders = 100
		var wg sync.WaitGroup

		// Concurrent readers
		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					logger := d.GetLogger()
					if logger == nil {
						t.Error("GetLogger() returned nil")
					}
					time.Sleep(time.Microsecond)
				}
			}()
		}

		wg.Wait()
	})

	t.Run("logger replacement", func(t *testing.T) {
		d := newTestDaemon()
		defer d.cancel()

		oldLogger := d.GetLogger()
		if oldLogger == nil {
			t.Fatal("Initial logger is nil")
		}

		// Create new logger
		var buf bytes.Buffer
		newLogger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))

		d.SetLogger(newLogger)

		currentLogger := d.GetLogger()
		if currentLogger == oldLogger {
			t.Error("Logger was not replaced")
		}
		if currentLogger != newLogger {
			t.Error("Logger was not set to new logger")
		}
	})

	t.Run("concurrent logging during swap", func(t *testing.T) {
		d := newTestDaemon()
		defer d.cancel()

		const numLoggers = 50
		var wg sync.WaitGroup

		// Concurrent logging and swapping
		for i := 0; i < numLoggers; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					logger := d.GetLogger()
					logger.Info("test log", "id", id, "iteration", j)

					if id%5 == 0 {
						newLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
							Level: slog.LevelWarn,
						}))
						d.SetLogger(newLogger)
					}
					time.Sleep(time.Microsecond)
				}
			}(i)
		}

		// Wait with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success - no panics or deadlocks
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out")
		}
	})
}

// TestDaemon_HTTPServerLifecycle tests HTTP server management
func TestDaemon_HTTPServerLifecycle(t *testing.T) {
	t.Run("start on port", func(t *testing.T) {
		d := newTestDaemon()
		defer d.cancel()

		port := 18080 // Use high port to avoid conflicts
		err := d.httpServer.Start(port)
		if err != nil {
			t.Fatalf("Failed to start HTTP server: %v", err)
		}
		defer d.httpServer.Stop()

		// Give server time to start
		time.Sleep(100 * time.Millisecond)

		// Verify server is listening
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
		if err != nil {
			t.Fatalf("HTTP server not responding: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("restart on new port", func(t *testing.T) {
		d := newTestDaemon()
		defer d.cancel()

		// Start on first port
		port1 := 18081
		err := d.httpServer.Start(port1)
		if err != nil {
			t.Fatalf("Failed to start HTTP server: %v", err)
		}
		time.Sleep(100 * time.Millisecond)

		// Restart on second port
		port2 := 18082
		err = d.httpServer.Start(port2)
		if err != nil {
			t.Fatalf("Failed to restart HTTP server: %v", err)
		}
		defer d.httpServer.Stop()
		time.Sleep(100 * time.Millisecond)

		// Old port should not respond
		_, err = http.Get(fmt.Sprintf("http://localhost:%d/health", port1))
		if err == nil {
			t.Error("Old port should not be listening")
		}

		// New port should respond
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port2))
		if err != nil {
			t.Fatalf("New port not responding: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("disable with port 0", func(t *testing.T) {
		d := newTestDaemon()
		defer d.cancel()

		// Start server
		port := 18083
		err := d.httpServer.Start(port)
		if err != nil {
			t.Fatalf("Failed to start HTTP server: %v", err)
		}
		time.Sleep(100 * time.Millisecond)

		// Disable by using port 0
		err = d.httpServer.Start(0)
		if err != nil {
			t.Fatalf("Failed to disable HTTP server: %v", err)
		}
		time.Sleep(100 * time.Millisecond)

		// Server should not be listening
		_, err = http.Get(fmt.Sprintf("http://localhost:%d/health", port))
		if err == nil {
			t.Error("Server should be disabled")
		}
	})

	t.Run("graceful shutdown", func(t *testing.T) {
		d := newTestDaemon()
		defer d.cancel()

		port := 18084
		err := d.httpServer.Start(port)
		if err != nil {
			t.Fatalf("Failed to start HTTP server: %v", err)
		}
		time.Sleep(100 * time.Millisecond)

		// Stop server
		err = d.httpServer.Stop()
		if err != nil {
			t.Fatalf("Failed to stop HTTP server: %v", err)
		}
		time.Sleep(100 * time.Millisecond)

		// Server should not respond
		_, err = http.Get(fmt.Sprintf("http://localhost:%d/health", port))
		if err == nil {
			t.Error("Server should be stopped")
		}
	})
}

// TestDaemon_UpdateSemanticProvider tests provider recreation
func TestDaemon_UpdateSemanticProvider(t *testing.T) {
	t.Run("create provider", func(t *testing.T) {
		d := newTestDaemon()
		defer d.cancel()

		cfg := &config.Config{
			Semantic: config.SemanticConfig{
				Enabled:     true,
				Provider:    "claude",
				MaxFileSize: 10 * 1024 * 1024,
				APIKey:      "test-key",
				Model:       "claude-sonnet-4-5-20250929",
				MaxTokens:   1000,
				Timeout:     30,
			},
		}

		err := d.updateSemanticProvider(cfg)
		if err != nil {
			t.Fatalf("updateSemanticProvider() failed: %v", err)
		}

		provider := d.GetSemanticProvider()
		if provider == nil {
			t.Error("Expected non-nil provider after update")
		}
	})

	t.Run("disable provider", func(t *testing.T) {
		d := newTestDaemon()
		defer d.cancel()

		// First create provider
		cfg1 := &config.Config{
			Semantic: config.SemanticConfig{
				Enabled:     true,
				Provider:    "claude",
				MaxFileSize: 10 * 1024 * 1024,
				APIKey:      "test-key",
				Model:       "claude-sonnet-4-5-20250929",
				MaxTokens:   1000,
				Timeout:     30,
			},
		}
		err := d.updateSemanticProvider(cfg1)
		if err != nil {
			t.Fatalf("updateSemanticProvider() failed: %v", err)
		}

		if d.GetSemanticProvider() == nil {
			t.Fatal("Provider should be non-nil")
		}

		// Disable
		cfg2 := &config.Config{
			Semantic: config.SemanticConfig{Enabled: false},
		}
		err = d.updateSemanticProvider(cfg2)
		if err != nil {
			t.Fatalf("updateSemanticProvider() failed: %v", err)
		}

		// Provider stays in memory (atomic.Value can't store nil), but
		// provider info fields are cleared to indicate disabled state
		provider, model := d.GetProviderInfo()
		if provider != "" || model != "" {
			t.Errorf("Expected empty provider info after disabling, got provider=%q model=%q", provider, model)
		}
	})
}

// TestDaemon_UpdateLogLevel tests log level changes
func TestDaemon_UpdateLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		wantErr  bool
	}{
		{name: "debug", logLevel: "debug", wantErr: false},
		{name: "info", logLevel: "info", wantErr: false},
		{name: "warn", logLevel: "warn", wantErr: false},
		{name: "error", logLevel: "error", wantErr: false},
		{name: "invalid", logLevel: "invalid", wantErr: true},
		{name: "DEBUG uppercase", logLevel: "DEBUG", wantErr: false}, // Should handle case
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newTestDaemon()
			defer d.cancel()

			oldLogger := d.GetLogger()

			cfg := &config.Config{
				Daemon: config.DaemonConfig{
					LogLevel: tt.logLevel,
				},
			}

			err := d.updateLogLevel(cfg)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error for invalid log level")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				newLogger := d.GetLogger()
				if newLogger == oldLogger {
					t.Error("Logger should be replaced")
				}
			}
		})
	}
}

// TestDaemon_DetectChanges tests change detection
func TestDaemon_DetectChanges(t *testing.T) {
	d := newTestDaemon()
	defer d.cancel()

	t.Run("no changes", func(t *testing.T) {
		cfg := &config.Config{
			Semantic: config.SemanticConfig{APIKey: "key1"},
			Daemon: config.DaemonConfig{Workers: 3, LogLevel: "info"},
		}

		changes := d.detectChanges(cfg, cfg)

		for key, changed := range changes {
			if changed {
				t.Errorf("Expected no changes, but %q changed", key)
			}
		}
	})

	t.Run("semantic changed", func(t *testing.T) {
		oldCfg := &config.Config{
			Semantic: config.SemanticConfig{APIKey: "old-key"},
		}
		newCfg := &config.Config{
			Semantic: config.SemanticConfig{APIKey: "new-key"},
		}

		changes := d.detectChanges(oldCfg, newCfg)

		if !changes["semantic"] {
			t.Error("Expected semantic to be detected as changed")
		}
	})

	t.Run("workers changed", func(t *testing.T) {
		oldCfg := &config.Config{
			Daemon: config.DaemonConfig{Workers: 3},
		}
		newCfg := &config.Config{
			Daemon: config.DaemonConfig{Workers: 8},
		}

		changes := d.detectChanges(oldCfg, newCfg)

		if !changes["workers"] {
			t.Error("Expected workers to be detected as changed")
		}
	})

	t.Run("log_level changed", func(t *testing.T) {
		oldCfg := &config.Config{
			Daemon: config.DaemonConfig{LogLevel: "info"},
		}
		newCfg := &config.Config{
			Daemon: config.DaemonConfig{LogLevel: "debug"},
		}

		changes := d.detectChanges(oldCfg, newCfg)

		if !changes["log_level"] {
			t.Error("Expected log_level to be detected as changed")
		}
	})
}
