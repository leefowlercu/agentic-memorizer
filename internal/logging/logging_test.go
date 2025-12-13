package logging

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

func TestNewProcessID(t *testing.T) {
	id := NewProcessID()
	if !IsValidUUIDv7(id) {
		t.Errorf("NewProcessID() generated invalid UUIDv7: %s", id)
	}
}

func TestNewSessionID(t *testing.T) {
	id := NewSessionID()
	if !IsValidUUIDv7(id) {
		t.Errorf("NewSessionID() generated invalid UUIDv7: %s", id)
	}
}

func TestNewClientID(t *testing.T) {
	id := NewClientID()
	if !IsValidUUIDv7(id) {
		t.Errorf("NewClientID() generated invalid UUIDv7: %s", id)
	}
}

func TestIsValidUUIDv7(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		valid bool
	}{
		{
			name:  "valid UUIDv7",
			id:    NewProcessID(),
			valid: true,
		},
		{
			name:  "invalid UUID",
			id:    "not-a-uuid",
			valid: false,
		},
		{
			name:  "empty string",
			id:    "",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidUUIDv7(tt.id)
			if result != tt.valid {
				t.Errorf("IsValidUUIDv7(%q) = %v, want %v", tt.id, result, tt.valid)
			}
		})
	}
}

func TestWithProcessID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	processID := "test-process-id"

	enriched := WithProcessID(logger, processID)
	if enriched == nil {
		t.Error("WithProcessID() returned nil")
	}
}

func TestWithSessionID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	sessionID := "test-session-id"

	enriched := WithSessionID(logger, sessionID)
	if enriched == nil {
		t.Error("WithSessionID() returned nil")
	}
}

func TestWithComponent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	enriched := WithComponent(logger, ComponentMCPServer)
	if enriched == nil {
		t.Error("WithComponent() returned nil")
	}
}

func TestWithClientInfo(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	enriched := WithClientInfo(logger, "test-client", "1.0.0")
	if enriched == nil {
		t.Error("WithClientInfo() returned nil")
	}
}

func TestWithMCPProcess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	processID := NewProcessID()

	enriched := WithMCPProcess(logger, processID)
	if enriched == nil {
		t.Error("WithMCPProcess() returned nil")
	}
}

func TestWithSSEClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	enriched := WithSSEClient(logger)
	if enriched == nil {
		t.Error("WithSSEClient() returned nil")
	}
}

func TestWithDaemonSSE(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	clientID := NewClientID()
	clientType := "mcp"
	clientVersion := "0.13.0"

	enriched := WithDaemonSSE(logger, clientID, clientType, clientVersion)
	if enriched == nil {
		t.Error("WithDaemonSSE() returned nil")
	}
}

func TestContextLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	// Store logger in context
	ctx = WithLogger(ctx, logger)

	// Retrieve logger from context
	retrieved := FromContext(ctx, nil)
	if retrieved == nil {
		t.Error("FromContext() returned nil")
	}
}

func TestContextLoggerFallback(t *testing.T) {
	fallback := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	// Retrieve from empty context should return fallback
	retrieved := FromContext(ctx, fallback)
	if retrieved != fallback {
		t.Error("FromContext() did not return fallback for empty context")
	}
}

func TestContextProcessID(t *testing.T) {
	ctx := context.Background()
	processID := "test-process-id"

	// Store processID in context
	ctx = WithProcessIDContext(ctx, processID)

	// Retrieve processID from context
	retrieved, ok := ProcessIDFromContext(ctx)
	if !ok {
		t.Error("ProcessIDFromContext() returned false")
	}
	if retrieved != processID {
		t.Errorf("ProcessIDFromContext() = %q, want %q", retrieved, processID)
	}
}

func TestContextProcessIDEmpty(t *testing.T) {
	ctx := context.Background()

	// Retrieve from empty context
	_, ok := ProcessIDFromContext(ctx)
	if ok {
		t.Error("ProcessIDFromContext() returned true for empty context")
	}
}

func TestFieldConstants(t *testing.T) {
	// Test that field constants don't collide and use correct dot notation
	fields := map[string]string{
		"process.id":     FieldProcessID,
		"session.id":     FieldSessionID,
		"request.id":     FieldRequestID,
		"client.id":      FieldClientID,
		"client.type":    FieldClientType,
		"client.version": FieldClientVersion,
		"component":      FieldComponent,
		"client_name":    FieldClientName,
		"trace_id":       FieldTraceID,
	}

	seen := make(map[string]bool)
	for expectedKey, actualValue := range fields {
		if expectedKey != actualValue {
			t.Errorf("Field constant mismatch: expected key %q but got value %q", expectedKey, actualValue)
		}
		if seen[actualValue] {
			t.Errorf("Duplicate field constant: %q", actualValue)
		}
		seen[actualValue] = true
	}
}

func TestComponentConstants(t *testing.T) {
	// Test that component constants don't collide
	components := map[string]string{
		"mcp-server":    ComponentMCPServer,
		"sse-client":    ComponentSSEClient,
		"daemon-sse":    ComponentDaemonSSE,
		"graph-manager": ComponentGraphManager,
	}

	seen := make(map[string]bool)
	for expectedKey, actualValue := range components {
		if expectedKey != actualValue {
			t.Errorf("Component constant mismatch: expected key %q but got value %q", expectedKey, actualValue)
		}
		if seen[actualValue] {
			t.Errorf("Duplicate component constant: %q", actualValue)
		}
		seen[actualValue] = true
	}
}
