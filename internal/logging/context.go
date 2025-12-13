package logging

import (
	"context"
	"log/slog"
)

type contextKey string

const (
	loggerKey    contextKey = "logger"
	processIDKey contextKey = "process_id"
	sessionIDKey contextKey = "session_id"
)

// WithLogger stores logger in context
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext retrieves logger from context, returns fallback if not found
func FromContext(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return logger
	}
	return fallback
}

// WithProcessIDContext stores process_id in context
func WithProcessIDContext(ctx context.Context, processID string) context.Context {
	return context.WithValue(ctx, processIDKey, processID)
}

// ProcessIDFromContext retrieves process_id from context
func ProcessIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(processIDKey).(string)
	return id, ok
}
