package events

import (
	"fmt"
	"reflect"
)

var payloadTypes = map[EventType]reflect.Type{
	FileDiscovered:             reflect.TypeOf(&FileEvent{}),
	FileChanged:                reflect.TypeOf(&FileEvent{}),
	PathDeleted:                reflect.TypeOf(&FileEvent{}),
	AnalysisComplete:           reflect.TypeOf(&AnalysisEvent{}),
	AnalysisFailed:             reflect.TypeOf(&AnalysisEvent{}),
	SemanticAnalysisFailed:     reflect.TypeOf(&AnalysisEvent{}),
	EmbeddingsGenerationFailed: reflect.TypeOf(&AnalysisEvent{}),
	GraphPersistenceFailed:     reflect.TypeOf(&GraphEvent{}),
	GraphFatal:                 reflect.TypeOf(&GraphFatalEvent{}),
	ConfigReloaded:             reflect.TypeOf(&ConfigReloadEvent{}),
	ConfigReloadFailed:         reflect.TypeOf(&ConfigReloadEvent{}),
	RememberedPathAdded:        reflect.TypeOf(&RememberedPathEvent{}),
	RememberedPathUpdated:      reflect.TypeOf(&RememberedPathEvent{}),
	RememberedPathRemoved:      reflect.TypeOf(&RememberedPathRemovedEvent{}),
	RebuildComplete:            reflect.TypeOf(&RebuildCompleteEvent{}),
	JobStarted:                 reflect.TypeOf(&JobStartedEvent{}),
	JobCompleted:               reflect.TypeOf(&JobCompletedEvent{}),
	JobFailed:                  reflect.TypeOf(&JobFailedEvent{}),
}

// PayloadType returns the expected payload type for an event type.
func PayloadType(eventType EventType) (reflect.Type, bool) {
	t, ok := payloadTypes[eventType]
	return t, ok
}

// ValidatePayload verifies that an event payload matches the expected type.
func ValidatePayload(event Event) error {
	if event.Payload == nil {
		return nil
	}

	expected, ok := payloadTypes[event.Type]
	if !ok {
		return fmt.Errorf("no payload mapping for event type %q", event.Type)
	}

	if reflect.TypeOf(event.Payload) != expected {
		return fmt.Errorf("event %q payload type mismatch: got %T, expected %s", event.Type, event.Payload, expected)
	}

	return nil
}
