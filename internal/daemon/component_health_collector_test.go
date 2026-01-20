package daemon

import (
	"log/slog"
	"testing"
	"time"
)

func TestNewComponentHealthCollector(t *testing.T) {
	bag := &ComponentBag{}
	collector := NewComponentHealthCollector(bag)

	if collector == nil {
		t.Fatal("expected non-nil collector")
	}
	if collector.bag != bag {
		t.Error("expected bag to be set")
	}
	if collector.jobResults == nil {
		t.Error("expected jobResults to be initialized")
	}
}

func TestComponentHealthCollector_Collect_EmptyBag(t *testing.T) {
	bag := &ComponentBag{}
	collector := NewComponentHealthCollector(bag)

	statuses := collector.Collect()
	if statuses == nil {
		t.Fatal("expected non-nil statuses")
	}
	if len(statuses) != 0 {
		t.Errorf("expected empty statuses for empty bag, got %d", len(statuses))
	}
}

func TestComponentHealthCollector_Collect_GraphDegraded(t *testing.T) {
	bag := &ComponentBag{
		GraphDegraded: true,
	}
	collector := NewComponentHealthCollector(bag)

	// Mock graph by setting GraphDegraded but no actual graph
	// When graph is nil but GraphDegraded is true, nothing is added
	statuses := collector.Collect()
	if _, ok := statuses["graph"]; ok {
		t.Error("expected no graph status when graph is nil")
	}
}

func TestComponentHealthCollector_RecordJobResult(t *testing.T) {
	bag := &ComponentBag{}
	collector := NewComponentHealthCollector(bag)

	result := RunResult{
		Status:     RunSuccess,
		StartedAt:  time.Now().Add(-time.Minute),
		FinishedAt: time.Now(),
		Counts:     map[string]int{"files": 10},
		Details:    map[string]any{"mode": "full"},
	}

	collector.RecordJobResult("job.test", result)

	retrieved, ok := collector.GetJobResult("job.test")
	if !ok {
		t.Fatal("expected job result to be found")
	}
	if retrieved.Status != RunSuccess {
		t.Errorf("expected status %v, got %v", RunSuccess, retrieved.Status)
	}
	if retrieved.Counts["files"] != 10 {
		t.Errorf("expected files count 10, got %d", retrieved.Counts["files"])
	}
}

func TestComponentHealthCollector_Collect_WithJobResults(t *testing.T) {
	bag := &ComponentBag{}
	collector := NewComponentHealthCollector(bag)

	tests := []struct {
		name           string
		jobName        string
		status         RunStatus
		expectedHealth ComponentStatus
	}{
		{
			name:           "success job",
			jobName:        "job.success",
			status:         RunSuccess,
			expectedHealth: ComponentStatusRunning,
		},
		{
			name:           "failed job",
			jobName:        "job.failed",
			status:         RunFailed,
			expectedHealth: ComponentStatusFailed,
		},
		{
			name:           "partial job",
			jobName:        "job.partial",
			status:         RunPartial,
			expectedHealth: ComponentStatusDegraded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector.RecordJobResult(tt.jobName, RunResult{
				Status:     tt.status,
				StartedAt:  time.Now().Add(-time.Minute),
				FinishedAt: time.Now(),
			})

			statuses := collector.Collect()
			status, ok := statuses[tt.jobName]
			if !ok {
				t.Fatalf("expected job %s in statuses", tt.jobName)
			}
			if status.Status != tt.expectedHealth {
				t.Errorf("expected health %v, got %v", tt.expectedHealth, status.Status)
			}
		})
	}
}

func TestComponentHealthCollector_SetBusDegraded(t *testing.T) {
	bag := &ComponentBag{}
	collector := NewComponentHealthCollector(bag)

	if collector.IsBusDegraded() {
		t.Error("expected bus not degraded initially")
	}

	collector.SetBusDegraded(true)
	if !collector.IsBusDegraded() {
		t.Error("expected bus degraded after SetBusDegraded(true)")
	}

	collector.SetBusDegraded(false)
	if collector.IsBusDegraded() {
		t.Error("expected bus not degraded after SetBusDegraded(false)")
	}
}

func TestComponentHealthCollector_GetJobResult_NotFound(t *testing.T) {
	bag := &ComponentBag{}
	collector := NewComponentHealthCollector(bag)

	_, ok := collector.GetJobResult("nonexistent")
	if ok {
		t.Error("expected ok=false for nonexistent job")
	}
}

func TestComponentHealthCollector_WithHealthCollectorLogger(t *testing.T) {
	bag := &ComponentBag{}
	collector := NewComponentHealthCollector(bag, WithHealthCollectorLogger(slog.Default()))

	if collector == nil {
		t.Fatal("expected non-nil collector")
	}
}
