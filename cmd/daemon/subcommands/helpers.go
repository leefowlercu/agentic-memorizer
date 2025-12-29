package subcommands

import (
	"github.com/leefowlercu/agentic-memorizer/cmd/shared"
	"github.com/leefowlercu/agentic-memorizer/internal/format"
)

// outputStatus formats and outputs a status message
// Delegates to shared.OutputStatus
func outputStatus(status *format.Status) error {
	return shared.OutputStatus(status)
}
