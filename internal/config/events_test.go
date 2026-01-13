package config

import (
	"testing"
)

func TestDetectChangedSections_NoChanges(t *testing.T) {
	cfg := NewDefaultConfig()
	old := &cfg
	new := &cfg

	changed := detectChangedSections(old, new)
	if len(changed) != 0 {
		t.Errorf("detectChangedSections() returned %v, want empty slice", changed)
	}
}

func TestDetectChangedSections_LogLevelChanged(t *testing.T) {
	old := NewDefaultConfig()
	new := NewDefaultConfig()
	new.LogLevel = "debug"

	changed := detectChangedSections(&old, &new)
	if len(changed) != 1 || changed[0] != "log_level" {
		t.Errorf("detectChangedSections() = %v, want [log_level]", changed)
	}
}

func TestDetectChangedSections_DaemonChanged(t *testing.T) {
	old := NewDefaultConfig()
	new := NewDefaultConfig()
	new.Daemon.HTTPPort = 9999

	changed := detectChangedSections(&old, &new)
	if len(changed) != 1 || changed[0] != "daemon" {
		t.Errorf("detectChangedSections() = %v, want [daemon]", changed)
	}
}

func TestDetectChangedSections_SemanticChanged(t *testing.T) {
	old := NewDefaultConfig()
	new := NewDefaultConfig()
	new.Semantic.Model = "different-model"

	changed := detectChangedSections(&old, &new)
	if len(changed) != 1 || changed[0] != "semantic" {
		t.Errorf("detectChangedSections() = %v, want [semantic]", changed)
	}
}

func TestDetectChangedSections_MultipleChanges(t *testing.T) {
	old := NewDefaultConfig()
	new := NewDefaultConfig()
	new.LogLevel = "debug"
	new.Daemon.HTTPPort = 9999
	new.Semantic.Model = "different-model"

	changed := detectChangedSections(&old, &new)
	if len(changed) != 3 {
		t.Errorf("detectChangedSections() returned %d sections, want 3", len(changed))
	}

	// Check all expected sections are present
	changedSet := make(map[string]bool)
	for _, s := range changed {
		changedSet[s] = true
	}
	if !changedSet["log_level"] || !changedSet["daemon"] || !changedSet["semantic"] {
		t.Errorf("detectChangedSections() = %v, missing expected sections", changed)
	}
}

func TestIsReloadable_AllReloadable(t *testing.T) {
	sections := []string{"log_level", "log_file", "semantic", "embeddings"}
	if !isReloadable(sections) {
		t.Error("isReloadable() = false, want true for reloadable sections")
	}
}

func TestIsReloadable_IncludesNonReloadable_Daemon(t *testing.T) {
	sections := []string{"log_level", "daemon"}
	if isReloadable(sections) {
		t.Error("isReloadable() = true, want false when daemon is included")
	}
}

func TestIsReloadable_IncludesNonReloadable_Graph(t *testing.T) {
	sections := []string{"semantic", "graph"}
	if isReloadable(sections) {
		t.Error("isReloadable() = true, want false when graph is included")
	}
}

func TestIsReloadable_Empty(t *testing.T) {
	if !isReloadable([]string{}) {
		t.Error("isReloadable() = false, want true for empty slice")
	}
}

func TestIsReloadable_OnlyReloadable(t *testing.T) {
	tests := []struct {
		name     string
		sections []string
		want     bool
	}{
		{"log_level only", []string{"log_level"}, true},
		{"semantic only", []string{"semantic"}, true},
		{"embeddings only", []string{"embeddings"}, true},
		{"log_file only", []string{"log_file"}, true},
		{"daemon only", []string{"daemon"}, false},
		{"graph only", []string{"graph"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isReloadable(tt.sections)
			if got != tt.want {
				t.Errorf("isReloadable(%v) = %v, want %v", tt.sections, got, tt.want)
			}
		})
	}
}
