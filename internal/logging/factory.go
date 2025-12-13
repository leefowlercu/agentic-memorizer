package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// HandlerType specifies the slog handler type
type HandlerType int

const (
	// HandlerJSON creates JSON formatted logs
	HandlerJSON HandlerType = iota
	// HandlerText creates human-readable text logs
	HandlerText
)

// LoggerConfig holds configuration for logger creation
type LoggerConfig struct {
	LogFile  string        // Path to log file (empty = stdout/stderr)
	LogLevel string        // "debug", "info", "warn", "error"
	Handler  HandlerType   // JSON or Text
	Outputs  []io.Writer   // Additional writers (for multi-writer)
}

// LoggerOption is a functional option for configuring logger creation
type LoggerOption func(*LoggerConfig)

// WithLogFile sets the log file path
func WithLogFile(path string) LoggerOption {
	return func(c *LoggerConfig) {
		c.LogFile = path
	}
}

// WithLogLevel sets the log level
func WithLogLevel(level string) LoggerOption {
	return func(c *LoggerConfig) {
		c.LogLevel = level
	}
}

// WithHandler sets the handler type
func WithHandler(handler HandlerType) LoggerOption {
	return func(c *LoggerConfig) {
		c.Handler = handler
	}
}

// WithAdditionalOutputs adds additional writers (e.g., stderr for MCP)
func WithAdditionalOutputs(writers ...io.Writer) LoggerOption {
	return func(c *LoggerConfig) {
		c.Outputs = append(c.Outputs, writers...)
	}
}

// NewLogger creates a logger with optional rotating file output
// Returns (*slog.Logger, *lumberjack.Logger, error)
// The lumberjack.Logger is returned for hot-reload scenarios (can be nil)
func NewLogger(opts ...LoggerOption) (*slog.Logger, *lumberjack.Logger, error) {
	cfg := &LoggerConfig{
		LogLevel: "info",
		Handler:  HandlerText, // Default to Text for human-readable logs
	}

	for _, opt := range opts {
		opt(cfg)
	}

	// Parse log level
	level, err := ParseLogLevel(cfg.LogLevel)
	if err != nil {
		return nil, nil, err
	}

	// Create handler based on configuration
	var handler slog.Handler
	var logWriter *lumberjack.Logger

	if cfg.LogFile != "" {
		// Create log directory
		if err := CreateLogDir(cfg.LogFile); err != nil {
			return nil, nil, err
		}

		// Create rotating log writer
		logWriter = NewRotatingWriter(cfg.LogFile)

		// Determine output writer(s)
		var writer io.Writer = logWriter
		if len(cfg.Outputs) > 0 {
			writers := append([]io.Writer{logWriter}, cfg.Outputs...)
			writer = io.MultiWriter(writers...)
		}

		// Create handler
		handler = createHandler(cfg.Handler, writer, level)
	} else {
		// No log file - use stdout/stderr
		var output io.Writer = os.Stdout
		if len(cfg.Outputs) > 0 {
			output = cfg.Outputs[0] // Use first output as default
		}
		handler = createHandler(cfg.Handler, output, level)
	}

	return slog.New(handler), logWriter, nil
}

// createHandler creates appropriate handler based on type
func createHandler(handlerType HandlerType, w io.Writer, level slog.Level) slog.Handler {
	opts := &slog.HandlerOptions{Level: level}

	switch handlerType {
	case HandlerJSON:
		return slog.NewJSONHandler(w, opts)
	case HandlerText:
		return slog.NewTextHandler(w, opts)
	default:
		return slog.NewTextHandler(w, opts)
	}
}

// ParseLogLevel converts string to slog.Level (case-insensitive)
func ParseLogLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level: %s (must be debug, info, warn, error)", level)
	}
}

// CreateLogDir creates the directory for a log file path
func CreateLogDir(logFilePath string) error {
	logDir := filepath.Dir(logFilePath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory %s; %w", logDir, err)
	}
	return nil
}
