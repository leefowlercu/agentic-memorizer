package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/events"
)

func TestReload_PublishesConfigReloadedEvent(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(configPath, []byte("daemon:\n  http_port: 8080\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	bus := events.NewBus()
	SetEventBus(bus)
	t.Cleanup(func() {
		SetEventBus(nil)
		_ = bus.Close()
		Reset()
	})

	received := make(chan events.Event, 1)
	unsubscribe := bus.Subscribe(events.ConfigReloaded, func(event events.Event) {
		received <- event
	})
	t.Cleanup(unsubscribe)

	if err := os.WriteFile(configPath, []byte("daemon:\n  http_port: 9999\n"), 0o644); err != nil {
		t.Fatalf("failed to update config file: %v", err)
	}

	if err := Reload(); err != nil {
		t.Fatalf("Reload() returned error: %v", err)
	}

	select {
	case event := <-received:
		if event.Type != events.ConfigReloaded {
			t.Fatalf("expected event type %s, got %s", events.ConfigReloaded, event.Type)
		}
		payload, ok := event.Payload.(events.ConfigReloadEvent)
		if !ok {
			t.Fatalf("expected payload type ConfigReloadEvent, got %T", event.Payload)
		}
		if !containsString(payload.ChangedSections, "daemon") {
			t.Errorf("expected changed sections to include daemon, got %v", payload.ChangedSections)
		}
		if payload.ReloadableChanges {
			t.Error("expected reloadable changes to be false for daemon changes")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected config reload event to be published")
	}
}

func TestReload_PublishesConfigReloadFailedEvent(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(configPath, []byte("daemon:\n  http_port: 8080\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("MEMORIZER_CONFIG_DIR", tmpDir)
	Reset()

	if err := Init(); err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}

	bus := events.NewBus()
	SetEventBus(bus)
	t.Cleanup(func() {
		SetEventBus(nil)
		_ = bus.Close()
		Reset()
	})

	received := make(chan events.Event, 1)
	unsubscribe := bus.Subscribe(events.ConfigReloadFailed, func(event events.Event) {
		received <- event
	})
	t.Cleanup(unsubscribe)

	if err := os.WriteFile(configPath, []byte("daemon:\n  http_port: [invalid yaml"), 0o644); err != nil {
		t.Fatalf("failed to corrupt config file: %v", err)
	}

	if err := Reload(); err == nil {
		t.Fatal("Reload() should return error for invalid YAML")
	}

	select {
	case event := <-received:
		if event.Type != events.ConfigReloadFailed {
			t.Fatalf("expected event type %s, got %s", events.ConfigReloadFailed, event.Type)
		}
		payload, ok := event.Payload.(events.ConfigReloadEvent)
		if !ok {
			t.Fatalf("expected payload type ConfigReloadEvent, got %T", event.Payload)
		}
		if payload.Error == "" {
			t.Error("expected error message in reload failed event")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected config reload failed event to be published")
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
