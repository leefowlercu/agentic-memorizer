package logging

import (
	"log/slog"
	"strings"
)

// DefaultLevel is the log level used when not configured.
const DefaultLevel = slog.LevelInfo

// ParseLevel converts a string log level to slog.Level.
// Supported values: "debug", "info", "warn", "error" (case-insensitive).
// Returns (DefaultLevel, false) if the string is not recognized.
func ParseLevel(s string) (level slog.Level, ok bool) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, true
	case "info":
		return slog.LevelInfo, true
	case "warn":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	default:
		return DefaultLevel, false
	}
}

// ParseLevelOrDefault converts a string log level to slog.Level.
// Returns DefaultLevel if the string is not recognized.
func ParseLevelOrDefault(s string) slog.Level {
	level, _ := ParseLevel(s)
	return level
}
