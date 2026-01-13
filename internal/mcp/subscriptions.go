package mcp

import (
	"sync"
	"time"
)

// Subscriber represents a client subscribed to resource updates.
type Subscriber struct {
	ID           string
	SessionID    string
	URI          string
	SubscribedAt time.Time
}

// SubscriptionManager manages resource subscriptions.
type SubscriptionManager struct {
	mu            sync.RWMutex
	subscriptions map[string]map[string]*Subscriber // uri -> subscriberID -> subscriber
}

// NewSubscriptionManager creates a new subscription manager.
func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		subscriptions: make(map[string]map[string]*Subscriber),
	}
}

// Subscribe adds a subscriber for a resource URI.
func (m *SubscriptionManager) Subscribe(uri string, subscriber *Subscriber) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.subscriptions[uri] == nil {
		m.subscriptions[uri] = make(map[string]*Subscriber)
	}

	subscriber.URI = uri
	subscriber.SubscribedAt = time.Now()
	m.subscriptions[uri][subscriber.ID] = subscriber
}

// Unsubscribe removes a subscriber from a resource URI.
func (m *SubscriptionManager) Unsubscribe(uri string, subscriberID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if subs, ok := m.subscriptions[uri]; ok {
		if _, exists := subs[subscriberID]; exists {
			delete(subs, subscriberID)
			if len(subs) == 0 {
				delete(m.subscriptions, uri)
			}
			return true
		}
	}
	return false
}

// UnsubscribeAll removes all subscriptions for a subscriber ID across all URIs.
func (m *SubscriptionManager) UnsubscribeAll(subscriberID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for uri, subs := range m.subscriptions {
		if _, exists := subs[subscriberID]; exists {
			delete(subs, subscriberID)
			count++
			if len(subs) == 0 {
				delete(m.subscriptions, uri)
			}
		}
	}
	return count
}

// GetSubscribers returns all subscribers for a resource URI.
func (m *SubscriptionManager) GetSubscribers(uri string) []*Subscriber {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subs, ok := m.subscriptions[uri]
	if !ok {
		return nil
	}

	result := make([]*Subscriber, 0, len(subs))
	for _, sub := range subs {
		result = append(result, sub)
	}
	return result
}

// GetAllSubscribers returns all subscribers across all URIs.
func (m *SubscriptionManager) GetAllSubscribers() []*Subscriber {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Subscriber
	for _, subs := range m.subscriptions {
		for _, sub := range subs {
			result = append(result, sub)
		}
	}
	return result
}

// SubscriberCount returns the number of subscribers for a URI.
func (m *SubscriptionManager) SubscriberCount(uri string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if subs, ok := m.subscriptions[uri]; ok {
		return len(subs)
	}
	return 0
}

// TotalSubscriptions returns the total number of subscriptions.
func (m *SubscriptionManager) TotalSubscriptions() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, subs := range m.subscriptions {
		count += len(subs)
	}
	return count
}

// HasSubscribers checks if any subscribers exist for a URI.
func (m *SubscriptionManager) HasSubscribers(uri string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subs, ok := m.subscriptions[uri]
	return ok && len(subs) > 0
}

// SubscribedURIs returns all URIs that have subscribers.
func (m *SubscriptionManager) SubscribedURIs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	uris := make([]string, 0, len(m.subscriptions))
	for uri := range m.subscriptions {
		uris = append(uris, uri)
	}
	return uris
}
