package steps

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"

	"github.com/leefowlercu/agentic-memorizer/internal/tui/styles"
)

// NavigationHelp returns the standard navigation help text for steps without text input.
func NavigationHelp() string {
	return renderHelp([]helpItem{
		{key: "↑/↓", desc: "navigate"},
		{key: "enter", desc: "select"},
		{key: "esc", desc: "back"},
		{key: "ctrl+c", desc: "quit"},
	})
}

// NavigationHelpWithInput returns the navigation help text for steps with text input.
func NavigationHelpWithInput() string {
	return renderHelp([]helpItem{
		{key: "enter", desc: "continue"},
		{key: "esc", desc: "back"},
		{key: "ctrl+c", desc: "quit"},
	})
}

type helpItem struct {
	key  string
	desc string
}

func renderHelp(items []helpItem) string {
	keyStyle := lipgloss.NewStyle().
		Foreground(styles.Secondary).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(styles.Muted)

	sep := lipgloss.NewStyle().
		Foreground(styles.Muted).
		Render(" • ")

	var result string
	for i, item := range items {
		if i > 0 {
			result += sep
		}
		result += keyStyle.Render(item.key) + " " + descStyle.Render(item.desc)
	}

	return result
}

// DetectAPIKey checks if an API key exists in the environment.
func DetectAPIKey(envVar string) bool {
	return os.Getenv(envVar) != ""
}

// GetAPIKey retrieves an API key from the environment.
func GetAPIKey(envVar string) string {
	return os.Getenv(envVar)
}

// FormatKeyStatus returns a formatted string indicating API key status.
func FormatKeyStatus(detected bool) string {
	if detected {
		return styles.SuccessText.Render("✓ API Key: Detected")
	}
	mutedStyle := lipgloss.NewStyle().Foreground(styles.Muted)
	return mutedStyle.Render("○ API Key: Not found")
}

// FormatError returns a formatted error message.
func FormatError(err error) string {
	if err == nil {
		return ""
	}
	return styles.ErrorText.Render(fmt.Sprintf("Error: %s", err.Error()))
}

// FormatSuccess returns a formatted success message.
func FormatSuccess(msg string) string {
	return styles.SuccessText.Render(fmt.Sprintf("✓ %s", msg))
}

// FormatWarning returns a formatted warning message.
func FormatWarning(msg string) string {
	warningStyle := lipgloss.NewStyle().
		Foreground(styles.Warning)
	return warningStyle.Render(fmt.Sprintf("⚠ %s", msg))
}
