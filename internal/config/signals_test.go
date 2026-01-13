package config

import (
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

// T045: TestSignalHandler_SIGHUP_TriggersReload
func TestSignalHandler_SIGHUP_TriggersReload(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Initial config
	if err := os.WriteFile(configPath, []byte("daemon:\n  http_port: 8080\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	// Setup signal handler
	SetupSignalHandler()

	// Verify initial value
	cfg := Get()
	if cfg.Daemon.HTTPPort != 8080 {
		t.Errorf("Get().Daemon.HTTPPort = %d, want 8080", cfg.Daemon.HTTPPort)
	}

	// Update config file
	if err := os.WriteFile(configPath, []byte("daemon:\n  http_port: 9999\n"), 0644); err != nil {
		t.Fatalf("failed to update config file: %v", err)
	}

	// Send SIGHUP to ourselves
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGHUP); err != nil {
		t.Fatalf("failed to send SIGHUP: %v", err)
	}

	// Wait briefly for signal to be processed
	time.Sleep(100 * time.Millisecond)

	// Verify config was reloaded
	cfg = Get()
	if cfg.Daemon.HTTPPort != 9999 {
		t.Errorf("Get().Daemon.HTTPPort = %d after SIGHUP, want 9999", cfg.Daemon.HTTPPort)
	}

	// Clean up signal handler
	StopSignalHandler()
}

// T049a: TestSignalHandler_ConcurrentSIGHUP_IgnoresSubsequent
func TestSignalHandler_ConcurrentSIGHUP_IgnoresSubsequent(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Initial config
	if err := os.WriteFile(configPath, []byte("daemon:\n  port: 8080\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	// Setup signal handler
	SetupSignalHandler()

	// Send multiple SIGHUPs rapidly
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
		}()
	}
	wg.Wait()

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Test passes if no panic or deadlock occurred
	// The mutex should prevent concurrent reloads

	// Clean up signal handler
	StopSignalHandler()
}
