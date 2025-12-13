package logging

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		want    slog.Level
		wantErr bool
	}{
		{
			name:    "debug level",
			level:   "debug",
			want:    slog.LevelDebug,
			wantErr: false,
		},
		{
			name:    "info level",
			level:   "info",
			want:    slog.LevelInfo,
			wantErr: false,
		},
		{
			name:    "warn level",
			level:   "warn",
			want:    slog.LevelWarn,
			wantErr: false,
		},
		{
			name:    "error level",
			level:   "error",
			want:    slog.LevelError,
			wantErr: false,
		},
		{
			name:    "invalid level",
			level:   "invalid",
			want:    slog.LevelInfo,
			wantErr: true,
		},
		{
			name:    "empty level",
			level:   "",
			want:    slog.LevelInfo,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLogLevel(tt.level)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLogLevel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseLogLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateLogDir(t *testing.T) {
	// Create temporary directory for testing
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "simple path",
			path:    filepath.Join(tmpDir, "logs", "test.log"),
			wantErr: false,
		},
		{
			name:    "nested path",
			path:    filepath.Join(tmpDir, "a", "b", "c", "test.log"),
			wantErr: false,
		},
		{
			name:    "existing directory",
			path:    filepath.Join(tmpDir, "test.log"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CreateLogDir(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateLogDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify directory was created
				dir := filepath.Dir(tt.path)
				if _, err := os.Stat(dir); os.IsNotExist(err) {
					t.Errorf("CreateLogDir() did not create directory %s", dir)
				}
			}
		})
	}
}

func TestNewLogger_NoFile(t *testing.T) {
	// Test logger without file (stdout)
	var buf bytes.Buffer

	logger, logWriter, err := NewLogger(
		WithLogLevel("info"),
		WithHandler(HandlerText),
		WithAdditionalOutputs(&buf),
	)

	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	if logger == nil {
		t.Fatal("NewLogger() returned nil logger")
	}

	if logWriter != nil {
		t.Error("NewLogger() should return nil logWriter when no file is configured")
	}

	// Test logging
	logger.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("Logger output missing 'test message': %s", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("Logger output missing 'key=value': %s", output)
	}
}

func TestNewLogger_WithFile(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	logger, logWriter, err := NewLogger(
		WithLogFile(logFile),
		WithLogLevel("debug"),
		WithHandler(HandlerText),
	)

	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	if logger == nil {
		t.Fatal("NewLogger() returned nil logger")
	}

	if logWriter == nil {
		t.Fatal("NewLogger() should return non-nil logWriter when file is configured")
	}

	// Test logging
	logger.Debug("debug message")
	logger.Info("info message")

	// Close the writer to flush
	logWriter.Close()

	// Verify log file was created
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Errorf("Log file was not created: %s", logFile)
	}

	// Read and verify content
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	output := string(content)
	if !strings.Contains(output, "debug message") {
		t.Errorf("Log file missing 'debug message': %s", output)
	}
	if !strings.Contains(output, "info message") {
		t.Errorf("Log file missing 'info message': %s", output)
	}
}

func TestNewLogger_JSONHandler(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	logger, logWriter, err := NewLogger(
		WithLogFile(logFile),
		WithLogLevel("info"),
		WithHandler(HandlerJSON),
	)

	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	if logger == nil {
		t.Fatal("NewLogger() returned nil logger")
	}

	// Test logging
	logger.Info("json test", "structured", "data")

	// Close the writer to flush
	logWriter.Close()

	// Read and verify JSON format
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	output := string(content)
	// JSON logs should contain quotes and braces
	if !strings.Contains(output, `"msg"`) {
		t.Errorf("JSON log missing '\"msg\"': %s", output)
	}
	if !strings.Contains(output, `"structured"`) {
		t.Errorf("JSON log missing '\"structured\"': %s", output)
	}
}

func TestNewLogger_MultiWriter(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")
	var buf bytes.Buffer

	logger, logWriter, err := NewLogger(
		WithLogFile(logFile),
		WithLogLevel("info"),
		WithHandler(HandlerText),
		WithAdditionalOutputs(&buf),
	)

	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	if logger == nil {
		t.Fatal("NewLogger() returned nil logger")
	}

	// Test logging
	logger.Info("multi writer test")

	// Close the writer to flush file
	logWriter.Close()

	// Verify buffer received output
	if !strings.Contains(buf.String(), "multi writer test") {
		t.Errorf("Buffer missing 'multi writer test': %s", buf.String())
	}

	// Verify file received output
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "multi writer test") {
		t.Errorf("File missing 'multi writer test': %s", string(content))
	}
}

func TestNewLogger_InvalidLogLevel(t *testing.T) {
	_, _, err := NewLogger(
		WithLogLevel("invalid"),
	)

	if err == nil {
		t.Error("NewLogger() should return error for invalid log level")
	}

	if !strings.Contains(err.Error(), "invalid log level") {
		t.Errorf("Error message should mention 'invalid log level', got: %v", err)
	}
}

func TestNewLogger_DefaultValues(t *testing.T) {
	var buf bytes.Buffer

	// Create logger with no options (use all defaults)
	logger, logWriter, err := NewLogger(
		WithAdditionalOutputs(&buf),
	)

	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	if logger == nil {
		t.Fatal("NewLogger() returned nil logger")
	}

	if logWriter != nil {
		t.Error("NewLogger() should return nil logWriter when no file is configured")
	}

	// Test that default level is info (debug should not appear)
	logger.Debug("should not appear")
	logger.Info("should appear")

	output := buf.String()
	if strings.Contains(output, "should not appear") {
		t.Error("Default log level should be info, but debug message appeared")
	}
	if !strings.Contains(output, "should appear") {
		t.Error("Info message should appear with default log level")
	}
}

func TestNewRotatingWriter(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "rotating.log")

	writer := NewRotatingWriter(logFile)

	if writer == nil {
		t.Fatal("NewRotatingWriter() returned nil")
	}

	// Verify default configuration
	if writer.Filename != logFile {
		t.Errorf("Filename = %s, want %s", writer.Filename, logFile)
	}
	if writer.MaxSize != DefaultMaxSize {
		t.Errorf("MaxSize = %d, want %d", writer.MaxSize, DefaultMaxSize)
	}
	if writer.MaxBackups != DefaultMaxBackups {
		t.Errorf("MaxBackups = %d, want %d", writer.MaxBackups, DefaultMaxBackups)
	}
	if writer.MaxAge != DefaultMaxAge {
		t.Errorf("MaxAge = %d, want %d", writer.MaxAge, DefaultMaxAge)
	}
	if writer.Compress != DefaultCompress {
		t.Errorf("Compress = %v, want %v", writer.Compress, DefaultCompress)
	}
}

func TestDefaultRotationConfig(t *testing.T) {
	cfg := DefaultRotationConfig()

	if cfg.MaxSize != DefaultMaxSize {
		t.Errorf("MaxSize = %d, want %d", cfg.MaxSize, DefaultMaxSize)
	}
	if cfg.MaxBackups != DefaultMaxBackups {
		t.Errorf("MaxBackups = %d, want %d", cfg.MaxBackups, DefaultMaxBackups)
	}
	if cfg.MaxAge != DefaultMaxAge {
		t.Errorf("MaxAge = %d, want %d", cfg.MaxAge, DefaultMaxAge)
	}
	if cfg.Compress != DefaultCompress {
		t.Errorf("Compress = %v, want %v", cfg.Compress, DefaultCompress)
	}
}
