// Package events provides an in-process pub/sub event bus for cross-component
// communication within the memorizer daemon.
package events

import (
	"time"
)

// EventType identifies the type of event being published.
type EventType string

const (
	// FileDiscovered is published when a new file is found during a walk.
	FileDiscovered EventType = "file.discovered"

	// FileChanged is published when an existing file is modified.
	FileChanged EventType = "file.changed"

	// PathDeleted is published when a file or directory is removed.
	PathDeleted EventType = "path.deleted"

	// AnalysisComplete is published when analysis finishes for a file.
	AnalysisComplete EventType = "analysis.complete"

	// AnalysisFailed is published when analysis fails for a file.
	AnalysisFailed EventType = "analysis.failed"

	// SemanticAnalysisFailed is published when semantic analysis fails for a file.
	SemanticAnalysisFailed EventType = "analysis.semantic_failed"

	// EmbeddingsGenerationFailed is published when embeddings generation fails for a file.
	EmbeddingsGenerationFailed EventType = "analysis.embeddings_failed"

	// GraphPersistenceFailed is published when writing analysis results to the graph fails.
	GraphPersistenceFailed EventType = "graph.persistence_failed"

	// GraphFatal signals a graph fatal error (e.g., connection loss).
	GraphFatal EventType = "graph.fatal"

	// ConfigReloaded is published when configuration is successfully reloaded.
	ConfigReloaded EventType = "config.reloaded"

	// ConfigReloadFailed is published when configuration reload fails.
	ConfigReloadFailed EventType = "config.reload_failed"

	// RememberedPathRemoved is published when a remembered path is removed.
	RememberedPathRemoved EventType = "remembered_path.removed"

	// RememberedPathAdded is published when a new path is remembered.
	RememberedPathAdded EventType = "remembered_path.added"

	// RememberedPathUpdated is published when a remembered path is updated.
	RememberedPathUpdated EventType = "remembered_path.updated"

	// RebuildComplete is published when a rebuild operation finishes.
	RebuildComplete EventType = "rebuild.complete"

	// JobStarted is published when a job starts.
	JobStarted EventType = "job.started"

	// JobCompleted is published when a job completes.
	JobCompleted EventType = "job.completed"

	// JobFailed is published when a job fails.
	JobFailed EventType = "job.failed"
)

// Event represents a published event in the system.
type Event struct {
	// Type identifies the event type.
	Type EventType

	// Timestamp is when the event was created.
	Timestamp time.Time

	// Payload contains event-specific data.
	Payload any
}

// NewEvent creates a new event with the given type and payload.
func NewEvent(eventType EventType, payload any) Event {
	return Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Payload:   payload,
	}
}

// EventHandler is a function that processes events.
type EventHandler func(event Event)
