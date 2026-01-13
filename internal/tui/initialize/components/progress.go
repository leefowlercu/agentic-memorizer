package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// Progress displays the current step in a wizard flow.
type Progress struct {
	steps   []string
	current int
}

// NewProgress creates a new progress indicator with the given step names.
func NewProgress(steps []string) Progress {
	return Progress{
		steps:   steps,
		current: 0,
	}
}

// SetCurrent sets the current step, clamping to valid range.
func (p *Progress) SetCurrent(step int) {
	if step < 0 {
		p.current = 0
	} else if step >= len(p.steps) {
		p.current = len(p.steps) - 1
	} else {
		p.current = step
	}
}

// Current returns the current step index.
func (p Progress) Current() int {
	return p.current
}

// Total returns the total number of steps.
func (p Progress) Total() int {
	return len(p.steps)
}

// CurrentName returns the name of the current step.
func (p Progress) CurrentName() string {
	if p.current < 0 || p.current >= len(p.steps) {
		return ""
	}
	return p.steps[p.current]
}

// IsFirst returns true if on the first step.
func (p Progress) IsFirst() bool {
	return p.current == 0
}

// IsLast returns true if on the last step.
func (p Progress) IsLast() bool {
	return p.current == len(p.steps)-1
}

// View renders the progress indicator.
func (p Progress) View() string {
	var b strings.Builder

	// Build dot-based progress indicator
	filledStyle := lipgloss.NewStyle().Foreground(styles.Primary)
	emptyStyle := lipgloss.NewStyle().Foreground(styles.Muted)

	for i := range p.steps {
		if i <= p.current {
			b.WriteString(filledStyle.Render(styles.ProgressFilled))
		} else {
			b.WriteString(emptyStyle.Render(styles.ProgressEmpty))
		}
		if i < len(p.steps)-1 {
			b.WriteString(" ")
		}
	}

	// Step counter and name
	b.WriteString("  ")
	b.WriteString(styles.MutedText.Render(fmt.Sprintf("Step %d of %d:", p.current+1, len(p.steps))))
	b.WriteString(" ")
	b.WriteString(styles.Title.Render(p.CurrentName()))
	b.WriteString("\n")

	return b.String()
}

// Steps returns all step names.
func (p Progress) Steps() []string {
	return p.steps
}

// Next advances to the next step if not at the end.
func (p *Progress) Next() bool {
	if p.current < len(p.steps)-1 {
		p.current++
		return true
	}
	return false
}

// Prev goes back to the previous step if not at the start.
func (p *Progress) Prev() bool {
	if p.current > 0 {
		p.current--
		return true
	}
	return false
}
