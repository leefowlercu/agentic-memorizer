package analysis

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/leefowlercu/agentic-memorizer/internal/events"
)

func TestWorkerPipelineEndToEnd(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	queue := NewQueue(bus)
	queue.ctx = context.Background()

	worker := NewWorker(0, queue)
	worker.SetSemanticProvider(&mockSemanticProvider{available: true})
	worker.SetEmbeddingsProvider(&mockEmbeddingsProvider{
		available: true,
		embedding: []float32{0.1, 0.2},
	})
	worker.SetGraph(&mockGraph{})

	dir := t.TempDir()
	path := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(path, []byte("hello pipeline"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file failed: %v", err)
	}

	err = worker.processItem(context.Background(), WorkItem{
		FilePath:  path,
		FileSize:  info.Size(),
		ModTime:   info.ModTime(),
		EventType: WorkItemNew,
	})
	if err != nil {
		t.Fatalf("processItem failed: %v", err)
	}

	mockG, ok := worker.graph.(*mockGraph)
	if !ok {
		t.Fatal("expected mock graph to be set on worker")
	}
	if len(mockG.chunks) == 0 {
		t.Fatal("expected persisted chunks in mock graph")
	}
}
