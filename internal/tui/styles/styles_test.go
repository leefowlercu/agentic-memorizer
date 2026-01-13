package styles

import (
	"testing"
)

func TestStylesAreDefined(t *testing.T) {
	tests := []struct {
		name  string
		style string
	}{
		{"Title", Title.Render("test")},
		{"Subtitle", Subtitle.Render("test")},
		{"Label", Label.Render("test")},
		{"ErrorText", ErrorText.Render("test")},
		{"SuccessText", SuccessText.Render("test")},
		{"MutedText", MutedText.Render("test")},
		{"HelpText", HelpText.Render("test")},
		{"Focused", Focused.Render("test")},
		{"Unfocused", Unfocused.Render("test")},
		{"Selected", Selected.Render("test")},
		{"Cursor", Cursor.Render("test")},
		{"Container", Container.Render("test")},
		{"StepContainer", StepContainer.Render("test")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.style == "" {
				t.Errorf("%s style should render non-empty output", tt.name)
			}
		})
	}
}

func TestColorsAreDefined(t *testing.T) {
	colors := []struct {
		name  string
		color string
	}{
		{"Primary", string(Primary)},
		{"Secondary", string(Secondary)},
		{"Success", string(Success)},
		{"Error", string(Error)},
		{"Warning", string(Warning)},
		{"Highlight", string(Highlight)},
		{"Muted", string(Muted)},
	}

	for _, c := range colors {
		t.Run(c.name, func(t *testing.T) {
			if c.color == "" {
				t.Errorf("%s color should not be empty", c.name)
			}
		})
	}
}

func TestIndicatorsAreDefined(t *testing.T) {
	indicators := []struct {
		name  string
		value string
	}{
		{"RadioSelected", RadioSelected},
		{"RadioUnselected", RadioUnselected},
		{"CheckboxSelected", CheckboxSelected},
		{"CheckboxUnselected", CheckboxUnselected},
		{"ProgressFilled", ProgressFilled},
		{"ProgressEmpty", ProgressEmpty},
		{"CursorIndicator", CursorIndicator},
	}

	for _, ind := range indicators {
		t.Run(ind.name, func(t *testing.T) {
			if ind.value == "" {
				t.Errorf("%s indicator should not be empty", ind.name)
			}
		})
	}
}
