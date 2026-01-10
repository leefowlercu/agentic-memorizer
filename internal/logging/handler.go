package logging

import (
	"context"
	"log/slog"
	"sync/atomic"
)

// SwappableHandler wraps a slog.Handler that can be atomically replaced at runtime.
// This enables the bootstrap-to-full mode transition without breaking existing logger references.
type SwappableHandler struct {
	handler atomic.Pointer[slog.Handler]
}

// NewSwappableHandler creates a handler with an initial handler.
func NewSwappableHandler(initial slog.Handler) *SwappableHandler {
	sh := &SwappableHandler{}
	sh.handler.Store(&initial)
	return sh
}

// Swap atomically replaces the underlying handler.
// This is thread-safe and can be called while logging is in progress.
func (sh *SwappableHandler) Swap(newHandler slog.Handler) {
	sh.handler.Store(&newHandler)
}

// current returns the current underlying handler.
func (sh *SwappableHandler) current() slog.Handler {
	return *sh.handler.Load()
}

// Enabled reports whether the handler handles records at the given level.
func (sh *SwappableHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return sh.current().Enabled(ctx, level)
}

// Handle handles the Record.
func (sh *SwappableHandler) Handle(ctx context.Context, r slog.Record) error {
	return sh.current().Handle(ctx, r)
}

// WithAttrs returns a new SwappableHandler whose underlying handler has the given attributes.
func (sh *SwappableHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandler := sh.current().WithAttrs(attrs)
	return NewSwappableHandler(newHandler)
}

// WithGroup returns a new SwappableHandler whose underlying handler has the given group.
func (sh *SwappableHandler) WithGroup(name string) slog.Handler {
	newHandler := sh.current().WithGroup(name)
	return NewSwappableHandler(newHandler)
}
