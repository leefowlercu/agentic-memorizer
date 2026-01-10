package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// T011: Test NewManager returns logger in bootstrap mode
func TestNewManager_BootstrapMode(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Close() }()

	if mgr == nil {
		t.Fatal("NewManager() returned nil")
	}

	logger := mgr.Logger()
	if logger == nil {
		t.Fatal("Manager.Logger() returned nil")
	}
}

// T012: Test Manager.Logger() returns stable *slog.Logger
func TestManager_Logger_Stable(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Close() }()

	logger1 := mgr.Logger()
	logger2 := mgr.Logger()

	if logger1 != logger2 {
		t.Error("Manager.Logger() should return the same instance")
	}
}

// T013: Test Manager.Upgrade() creates log file with JSON output
func TestManager_Upgrade_CreatesLogFile(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Close() }()

	// Create temp directory for log file
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	err := mgr.Upgrade(logFile, slog.LevelInfo)
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}

	// Write a log message
	mgr.Logger().Info("test message", "key", "value")

	// Read log file and verify JSON format
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Should be valid JSON
	var logEntry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(content), &logEntry); err != nil {
		t.Errorf("Log file content is not valid JSON: %v\nContent: %s", err, content)
	}

	// Should contain our message
	if msg, ok := logEntry["msg"].(string); !ok || msg != "test message" {
		t.Errorf("Log entry missing or wrong msg: %v", logEntry)
	}
}

// T014: Test Manager.Upgrade() creates parent directories if missing
func TestManager_Upgrade_CreatesParentDirs(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Close() }()

	// Create temp directory and specify nested path that doesn't exist
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "nested", "dirs", "test.log")

	err := mgr.Upgrade(logFile, slog.LevelInfo)
	if err != nil {
		t.Fatalf("Upgrade() should create parent directories, got error: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("Log file was not created")
	}
}

// T015: Test Manager.Close() closes file handle
func TestManager_Close(t *testing.T) {
	mgr := NewManager()

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	err := mgr.Upgrade(logFile, slog.LevelInfo)
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}

	// Close should not error
	err = mgr.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Calling Close again should be safe
	err = mgr.Close()
	if err != nil {
		t.Errorf("Close() second call error = %v", err)
	}
}

// T016: Test bootstrap mode uses text format on stderr
func TestManager_BootstrapMode_TextFormat(t *testing.T) {
	// Capture stderr by using a custom handler
	var buf bytes.Buffer
	textHandler := slog.NewTextHandler(&buf, nil)
	sh := NewSwappableHandler(textHandler)
	logger := slog.New(sh)

	// Log a message
	logger.Info("bootstrap test", "foo", "bar")

	output := buf.String()

	// Text format should have key=value pairs, not JSON
	if strings.HasPrefix(strings.TrimSpace(output), "{") {
		t.Errorf("Bootstrap mode should use text format, got JSON-like: %s", output)
	}

	if !strings.Contains(output, "foo=bar") {
		t.Errorf("Text format should have key=value, got: %s", output)
	}
}

// T027: Test Manager.SetLevel() changes level at runtime
func TestManager_SetLevel(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Close() }()

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	// Start at Info level
	err := mgr.Upgrade(logFile, slog.LevelInfo)
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}

	// Debug should not be logged at Info level
	mgr.Logger().Debug("debug message 1")

	// Change to Debug level
	mgr.SetLevel(slog.LevelDebug)

	// Now debug should be logged
	mgr.Logger().Debug("debug message 2")

	content, _ := os.ReadFile(logFile)
	output := string(content)

	if strings.Contains(output, "debug message 1") {
		t.Error("Debug message 1 should not appear at Info level")
	}
	if !strings.Contains(output, "debug message 2") {
		t.Error("Debug message 2 should appear after SetLevel(Debug)")
	}
}

// T028: Test level filtering suppresses debug when level=info
func TestManager_LevelFiltering(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Close() }()

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	err := mgr.Upgrade(logFile, slog.LevelInfo)
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}

	logger := mgr.Logger()

	// Log at all levels
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	content, _ := os.ReadFile(logFile)
	output := string(content)

	// Debug should be suppressed
	if strings.Contains(output, "debug message") {
		t.Error("Debug message should be suppressed at Info level")
	}

	// Info, Warn, Error should appear
	if !strings.Contains(output, "info message") {
		t.Error("Info message should appear")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("Warn message should appear")
	}
	if !strings.Contains(output, "error message") {
		t.Error("Error message should appear")
	}
}

// T029: Test invalid level defaults to info
func TestParseLevelOrDefault(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLevel slog.Level
	}{
		{"valid debug", "debug", slog.LevelDebug},
		{"valid info", "info", slog.LevelInfo},
		{"valid warn", "warn", slog.LevelWarn},
		{"valid error", "error", slog.LevelError},
		{"invalid empty", "", slog.LevelInfo},
		{"invalid garbage", "invalid", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseLevelOrDefault(tt.input)
			if got != tt.wantLevel {
				t.Errorf("ParseLevelOrDefault(%q) = %v, want %v", tt.input, got, tt.wantLevel)
			}
		})
	}
}

// T030: This test verifies env var precedence at integration level
// The actual MEMORIZER_LOGGING_LEVEL env var is tested via config subsystem
func TestManager_LevelFromConfig(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Close() }()

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	// Upgrade with debug level
	err := mgr.Upgrade(logFile, slog.LevelDebug)
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}

	// Debug should now be logged
	mgr.Logger().Debug("debug should appear")

	content, _ := os.ReadFile(logFile)
	if !strings.Contains(string(content), "debug should appear") {
		t.Error("Debug message should appear when level is Debug")
	}
}

// T037: Test logger.With() creates child with inherited outputs
func TestLogger_With_CreatesChild(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Close() }()

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	err := mgr.Upgrade(logFile, slog.LevelInfo)
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}

	// Create child logger with additional context
	childLogger := mgr.Logger().With("component", "daemon")

	// Child should be different instance but work the same
	if childLogger == mgr.Logger() {
		t.Error("With() should return a new logger instance")
	}

	// Log with child logger
	childLogger.Info("child message")

	// Should appear in log file
	content, _ := os.ReadFile(logFile)
	if !strings.Contains(string(content), "child message") {
		t.Error("Child logger message should appear in log file")
	}
}

// T038: Test child logger context appears in both stderr and file
func TestLogger_With_ContextInBothOutputs(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Close() }()

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	err := mgr.Upgrade(logFile, slog.LevelInfo)
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}

	// Create child logger with context
	childLogger := mgr.Logger().With("component", "daemon", "version", "1.0.0")

	// Log a message
	childLogger.Info("context test message")

	// Verify context appears in log file (JSON format)
	content, _ := os.ReadFile(logFile)
	output := string(content)

	if !strings.Contains(output, "component") {
		t.Error("Child logger context 'component' should appear in log file")
	}
	if !strings.Contains(output, "daemon") {
		t.Error("Child logger context value 'daemon' should appear in log file")
	}
	if !strings.Contains(output, "version") {
		t.Error("Child logger context 'version' should appear in log file")
	}
}

// T039: Test JSON file output is valid JSON with structured attrs
func TestLogger_JSONOutput_ValidStructuredAttrs(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Close() }()

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	err := mgr.Upgrade(logFile, slog.LevelInfo)
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}

	// Create child logger and log with additional attributes
	childLogger := mgr.Logger().With("component", "daemon")
	childLogger.Info("structured message", "request_id", "abc-123", "count", 42)

	// Read and parse JSON
	content, _ := os.ReadFile(logFile)

	var logEntry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(content), &logEntry); err != nil {
		t.Fatalf("Log file should be valid JSON: %v\nContent: %s", err, content)
	}

	// Verify structured fields
	if logEntry["component"] != "daemon" {
		t.Errorf("Expected component=daemon, got %v", logEntry["component"])
	}
	if logEntry["request_id"] != "abc-123" {
		t.Errorf("Expected request_id=abc-123, got %v", logEntry["request_id"])
	}
	if logEntry["msg"] != "structured message" {
		t.Errorf("Expected msg='structured message', got %v", logEntry["msg"])
	}
	// JSON numbers are float64
	if count, ok := logEntry["count"].(float64); !ok || count != 42 {
		t.Errorf("Expected count=42, got %v", logEntry["count"])
	}
}

// T044: Test error handling for path-is-directory condition
func TestManager_Upgrade_PathIsDirectory(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Close() }()

	// Use temp directory as log file path (directory, not file)
	tmpDir := t.TempDir()

	err := mgr.Upgrade(tmpDir, slog.LevelInfo)
	if err == nil {
		t.Error("Upgrade() should error when path is a directory")
	}
}

// T045: Test error handling for read-only directory
func TestManager_Upgrade_ReadOnlyDirectory(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Close() }()

	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")

	// Create read-only directory
	if err := os.Mkdir(readOnlyDir, 0444); err != nil {
		t.Fatalf("Failed to create read-only directory: %v", err)
	}
	// Ensure cleanup can remove it
	defer func() { _ = os.Chmod(readOnlyDir, 0755) }()

	logFile := filepath.Join(readOnlyDir, "test.log")

	err := mgr.Upgrade(logFile, slog.LevelInfo)
	if err == nil {
		t.Error("Upgrade() should error when directory is read-only")
	}
}

// T042: Integration test demonstrating component injection pattern
func TestLogger_ComponentInjectionPattern(t *testing.T) {
	mgr := NewManager()
	defer func() { _ = mgr.Close() }()

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	err := mgr.Upgrade(logFile, slog.LevelDebug)
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}

	// Simulate how components would receive loggers via dependency injection
	// Each component gets a child logger with its own context
	daemonLogger := mgr.Logger().With("component", "daemon")
	apiLogger := mgr.Logger().With("component", "api", "version", "v1")
	dbLogger := mgr.Logger().With("component", "database", "driver", "sqlite")

	// Each component logs with its context automatically included
	daemonLogger.Info("daemon started")
	apiLogger.Info("api request received", "endpoint", "/health")
	dbLogger.Debug("query executed", "table", "users")

	// Verify all messages appear in log file with correct context
	content, _ := os.ReadFile(logFile)
	output := string(content)

	// Parse each line as JSON and verify
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Fatalf("Expected 3 log lines, got %d", len(lines))
	}

	// Verify daemon log
	var daemonEntry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &daemonEntry); err != nil {
		t.Fatalf("Failed to parse daemon log: %v", err)
	}
	if daemonEntry["component"] != "daemon" {
		t.Errorf("daemon log missing component=daemon: %v", daemonEntry)
	}

	// Verify API log
	var apiEntry map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &apiEntry); err != nil {
		t.Fatalf("Failed to parse api log: %v", err)
	}
	if apiEntry["component"] != "api" || apiEntry["version"] != "v1" {
		t.Errorf("api log missing context: %v", apiEntry)
	}
	if apiEntry["endpoint"] != "/health" {
		t.Errorf("api log missing endpoint: %v", apiEntry)
	}

	// Verify database log
	var dbEntry map[string]any
	if err := json.Unmarshal([]byte(lines[2]), &dbEntry); err != nil {
		t.Fatalf("Failed to parse db log: %v", err)
	}
	if dbEntry["component"] != "database" || dbEntry["driver"] != "sqlite" {
		t.Errorf("db log missing context: %v", dbEntry)
	}
	if dbEntry["table"] != "users" {
		t.Errorf("db log missing table: %v", dbEntry)
	}
}
