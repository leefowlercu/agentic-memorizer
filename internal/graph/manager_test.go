package graph

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/leefowlercu/agentic-memorizer/pkg/types"
)

func TestDefaultManagerConfig(t *testing.T) {
	config := DefaultManagerConfig()

	// Check client config
	if config.Client.Host != "localhost" {
		t.Errorf("expected Client.Host 'localhost', got %q", config.Client.Host)
	}
	if config.Client.Port != 6379 {
		t.Errorf("expected Client.Port 6379, got %d", config.Client.Port)
	}
	if config.Client.Database != "memorizer" {
		t.Errorf("expected Client.Database 'memorizer', got %q", config.Client.Database)
	}
}

func TestNewManager(t *testing.T) {
	tests := []struct {
		name   string
		config ManagerConfig
		logger *slog.Logger
	}{
		{
			name:   "with default config and nil logger",
			config: DefaultManagerConfig(),
			logger: nil,
		},
		{
			name:   "with default config and custom logger",
			config: DefaultManagerConfig(),
			logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		},
		{
			name: "with custom config",
			config: ManagerConfig{
				Client: ClientConfig{
					Host:     "custom-host",
					Port:     16379,
					Database: "test-db",
				},
				MemoryRoot: "/test/memory",
			},
			logger: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager(tt.config, tt.logger)

			if manager == nil {
				t.Fatal("expected non-nil manager")
			}

			if manager.client == nil {
				t.Error("expected non-nil client")
			}
			if manager.logger == nil {
				t.Error("expected non-nil logger")
			}
			if manager.config.Client.Host != tt.config.Client.Host {
				t.Errorf("expected Host %q, got %q", tt.config.Client.Host, manager.config.Client.Host)
			}
		})
	}
}

func TestManager_IsConnected_NotInitialized(t *testing.T) {
	manager := NewManager(DefaultManagerConfig(), nil)

	if manager.IsConnected() {
		t.Error("expected IsConnected to return false for uninitialized manager")
	}
}

func TestManager_Close_NotInitialized(t *testing.T) {
	manager := NewManager(DefaultManagerConfig(), nil)

	err := manager.Close()
	if err != nil {
		t.Errorf("expected no error closing uninitialized manager, got: %v", err)
	}
}

func TestManager_Health_NotInitialized(t *testing.T) {
	manager := NewManager(DefaultManagerConfig(), nil)
	ctx := context.Background()

	status, err := manager.Health(ctx)
	if err != nil {
		t.Errorf("expected no error for health check, got: %v", err)
	}

	if status.Connected {
		t.Error("expected Connected to be false")
	}
	if status.Error == "" {
		t.Error("expected Error message to be set")
	}
}

func TestManager_UpdateSingle_NotConnected(t *testing.T) {
	manager := NewManager(DefaultManagerConfig(), nil)
	ctx := context.Background()

	entry := types.IndexEntry{
		Metadata: types.FileMetadata{
			FileInfo: types.FileInfo{
				Path: "/test/file.txt",
			},
		},
	}

	_, err := manager.UpdateSingle(ctx, entry, UpdateInfo{})
	if err == nil {
		t.Error("expected error when updating on unconnected manager")
	}
}

func TestManager_RemoveFile_NotConnected(t *testing.T) {
	manager := NewManager(DefaultManagerConfig(), nil)
	ctx := context.Background()

	err := manager.RemoveFile(ctx, "/test/file.txt")
	if err == nil {
		t.Error("expected error when removing file on unconnected manager")
	}
}

func TestManager_GetAll_NotConnected(t *testing.T) {
	manager := NewManager(DefaultManagerConfig(), nil)
	ctx := context.Background()

	_, err := manager.GetAll(ctx)
	if err == nil {
		t.Error("expected error when getting all on unconnected manager")
	}
}

func TestManager_GetFile_NotConnected(t *testing.T) {
	manager := NewManager(DefaultManagerConfig(), nil)
	ctx := context.Background()

	_, err := manager.GetFile(ctx, "/test/file.txt")
	if err == nil {
		t.Error("expected error when getting file on unconnected manager")
	}
}

func TestManager_GetStats_NotConnected(t *testing.T) {
	manager := NewManager(DefaultManagerConfig(), nil)
	ctx := context.Background()

	_, err := manager.GetStats(ctx)
	if err == nil {
		t.Error("expected error when getting stats on unconnected manager")
	}
}

func TestManager_Search_NotConnected(t *testing.T) {
	manager := NewManager(DefaultManagerConfig(), nil)
	ctx := context.Background()

	_, err := manager.Search(ctx, "test", 10, "")
	if err == nil {
		t.Error("expected error when searching on unconnected manager")
	}
}

func TestManager_GetRecentFiles_NotConnected(t *testing.T) {
	manager := NewManager(DefaultManagerConfig(), nil)
	ctx := context.Background()

	_, err := manager.GetRecentFiles(ctx, 7, 10)
	if err == nil {
		t.Error("expected error when getting recent files on unconnected manager")
	}
}

func TestManager_GetRelatedFiles_NotConnected(t *testing.T) {
	manager := NewManager(DefaultManagerConfig(), nil)
	ctx := context.Background()

	_, err := manager.GetRelatedFiles(ctx, "/test/file.txt", 10)
	if err == nil {
		t.Error("expected error when getting related files on unconnected manager")
	}
}

func TestManager_GetFileConnections_NotConnected(t *testing.T) {
	manager := NewManager(DefaultManagerConfig(), nil)
	ctx := context.Background()

	_, err := manager.GetFileConnections(ctx, "/test/file.txt")
	if err == nil {
		t.Error("expected error when getting file connections on unconnected manager")
	}
}

func TestManager_ClearGraph_NotConnected(t *testing.T) {
	manager := NewManager(DefaultManagerConfig(), nil)
	ctx := context.Background()

	err := manager.ClearGraph(ctx)
	if err == nil {
		t.Error("expected error when clearing graph on unconnected manager")
	}
}

func TestManager_Cleanup_NotConnected(t *testing.T) {
	manager := NewManager(DefaultManagerConfig(), nil)
	ctx := context.Background()

	_, err := manager.Cleanup(ctx)
	if err == nil {
		t.Error("expected error when cleaning up on unconnected manager")
	}
}

func TestManager_RemoveStaleFiles_NotConnected(t *testing.T) {
	manager := NewManager(DefaultManagerConfig(), nil)
	ctx := context.Background()

	currentPaths := map[string]bool{
		"/test/file1.txt": true,
		"/test/file2.txt": true,
	}

	_, err := manager.RemoveStaleFiles(ctx, currentPaths)
	if err == nil {
		t.Error("expected error when removing stale files on unconnected manager")
	}
}

func TestManager_Accessors(t *testing.T) {
	manager := NewManager(DefaultManagerConfig(), nil)

	// Before initialization, these should return nil
	if manager.Nodes() != nil {
		t.Error("expected Nodes() to return nil before initialization")
	}
	if manager.Edges() != nil {
		t.Error("expected Edges() to return nil before initialization")
	}
	if manager.Queries() != nil {
		t.Error("expected Queries() to return nil before initialization")
	}

	// Client should never be nil
	if manager.Client() == nil {
		t.Error("expected Client() to return non-nil")
	}
}

func TestUpdateInfo_Struct(t *testing.T) {
	info := UpdateInfo{
		WasAnalyzed: true,
		WasCached:   false,
		HadError:    false,
	}

	if !info.WasAnalyzed {
		t.Error("expected WasAnalyzed to be true")
	}
	if info.WasCached {
		t.Error("expected WasCached to be false")
	}
	if info.HadError {
		t.Error("expected HadError to be false")
	}
}

func TestUpdateResult_Struct(t *testing.T) {
	result := UpdateResult{
		Added:   true,
		Updated: false,
	}

	if !result.Added {
		t.Error("expected Added to be true")
	}
	if result.Updated {
		t.Error("expected Updated to be false")
	}
}

func TestEntity_Struct(t *testing.T) {
	entity := Entity{
		Name: "Terraform",
		Type: "technology",
	}

	if entity.Name != "Terraform" {
		t.Errorf("expected Name 'Terraform', got %q", entity.Name)
	}
	if entity.Type != "technology" {
		t.Errorf("expected Type 'technology', got %q", entity.Type)
	}
}

func TestReferenceInfo_Struct(t *testing.T) {
	ref := ReferenceInfo{
		Topic:      "infrastructure",
		RefType:    "requires",
		Confidence: 0.95,
	}

	if ref.Topic != "infrastructure" {
		t.Errorf("expected Topic 'infrastructure', got %q", ref.Topic)
	}
	if ref.RefType != "requires" {
		t.Errorf("expected RefType 'requires', got %q", ref.RefType)
	}
	if ref.Confidence != 0.95 {
		t.Errorf("expected Confidence 0.95, got %f", ref.Confidence)
	}
}

func TestStats_Struct(t *testing.T) {
	categories := map[string]int64{
		"documents": 10,
		"code":      20,
	}

	stats := Stats{
		TotalFiles:    30,
		TotalTags:     50,
		TotalTopics:   20,
		TotalEntities: 100,
		TotalEdges:    500,
		Categories:    categories,
	}

	if stats.TotalFiles != 30 {
		t.Errorf("expected TotalFiles 30, got %d", stats.TotalFiles)
	}
	if stats.TotalTags != 50 {
		t.Errorf("expected TotalTags 50, got %d", stats.TotalTags)
	}
	if stats.Categories["code"] != 20 {
		t.Errorf("expected Categories['code'] 20, got %d", stats.Categories["code"])
	}
}

// Integration tests - require running FalkorDB
func TestManager_Initialize_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	manager := NewManager(ManagerConfig{
		Client: ClientConfig{
			Host:     "localhost",
			Port:     6379,
			Database: "test_memorizer_manager",
		},
		MemoryRoot: "/test/memory",
	}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := manager.Initialize(ctx)
	if err != nil {
		t.Skipf("Skipping test (requires FalkorDB); %v", err)
	}
	defer manager.Close()

	if !manager.IsConnected() {
		t.Error("expected IsConnected to return true after initialization")
	}

	// Verify accessors are available
	if manager.Nodes() == nil {
		t.Error("expected Nodes() to return non-nil after initialization")
	}
	if manager.Edges() == nil {
		t.Error("expected Edges() to return non-nil after initialization")
	}
	if manager.Queries() == nil {
		t.Error("expected Queries() to return non-nil after initialization")
	}

	// Test health
	status, err := manager.Health(ctx)
	if err != nil {
		t.Errorf("Health check failed; %v", err)
	}
	if !status.Connected {
		t.Error("expected Connected to be true in health status")
	}
}
