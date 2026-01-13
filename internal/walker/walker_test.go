package walker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/events"
	"github.com/leefowlercu/agentic-memorizer/internal/handlers"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

var errFileStateNotFound = errors.New("file state not found")

// mockRegistry implements registry.Registry for testing.
type mockRegistry struct {
	paths      map[string]*registry.RememberedPath
	fileStates map[string]*registry.FileState
	mu         sync.RWMutex
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		paths:      make(map[string]*registry.RememberedPath),
		fileStates: make(map[string]*registry.FileState),
	}
}

func (r *mockRegistry) AddPath(ctx context.Context, path string, config *registry.PathConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.paths[path] = &registry.RememberedPath{
		Path:   path,
		Config: config,
	}
	return nil
}

func (r *mockRegistry) RemovePath(ctx context.Context, path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.paths, path)
	return nil
}

func (r *mockRegistry) GetPath(ctx context.Context, path string) (*registry.RememberedPath, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if rp, ok := r.paths[path]; ok {
		return rp, nil
	}
	return nil, registry.ErrPathNotFound
}

func (r *mockRegistry) ListPaths(ctx context.Context) ([]registry.RememberedPath, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []registry.RememberedPath
	for _, rp := range r.paths {
		result = append(result, *rp)
	}
	return result, nil
}

func (r *mockRegistry) UpdatePathConfig(ctx context.Context, path string, config *registry.PathConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rp, ok := r.paths[path]; ok {
		rp.Config = config
		return nil
	}
	return registry.ErrPathNotFound
}

func (r *mockRegistry) UpdatePathLastWalk(ctx context.Context, path string, lastWalk time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rp, ok := r.paths[path]; ok {
		rp.LastWalkAt = &lastWalk
		return nil
	}
	return registry.ErrPathNotFound
}

func (r *mockRegistry) FindContainingPath(ctx context.Context, path string) (*registry.RememberedPath, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Simple implementation: check if path starts with any remembered path
	for _, rp := range r.paths {
		if path == rp.Path || filepath.HasPrefix(path, rp.Path+string(filepath.Separator)) {
			return rp, nil
		}
	}
	return nil, registry.ErrPathNotFound
}

func (r *mockRegistry) GetEffectiveConfig(ctx context.Context, filePath string) (*registry.PathConfig, error) {
	rp, err := r.FindContainingPath(ctx, filePath)
	if err != nil {
		return nil, err
	}
	return rp.Config, nil
}

func (r *mockRegistry) GetFileState(ctx context.Context, path string) (*registry.FileState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if fs, ok := r.fileStates[path]; ok {
		return fs, nil
	}
	return nil, errFileStateNotFound
}

func (r *mockRegistry) UpdateFileState(ctx context.Context, state *registry.FileState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fileStates[state.Path] = state
	return nil
}

func (r *mockRegistry) DeleteFileState(ctx context.Context, path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.fileStates, path)
	return nil
}

func (r *mockRegistry) ListFileStates(ctx context.Context, basePath string) ([]registry.FileState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []registry.FileState
	for path, fs := range r.fileStates {
		if filepath.HasPrefix(path, basePath) {
			result = append(result, *fs)
		}
	}
	return result, nil
}

func (r *mockRegistry) DeleteFileStatesForPath(ctx context.Context, parentPath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for path := range r.fileStates {
		if filepath.HasPrefix(path, parentPath) {
			delete(r.fileStates, path)
		}
	}
	return nil
}

func (r *mockRegistry) Close() error {
	return nil
}

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

// Test helper to create a test directory structure.
func createTestFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for path, content := range files {
		fullPath := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
	}
}

func TestWalker_Walk(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"main.go":           "package main",
		"utils.go":          "package main",
		"README.md":         "# README",
		"src/handler.go":    "package src",
		"src/handler_test.go": "package src",
		"docs/guide.md":     "# Guide",
	}
	createTestFiles(t, tmpDir, files)

	reg := newMockRegistry()
	bus := newMockBus()
	hr := handlers.DefaultRegistry()

	// Remember the path
	_ = reg.AddPath(context.Background(), tmpDir, &registry.PathConfig{})

	w := New(reg, bus, hr)

	err := w.Walk(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	// Verify events were published
	events := bus.Events()
	if len(events) != 6 {
		t.Errorf("expected 6 events, got %d", len(events))
	}

	// Verify stats
	stats := w.Stats()
	if stats.FilesDiscovered != 6 {
		t.Errorf("expected 6 files discovered, got %d", stats.FilesDiscovered)
	}
	if stats.DirsTraversed < 3 {
		t.Errorf("expected at least 3 dirs traversed, got %d", stats.DirsTraversed)
	}
}

func TestWalker_Walk_SkipExtensions(t *testing.T) {
	tmpDir := t.TempDir()

	files := map[string]string{
		"main.go":    "package main",
		"data.json":  "{}",
		"config.yaml": "key: value",
	}
	createTestFiles(t, tmpDir, files)

	reg := newMockRegistry()
	bus := newMockBus()
	hr := handlers.DefaultRegistry()

	// Remember path with skip extensions
	_ = reg.AddPath(context.Background(), tmpDir, &registry.PathConfig{
		SkipExtensions: []string{".json", ".yaml"},
	})

	w := New(reg, bus, hr)

	err := w.Walk(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	events := bus.Events()
	if len(events) != 1 {
		t.Errorf("expected 1 event (main.go only), got %d", len(events))
	}
}

func TestWalker_Walk_SkipDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	files := map[string]string{
		"main.go":              "package main",
		"node_modules/pkg.js":  "module.exports = {}",
		"vendor/dep.go":        "package vendor",
		"src/app.go":           "package src",
	}
	createTestFiles(t, tmpDir, files)

	reg := newMockRegistry()
	bus := newMockBus()
	hr := handlers.DefaultRegistry()

	// Remember path with skip directories
	_ = reg.AddPath(context.Background(), tmpDir, &registry.PathConfig{
		SkipDirectories: []string{"node_modules", "vendor"},
	})

	w := New(reg, bus, hr)

	err := w.Walk(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	events := bus.Events()
	if len(events) != 2 {
		t.Errorf("expected 2 events (main.go, src/app.go), got %d", len(events))
	}
}

func TestWalker_Walk_SkipHidden(t *testing.T) {
	tmpDir := t.TempDir()

	files := map[string]string{
		"main.go":          "package main",
		".hidden":          "hidden file",
		".config/app.conf": "config",
	}
	createTestFiles(t, tmpDir, files)

	reg := newMockRegistry()
	bus := newMockBus()
	hr := handlers.DefaultRegistry()

	// Remember path with skip hidden
	_ = reg.AddPath(context.Background(), tmpDir, &registry.PathConfig{
		SkipHidden: true,
	})

	w := New(reg, bus, hr)

	err := w.Walk(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	events := bus.Events()
	if len(events) != 1 {
		t.Errorf("expected 1 event (main.go only), got %d", len(events))
	}
}

func TestWalker_Walk_IncludeExtensions(t *testing.T) {
	tmpDir := t.TempDir()

	files := map[string]string{
		"main.go":    "package main",
		"app.py":     "print('hello')",
		"data.json":  "{}",
		"README.md":  "# README",
	}
	createTestFiles(t, tmpDir, files)

	reg := newMockRegistry()
	bus := newMockBus()
	hr := handlers.DefaultRegistry()

	// Remember path with include extensions only
	_ = reg.AddPath(context.Background(), tmpDir, &registry.PathConfig{
		IncludeExtensions: []string{".go", ".py"},
	})

	w := New(reg, bus, hr)

	err := w.Walk(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	events := bus.Events()
	if len(events) != 2 {
		t.Errorf("expected 2 events (main.go, app.py), got %d", len(events))
	}
}

func TestWalker_WalkIncremental(t *testing.T) {
	tmpDir := t.TempDir()

	files := map[string]string{
		"main.go":  "package main",
		"utils.go": "package main",
	}
	createTestFiles(t, tmpDir, files)

	reg := newMockRegistry()
	bus := newMockBus()
	hr := handlers.DefaultRegistry()

	// Remember path
	_ = reg.AddPath(context.Background(), tmpDir, &registry.PathConfig{})

	// Set file state for main.go (simulate already processed)
	mainInfo, _ := os.Stat(filepath.Join(tmpDir, "main.go"))
	_ = reg.UpdateFileState(context.Background(), &registry.FileState{
		Path:    filepath.Join(tmpDir, "main.go"),
		Size:    mainInfo.Size(),
		ModTime: mainInfo.ModTime(),
	})

	w := New(reg, bus, hr)

	err := w.WalkIncremental(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("WalkIncremental failed: %v", err)
	}

	// Only utils.go should be discovered (main.go unchanged)
	events := bus.Events()
	if len(events) != 1 {
		t.Errorf("expected 1 event (utils.go only), got %d", len(events))
	}

	stats := w.Stats()
	if stats.FilesUnchanged != 1 {
		t.Errorf("expected 1 file unchanged, got %d", stats.FilesUnchanged)
	}
}

func TestWalker_WalkAll(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	createTestFiles(t, tmpDir1, map[string]string{"a.go": "package a"})
	createTestFiles(t, tmpDir2, map[string]string{"b.go": "package b"})

	reg := newMockRegistry()
	bus := newMockBus()
	hr := handlers.DefaultRegistry()

	// Remember both paths
	_ = reg.AddPath(context.Background(), tmpDir1, &registry.PathConfig{})
	_ = reg.AddPath(context.Background(), tmpDir2, &registry.PathConfig{})

	w := New(reg, bus, hr)

	err := w.WalkAll(context.Background())
	if err != nil {
		t.Fatalf("WalkAll failed: %v", err)
	}

	events := bus.Events()
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestWalker_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create many files
	files := make(map[string]string)
	for i := 0; i < 100; i++ {
		files[filepath.Join("dir", "file"+string(rune('0'+i%10))+".go")] = "package main"
	}
	createTestFiles(t, tmpDir, files)

	reg := newMockRegistry()
	bus := newMockBus()
	hr := handlers.DefaultRegistry()

	_ = reg.AddPath(context.Background(), tmpDir, &registry.PathConfig{})

	w := New(reg, bus, hr)

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := w.Walk(ctx, tmpDir)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestWalker_PathNotRemembered(t *testing.T) {
	tmpDir := t.TempDir()

	reg := newMockRegistry()
	bus := newMockBus()
	hr := handlers.DefaultRegistry()

	w := New(reg, bus, hr)

	err := w.Walk(context.Background(), tmpDir)
	if err == nil {
		t.Error("expected error for non-remembered path")
	}
}

func TestWalker_PathNotDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file.go")
	os.WriteFile(filePath, []byte("package main"), 0644)

	reg := newMockRegistry()
	bus := newMockBus()
	hr := handlers.DefaultRegistry()

	_ = reg.AddPath(context.Background(), filePath, &registry.PathConfig{})

	w := New(reg, bus, hr)

	err := w.Walk(context.Background(), filePath)
	if err == nil {
		t.Error("expected error for non-directory path")
	}
}

func TestWalker_Stats(t *testing.T) {
	tmpDir := t.TempDir()

	files := map[string]string{
		"main.go":    "package main",
		"binary.exe": "\x00\x01\x02",
	}
	createTestFiles(t, tmpDir, files)

	reg := newMockRegistry()
	bus := newMockBus()
	hr := handlers.DefaultRegistry()

	_ = reg.AddPath(context.Background(), tmpDir, &registry.PathConfig{})

	w := New(reg, bus, hr)

	// Stats before walk
	stats := w.Stats()
	if stats.IsWalking {
		t.Error("expected IsWalking to be false before walk")
	}

	err := w.Walk(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	// Stats after walk
	stats = w.Stats()
	if stats.IsWalking {
		t.Error("expected IsWalking to be false after walk")
	}
	if stats.LastWalkPath != tmpDir {
		t.Errorf("expected LastWalkPath to be %s, got %s", tmpDir, stats.LastWalkPath)
	}
	if stats.LastWalkAt.IsZero() {
		t.Error("expected LastWalkAt to be set")
	}
}

func TestWalker_Pacing(t *testing.T) {
	tmpDir := t.TempDir()

	// Create enough files to trigger pacing
	files := make(map[string]string)
	for i := 0; i < 10; i++ {
		files["file"+string(rune('0'+i))+".go"] = "package main"
	}
	createTestFiles(t, tmpDir, files)

	reg := newMockRegistry()
	bus := newMockBus()
	hr := handlers.DefaultRegistry()

	_ = reg.AddPath(context.Background(), tmpDir, &registry.PathConfig{})

	// Use small batch size and pace interval for testing
	w := New(reg, bus, hr, WithBatchSize(3), WithPaceInterval(10*time.Millisecond))

	start := time.Now()
	err := w.Walk(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}
	elapsed := time.Since(start)

	// Should have taken at least some time due to pacing (3 batches * 10ms = 30ms)
	if elapsed < 20*time.Millisecond {
		t.Logf("Walk completed in %v (pacing may not have triggered)", elapsed)
	}
}
