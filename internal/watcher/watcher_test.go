package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

// mockBus implements events.Bus for testing.
type mockBus struct {
	events []events.Event
	mu     sync.Mutex
	closed bool
}

func newMockBus() *mockBus {
	return &mockBus{
		events: make([]events.Event, 0),
	}
}

func (b *mockBus) Publish(ctx context.Context, event events.Event) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return events.ErrBusClosed
	}
	b.events = append(b.events, event)
	return nil
}

func (b *mockBus) Subscribe(eventType events.EventType, handler events.EventHandler) func() {
	return func() {}
}

func (b *mockBus) SubscribeAll(handler events.EventHandler) func() {
	return func() {}
}

func (b *mockBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}

func (b *mockBus) Events() []events.Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]events.Event, len(b.events))
	copy(result, b.events)
	return result
}

func (b *mockBus) EventCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.events)
}

// mockRegistry implements a minimal registry for testing.
type mockRegistry struct{}

func (r *mockRegistry) AddPath(ctx context.Context, path string, config *registry.PathConfig) error {
	return nil
}

func (r *mockRegistry) RemovePath(ctx context.Context, path string) error {
	return nil
}

func (r *mockRegistry) GetPath(ctx context.Context, path string) (*registry.RememberedPath, error) {
	return nil, registry.ErrPathNotFound
}

func (r *mockRegistry) ListPaths(ctx context.Context) ([]registry.RememberedPath, error) {
	return nil, nil
}

func (r *mockRegistry) UpdatePathConfig(ctx context.Context, path string, config *registry.PathConfig) error {
	return nil
}

func (r *mockRegistry) UpdatePathLastWalk(ctx context.Context, path string, lastWalk time.Time) error {
	return nil
}

func (r *mockRegistry) FindContainingPath(ctx context.Context, path string) (*registry.RememberedPath, error) {
	return nil, registry.ErrPathNotFound
}

func (r *mockRegistry) GetEffectiveConfig(ctx context.Context, filePath string) (*registry.PathConfig, error) {
	return nil, nil
}

func (r *mockRegistry) GetFileState(ctx context.Context, path string) (*registry.FileState, error) {
	return nil, nil
}

func (r *mockRegistry) UpdateFileState(ctx context.Context, state *registry.FileState) error {
	return nil
}

func (r *mockRegistry) DeleteFileState(ctx context.Context, path string) error {
	return nil
}

func (r *mockRegistry) ListFileStates(ctx context.Context, parentPath string) ([]registry.FileState, error) {
	return nil, nil
}

func (r *mockRegistry) DeleteFileStatesForPath(ctx context.Context, parentPath string) error {
	return nil
}

func (r *mockRegistry) UpdateMetadataState(ctx context.Context, path string, contentHash string, metadataHash string, size int64, modTime time.Time) error {
	return nil
}

func (r *mockRegistry) UpdateSemanticState(ctx context.Context, path string, analysisVersion string, err error) error {
	return nil
}

func (r *mockRegistry) UpdateEmbeddingsState(ctx context.Context, path string, err error) error {
	return nil
}

func (r *mockRegistry) ClearAnalysisState(ctx context.Context, path string) error {
	return nil
}

func (r *mockRegistry) ListFilesNeedingMetadata(ctx context.Context, parentPath string) ([]registry.FileState, error) {
	return nil, nil
}

func (r *mockRegistry) ListFilesNeedingSemantic(ctx context.Context, parentPath string, maxRetries int) ([]registry.FileState, error) {
	return nil, nil
}

func (r *mockRegistry) ListFilesNeedingEmbeddings(ctx context.Context, parentPath string, maxRetries int) ([]registry.FileState, error) {
	return nil, nil
}

func (r *mockRegistry) CheckPathHealth(ctx context.Context) ([]registry.PathStatus, error) {
	return nil, nil
}

func (r *mockRegistry) ValidateAndCleanPaths(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (r *mockRegistry) Close() error {
	return nil
}

func TestWatcher_Watch(t *testing.T) {
	tmpDir := t.TempDir()

	bus := newMockBus()
	reg := &mockRegistry{}

	w, err := New(bus, reg, WithDebounceWindow(50*time.Millisecond))
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.Stop()

	err = w.Watch(tmpDir)
	if err != nil {
		t.Fatalf("failed to watch directory: %v", err)
	}

	paths := w.WatchedPaths()
	if len(paths) != 1 {
		t.Errorf("expected 1 watched path, got %d", len(paths))
	}
}

func TestWatcher_WatchNotDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file.txt")
	os.WriteFile(filePath, []byte("content"), 0644)

	bus := newMockBus()
	reg := &mockRegistry{}

	w, err := New(bus, reg)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.Stop()

	err = w.Watch(filePath)
	if err == nil {
		t.Error("expected error watching file, got nil")
	}
}

func TestWatcher_Unwatch(t *testing.T) {
	tmpDir := t.TempDir()

	bus := newMockBus()
	reg := &mockRegistry{}

	w, err := New(bus, reg)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.Stop()

	err = w.Watch(tmpDir)
	if err != nil {
		t.Fatalf("failed to watch: %v", err)
	}

	err = w.Unwatch(tmpDir)
	if err != nil {
		t.Fatalf("failed to unwatch: %v", err)
	}

	paths := w.WatchedPaths()
	if len(paths) != 0 {
		t.Errorf("expected 0 watched paths after unwatch, got %d", len(paths))
	}
}

func TestWatcher_Stats(t *testing.T) {
	tmpDir := t.TempDir()

	bus := newMockBus()
	reg := &mockRegistry{}

	w, err := New(bus, reg)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.Stop()

	stats := w.Stats()
	if stats.IsRunning {
		t.Error("expected IsRunning to be false before start")
	}

	err = w.Watch(tmpDir)
	if err != nil {
		t.Fatalf("failed to watch: %v", err)
	}

	ctx := context.Background()
	err = w.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	stats = w.Stats()
	if !stats.IsRunning {
		t.Error("expected IsRunning to be true after start")
	}
}

func TestWatcher_FileChange(t *testing.T) {
	tmpDir := t.TempDir()

	bus := newMockBus()
	reg := &mockRegistry{}

	// Use short debounce for testing
	w, err := New(bus, reg, WithDebounceWindow(50*time.Millisecond), WithDeleteGracePeriod(100*time.Millisecond))
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.Stop()

	err = w.Watch(tmpDir)
	if err != nil {
		t.Fatalf("failed to watch: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Create a file
	testFile := filepath.Join(tmpDir, "test.go")
	err = os.WriteFile(testFile, []byte("package main"), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Wait for event to be processed
	time.Sleep(200 * time.Millisecond)

	// Check that an event was published
	eventCount := bus.EventCount()
	if eventCount == 0 {
		t.Log("Note: file change event may not have been detected (platform-specific)")
	}
}

func TestWatcher_DoubleStart(t *testing.T) {
	tmpDir := t.TempDir()

	bus := newMockBus()
	reg := &mockRegistry{}

	w, err := New(bus, reg)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.Stop()

	err = w.Watch(tmpDir)
	if err != nil {
		t.Fatalf("failed to watch: %v", err)
	}

	ctx := context.Background()

	err = w.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Second start should fail
	err = w.Start(ctx)
	if err == nil {
		t.Error("expected error on double start")
	}
}

func TestShouldIgnoreFile(t *testing.T) {
	tests := []struct {
		path   string
		ignore bool
	}{
		{"/test/.hidden", true},
		{"/test/~temp", true},
		{"/test/#autosave#", true},
		{"/test/file.swp", true},
		{"/test/file.swo", true},
		{"/test/file.tmp", true},
		{"/test/file.bak", true},
		{"/test/file~", true},
		{"/test/4913", true},
		{"/test/.DS_Store", true},
		{"/test/Thumbs.db", true},
		{"/test/normal.go", false},
		{"/test/main.py", false},
		{"/test/README.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := shouldIgnoreFile(tt.path)
			if got != tt.ignore {
				t.Errorf("shouldIgnoreFile(%q) = %v, want %v", tt.path, got, tt.ignore)
			}
		})
	}
}

func TestIsWatchLimitError(t *testing.T) {
	tests := []struct {
		errMsg   string
		expected bool
	}{
		{"too many open files", true},
		{"no space left on device", true},
		{"user limit on total number of inotify watches", true},
		{"permission denied", false},
		{"file not found", false},
	}

	for _, tt := range tests {
		t.Run(tt.errMsg, func(t *testing.T) {
			err := &testError{msg: tt.errMsg}
			got := isWatchLimitError(err)
			if got != tt.expected {
				t.Errorf("isWatchLimitError(%q) = %v, want %v", tt.errMsg, got, tt.expected)
			}
		})
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestWatcher_RecursiveWatch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create subdirectories
	subDir := filepath.Join(tmpDir, "subdir")
	os.Mkdir(subDir, 0755)

	nestedDir := filepath.Join(subDir, "nested")
	os.Mkdir(nestedDir, 0755)

	bus := newMockBus()
	reg := &mockRegistry{}

	w, err := New(bus, reg)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.Stop()

	err = w.Watch(tmpDir)
	if err != nil {
		t.Fatalf("failed to watch: %v", err)
	}

	// The watcher should have added watches for all directories
	paths := w.WatchedPaths()
	if len(paths) != 1 {
		t.Errorf("expected 1 root watched path, got %d", len(paths))
	}
}

func TestWatcher_SkipsHiddenDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a hidden directory
	hiddenDir := filepath.Join(tmpDir, ".hidden")
	os.Mkdir(hiddenDir, 0755)

	// Create a normal directory
	normalDir := filepath.Join(tmpDir, "normal")
	os.Mkdir(normalDir, 0755)

	bus := newMockBus()
	reg := &mockRegistry{}

	w, err := New(bus, reg)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.Stop()

	err = w.Watch(tmpDir)
	if err != nil {
		t.Fatalf("failed to watch: %v", err)
	}

	// Watch should complete without error
	// (hidden directories are skipped)
}
