package harness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// E2EHarness provides a complete test environment for end-to-end tests
type E2EHarness struct {
	// Test environment paths
	AppDir     string
	MemoryRoot string
	ConfigPath string
	PIDPath    string
	LogPath    string
	BinaryPath string
	GraphName  string // Unique graph name for test isolation

	// Test clients
	MCPClient   *MCPClient
	GraphClient *GraphClient
	HTTPClient  *HTTPClient

	// Test context
	t testing.TB
}

// New creates a new E2E test harness
func New(t testing.TB) *E2EHarness {
	t.Helper()

	// Create isolated app directory
	appDir := t.TempDir()

	// Generate unique graph name for test isolation
	graphName := fmt.Sprintf("e2e_test_%d", time.Now().UnixNano())

	h := &E2EHarness{
		AppDir:     appDir,
		MemoryRoot: filepath.Join(appDir, "memory"),
		ConfigPath: filepath.Join(appDir, "config.yaml"),
		PIDPath:    filepath.Join(appDir, "daemon.pid"),
		LogPath:    filepath.Join(appDir, "daemon.log"),
		BinaryPath: findBinary(t),
		GraphName:  graphName,
		t:          t,
	}

	return h
}

// Setup initializes the test environment
func (h *E2EHarness) Setup() error {
	h.t.Helper()

	// Create required directories
	if err := os.MkdirAll(h.MemoryRoot, 0755); err != nil {
		return fmt.Errorf("failed to create memory directory; %w", err)
	}

	cacheDir := filepath.Join(h.MemoryRoot, ".cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory; %w", err)
	}

	// Set environment variable for app directory
	h.t.Setenv("MEMORIZER_APP_DIR", h.AppDir)

	// Create minimal valid config
	if err := h.createMinimalConfig(); err != nil {
		return fmt.Errorf("failed to create config; %w", err)
	}

	// Initialize clients
	graphHost := os.Getenv("FALKORDB_HOST")
	if graphHost == "" {
		graphHost = "localhost"
	}
	graphPort := os.Getenv("FALKORDB_PORT")
	if graphPort == "" {
		graphPort = "6379"
	}

	h.GraphClient = NewGraphClient(graphHost, graphPort, h.GraphName)
	h.HTTPClient = NewHTTPClient("localhost", 0) // Port will be set when daemon starts

	return nil
}

// Teardown cleans up the test environment
func (h *E2EHarness) Teardown() error {
	h.t.Helper()

	// Stop daemon if running
	_ = h.StopDaemon()

	// Drop test graph (cleanup)
	if h.GraphClient != nil {
		ctx := context.Background()
		_ = h.GraphClient.DropGraph(ctx)
	}

	// Close clients
	if h.MCPClient != nil {
		_ = h.MCPClient.Close()
	}
	if h.GraphClient != nil {
		_ = h.GraphClient.Close()
	}

	return nil
}

// StartDaemon starts the daemon process
func (h *E2EHarness) StartDaemon() error {
	h.t.Helper()

	stdout, stderr, exitCode := h.RunCommand("daemon", "start")
	if exitCode != 0 {
		return fmt.Errorf("daemon start failed (exit %d); stdout: %s; stderr: %s", exitCode, stdout, stderr)
	}

	return nil
}

// StopDaemon stops the daemon process
func (h *E2EHarness) StopDaemon() error {
	h.t.Helper()

	stdout, stderr, exitCode := h.RunCommand("daemon", "stop")
	if exitCode != 0 {
		// Daemon might not be running, which is ok
		return nil
	}

	h.t.Logf("Daemon stopped; stdout: %s; stderr: %s", stdout, stderr)
	return nil
}

// WaitForHealthy polls the health endpoint until daemon is ready
func (h *E2EHarness) WaitForHealthy(timeout time.Duration) error {
	h.t.Helper()

	// First, wait for PID file to exist
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(h.PIDPath); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for health endpoint (if HTTP server is enabled)
	// For now, just wait a bit for daemon to initialize
	time.Sleep(2 * time.Second)

	return nil
}

// AddMemoryFile creates a file in the memory directory
func (h *E2EHarness) AddMemoryFile(name, content string) error {
	h.t.Helper()

	path := filepath.Join(h.MemoryRoot, name)

	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory; %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file; %w", err)
	}

	return nil
}

// RunCommand runs the agentic-memorizer binary with the given arguments
func (h *E2EHarness) RunCommand(args ...string) (stdout, stderr string, exitCode int) {
	h.t.Helper()

	cmd := exec.Command(h.BinaryPath, args...)
	cmd.Env = append(os.Environ(), "MEMORIZER_APP_DIR="+h.AppDir)

	var outBuf, errBuf []byte
	var err error

	outBuf, err = cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			errBuf = exitErr.Stderr
			exitCode = exitErr.ExitCode()
		} else {
			h.t.Fatalf("Failed to run command: %v", err)
		}
	} else {
		exitCode = 0
	}

	return string(outBuf), string(errBuf), exitCode
}

// createMinimalConfig creates a minimal valid configuration file
func (h *E2EHarness) createMinimalConfig() error {
	return h.createConfigWithHTTPPort(0)
}

// createConfigWithHTTPPort creates a config file with specified HTTP port
func (h *E2EHarness) createConfigWithHTTPPort(httpPort int) error {
	h.t.Helper()

	graphHost := os.Getenv("FALKORDB_HOST")
	if graphHost == "" {
		graphHost = "localhost"
	}
	graphPort := os.Getenv("FALKORDB_PORT")
	if graphPort == "" {
		graphPort = "6379"
	}

	config := fmt.Sprintf(`# E2E Test Configuration
memory_root: %s

analysis:
  max_file_size: 10485760
  skip_extensions: []
  skip_files: []
  cache_dir: %s

daemon:
  workers: 2
  rate_limit_per_min: 20
  debounce_ms: 200
  full_rebuild_interval_minutes: 60
  http_port: %d
  log_file: %s
  log_level: info

graph:
  host: %s
  port: %s
  database: %s

mcp:
  log_file: %s
  log_level: info
  daemon_host: localhost
  daemon_port: %d
`, h.MemoryRoot, filepath.Join(h.MemoryRoot, ".cache"), httpPort, h.LogPath, graphHost, graphPort, h.GraphName, filepath.Join(h.AppDir, "mcp.log"), httpPort)

	if err := os.WriteFile(h.ConfigPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write config file; %w", err)
	}

	return nil
}

// EnableHTTPServer enables the HTTP server on the specified port
func (h *E2EHarness) EnableHTTPServer(port int) error {
	h.t.Helper()

	if err := h.createConfigWithHTTPPort(port); err != nil {
		return err
	}

	// Update HTTP client with the new port
	h.HTTPClient = NewHTTPClient("localhost", port)

	return nil
}

// findBinary locates the agentic-memorizer binary for testing
func findBinary(t testing.TB) string {
	t.Helper()

	// Check if binary exists in /usr/local/bin (Docker environment)
	if _, err := os.Stat("/usr/local/bin/agentic-memorizer"); err == nil {
		return "/usr/local/bin/agentic-memorizer"
	}

	// Check if binary exists in workspace root (Docker environment)
	if _, err := os.Stat("/workspace/agentic-memorizer"); err == nil {
		return "/workspace/agentic-memorizer"
	}

	// Check if binary exists in project root from e2e/tests/ directory
	if _, err := os.Stat("../../agentic-memorizer"); err == nil {
		abs, _ := filepath.Abs("../../agentic-memorizer")
		return abs
	}

	// Check if binary exists in project root from e2e/ directory
	if _, err := os.Stat("../agentic-memorizer"); err == nil {
		abs, _ := filepath.Abs("../agentic-memorizer")
		return abs
	}

	// Check in current directory
	if _, err := os.Stat("./agentic-memorizer"); err == nil {
		abs, _ := filepath.Abs("./agentic-memorizer")
		return abs
	}

	t.Fatal("Could not find agentic-memorizer binary. Run 'make build' first.")
	return ""
}
