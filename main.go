package main

import (
	"os"

	"github.com/leefowlercu/agentic-memorizer/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
