package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig_ReturnsTypedConfig(t *testing.T) {
	// Create temp directory with config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `log_level: debug
log_file: /var/log/test.log
daemon:
  http_port: 8080
  http_bind: "0.0.0.0"
  shutdown_timeout: 60
  pid_file: /tmp/test.pid
  registry_path: /tmp/registry.db
  metrics:
    collection_interval: 30
  event_bus:
    buffer_size: 200
    critical_queue_path: /tmp/critqueue.db
    critical_queue_capacity: 2000
graph:
  host: redis.example.com
  port: 6380
  name: testgraph
  password_env: TEST_PASSWORD
  max_retries: 5
  retry_delay_ms: 2000
  write_queue_size: 500
semantic:
  provider: openai
  model: gpt-4
  rate_limit: 100
  api_key_env: TEST_API_KEY
embeddings:
  enabled: true
  provider: openai
  model: text-embedding-3-small
  dimensions: 1536
  api_key_env: TEST_EMBED_KEY
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config; %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}

	// Verify values
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
	if cfg.Daemon.HTTPPort != 8080 {
		t.Errorf("Daemon.HTTPPort = %d, want %d", cfg.Daemon.HTTPPort, 8080)
	}
	if cfg.Daemon.EventBus.BufferSize != 200 {
		t.Errorf("Daemon.EventBus.BufferSize = %d, want %d", cfg.Daemon.EventBus.BufferSize, 200)
	}
	if cfg.Graph.Host != "redis.example.com" {
		t.Errorf("Graph.Host = %q, want %q", cfg.Graph.Host, "redis.example.com")
	}
	if cfg.Semantic.Provider != "openai" {
		t.Errorf("Semantic.Provider = %q, want %q", cfg.Semantic.Provider, "openai")
	}
	if cfg.Embeddings.Dimensions != 1536 {
		t.Errorf("Embeddings.Dimensions = %d, want %d", cfg.Embeddings.Dimensions, 1536)
	}
}

func TestLoad_InvalidConfig_ReturnsValidationError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Invalid port number
	configContent := `daemon:
  http_port: 99999
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config; %v", err)
	}

	_, err := LoadFromPath(configPath)
	if err == nil {
		t.Fatal("LoadFromPath() expected error for invalid port")
	}

	if !IsValidationError(err) {
		t.Errorf("expected validation error, got %T: %v", err, err)
	}
}

func TestLoad_MissingFile_ReturnsError(t *testing.T) {
	_, err := LoadFromPath("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("LoadFromPath() expected error for missing file")
	}
}

func TestLoad_InvalidYAML_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `invalid: [yaml: content`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config; %v", err)
	}

	_, err := LoadFromPath(configPath)
	if err == nil {
		t.Fatal("LoadFromPath() expected error for invalid YAML")
	}
}

func TestLoadWithDefaults_ReturnsDefaultConfig(t *testing.T) {
	cfg := LoadWithDefaults()

	if cfg == nil {
		t.Fatal("LoadWithDefaults() returned nil")
	}

	if cfg.LogLevel != DefaultLogLevel {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, DefaultLogLevel)
	}
	if cfg.Daemon.HTTPPort != DefaultDaemonHTTPPort {
		t.Errorf("Daemon.HTTPPort = %d, want %d", cfg.Daemon.HTTPPort, DefaultDaemonHTTPPort)
	}
	if cfg.Semantic.Provider != DefaultSemanticProvider {
		t.Errorf("Semantic.Provider = %q, want %q", cfg.Semantic.Provider, DefaultSemanticProvider)
	}
}

func TestLoad_UsesViperDefaults_WhenKeysNotInFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Minimal config - should get defaults for unspecified keys
	configContent := `log_level: warn
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config; %v", err)
	}

	cfg, err := LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}

	// Specified value
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "warn")
	}

	// Default values
	if cfg.Daemon.HTTPPort != DefaultDaemonHTTPPort {
		t.Errorf("Daemon.HTTPPort = %d, want default %d", cfg.Daemon.HTTPPort, DefaultDaemonHTTPPort)
	}
	if cfg.Semantic.Provider != DefaultSemanticProvider {
		t.Errorf("Semantic.Provider = %q, want default %q", cfg.Semantic.Provider, DefaultSemanticProvider)
	}
}
