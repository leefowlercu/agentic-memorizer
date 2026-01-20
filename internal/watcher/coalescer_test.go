package watcher

import (
	"testing"
	"time"
)

func TestCoalescer_SingleEvent(t *testing.T) {
	c := NewCoalescer(50*time.Millisecond, 100*time.Millisecond)
	defer c.Stop()

	c.Add(CoalescedEvent{
		Path:      "/test/file.go",
		Type:      EventModify,
		Timestamp: time.Now(),
	})

	select {
	case event := <-c.Events():
		if event.Path != "/test/file.go" {
			t.Errorf("expected path /test/file.go, got %s", event.Path)
		}
		if event.Type != EventModify {
			t.Errorf("expected type EventModify, got %v", event.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}

func TestCoalescer_MultipleModifiesCoalesced(t *testing.T) {
	c := NewCoalescer(100*time.Millisecond, 200*time.Millisecond)
	defer c.Stop()

	// Rapid modifications should be coalesced
	for i := 0; i < 5; i++ {
		c.Add(CoalescedEvent{
			Path:      "/test/file.go",
			Type:      EventModify,
			Timestamp: time.Now(),
		})
		time.Sleep(10 * time.Millisecond)
	}

	// Should only receive one event
	select {
	case event := <-c.Events():
		if event.Type != EventModify {
			t.Errorf("expected type EventModify, got %v", event.Type)
		}
	case <-time.After(300 * time.Millisecond):
		t.Error("timeout waiting for event")
	}

	// Verify no more events
	select {
	case event := <-c.Events():
		t.Errorf("unexpected event: %+v", event)
	case <-time.After(150 * time.Millisecond):
		// Expected - no more events
	}
}

func TestCoalescer_CreateThenModify(t *testing.T) {
	c := NewCoalescer(50*time.Millisecond, 100*time.Millisecond)
	defer c.Stop()

	c.Add(CoalescedEvent{
		Path:      "/test/file.go",
		Type:      EventCreate,
		Timestamp: time.Now(),
	})

	time.Sleep(10 * time.Millisecond)

	c.Add(CoalescedEvent{
		Path:      "/test/file.go",
		Type:      EventModify,
		Timestamp: time.Now(),
	})

	// Should receive Create (Create + Modify = Create)
	select {
	case event := <-c.Events():
		if event.Type != EventCreate {
			t.Errorf("expected type EventCreate, got %v", event.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}

func TestCoalescer_DeleteThenCreate(t *testing.T) {
	c := NewCoalescer(50*time.Millisecond, 100*time.Millisecond)
	defer c.Stop()

	c.Add(CoalescedEvent{
		Path:      "/test/file.go",
		Type:      EventDelete,
		Timestamp: time.Now(),
	})

	time.Sleep(10 * time.Millisecond)

	c.Add(CoalescedEvent{
		Path:      "/test/file.go",
		Type:      EventCreate,
		Timestamp: time.Now(),
	})

	// Should receive Modify (Delete + Create = Modify, i.e., file replaced)
	select {
	case event := <-c.Events():
		if event.Type != EventModify {
			t.Errorf("expected type EventModify (file replaced), got %v", event.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}

func TestCoalescer_DeleteGracePeriod(t *testing.T) {
	// Delete events should wait longer than modify events
	debounce := 50 * time.Millisecond
	gracePeriod := 150 * time.Millisecond

	c := NewCoalescer(debounce, gracePeriod)
	defer c.Stop()

	start := time.Now()

	c.Add(CoalescedEvent{
		Path:      "/test/file.go",
		Type:      EventDelete,
		Timestamp: time.Now(),
	})

	select {
	case <-c.Events():
		elapsed := time.Since(start)
		if elapsed < gracePeriod-10*time.Millisecond {
			t.Errorf("delete event emitted too early: %v < %v", elapsed, gracePeriod)
		}
	case <-time.After(300 * time.Millisecond):
		t.Error("timeout waiting for delete event")
	}
}

func TestCoalescer_DifferentPaths(t *testing.T) {
	c := NewCoalescer(50*time.Millisecond, 100*time.Millisecond)
	defer c.Stop()

	c.Add(CoalescedEvent{
		Path:      "/test/file1.go",
		Type:      EventModify,
		Timestamp: time.Now(),
	})

	c.Add(CoalescedEvent{
		Path:      "/test/file2.go",
		Type:      EventModify,
		Timestamp: time.Now(),
	})

	received := make(map[string]bool)

	for i := 0; i < 2; i++ {
		select {
		case event := <-c.Events():
			received[event.Path] = true
		case <-time.After(200 * time.Millisecond):
			t.Error("timeout waiting for event")
		}
	}

	if !received["/test/file1.go"] {
		t.Error("missing event for file1.go")
	}
	if !received["/test/file2.go"] {
		t.Error("missing event for file2.go")
	}
}

func TestCoalescer_Stop(t *testing.T) {
	c := NewCoalescer(50*time.Millisecond, 100*time.Millisecond)

	c.Add(CoalescedEvent{
		Path:      "/test/file.go",
		Type:      EventModify,
		Timestamp: time.Now(),
	})

	c.Stop()

	// Adding after stop should not panic
	c.Add(CoalescedEvent{
		Path:      "/test/file2.go",
		Type:      EventModify,
		Timestamp: time.Now(),
	})

	// Events channel should be closed - may receive pending events which is fine
	select {
	case <-c.Events():
	case <-time.After(100 * time.Millisecond):
	}
}

func TestCoalescer_PendingCount(t *testing.T) {
	c := NewCoalescer(100*time.Millisecond, 200*time.Millisecond)
	defer c.Stop()

	if c.PendingCount() != 0 {
		t.Error("expected 0 pending events initially")
	}

	c.Add(CoalescedEvent{
		Path:      "/test/file1.go",
		Type:      EventModify,
		Timestamp: time.Now(),
	})

	c.Add(CoalescedEvent{
		Path:      "/test/file2.go",
		Type:      EventModify,
		Timestamp: time.Now(),
	})

	if c.PendingCount() != 2 {
		t.Errorf("expected 2 pending events, got %d", c.PendingCount())
	}
}

func TestCoalescer_ModifyThenDelete(t *testing.T) {
	c := NewCoalescer(50*time.Millisecond, 100*time.Millisecond)
	defer c.Stop()

	c.Add(CoalescedEvent{
		Path:      "/test/file.go",
		Type:      EventModify,
		Timestamp: time.Now(),
	})

	time.Sleep(10 * time.Millisecond)

	c.Add(CoalescedEvent{
		Path:      "/test/file.go",
		Type:      EventDelete,
		Timestamp: time.Now(),
	})

	// Should receive Delete (Modify + Delete = Delete)
	select {
	case event := <-c.Events():
		if event.Type != EventDelete {
			t.Errorf("expected type EventDelete, got %v", event.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}
