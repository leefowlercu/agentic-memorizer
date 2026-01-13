package mcp

import (
	"testing"
)

func TestNewSubscriptionManager(t *testing.T) {
	m := NewSubscriptionManager()

	if m == nil {
		t.Fatal("NewSubscriptionManager returned nil")
	}
	if m.subscriptions == nil {
		t.Error("subscriptions map is nil")
	}
}

func TestSubscriptionManager_Subscribe(t *testing.T) {
	m := NewSubscriptionManager()

	sub := &Subscriber{
		ID:        "sub1",
		SessionID: "session1",
	}

	m.Subscribe(ResourceURIIndex, sub)

	if m.SubscriberCount(ResourceURIIndex) != 1 {
		t.Errorf("SubscriberCount = %d, want 1", m.SubscriberCount(ResourceURIIndex))
	}
	if sub.URI != ResourceURIIndex {
		t.Errorf("URI = %q, want %q", sub.URI, ResourceURIIndex)
	}
	if sub.SubscribedAt.IsZero() {
		t.Error("SubscribedAt should be set")
	}
}

func TestSubscriptionManager_Unsubscribe(t *testing.T) {
	m := NewSubscriptionManager()

	sub := &Subscriber{ID: "sub1", SessionID: "session1"}
	m.Subscribe(ResourceURIIndex, sub)

	// Unsubscribe existing
	ok := m.Unsubscribe(ResourceURIIndex, "sub1")
	if !ok {
		t.Error("Unsubscribe should return true for existing subscriber")
	}
	if m.SubscriberCount(ResourceURIIndex) != 0 {
		t.Errorf("SubscriberCount = %d, want 0", m.SubscriberCount(ResourceURIIndex))
	}

	// Unsubscribe non-existing
	ok = m.Unsubscribe(ResourceURIIndex, "sub1")
	if ok {
		t.Error("Unsubscribe should return false for non-existing subscriber")
	}
}

func TestSubscriptionManager_UnsubscribeAll(t *testing.T) {
	m := NewSubscriptionManager()

	sub := &Subscriber{ID: "sub1", SessionID: "session1"}
	m.Subscribe(ResourceURIIndex, sub)
	m.Subscribe(ResourceURIIndexJSON, sub)
	m.Subscribe(ResourceURIIndexXML, sub)

	count := m.UnsubscribeAll("sub1")
	if count != 3 {
		t.Errorf("UnsubscribeAll returned %d, want 3", count)
	}
	if m.TotalSubscriptions() != 0 {
		t.Errorf("TotalSubscriptions = %d, want 0", m.TotalSubscriptions())
	}
}

func TestSubscriptionManager_GetSubscribers(t *testing.T) {
	m := NewSubscriptionManager()

	sub1 := &Subscriber{ID: "sub1", SessionID: "session1"}
	sub2 := &Subscriber{ID: "sub2", SessionID: "session2"}

	m.Subscribe(ResourceURIIndex, sub1)
	m.Subscribe(ResourceURIIndex, sub2)

	subs := m.GetSubscribers(ResourceURIIndex)
	if len(subs) != 2 {
		t.Errorf("GetSubscribers returned %d subscribers, want 2", len(subs))
	}

	// Non-existing URI
	subs = m.GetSubscribers("memorizer://nonexistent")
	if subs != nil {
		t.Error("GetSubscribers should return nil for non-existing URI")
	}
}

func TestSubscriptionManager_GetAllSubscribers(t *testing.T) {
	m := NewSubscriptionManager()

	sub1 := &Subscriber{ID: "sub1", SessionID: "session1"}
	sub2 := &Subscriber{ID: "sub2", SessionID: "session2"}

	m.Subscribe(ResourceURIIndex, sub1)
	m.Subscribe(ResourceURIIndexJSON, sub2)

	subs := m.GetAllSubscribers()
	if len(subs) != 2 {
		t.Errorf("GetAllSubscribers returned %d subscribers, want 2", len(subs))
	}
}

func TestSubscriptionManager_HasSubscribers(t *testing.T) {
	m := NewSubscriptionManager()

	if m.HasSubscribers(ResourceURIIndex) {
		t.Error("HasSubscribers should return false for empty URI")
	}

	sub := &Subscriber{ID: "sub1", SessionID: "session1"}
	m.Subscribe(ResourceURIIndex, sub)

	if !m.HasSubscribers(ResourceURIIndex) {
		t.Error("HasSubscribers should return true after subscription")
	}
}

func TestSubscriptionManager_SubscribedURIs(t *testing.T) {
	m := NewSubscriptionManager()

	sub := &Subscriber{ID: "sub1", SessionID: "session1"}
	m.Subscribe(ResourceURIIndex, sub)
	m.Subscribe(ResourceURIIndexJSON, sub)

	uris := m.SubscribedURIs()
	if len(uris) != 2 {
		t.Errorf("SubscribedURIs returned %d URIs, want 2", len(uris))
	}
}

func TestSubscriptionManager_TotalSubscriptions(t *testing.T) {
	m := NewSubscriptionManager()

	if m.TotalSubscriptions() != 0 {
		t.Error("TotalSubscriptions should be 0 initially")
	}

	sub1 := &Subscriber{ID: "sub1", SessionID: "session1"}
	sub2 := &Subscriber{ID: "sub2", SessionID: "session2"}

	m.Subscribe(ResourceURIIndex, sub1)
	m.Subscribe(ResourceURIIndex, sub2)
	m.Subscribe(ResourceURIIndexJSON, sub1)

	if m.TotalSubscriptions() != 3 {
		t.Errorf("TotalSubscriptions = %d, want 3", m.TotalSubscriptions())
	}
}
