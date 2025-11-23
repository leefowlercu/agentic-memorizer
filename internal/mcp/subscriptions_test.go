package mcp

import (
	"sync"
	"testing"
)

// TestSubscriptionManager_Subscribe tests basic subscription functionality
func TestSubscriptionManager_Subscribe(t *testing.T) {
	sm := NewSubscriptionManager()

	uri := "memorizer://index"
	sm.Subscribe(uri)

	if !sm.IsSubscribed(uri) {
		t.Errorf("Expected %q to be subscribed", uri)
	}
}

// TestSubscriptionManager_Unsubscribe tests basic unsubscription functionality
func TestSubscriptionManager_Unsubscribe(t *testing.T) {
	sm := NewSubscriptionManager()

	uri := "memorizer://index"
	sm.Subscribe(uri)

	if !sm.IsSubscribed(uri) {
		t.Fatalf("Expected %q to be subscribed before unsubscribe", uri)
	}

	sm.Unsubscribe(uri)

	if sm.IsSubscribed(uri) {
		t.Errorf("Expected %q to be unsubscribed", uri)
	}
}

// TestSubscriptionManager_MultipleURIs tests managing multiple subscriptions
func TestSubscriptionManager_MultipleURIs(t *testing.T) {
	sm := NewSubscriptionManager()

	uris := []string{
		"memorizer://index",
		"memorizer://index/markdown",
		"memorizer://index/json",
	}

	for _, uri := range uris {
		sm.Subscribe(uri)
	}

	subscriptions := sm.GetSubscriptions()
	if len(subscriptions) != len(uris) {
		t.Errorf("Expected %d subscriptions, got %d", len(uris), len(subscriptions))
	}

	for _, uri := range uris {
		if !sm.IsSubscribed(uri) {
			t.Errorf("Expected %q to be subscribed", uri)
		}
	}
}

// TestSubscriptionManager_DuplicateSubscribe tests subscribing to same URI multiple times
func TestSubscriptionManager_DuplicateSubscribe(t *testing.T) {
	sm := NewSubscriptionManager()

	uri := "memorizer://index"
	sm.Subscribe(uri)
	sm.Subscribe(uri)
	sm.Subscribe(uri)

	subscriptions := sm.GetSubscriptions()
	if len(subscriptions) != 1 {
		t.Errorf("Expected 1 subscription, got %d", len(subscriptions))
	}

	if !sm.IsSubscribed(uri) {
		t.Errorf("Expected %q to be subscribed", uri)
	}
}

// TestSubscriptionManager_UnsubscribeNonexistent tests unsubscribing from non-subscribed URI
func TestSubscriptionManager_UnsubscribeNonexistent(t *testing.T) {
	sm := NewSubscriptionManager()

	uri := "memorizer://index"
	sm.Unsubscribe(uri) // Should not panic

	if sm.IsSubscribed(uri) {
		t.Errorf("Expected %q to not be subscribed", uri)
	}
}

// TestSubscriptionManager_Clear tests clearing all subscriptions
func TestSubscriptionManager_Clear(t *testing.T) {
	sm := NewSubscriptionManager()

	uris := []string{
		"memorizer://index",
		"memorizer://index/markdown",
		"memorizer://index/json",
	}

	for _, uri := range uris {
		sm.Subscribe(uri)
	}

	if sm.Count() != len(uris) {
		t.Fatalf("Expected %d subscriptions before clear, got %d", len(uris), sm.Count())
	}

	sm.Clear()

	if sm.Count() != 0 {
		t.Errorf("Expected 0 subscriptions after clear, got %d", sm.Count())
	}

	for _, uri := range uris {
		if sm.IsSubscribed(uri) {
			t.Errorf("Expected %q to not be subscribed after clear", uri)
		}
	}
}

// TestSubscriptionManager_Count tests subscription counting
func TestSubscriptionManager_Count(t *testing.T) {
	sm := NewSubscriptionManager()

	if sm.Count() != 0 {
		t.Errorf("Expected 0 subscriptions initially, got %d", sm.Count())
	}

	sm.Subscribe("memorizer://index")
	if sm.Count() != 1 {
		t.Errorf("Expected 1 subscription, got %d", sm.Count())
	}

	sm.Subscribe("memorizer://index/markdown")
	if sm.Count() != 2 {
		t.Errorf("Expected 2 subscriptions, got %d", sm.Count())
	}

	sm.Unsubscribe("memorizer://index")
	if sm.Count() != 1 {
		t.Errorf("Expected 1 subscription after unsubscribe, got %d", sm.Count())
	}
}

// TestSubscriptionManager_GetSubscriptions tests retrieving all subscriptions
func TestSubscriptionManager_GetSubscriptions(t *testing.T) {
	sm := NewSubscriptionManager()

	expectedURIs := map[string]bool{
		"memorizer://index":          true,
		"memorizer://index/markdown": true,
		"memorizer://index/json":     true,
	}

	for uri := range expectedURIs {
		sm.Subscribe(uri)
	}

	subscriptions := sm.GetSubscriptions()

	if len(subscriptions) != len(expectedURIs) {
		t.Fatalf("Expected %d subscriptions, got %d", len(expectedURIs), len(subscriptions))
	}

	for _, uri := range subscriptions {
		if !expectedURIs[uri] {
			t.Errorf("Unexpected subscription: %q", uri)
		}
	}
}

// TestSubscriptionManager_ThreadSafety tests concurrent access
func TestSubscriptionManager_ThreadSafety(t *testing.T) {
	sm := NewSubscriptionManager()

	const numGoroutines = 10
	const opsPerGoroutine = 100

	uris := []string{
		"memorizer://index",
		"memorizer://index/markdown",
		"memorizer://index/json",
	}

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Run concurrent operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < opsPerGoroutine; j++ {
				uri := uris[j%len(uris)]

				// Mix of operations
				switch j % 4 {
				case 0:
					sm.Subscribe(uri)
				case 1:
					sm.IsSubscribed(uri)
				case 2:
					sm.GetSubscriptions()
				case 3:
					sm.Unsubscribe(uri)
				}
			}
		}(i)
	}

	wg.Wait()

	// Should not have crashed due to race conditions
	count := sm.Count()
	t.Logf("Final subscription count after concurrent operations: %d", count)
}

// TestSubscriptionManager_ConcurrentSubscribeUnsubscribe tests subscribe/unsubscribe races
func TestSubscriptionManager_ConcurrentSubscribeUnsubscribe(t *testing.T) {
	sm := NewSubscriptionManager()

	const numGoroutines = 50
	uri := "memorizer://index"

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2)

	// Half subscribe, half unsubscribe
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			sm.Subscribe(uri)
		}()

		go func() {
			defer wg.Done()
			sm.Unsubscribe(uri)
		}()
	}

	wg.Wait()

	// Should not crash - final state is undefined but should be consistent
	isSubscribed := sm.IsSubscribed(uri)
	count := sm.Count()

	if isSubscribed && count != 1 {
		t.Errorf("Inconsistent state: IsSubscribed=%v but Count=%d", isSubscribed, count)
	}
	if !isSubscribed && count != 0 {
		t.Errorf("Inconsistent state: IsSubscribed=%v but Count=%d", isSubscribed, count)
	}
}
