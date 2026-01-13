package components

import (
	"strings"
	"testing"
)

func TestProgress_New(t *testing.T) {
	steps := []string{"Step 1", "Step 2", "Step 3"}
	p := NewProgress(steps)

	if p.current != 0 {
		t.Errorf("expected current step 0, got %d", p.current)
	}

	if p.Total() != 3 {
		t.Errorf("expected total 3, got %d", p.Total())
	}
}

func TestProgress_SetCurrent(t *testing.T) {
	steps := []string{"Step 1", "Step 2", "Step 3"}
	p := NewProgress(steps)

	p.SetCurrent(1)
	if p.current != 1 {
		t.Errorf("expected current 1, got %d", p.current)
	}

	// Out of bounds should be clamped
	p.SetCurrent(10)
	if p.current != 2 {
		t.Errorf("expected current to be clamped to 2, got %d", p.current)
	}

	p.SetCurrent(-1)
	if p.current != 0 {
		t.Errorf("expected current to be clamped to 0, got %d", p.current)
	}
}

func TestProgress_View(t *testing.T) {
	steps := []string{"Step 1", "Step 2", "Step 3"}
	p := NewProgress(steps)

	view := p.View()

	// Should show step indicator
	if !strings.Contains(view, "1") || !strings.Contains(view, "3") {
		t.Error("view should contain step numbers")
	}

	// Should show current step name
	if !strings.Contains(view, "Step 1") {
		t.Error("view should contain current step name")
	}
}

func TestProgress_CurrentName(t *testing.T) {
	steps := []string{"Welcome", "Config", "Done"}
	p := NewProgress(steps)

	if p.CurrentName() != "Welcome" {
		t.Errorf("expected 'Welcome', got '%s'", p.CurrentName())
	}

	p.SetCurrent(1)
	if p.CurrentName() != "Config" {
		t.Errorf("expected 'Config', got '%s'", p.CurrentName())
	}
}

func TestProgress_IsFirst(t *testing.T) {
	steps := []string{"Step 1", "Step 2"}
	p := NewProgress(steps)

	if !p.IsFirst() {
		t.Error("expected IsFirst to be true at step 0")
	}

	p.SetCurrent(1)
	if p.IsFirst() {
		t.Error("expected IsFirst to be false at step 1")
	}
}

func TestProgress_IsLast(t *testing.T) {
	steps := []string{"Step 1", "Step 2"}
	p := NewProgress(steps)

	if p.IsLast() {
		t.Error("expected IsLast to be false at step 0")
	}

	p.SetCurrent(1)
	if !p.IsLast() {
		t.Error("expected IsLast to be true at last step")
	}
}
