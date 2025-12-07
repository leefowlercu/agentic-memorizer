package subcommands

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
	_ "github.com/leefowlercu/agentic-memorizer/internal/format/formatters" // Register formatters
)

// outputStatus formats and outputs a status message
func outputStatus(status *format.Status) error {
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
