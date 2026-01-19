package config

import (
	"context"
	"log/slog"
	"reflect"
	"sync"

	"github.com/leefowlercu/agentic-memorizer/internal/events"
)

// eventBusMu protects eventBus
var eventBusMu sync.RWMutex

// eventBus is the event bus instance for publishing config events.
// Set via SetEventBus().
var eventBus events.Bus

// SetEventBus sets the event bus instance for publishing config reload events.
// Must be called before config reload events will be published.
func SetEventBus(bus events.Bus) {
	eventBusMu.Lock()
	defer eventBusMu.Unlock()
	eventBus = bus
}

// ReloadableSections lists the config sections that can be hot-reloaded.
// Changes to other sections require a daemon restart.
var ReloadableSections = []string{"log_level", "log_file", "semantic", "embeddings"}

// detectChangedSections compares old and new configs and returns a list of changed sections.
func detectChangedSections(old, new *Config) []string {
	var changed []string

	if old.LogLevel != new.LogLevel {
		changed = append(changed, "log_level")
	}
	if old.LogFile != new.LogFile {
		changed = append(changed, "log_file")
	}
	if !reflect.DeepEqual(old.Daemon, new.Daemon) {
		changed = append(changed, "daemon")
	}
	if !reflect.DeepEqual(old.Graph, new.Graph) {
		changed = append(changed, "graph")
	}
	if !reflect.DeepEqual(old.Semantic, new.Semantic) {
		changed = append(changed, "semantic")
	}
	if !reflect.DeepEqual(old.Embeddings, new.Embeddings) {
		changed = append(changed, "embeddings")
	}

	return changed
}

// isReloadable checks if all changed sections are hot-reloadable.
func isReloadable(changedSections []string) bool {
	reloadableSet := make(map[string]bool)
	for _, s := range ReloadableSections {
		reloadableSet[s] = true
	}

	for _, section := range changedSections {
		if !reloadableSet[section] {
			return false
		}
	}

	return true
}

// publishConfigReloaded publishes a config.reloaded event.
func publishConfigReloaded(old, new *Config) {
	eventBusMu.RLock()
	bus := eventBus
	eventBusMu.RUnlock()

	if bus == nil {
		return
	}

	changedSections := detectChangedSections(old, new)
	reloadable := isReloadable(changedSections)

	if !reloadable {
		slog.Warn("config reload includes non-reloadable sections; some changes require daemon restart",
			"changed_sections", changedSections)
	}

	event := events.NewConfigReloaded(changedSections, reloadable)
	if err := bus.Publish(context.Background(), event); err != nil {
		slog.Error("failed to publish config reload event", "error", err)
	}
}

// publishConfigReloadFailed publishes a config.reload_failed event.
func publishConfigReloadFailed(err error) {
	eventBusMu.RLock()
	bus := eventBus
	eventBusMu.RUnlock()

	if bus == nil {
		return
	}

	event := events.NewConfigReloadFailed(err)
	if pubErr := bus.Publish(context.Background(), event); pubErr != nil {
		slog.Error("failed to publish config reload failed event", "error", pubErr)
	}
}
