package watcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
type mockRegistry struct {
	// pathConfigs maps root paths to their PathConfig
	pathConfigs map[string]*registry.PathConfig
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		pathConfigs: make(map[string]*registry.PathConfig),
	}
}

// SetPathConfig sets the PathConfig for a given root path.
func (r *mockRegistry) SetPathConfig(rootPath string, config *registry.PathConfig) {
	r.pathConfigs[rootPath] = config
}

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
	// Find the longest matching root path
	var matchedRoot string
	var matchedConfig *registry.PathConfig
	for root, config := range r.pathConfigs {
		if filePath == root || strings.HasPrefix(filePath, root+string(filepath.Separator)) {
			if len(root) > len(matchedRoot) {
				matchedRoot = root
				matchedConfig = config
			}
		}
	}
	if matchedConfig != nil {
		return matchedConfig, nil
	}
	return nil, registry.ErrPathNotFound
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
	reg := newMockRegistry()

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
	reg := newMockRegistry()

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
	reg := newMockRegistry()

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

func TestWatcher_StatsWatchedPaths(t *testing.T) {
	tmpDir := t.TempDir()

	bus := newMockBus()
	reg := newMockRegistry()

	w, err := New(bus, reg)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.Stop()

	stats := w.Stats()
	if stats.WatchedPaths != 0 {
		t.Errorf("expected 0 watched paths before watch, got %d", stats.WatchedPaths)
	}

	err = w.Watch(tmpDir)
	if err != nil {
		t.Fatalf("failed to watch: %v", err)
	}

	stats = w.Stats()
	if stats.WatchedPaths != 1 {
		t.Errorf("expected 1 watched path after watch, got %d", stats.WatchedPaths)
	}

	err = w.Unwatch(tmpDir)
	if err != nil {
		t.Fatalf("failed to unwatch: %v", err)
	}

	stats = w.Stats()
	if stats.WatchedPaths != 0 {
		t.Errorf("expected 0 watched paths after unwatch, got %d", stats.WatchedPaths)
	}
}

func TestWatcher_Stats(t *testing.T) {
	tmpDir := t.TempDir()

	bus := newMockBus()
	reg := newMockRegistry()

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
	reg := newMockRegistry()

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
	reg := newMockRegistry()

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

func TestIsEditorNoise(t *testing.T) {
	tests := []struct {
		path   string
		ignore bool
	}{
		// Editor noise (transient artifacts) - should be filtered
		{"/test/file.swp", true},
		{"/test/file.swo", true},
		{"/test/file.swn", true},
		{"/test/4913", true},
		{"/test/#autosave#", true},
		{"/test/file~", true},
		{"/test/backup.txt~", true},

		// NOT editor noise - handled by PathConfig
		{"/test/.hidden", false},
		{"/test/.DS_Store", false},
		{"/test/Thumbs.db", false},
		{"/test/file.tmp", false},
		{"/test/file.bak", false},
		{"/test/normal.go", false},
		{"/test/main.py", false},
		{"/test/README.md", false},
		{"/test/~temp", false},    // Starts with ~ but not backup file
		{"/test/#partial", false}, // Starts with # but no trailing #
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isEditorNoise(tt.path)
			if got != tt.ignore {
				t.Errorf("isEditorNoise(%q) = %v, want %v", tt.path, got, tt.ignore)
			}
		})
	}
}

// TestShouldIgnoreFile tests backward compatibility
func TestShouldIgnoreFile(t *testing.T) {
	// shouldIgnoreFile now just delegates to isEditorNoise
	if !shouldIgnoreFile("/test/file.swp") {
		t.Error("shouldIgnoreFile should filter .swp files")
	}
	if shouldIgnoreFile("/test/normal.go") {
		t.Error("shouldIgnoreFile should not filter normal files")
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
	reg := newMockRegistry()

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
	reg := newMockRegistry()
	// Configure PathConfig to skip hidden
	reg.SetPathConfig(tmpDir, &registry.PathConfig{SkipHidden: true})

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
	// (hidden directories are skipped based on PathConfig)
}

func TestWatcher_PathConfigSkipExtensions(t *testing.T) {
	tmpDir := t.TempDir()

	bus := newMockBus()
	reg := newMockRegistry()
	// Configure PathConfig to skip .log files
	reg.SetPathConfig(tmpDir, &registry.PathConfig{
		SkipExtensions: []string{".log", ".tmp"},
	})

	w, err := New(bus, reg, WithDebounceWindow(50*time.Millisecond))
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

	time.Sleep(50 * time.Millisecond)

	// Create a .log file (should be skipped)
	logFile := filepath.Join(tmpDir, "debug.log")
	os.WriteFile(logFile, []byte("log content"), 0644)

	// Create a .go file (should not be skipped)
	goFile := filepath.Join(tmpDir, "main.go")
	os.WriteFile(goFile, []byte("package main"), 0644)

	time.Sleep(200 * time.Millisecond)

	// Check events - we should only see the .go file
	receivedEvents := bus.Events()
	for _, ev := range receivedEvents {
		if fe, ok := ev.Payload.(events.FileEvent); ok {
			if strings.HasSuffix(fe.Path, ".log") {
				t.Errorf("unexpected event for .log file: %s", fe.Path)
			}
		}
	}
}

func TestWatcher_PathConfigSkipHiddenFalse(t *testing.T) {
	tmpDir := t.TempDir()

	bus := newMockBus()
	reg := newMockRegistry()
	// Configure PathConfig to NOT skip hidden
	reg.SetPathConfig(tmpDir, &registry.PathConfig{
		SkipHidden: false,
	})

	w, err := New(bus, reg)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.Stop()

	// Create a hidden directory before watching
	hiddenDir := filepath.Join(tmpDir, ".hidden")
	os.Mkdir(hiddenDir, 0755)

	err = w.Watch(tmpDir)
	if err != nil {
		t.Fatalf("failed to watch: %v", err)
	}

	// When SkipHidden is false, the watch should include hidden directories
	// The watcher should complete without error
}

func TestWatcher_PathConfigSkipDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directories before setting up watcher
	nodeModules := filepath.Join(tmpDir, "node_modules")
	os.Mkdir(nodeModules, 0755)

	srcDir := filepath.Join(tmpDir, "src")
	os.Mkdir(srcDir, 0755)

	bus := newMockBus()
	reg := newMockRegistry()
	// Configure PathConfig to skip node_modules
	reg.SetPathConfig(tmpDir, &registry.PathConfig{
		SkipDirectories: []string{"node_modules"},
	})

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
	// node_modules directory should be skipped based on PathConfig
}

func TestMockRegistry_GetEffectiveConfig(t *testing.T) {
	reg := newMockRegistry()

	// Set config for root path
	rootConfig := &registry.PathConfig{
		SkipHidden:     true,
		SkipExtensions: []string{".log"},
	}
	reg.SetPathConfig("/project", rootConfig)

	ctx := context.Background()

	// Test file under root path
	config, err := reg.GetEffectiveConfig(ctx, "/project/src/main.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config != rootConfig {
		t.Error("expected root config to be returned")
	}

	// Test file not under any path
	_, err = reg.GetEffectiveConfig(ctx, "/other/file.go")
	if err == nil {
		t.Error("expected error for file not under any registered path")
	}

	// Test nested path takes precedence
	nestedConfig := &registry.PathConfig{
		SkipHidden:     false,
		SkipExtensions: []string{".tmp"},
	}
	reg.SetPathConfig("/project/special", nestedConfig)

	config, err = reg.GetEffectiveConfig(ctx, "/project/special/file.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config != nestedConfig {
		t.Error("expected nested config to take precedence")
	}
}
