package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestSwappableHandler_Enabled(t *testing.T) {
	var buf bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	textHandler := slog.NewTextHandler(&buf, opts)
	sh := NewSwappableHandler(textHandler)

	ctx := context.Background()

	// Debug should not be enabled at Info level
	if sh.Enabled(ctx, slog.LevelDebug) {
		t.Error("Enabled(Debug) = true, want false at Info level")
	}

	// Info should be enabled
	if !sh.Enabled(ctx, slog.LevelInfo) {
		t.Error("Enabled(Info) = false, want true at Info level")
	}

	// Error should be enabled
	if !sh.Enabled(ctx, slog.LevelError) {
		t.Error("Enabled(Error) = false, want true at Info level")
	}
}

func TestSwappableHandler_Handle(t *testing.T) {
	var buf bytes.Buffer
	textHandler := slog.NewTextHandler(&buf, nil)
	sh := NewSwappableHandler(textHandler)

	logger := slog.New(sh)
	logger.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("Handle did not write message, got: %s", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("Handle did not write attributes, got: %s", output)
	}
}

func TestSwappableHandler_Swap(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	handler1 := slog.NewTextHandler(&buf1, nil)
	handler2 := slog.NewTextHandler(&buf2, nil)

	sh := NewSwappableHandler(handler1)
	logger := slog.New(sh)

	// Log to first handler
	logger.Info("message 1")

	// Swap handlers
	sh.Swap(handler2)

	// Log to second handler
	logger.Info("message 2")

	// Verify message routing
	if !strings.Contains(buf1.String(), "message 1") {
		t.Error("message 1 not in buf1")
	}
	if strings.Contains(buf1.String(), "message 2") {
		t.Error("message 2 should not be in buf1")
	}
	if !strings.Contains(buf2.String(), "message 2") {
		t.Error("message 2 not in buf2")
	}
	if strings.Contains(buf2.String(), "message 1") {
		t.Error("message 1 should not be in buf2")
	}
}

func TestSwappableHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	textHandler := slog.NewTextHandler(&buf, nil)
	sh := NewSwappableHandler(textHandler)

	// Create handler with attributes
	shWithAttrs := sh.WithAttrs([]slog.Attr{slog.String("component", "test")})

	// Verify it returns a SwappableHandler
	_, ok := shWithAttrs.(*SwappableHandler)
	if !ok {
		t.Error("WithAttrs should return *SwappableHandler")
	}

	// Verify attributes are included
	logger := slog.New(shWithAttrs)
	logger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "component=test") {
		t.Errorf("WithAttrs did not include attribute, got: %s", output)
	}
}

func TestSwappableHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, nil)
	sh := NewSwappableHandler(jsonHandler)

	// Create handler with group
	shWithGroup := sh.WithGroup("mygroup")

	// Verify it returns a SwappableHandler
	_, ok := shWithGroup.(*SwappableHandler)
	if !ok {
		t.Error("WithGroup should return *SwappableHandler")
	}

	// Verify group is applied
	logger := slog.New(shWithGroup)
	logger.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "mygroup") {
		t.Errorf("WithGroup did not apply group, got: %s", output)
	}
}
