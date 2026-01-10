package logging

import (
	"log/slog"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLevel slog.Level
		wantOK    bool
	}{
		// Valid levels - lowercase
		{"debug lowercase", "debug", slog.LevelDebug, true},
		{"info lowercase", "info", slog.LevelInfo, true},
		{"warn lowercase", "warn", slog.LevelWarn, true},
		{"error lowercase", "error", slog.LevelError, true},

		// Valid levels - uppercase
		{"DEBUG uppercase", "DEBUG", slog.LevelDebug, true},
		{"INFO uppercase", "INFO", slog.LevelInfo, true},
		{"WARN uppercase", "WARN", slog.LevelWarn, true},
		{"ERROR uppercase", "ERROR", slog.LevelError, true},

		// Valid levels - mixed case
		{"Debug mixed", "Debug", slog.LevelDebug, true},
		{"Info mixed", "Info", slog.LevelInfo, true},
		{"Warn mixed", "Warn", slog.LevelWarn, true},
		{"Error mixed", "Error", slog.LevelError, true},

		// Invalid levels
		{"empty string", "", slog.LevelInfo, false},
		{"unknown level", "trace", slog.LevelInfo, false},
		{"typo", "infoo", slog.LevelInfo, false},
		{"warning full word", "warning", slog.LevelInfo, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLevel, gotOK := ParseLevel(tt.input)
			if gotOK != tt.wantOK {
				t.Errorf("ParseLevel(%q) ok = %v, want %v", tt.input, gotOK, tt.wantOK)
			}
			if gotOK && gotLevel != tt.wantLevel {
				t.Errorf("ParseLevel(%q) level = %v, want %v", tt.input, gotLevel, tt.wantLevel)
			}
		})
	}
}
