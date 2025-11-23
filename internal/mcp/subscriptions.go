package mcp

import (
	"sync"
)

// SubscriptionManager manages resource subscriptions for MCP clients
type SubscriptionManager struct {
	mu            sync.RWMutex
	subscriptions map[string]bool // URI -> subscribed
}

// NewSubscriptionManager creates a new subscription manager
func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		subscriptions: make(map[string]bool),
	}
}

// Subscribe adds a subscription for the given URI
func (sm *SubscriptionManager) Subscribe(uri string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.subscriptions[uri] = true
}

// Unsubscribe removes a subscription for the given URI
func (sm *SubscriptionManager) Unsubscribe(uri string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.subscriptions, uri)
}

// IsSubscribed checks if a URI is subscribed
func (sm *SubscriptionManager) IsSubscribed(uri string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.subscriptions[uri]
}

// GetSubscriptions returns a list of all subscribed URIs
func (sm *SubscriptionManager) GetSubscriptions() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	uris := make([]string, 0, len(sm.subscriptions))
	for uri := range sm.subscriptions {
		uris = append(uris, uri)
	}
	return uris
}

// Clear removes all subscriptions
func (sm *SubscriptionManager) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.subscriptions = make(map[string]bool)
}

// Count returns the number of active subscriptions
func (sm *SubscriptionManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.subscriptions)
}
