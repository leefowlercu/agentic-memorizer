package logging

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	slogmulti "github.com/samber/slog-multi"
)

// Manager handles logger lifecycle including bootstrap-to-full mode transitions.
// Components should obtain a logger via Logger() and use it for all logging.
type Manager struct {
	handler *SwappableHandler
	logger  *slog.Logger
	logFile *os.File
	level   *slog.LevelVar
	mu      sync.Mutex
}

// NewManager creates a logging manager in bootstrap mode.
// Bootstrap mode writes only to stderr using text format.
// Call Upgrade() after config is available to enable file logging.
func NewManager() *Manager {
	level := new(slog.LevelVar)
	level.Set(slog.LevelInfo)

	// Bootstrap mode: text to stderr only
	opts := &slog.HandlerOptions{Level: level}
	bootstrap := slog.NewTextHandler(os.Stderr, opts)

	handler := NewSwappableHandler(bootstrap)
	logger := slog.New(handler)

	return &Manager{
		handler: handler,
		logger:  logger,
		level:   level,
	}
}

// Logger returns the current logger instance.
// The returned logger is stable across Upgrade calls.
func (m *Manager) Logger() *slog.Logger {
	return m.logger
}

// Upgrade transitions from bootstrap mode (stderr-only) to full mode
// (stderr text + file JSON). Call after config subsystem is initialized.
// Returns error if log file cannot be opened/created.
func (m *Manager) Upgrade(logFilePath string, level slog.Level) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create parent directories if needed
	dir := filepath.Dir(logFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory %q; %w", dir, err)
	}

	// Open log file for append
	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file %q; %w", logFilePath, err)
	}

	// Close previous file if any
	if m.logFile != nil {
		_ = m.logFile.Close()
	}
	m.logFile = file

	// Set the new level
	m.level.Set(level)

	opts := &slog.HandlerOptions{Level: m.level}

	// Full mode: text to stderr + JSON to file
	fullHandler := slogmulti.Fanout(
		slog.NewTextHandler(os.Stderr, opts),
		slog.NewJSONHandler(file, opts),
	)

	// Atomic swap - all future log calls use the new handler
	m.handler.Swap(fullHandler)

	return nil
}

// SetLevel changes the log level at runtime.
// Applies immediately to all future log calls.
func (m *Manager) SetLevel(level slog.Level) {
	m.level.Set(level)
}

// Close cleanly shuts down the logger, closing any open file handles.
// Should be called during application shutdown.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.logFile != nil {
		err := m.logFile.Close()
		m.logFile = nil
		return err
	}
	return nil
}
