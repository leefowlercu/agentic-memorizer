package components

import (
	"fmt"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// Progress displays step progress
type Progress struct {
	steps       []string
	currentStep int
}

// NewProgress creates a new progress indicator
func NewProgress(steps []string) *Progress {
	return &Progress{
		steps:       steps,
		currentStep: 0,
	}
}

// SetStep sets the current step index
func (p *Progress) SetStep(step int) {
	if step >= 0 && step < len(p.steps) {
		p.currentStep = step
	}
}

// CurrentStep returns the current step index
func (p *Progress) CurrentStep() int {
	return p.currentStep
}

// TotalSteps returns the total number of steps
func (p *Progress) TotalSteps() int {
	return len(p.steps)
}

// CurrentStepName returns the name of the current step
func (p *Progress) CurrentStepName() string {
	if p.currentStep >= 0 && p.currentStep < len(p.steps) {
		return p.steps[p.currentStep]
	}
	return ""
}

// View renders the progress indicator
func (p *Progress) View() string {
	var b strings.Builder

	// Step counter
	counter := fmt.Sprintf("Step %d of %d", p.currentStep+1, len(p.steps))
	b.WriteString(styles.MutedText.Render(counter))
	b.WriteString("\n")

	// Progress dots
	var dots strings.Builder
	for i := range p.steps {
		if i == p.currentStep {
			dots.WriteString(styles.Focused.Render(styles.ProgressFilled))
		} else if i < p.currentStep {
			dots.WriteString(styles.SuccessText.Render(styles.ProgressFilled))
		} else {
			dots.WriteString(styles.MutedText.Render(styles.ProgressEmpty))
		}
		if i < len(p.steps)-1 {
			dots.WriteString(" ")
		}
	}
	b.WriteString(dots.String())
	b.WriteString("\n\n")

	// Current step title
	if p.currentStep >= 0 && p.currentStep < len(p.steps) {
		title := styles.Title.Render(p.steps[p.currentStep])
		b.WriteString(title)
	}

	return b.String()
}

// ViewCompact renders a compact progress indicator (single line)
func (p *Progress) ViewCompact() string {
	counter := fmt.Sprintf("[%d/%d] %s", p.currentStep+1, len(p.steps), p.CurrentStepName())
	return styles.Title.Render(counter)
}
