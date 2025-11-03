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
	"github.com/leefowlercu/agentic-memorizer/internal/semantic"
	"gopkg.in/natefinch/lumberjack.v2"
)

// newTestDaemon creates a minimal daemon instance for testing
func newTestDaemon() *Daemon {
	ctx, cancel := context.WithCancel(context.Background())

	cfg := &config.Config{
		MemoryRoot: "/test/memory",
		Analysis:   config.AnalysisConfig{CacheDir: "/test/cache"},
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

	d := &Daemon{
		cfg:               cfg,
		logger:            logger,
		logWriter:         logWriter,
		ctx:               ctx,
		cancel:            cancel,
		rebuildIntervalCh: make(chan time.Duration, 1),
		healthMetrics:     &HealthMetrics{},
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

// TestDaemon_SemanticAnalyzerSwap tests atomic analyzer replacement
func TestDaemon_SemanticAnalyzerSwap(t *testing.T) {
	t.Run("concurrent access", func(t *testing.T) {
		d := newTestDaemon()
		defer d.cancel()

		// Set initial analyzer
		client := semantic.NewClient("test-key", "claude-3-sonnet", 1000, 30)
		analyzer1 := semantic.NewAnalyzer(client, false, 10*1024*1024)
		d.SetSemanticAnalyzer(analyzer1)

		const numReaders = 100
		var wg sync.WaitGroup
		errors := make(chan error, numReaders)

		// Launch concurrent readers
		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					a := d.GetSemanticAnalyzer()
					if a == nil {
						errors <- fmt.Errorf("GetSemanticAnalyzer() returned nil")
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

		// Set initial analyzer
		client := semantic.NewClient("test-key", "claude-3-sonnet", 1000, 30)
		analyzer1 := semantic.NewAnalyzer(client, false, 10*1024*1024)
		d.SetSemanticAnalyzer(analyzer1)

		const numReaders = 50
		const numWriters = 10
		var wg sync.WaitGroup

		// Concurrent readers
		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					a := d.GetSemanticAnalyzer()
					if a == nil {
						t.Error("GetSemanticAnalyzer() returned nil during swap")
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
					client := semantic.NewClient(fmt.Sprintf("key-%d", id), "claude-3-sonnet", 1000, 30)
					analyzer := semantic.NewAnalyzer(client, false, 10*1024*1024)
					d.SetSemanticAnalyzer(analyzer)
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

		// Initially nil
		if d.GetSemanticAnalyzer() != nil {
			t.Error("Expected nil analyzer initially")
		}

		// Set to non-nil
		client := semantic.NewClient("test-key", "claude-3-sonnet", 1000, 30)
		analyzer := semantic.NewAnalyzer(client, false, 10*1024*1024)
		d.SetSemanticAnalyzer(analyzer)

		if d.GetSemanticAnalyzer() == nil {
			t.Error("Expected non-nil analyzer after setting")
		}

		// Set back to nil
		d.SetSemanticAnalyzer(nil)

		if d.GetSemanticAnalyzer() != nil {
			t.Error("Expected nil analyzer after setting to nil")
		}
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

// TestDaemon_HealthServerLifecycle tests health server management
func TestDaemon_HealthServerLifecycle(t *testing.T) {
	t.Run("start on port", func(t *testing.T) {
		d := newTestDaemon()
		defer d.cancel()

		port := 18080 // Use high port to avoid conflicts
		err := d.startHealthServer(port)
		if err != nil {
			t.Fatalf("Failed to start health server: %v", err)
		}
		defer d.stopHealthServer()

		// Give server time to start
		time.Sleep(100 * time.Millisecond)

		// Verify server is listening
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d", port))
		if err != nil {
			t.Fatalf("Health server not responding: %v", err)
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
		err := d.startHealthServer(port1)
		if err != nil {
			t.Fatalf("Failed to start health server: %v", err)
		}
		time.Sleep(100 * time.Millisecond)

		// Restart on second port
		port2 := 18082
		err = d.startHealthServer(port2)
		if err != nil {
			t.Fatalf("Failed to restart health server: %v", err)
		}
		defer d.stopHealthServer()
		time.Sleep(100 * time.Millisecond)

		// Old port should not respond
		_, err = http.Get(fmt.Sprintf("http://localhost:%d", port1))
		if err == nil {
			t.Error("Old port should not be listening")
		}

		// New port should respond
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d", port2))
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
		err := d.startHealthServer(port)
		if err != nil {
			t.Fatalf("Failed to start health server: %v", err)
		}
		time.Sleep(100 * time.Millisecond)

		// Disable by using port 0
		err = d.startHealthServer(0)
		if err != nil {
			t.Fatalf("Failed to disable health server: %v", err)
		}
		time.Sleep(100 * time.Millisecond)

		// Server should not be listening
		_, err = http.Get(fmt.Sprintf("http://localhost:%d", port))
		if err == nil {
			t.Error("Server should be disabled")
		}
	})

	t.Run("graceful shutdown", func(t *testing.T) {
		d := newTestDaemon()
		defer d.cancel()

		port := 18084
		err := d.startHealthServer(port)
		if err != nil {
			t.Fatalf("Failed to start health server: %v", err)
		}
		time.Sleep(100 * time.Millisecond)

		// Stop server
		err = d.stopHealthServer()
		if err != nil {
			t.Fatalf("Failed to stop health server: %v", err)
		}
		time.Sleep(100 * time.Millisecond)

		// Server should not respond
		_, err = http.Get(fmt.Sprintf("http://localhost:%d", port))
		if err == nil {
			t.Error("Server should be stopped")
		}
	})
}

// TestDaemon_UpdateSemanticAnalyzer tests analyzer recreation
func TestDaemon_UpdateSemanticAnalyzer(t *testing.T) {
	t.Run("create analyzer", func(t *testing.T) {
		d := newTestDaemon()
		defer d.cancel()

		cfg := &config.Config{
			Analysis: config.AnalysisConfig{
				Enable:      true,
				MaxFileSize: 10 * 1024 * 1024,
			},
			Claude: config.ClaudeConfig{
				APIKey:         "test-key",
				Model:          "claude-3-sonnet",
				MaxTokens:      1000,
				TimeoutSeconds: 30,
				EnableVision:   true,
			},
		}

		err := d.updateSemanticAnalyzer(cfg)
		if err != nil {
			t.Fatalf("updateSemanticAnalyzer() failed: %v", err)
		}

		analyzer := d.GetSemanticAnalyzer()
		if analyzer == nil {
			t.Error("Expected non-nil analyzer after update")
		}
	})

	t.Run("disable analyzer", func(t *testing.T) {
		d := newTestDaemon()
		defer d.cancel()

		// First create analyzer
		cfg1 := &config.Config{
			Analysis: config.AnalysisConfig{Enable: true, MaxFileSize: 10 * 1024 * 1024},
			Claude:   config.ClaudeConfig{APIKey: "key", Model: "claude-3-sonnet"},
		}
		d.updateSemanticAnalyzer(cfg1)

		if d.GetSemanticAnalyzer() == nil {
			t.Fatal("Analyzer should be non-nil")
		}

		// Disable
		cfg2 := &config.Config{
			Analysis: config.AnalysisConfig{Enable: false},
		}
		err := d.updateSemanticAnalyzer(cfg2)
		if err != nil {
			t.Fatalf("updateSemanticAnalyzer() failed: %v", err)
		}

		if d.GetSemanticAnalyzer() != nil {
			t.Error("Expected nil analyzer after disabling")
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
			Claude: config.ClaudeConfig{APIKey: "key1"},
			Daemon: config.DaemonConfig{Workers: 3, LogLevel: "info"},
		}

		changes := d.detectChanges(cfg, cfg)

		for key, changed := range changes {
			if changed {
				t.Errorf("Expected no changes, but %q changed", key)
			}
		}
	})

	t.Run("claude changed", func(t *testing.T) {
		oldCfg := &config.Config{
			Claude: config.ClaudeConfig{APIKey: "old-key"},
		}
		newCfg := &config.Config{
			Claude: config.ClaudeConfig{APIKey: "new-key"},
		}

		changes := d.detectChanges(oldCfg, newCfg)

		if !changes["claude"] {
			t.Error("Expected claude to be detected as changed")
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
