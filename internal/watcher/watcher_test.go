package watcher

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcher_CreateFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "watcher-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create watcher
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	w, err := New(tmpDir, []string{".cache", ".git"}, []string{"test-skip"}, 200, logger)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	if err := w.Start(); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}
	defer w.Stop()

	// Wait for watcher to initialize
	time.Sleep(100 * time.Millisecond)

	// Create a file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Wait for debounce
	time.Sleep(300 * time.Millisecond)

	// Check for event
	select {
	case event := <-w.Events():
		if event.Type != EventCreate {
			t.Errorf("expected Create event, got %v", event.Type)
		}
		if event.Path != testFile {
			t.Errorf("expected path %s, got %s", testFile, event.Path)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for create event")
	}
}

func TestWatcher_ModifyFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "watcher-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create initial file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create watcher
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	w, err := New(tmpDir, []string{}, []string{}, 200, logger)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	if err := w.Start(); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}
	defer w.Stop()

	time.Sleep(100 * time.Millisecond)

	// Modify the file
	if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
		t.Fatalf("failed to modify test file: %v", err)
	}

	// Wait for debounce
	time.Sleep(300 * time.Millisecond)

	// Check for event
	select {
	case event := <-w.Events():
		if event.Type != EventModify {
			t.Errorf("expected Modify event, got %v", event.Type)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for modify event")
	}
}

func TestWatcher_DeleteFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "watcher-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create initial file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create watcher
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	w, err := New(tmpDir, []string{}, []string{}, 200, logger)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	if err := w.Start(); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}
	defer w.Stop()

	time.Sleep(100 * time.Millisecond)

	// Delete the file
	if err := os.Remove(testFile); err != nil {
		t.Fatalf("failed to delete test file: %v", err)
	}

	// Wait for debounce
	time.Sleep(300 * time.Millisecond)

	// Check for event
	select {
	case event := <-w.Events():
		if event.Type != EventDelete {
			t.Errorf("expected Delete event, got %v", event.Type)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for delete event")
	}
}

func TestWatcher_SkipHiddenFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "watcher-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	w, err := New(tmpDir, []string{}, []string{}, 200, logger)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	if err := w.Start(); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}
	defer w.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create hidden file (should be skipped)
	hiddenFile := filepath.Join(tmpDir, ".hidden")
	if err := os.WriteFile(hiddenFile, []byte("hidden"), 0644); err != nil {
		t.Fatalf("failed to create hidden file: %v", err)
	}

	// Wait for debounce
	time.Sleep(300 * time.Millisecond)

	// Should not receive any events
	select {
	case event := <-w.Events():
		t.Errorf("unexpected event for hidden file: %v", event)
	case <-time.After(500 * time.Millisecond):
		// Expected - no event
	}
}

func TestWatcher_Debouncing(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "watcher-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	w, err := New(tmpDir, []string{}, []string{}, 200, logger)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	if err := w.Start(); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}
	defer w.Stop()

	time.Sleep(100 * time.Millisecond)

	// Rapidly modify file multiple times
	testFile := filepath.Join(tmpDir, "test.txt")
	for i := 0; i < 10; i++ {
		if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for debounce
	time.Sleep(300 * time.Millisecond)

	// Should receive only one batched event
	eventCount := 0
	timeout := time.After(500 * time.Millisecond)

loop:
	for {
		select {
		case <-w.Events():
			eventCount++
		case <-timeout:
			break loop
		}
	}

	// Should be 1 or 2 events max (due to batching)
	if eventCount > 2 {
		t.Errorf("expected 1-2 batched events, got %d", eventCount)
	}
}

func TestWatcher_NewDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "watcher-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	w, err := New(tmpDir, []string{}, []string{}, 200, logger)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	if err := w.Start(); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}
	defer w.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create new directory
	newDir := filepath.Join(tmpDir, "newdir")
	if err := os.Mkdir(newDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Wait for watcher to add new directory
	time.Sleep(200 * time.Millisecond)

	// Create file in new directory
	testFile := filepath.Join(newDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create file in new directory: %v", err)
	}

	// Wait for debounce
	time.Sleep(300 * time.Millisecond)

	// Should receive events for both directory and file
	eventCount := 0
	timeout := time.After(1 * time.Second)

loop:
	for eventCount < 2 {
		select {
		case <-w.Events():
			eventCount++
		case <-timeout:
			break loop
		}
	}

	if eventCount < 1 {
		t.Error("expected at least one event for new directory or file")
	}
}
