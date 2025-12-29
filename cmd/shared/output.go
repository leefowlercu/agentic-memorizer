package shared

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
	_ "github.com/leefowlercu/agentic-memorizer/internal/format/formatters" // Register formatters
)

// OutputStatus formats and outputs a status message to stdout.
// Uses the text formatter for human-readable output.
func OutputStatus(status *format.Status) error {
	formatter, err := format.GetFormatter("text")
	if err != nil {
		return fmt.Errorf("failed to get formatter; %w", err)
	}
	output, err := formatter.Format(status)
	if err != nil {
		return fmt.Errorf("failed to format status; %w", err)
	}
	fmt.Println(output)
	return nil
}

// OutputSection formats and outputs a section to stdout.
// Uses the text formatter for human-readable output.
func OutputSection(section *format.Section) error {
	formatter, err := format.GetFormatter("text")
	if err != nil {
		return fmt.Errorf("failed to get formatter; %w", err)
	}
	output, err := formatter.Format(section)
	if err != nil {
		return fmt.Errorf("failed to format section; %w", err)
	}
	fmt.Println(output)
	return nil
}

// OutputError formats and outputs an error with an optional suggestion.
// Uses the error formatter for consistent error display.
func OutputError(err error, suggestion string) error {
	errContent := format.NewError(format.ErrorTypeRuntime, err.Error())
	if suggestion != "" {
		errContent.WithSuggestion(suggestion)
	}

	formatter, fmtErr := format.GetFormatter("text")
	if fmtErr != nil {
		// Fallback to simple error output
		fmt.Printf("Error: %v\n", err)
		if suggestion != "" {
			fmt.Printf("\nSuggestion: %s\n", suggestion)
		}
		return nil
	}

	output, fmtErr := formatter.Format(errContent)
	if fmtErr != nil {
		// Fallback to simple error output
		fmt.Printf("Error: %v\n", err)
		if suggestion != "" {
			fmt.Printf("\nSuggestion: %s\n", suggestion)
		}
		return nil
	}

	fmt.Println(output)
	return nil
}
