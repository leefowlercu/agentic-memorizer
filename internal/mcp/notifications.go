package mcp

import (
	"context"
	"log/slog"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/events"
)

// startEventListener starts the event listener goroutine that subscribes to
// events from the event bus and sends MCP notifications to subscribed clients.
func (s *Server) startEventListener(ctx context.Context) {
	if s.bus == nil {
		slog.Debug("MCP event listener not started; no event bus")
		return
	}

	s.stopChan = make(chan struct{})

	// Subscribe to AnalysisComplete events for file notifications
	unsubAnalysis := s.bus.Subscribe(events.AnalysisComplete, s.handleAnalysisCompleteEvent)

	// Subscribe to RebuildComplete events for index notifications
	unsubRebuild := s.bus.Subscribe(events.RebuildComplete, s.handleRebuildCompleteEvent)

	// Combine unsubscribe functions
	s.unsubscribe = func() {
		unsubAnalysis()
		unsubRebuild()
	}

	slog.Info("MCP event listener started")
}

// stopEventListener stops the event listener and unsubscribes from the event bus.
func (s *Server) stopEventListener() {
	if s.stopChan != nil {
		close(s.stopChan)
		s.stopChan = nil
	}

	if s.unsubscribe != nil {
		s.unsubscribe()
		s.unsubscribe = nil
	}

	slog.Debug("MCP event listener stopped")
}

// handleAnalysisCompleteEvent handles AnalysisComplete events and sends
// file resource update notifications to subscribed clients.
func (s *Server) handleAnalysisCompleteEvent(event events.Event) {
	analysisEvent, ok := event.Payload.(*events.AnalysisEvent)
	if !ok {
		slog.Warn("MCP received invalid AnalysisComplete event payload",
			"type", event.Type,
		)
		return
	}

	path := analysisEvent.Path
	if path == "" {
		return
	}

	// Construct the file resource URI
	fileURI := ResourceURIFilePrefix + path

	// Check if any clients are subscribed to this specific file
	if s.subs.HasSubscribers(fileURI) {
		slog.Debug("MCP sending file update notification",
			"uri", fileURI,
			"path", path,
		)
		s.NotifyResourceChanged(fileURI)
	}

	// Also notify subscribers of index resources since the index has changed
	s.notifyIndexSubscribers()
}

// handleRebuildCompleteEvent handles RebuildComplete events and sends
// index resource update notifications to subscribed clients.
func (s *Server) handleRebuildCompleteEvent(event events.Event) {
	rebuildEvent, ok := event.Payload.(*events.RebuildCompleteEvent)
	if !ok {
		slog.Warn("MCP received invalid RebuildComplete event payload",
			"type", event.Type,
		)
		return
	}

	slog.Debug("MCP received rebuild complete event",
		"files_queued", rebuildEvent.FilesQueued,
		"dirs_processed", rebuildEvent.DirsProcessed,
		"duration", rebuildEvent.Duration,
		"full", rebuildEvent.Full,
	)

	// Notify all index subscribers
	s.notifyIndexSubscribers()
}

// notifyIndexSubscribers sends update notifications to all index resource subscribers.
func (s *Server) notifyIndexSubscribers() {
	indexURIs := []string{
		ResourceURIIndex,
		ResourceURIIndexXML,
		ResourceURIIndexJSON,
		ResourceURIIndexTOON,
	}

	for _, uri := range indexURIs {
		if s.subs.HasSubscribers(uri) {
			slog.Debug("MCP sending index update notification", "uri", uri)
			s.NotifyResourceChanged(uri)
		}
	}
}

// NotifyRebuildComplete notifies all index subscribers that a rebuild has completed.
// This is called by the orchestrator after a rebuild operation finishes.
func (s *Server) NotifyRebuildComplete() {
	slog.Debug("MCP notifying rebuild complete")
	s.notifyIndexSubscribers()
}

// NotifyFileChanged notifies subscribers that a specific file resource has changed.
// The path should be the absolute file path.
func (s *Server) NotifyFileChanged(path string) {
	if path == "" {
		return
	}

	fileURI := ResourceURIFilePrefix + path
	if s.subs.HasSubscribers(fileURI) {
		slog.Debug("MCP sending file change notification",
			"uri", fileURI,
			"path", path,
		)
		s.NotifyResourceChanged(fileURI)
	}
}

// GetSubscribedURIsForPath returns all subscribed URIs that match a given file path.
// This includes exact file URI matches and any index URIs.
func (s *Server) GetSubscribedURIsForPath(path string) []string {
	var uris []string

	fileURI := ResourceURIFilePrefix + path
	if s.subs.HasSubscribers(fileURI) {
		uris = append(uris, fileURI)
	}

	// Index resources are always potentially relevant
	indexURIs := []string{
		ResourceURIIndex,
		ResourceURIIndexXML,
		ResourceURIIndexJSON,
		ResourceURIIndexTOON,
	}

	for _, uri := range indexURIs {
		if s.subs.HasSubscribers(uri) {
			uris = append(uris, uri)
		}
	}

	return uris
}

// isIndexURI returns true if the URI is an index resource URI.
func isIndexURI(uri string) bool {
	return uri == ResourceURIIndex ||
		uri == ResourceURIIndexXML ||
		uri == ResourceURIIndexJSON ||
		uri == ResourceURIIndexTOON
}

// isFileURI returns true if the URI is a file resource URI.
func isFileURI(uri string) bool {
	return strings.HasPrefix(uri, ResourceURIFilePrefix)
}
