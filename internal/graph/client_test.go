package graph

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestDefaultClientConfig(t *testing.T) {
	config := DefaultClientConfig()

	if config.Host != "localhost" {
		t.Errorf("expected Host 'localhost', got %q", config.Host)
	}
	if config.Port != 6379 {
		t.Errorf("expected Port 6379, got %d", config.Port)
	}
	if config.Database != "memorizer" {
		t.Errorf("expected Database 'memorizer', got %q", config.Database)
	}
	if config.Password != "" {
		t.Errorf("expected empty Password, got %q", config.Password)
	}
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name   string
		config ClientConfig
		logger *slog.Logger
	}{
		{
			name:   "with default config and nil logger",
			config: DefaultClientConfig(),
			logger: nil,
		},
		{
			name:   "with default config and custom logger",
			config: DefaultClientConfig(),
			logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		},
		{
			name: "with custom config",
			config: ClientConfig{
				Host:     "custom-host",
				Port:     16379,
				Database: "test-db",
				Password: "secret",
			},
			logger: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.config, tt.logger)

			if client == nil {
				t.Fatal("expected non-nil client")
			}

			if client.config.Host != tt.config.Host {
				t.Errorf("expected Host %q, got %q", tt.config.Host, client.config.Host)
			}
			if client.config.Port != tt.config.Port {
				t.Errorf("expected Port %d, got %d", tt.config.Port, client.config.Port)
			}
			if client.config.Database != tt.config.Database {
				t.Errorf("expected Database %q, got %q", tt.config.Database, client.config.Database)
			}
			if client.config.Password != tt.config.Password {
				t.Errorf("expected Password %q, got %q", tt.config.Password, client.config.Password)
			}
			if client.logger == nil {
				t.Error("expected non-nil logger even when nil is passed")
			}
		})
	}
}

func TestClient_IsConnected_NotConnected(t *testing.T) {
	client := NewClient(DefaultClientConfig(), nil)

	if client.IsConnected() {
		t.Error("expected IsConnected to return false for new client")
	}
}

func TestClient_IsConnected_AfterClose(t *testing.T) {
	client := NewClient(DefaultClientConfig(), nil)

	// Close without connecting should work
	err := client.Close()
	if err != nil {
		t.Errorf("expected no error closing unconnected client, got: %v", err)
	}

	if client.IsConnected() {
		t.Error("expected IsConnected to return false after close")
	}
}

func TestClient_Close_MultipleCallsIdempotent(t *testing.T) {
	client := NewClient(DefaultClientConfig(), nil)

	// Close multiple times should not error
	for i := 0; i < 3; i++ {
		err := client.Close()
		if err != nil {
			t.Errorf("close call %d: expected no error, got: %v", i+1, err)
		}
	}
}

func TestClient_Ping_NotConnected(t *testing.T) {
	client := NewClient(DefaultClientConfig(), nil)
	ctx := context.Background()

	err := client.Ping(ctx)
	if err == nil {
		t.Error("expected error when pinging unconnected client")
	}
}

func TestClient_Health_NotConnected(t *testing.T) {
	client := NewClient(DefaultClientConfig(), nil)
	ctx := context.Background()

	status, err := client.Health(ctx)
	if err != nil {
		t.Errorf("expected no error for health check, got: %v", err)
	}

	if status.Connected {
		t.Error("expected Connected to be false")
	}
	if status.Error == "" {
		t.Error("expected Error message to be set")
	}
}

func TestClient_GetGraphStats_NotConnected(t *testing.T) {
	client := NewClient(DefaultClientConfig(), nil)
	ctx := context.Background()

	_, err := client.GetGraphStats(ctx)
	if err == nil {
		t.Error("expected error when getting stats from unconnected client")
	}
}

func TestClient_Query_NotConnected(t *testing.T) {
	client := NewClient(DefaultClientConfig(), nil)
	ctx := context.Background()

	_, err := client.Query(ctx, "RETURN 1", nil)
	if err == nil {
		t.Error("expected error when querying unconnected client")
	}
}

func TestClient_Graph_NotConnected(t *testing.T) {
	client := NewClient(DefaultClientConfig(), nil)

	graph := client.Graph()
	if graph != nil {
		t.Error("expected nil graph for unconnected client")
	}
}

func TestHealthStatus_Fields(t *testing.T) {
	status := &HealthStatus{
		Connected: true,
		Database:  "test-db",
		Error:     "",
		Timestamp: time.Now(),
		Stats: &GraphStats{
			NodeCount:         100,
			RelationshipCount: 50,
			LabelCount:        5,
		},
	}

	if !status.Connected {
		t.Error("expected Connected to be true")
	}
	if status.Database != "test-db" {
		t.Errorf("expected Database 'test-db', got %q", status.Database)
	}
	if status.Stats == nil {
		t.Error("expected Stats to be non-nil")
	}
	if status.Stats.NodeCount != 100 {
		t.Errorf("expected NodeCount 100, got %d", status.Stats.NodeCount)
	}
}

// Integration tests - require running FalkorDB
func TestClient_Connect_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewClient(ClientConfig{
		Host:     "localhost",
		Port:     6379,
		Database: "test_memorizer",
	}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		t.Skipf("Skipping test (requires FalkorDB); %v", err)
	}
	defer client.Close()

	if !client.IsConnected() {
		t.Error("expected IsConnected to return true after successful connect")
	}

	// Test ping
	if err := client.Ping(ctx); err != nil {
		t.Errorf("Ping failed; %v", err)
	}

	// Test health
	status, err := client.Health(ctx)
	if err != nil {
		t.Errorf("Health check failed; %v", err)
	}
	if !status.Connected {
		t.Error("expected Connected to be true in health status")
	}

	// Test query
	result, err := client.Query(ctx, "RETURN 1 as num", nil)
	if err != nil {
		t.Errorf("Query failed; %v", err)
	}
	if result.Empty() {
		t.Error("expected non-empty result")
	}
	if result.Next() {
		record := result.Record()
		val := record.GetInt64(0, -1)
		if val != 1 {
			t.Errorf("expected 1, got %d", val)
		}
	}
}
