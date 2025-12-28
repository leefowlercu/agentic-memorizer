package graph

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestFactConstants(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		expected int
	}{
		{"MinFactContentLength", MinFactContentLength, 10},
		{"MaxFactContentLength", MaxFactContentLength, 500},
		{"MaxTotalFacts", MaxTotalFacts, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != tt.expected {
				t.Errorf("%s = %d, want %d", tt.name, tt.value, tt.expected)
			}
		})
	}
}

func TestNewFacts(t *testing.T) {
	tests := []struct {
		name   string
		logger *slog.Logger
	}{
		{
			name:   "with nil logger",
			logger: nil,
		},
		{
			name:   "with custom logger",
			logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			facts := NewFacts(nil, tt.logger)

			if facts == nil {
				t.Fatal("expected non-nil facts handler")
			}
			if facts.logger == nil {
				t.Error("expected non-nil logger even when nil is passed")
			}
		})
	}
}

func TestFactNode_Fields(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)

	fact := FactNode{
		ID:        "test-uuid",
		Content:   "test content",
		CreatedAt: now,
		UpdatedAt: later,
		Source:    "cli",
	}

	if fact.ID != "test-uuid" {
		t.Errorf("expected ID 'test-uuid', got %q", fact.ID)
	}
	if fact.Content != "test content" {
		t.Errorf("expected Content 'test content', got %q", fact.Content)
	}
	if !fact.CreatedAt.Equal(now) {
		t.Errorf("expected CreatedAt %v, got %v", now, fact.CreatedAt)
	}
	if !fact.UpdatedAt.Equal(later) {
		t.Errorf("expected UpdatedAt %v, got %v", later, fact.UpdatedAt)
	}
	if fact.Source != "cli" {
		t.Errorf("expected Source 'cli', got %q", fact.Source)
	}
}

// Integration tests - require running FalkorDB
func TestFacts_CRUD_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient(ClientConfig{
		Host:     "localhost",
		Port:     6379,
		Database: "test_memorizer_facts",
	}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		t.Skipf("skipping test (requires FalkorDB); %v", err)
	}
	defer client.Close()

	// Initialize schema for the Fact index
	schema := NewSchema(client, DefaultSchemaConfig(), nil)
	if err := schema.Initialize(ctx); err != nil {
		t.Fatalf("failed to initialize schema; %v", err)
	}

	// Clean up before tests
	_, _ = client.Query(ctx, "MATCH (f:Fact) DELETE f", nil)

	facts := NewFacts(client, nil)

	t.Run("Create", func(t *testing.T) {
		fact, err := facts.Create(ctx, "This is a test fact for integration testing", "cli")
		if err != nil {
			t.Fatalf("Create failed; %v", err)
		}

		if fact.ID == "" {
			t.Error("expected non-empty ID")
		}
		if _, err := uuid.Parse(fact.ID); err != nil {
			t.Errorf("expected valid UUID, got %q", fact.ID)
		}
		if fact.Content != "This is a test fact for integration testing" {
			t.Errorf("expected content to match, got %q", fact.Content)
		}
		if fact.Source != "cli" {
			t.Errorf("expected source 'cli', got %q", fact.Source)
		}
		if fact.CreatedAt.IsZero() {
			t.Error("expected non-zero CreatedAt")
		}
	})

	t.Run("GetByID", func(t *testing.T) {
		// Create a fact first
		created, err := facts.Create(ctx, "Fact for GetByID test", "cli")
		if err != nil {
			t.Fatalf("Create failed; %v", err)
		}

		// Get it back
		found, err := facts.GetByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetByID failed; %v", err)
		}
		if found == nil {
			t.Fatal("expected to find fact")
		}
		if found.ID != created.ID {
			t.Errorf("expected ID %q, got %q", created.ID, found.ID)
		}
		if found.Content != created.Content {
			t.Errorf("expected Content %q, got %q", created.Content, found.Content)
		}
	})

	t.Run("GetByID_NotFound", func(t *testing.T) {
		found, err := facts.GetByID(ctx, "non-existent-uuid")
		if err != nil {
			t.Fatalf("GetByID failed; %v", err)
		}
		if found != nil {
			t.Error("expected nil for non-existent fact")
		}
	})

	t.Run("GetByContent", func(t *testing.T) {
		content := "Unique content for GetByContent test"
		created, err := facts.Create(ctx, content, "cli")
		if err != nil {
			t.Fatalf("Create failed; %v", err)
		}

		found, err := facts.GetByContent(ctx, content)
		if err != nil {
			t.Fatalf("GetByContent failed; %v", err)
		}
		if found == nil {
			t.Fatal("expected to find fact by content")
		}
		if found.ID != created.ID {
			t.Errorf("expected ID %q, got %q", created.ID, found.ID)
		}
	})

	t.Run("GetByContent_NotFound", func(t *testing.T) {
		found, err := facts.GetByContent(ctx, "content that does not exist anywhere")
		if err != nil {
			t.Fatalf("GetByContent failed; %v", err)
		}
		if found != nil {
			t.Error("expected nil for non-existent content")
		}
	})

	t.Run("Update", func(t *testing.T) {
		created, err := facts.Create(ctx, "Original content for update test", "cli")
		if err != nil {
			t.Fatalf("Create failed; %v", err)
		}

		// Small delay to ensure UpdatedAt will be different from CreatedAt
		time.Sleep(10 * time.Millisecond)

		updated, err := facts.Update(ctx, created.ID, "Updated content for update test")
		if err != nil {
			t.Fatalf("Update failed; %v", err)
		}
		if updated.Content != "Updated content for update test" {
			t.Errorf("expected updated content, got %q", updated.Content)
		}
		if updated.UpdatedAt.IsZero() {
			t.Error("expected non-zero UpdatedAt after update")
		}
		// UpdatedAt should be at or after CreatedAt (allow equal for fast execution)
		if !updated.UpdatedAt.IsZero() && !updated.CreatedAt.IsZero() {
			if updated.UpdatedAt.Before(updated.CreatedAt) {
				t.Errorf("expected UpdatedAt (%v) to be at or after CreatedAt (%v)",
					updated.UpdatedAt, updated.CreatedAt)
			}
		}
	})

	t.Run("Update_NotFound", func(t *testing.T) {
		_, err := facts.Update(ctx, "non-existent-uuid", "new content")
		if err == nil {
			t.Error("expected error when updating non-existent fact")
		}
	})

	t.Run("List", func(t *testing.T) {
		// Clean up and create known facts
		_, _ = client.Query(ctx, "MATCH (f:Fact) DELETE f", nil)

		_, _ = facts.Create(ctx, "First fact for list test", "cli")
		_, _ = facts.Create(ctx, "Second fact for list test", "cli")
		_, _ = facts.Create(ctx, "Third fact for list test", "cli")

		list, err := facts.List(ctx)
		if err != nil {
			t.Fatalf("List failed; %v", err)
		}
		if len(list) != 3 {
			t.Errorf("expected 3 facts, got %d", len(list))
		}
	})

	t.Run("Count", func(t *testing.T) {
		// Clean up and create known facts
		_, _ = client.Query(ctx, "MATCH (f:Fact) DELETE f", nil)

		_, _ = facts.Create(ctx, "Fact one for count test", "cli")
		_, _ = facts.Create(ctx, "Fact two for count test", "cli")

		count, err := facts.Count(ctx)
		if err != nil {
			t.Fatalf("Count failed; %v", err)
		}
		if count != 2 {
			t.Errorf("expected count 2, got %d", count)
		}
	})

	t.Run("Exists", func(t *testing.T) {
		created, err := facts.Create(ctx, "Fact for exists test", "cli")
		if err != nil {
			t.Fatalf("Create failed; %v", err)
		}

		exists, err := facts.Exists(ctx, created.ID)
		if err != nil {
			t.Fatalf("Exists failed; %v", err)
		}
		if !exists {
			t.Error("expected fact to exist")
		}

		notExists, err := facts.Exists(ctx, "non-existent-uuid")
		if err != nil {
			t.Fatalf("Exists failed; %v", err)
		}
		if notExists {
			t.Error("expected non-existent fact to not exist")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		created, err := facts.Create(ctx, "Fact to be deleted", "cli")
		if err != nil {
			t.Fatalf("Create failed; %v", err)
		}

		err = facts.Delete(ctx, created.ID)
		if err != nil {
			t.Fatalf("Delete failed; %v", err)
		}

		// Verify it's gone
		found, err := facts.GetByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetByID failed; %v", err)
		}
		if found != nil {
			t.Error("expected fact to be deleted")
		}
	})

	// Clean up after all tests
	_, _ = client.Query(ctx, "MATCH (f:Fact) DELETE f", nil)
}
