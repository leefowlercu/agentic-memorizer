package main

import (
	"os"

	"github.com/leefowlercu/agentic-memorizer/cmd"

	// Import formatters to register them via init() functions
	_ "github.com/leefowlercu/agentic-memorizer/internal/format/formatters"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
