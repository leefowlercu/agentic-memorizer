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
	if collector.jobRunning == nil {
		t.Error("expected jobRunning to be initialized")
	}
}

func TestComponentHealthCollector_Collect_EmptyBag(t *testing.T) {
	bag := &ComponentBag{}
	collector := NewComponentHealthCollector(bag)

	statuses := collector.CollectComponents()
	if statuses == nil {
		t.Fatal("expected non-nil component statuses")
	}
	if len(statuses) != 0 {
		t.Errorf("expected empty component statuses for empty bag, got %d", len(statuses))
	}

	jobs := collector.CollectJobs()
	if jobs == nil {
		t.Fatal("expected non-nil job statuses")
	}
	if len(jobs) != 0 {
		t.Errorf("expected empty job statuses for empty bag, got %d", len(jobs))
	}
}

func TestComponentHealthCollector_Collect_GraphDegraded(t *testing.T) {
	bag := &ComponentBag{
		GraphDegraded: true,
	}
	collector := NewComponentHealthCollector(bag)

	// Mock graph by setting GraphDegraded but no actual graph
	// When graph is nil but GraphDegraded is true, nothing is added
	statuses := collector.CollectComponents()
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

func TestComponentHealthCollector_RecordJobStart(t *testing.T) {
	bag := &ComponentBag{}
	collector := NewComponentHealthCollector(bag)

	startedAt := time.Now().Add(-time.Second)
	collector.RecordJobStart("job.running", startedAt)

	jobs := collector.CollectJobs()
	job, ok := jobs["job.running"]
	if !ok {
		t.Fatal("expected job.running in job statuses")
	}
	if job.Status != JobStatusRunning {
		t.Errorf("job status = %v, want %v", job.Status, JobStatusRunning)
	}
	if !job.StartedAt.Equal(startedAt) {
		t.Errorf("job started_at = %v, want %v", job.StartedAt, startedAt)
	}
}

func TestComponentHealthCollector_Collect_WithJobResults(t *testing.T) {
	bag := &ComponentBag{}
	collector := NewComponentHealthCollector(bag)

	tests := []struct {
		name           string
		jobName        string
		status         RunStatus
		expectedStatus JobStatus
	}{
		{
			name:           "success job",
			jobName:        "job.success",
			status:         RunSuccess,
			expectedStatus: JobStatusSuccess,
		},
		{
			name:           "failed job",
			jobName:        "job.failed",
			status:         RunFailed,
			expectedStatus: JobStatusFailed,
		},
		{
			name:           "partial job",
			jobName:        "job.partial",
			status:         RunPartial,
			expectedStatus: JobStatusPartial,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector.RecordJobResult(tt.jobName, RunResult{
				Status:     tt.status,
				StartedAt:  time.Now().Add(-time.Minute),
				FinishedAt: time.Now(),
			})

			statuses := collector.CollectJobs()
			status, ok := statuses[tt.jobName]
			if !ok {
				t.Fatalf("expected job %s in job statuses", tt.jobName)
			}
			if status.Status != tt.expectedStatus {
				t.Errorf("expected job status %v, got %v", tt.expectedStatus, status.Status)
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
