package watcher

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/skip"
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
	cfg := &skip.Config{
		SkipHidden: true,
		SkipDirs:   []string{".cache", ".git"},
		SkipFiles:  []string{"test-skip"},
	}
	w, err := New(tmpDir, cfg, 200, logger)
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
	cfg := &skip.Config{SkipHidden: true}
	w, err := New(tmpDir, cfg, 200, logger)
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
	cfg := &skip.Config{SkipHidden: true}
	w, err := New(tmpDir, cfg, 200, logger)
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
	cfg := &skip.Config{SkipHidden: true}
	w, err := New(tmpDir, cfg, 200, logger)
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
	cfg := &skip.Config{SkipHidden: true}
	w, err := New(tmpDir, cfg, 200, logger)
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
	cfg := &skip.Config{SkipHidden: true}
	w, err := New(tmpDir, cfg, 200, logger)
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

func TestWatcher_DebounceUpdate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "watcher-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create watcher with initial short debounce (100ms)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg := &skip.Config{SkipHidden: true}
	w, err := New(tmpDir, cfg, 100, logger)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	if err := w.Start(); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}
	defer w.Stop()

	time.Sleep(100 * time.Millisecond)

	t.Run("update to longer interval", func(t *testing.T) {
		// Update to longer debounce interval (500ms)
		w.UpdateDebounceInterval(500)

		// Give the update time to apply
		time.Sleep(50 * time.Millisecond)

		testFile := filepath.Join(tmpDir, "test1.txt")

		// Create rapid file changes
		for i := 0; i < 5; i++ {
			if err := os.WriteFile(testFile, []byte(fmt.Sprintf("content-%d", i)), 0644); err != nil {
				t.Fatalf("failed to write file: %v", err)
			}
			time.Sleep(50 * time.Millisecond)
		}

		// With 500ms debounce, all changes should batch into one event
		// Wait slightly longer than debounce interval
		time.Sleep(600 * time.Millisecond)

		// Should receive exactly one batched event
		eventCount := 0
		timeout := time.After(100 * time.Millisecond)

	loop1:
		for {
			select {
			case <-w.Events():
				eventCount++
			case <-timeout:
				break loop1
			}
		}

		if eventCount == 0 {
			t.Error("expected at least one event")
		}
		if eventCount > 2 {
			// Allow up to 2 events due to timing uncertainty
			t.Errorf("expected events to be batched, got %d events", eventCount)
		}
	})

	t.Run("update to shorter interval", func(t *testing.T) {
		// Update to very short debounce (50ms)
		w.UpdateDebounceInterval(50)

		// Give the update time to apply
		time.Sleep(100 * time.Millisecond)

		testFile := filepath.Join(tmpDir, "test2.txt")

		// Create file changes with gaps longer than debounce
		for i := 0; i < 3; i++ {
			if err := os.WriteFile(testFile, []byte(fmt.Sprintf("content-%d", i)), 0644); err != nil {
				t.Fatalf("failed to write file: %v", err)
			}
			time.Sleep(100 * time.Millisecond) // Wait longer than 50ms debounce
		}

		// With 50ms debounce and 100ms waits, should get multiple events
		// Wait for all events to arrive
		time.Sleep(200 * time.Millisecond)

		eventCount := 0
		timeout := time.After(100 * time.Millisecond)

	loop2:
		for {
			select {
			case <-w.Events():
				eventCount++
			case <-timeout:
				break loop2
			}
		}

		if eventCount < 2 {
			// Should get at least 2 separate events with short debounce
			t.Errorf("expected multiple events with short debounce, got %d", eventCount)
		}
	})

	t.Run("channel full handling", func(t *testing.T) {
		// The UpdateDebounceInterval method should be non-blocking
		// even if called many times rapidly

		for i := 0; i < 10; i++ {
			w.UpdateDebounceInterval(100 + i*10)
		}

		// Should not hang or panic
		time.Sleep(50 * time.Millisecond)
	})
}

func TestWatcher_EventPriority(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "watcher-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg := &skip.Config{SkipHidden: true}
	w, err := New(tmpDir, cfg, 500, logger)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	if err := w.Start(); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}
	defer w.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create file with WriteFile which can trigger both CREATE and WRITE events
	testFile := filepath.Join(tmpDir, "priority-test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Wait for debounce (500ms debounce interval)
	time.Sleep(700 * time.Millisecond)

	// Should receive CREATE event, not MODIFY, even if both events occurred
	select {
	case event := <-w.Events():
		if event.Type != EventCreate {
			t.Errorf("expected Create event (priority over Modify), got %v", event.Type)
		}
		if event.Path != testFile {
			t.Errorf("expected path %s, got %s", testFile, event.Path)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for create event")
	}
}

func TestWatcher_AlwaysSkipDirs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "watcher-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create always-skip directories
	for _, dir := range []string{".git", ".cache", ".forgotten"} {
		dirPath := filepath.Join(tmpDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Even with SkipHidden=false, always-skip dirs should not be watched
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg := &skip.Config{SkipHidden: false}
	w, err := New(tmpDir, cfg, 200, logger)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	if err := w.Start(); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}
	defer w.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create files in always-skip directories
	for _, dir := range []string{".git", ".cache", ".forgotten"} {
		testFile := filepath.Join(tmpDir, dir, "test.txt")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create file in %s: %v", dir, err)
		}
	}

	// Wait for debounce
	time.Sleep(300 * time.Millisecond)

	// Should not receive any events
	select {
	case event := <-w.Events():
		t.Errorf("unexpected event for file in always-skip dir: %v", event)
	case <-time.After(500 * time.Millisecond):
		// Expected - no events from always-skip directories
	}
}

func TestWatcher_SkipExtensions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "watcher-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg := &skip.Config{
		SkipHidden:     true,
		SkipExtensions: []string{".tmp", ".bak"},
	}
	w, err := New(tmpDir, cfg, 200, logger)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	if err := w.Start(); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}
	defer w.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create files with skipped extensions (should be skipped)
	tmpFile := filepath.Join(tmpDir, "file.tmp")
	if err := os.WriteFile(tmpFile, []byte("tmp"), 0644); err != nil {
		t.Fatalf("failed to create tmp file: %v", err)
	}

	bakFile := filepath.Join(tmpDir, "file.bak")
	if err := os.WriteFile(bakFile, []byte("bak"), 0644); err != nil {
		t.Fatalf("failed to create bak file: %v", err)
	}

	// Wait for debounce
	time.Sleep(300 * time.Millisecond)

	// Should not receive events for skipped extensions
	select {
	case event := <-w.Events():
		t.Errorf("unexpected event for skipped extension file: %v", event)
	case <-time.After(500 * time.Millisecond):
		// Expected - no events
	}

	// Create a non-skipped file (should trigger event)
	txtFile := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(txtFile, []byte("txt"), 0644); err != nil {
		t.Fatalf("failed to create txt file: %v", err)
	}

	// Wait for debounce
	time.Sleep(300 * time.Millisecond)

	// Should receive event for non-skipped file
	select {
	case event := <-w.Events():
		if event.Type != EventCreate {
			t.Errorf("expected Create event, got %v", event.Type)
		}
		if event.Path != txtFile {
			t.Errorf("expected path %s, got %s", txtFile, event.Path)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for create event")
	}
}
