package subcommands

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/spf13/cobra"
)

var DetectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Detect installed agent frameworks",
	Long: "\nDetect which agent frameworks are installed on this system.\n\n" +
		"Scans for configuration files and directories of supported frameworks like Claude Code, " +
		"Continue.dev, Cline, etc.",
	RunE: runDetect,
}

func runDetect(cmd *cobra.Command, args []string) error {
	registry := integrations.GlobalRegistry()
	detected := registry.DetectAvailable()

	if len(detected) == 0 {
		fmt.Println("No agent frameworks detected on this system")
		fmt.Println()
		fmt.Println("Supported frameworks:")
		for _, integration := range registry.List() {
			fmt.Printf("  - %s\n", integration.GetName())
		}
		return nil
	}

	fmt.Println("Detected Agent Frameworks:")
	fmt.Println()

	for _, integration := range detected {
		enabled, _ := integration.IsEnabled()
		statusSymbol := "○"
		statusText := "not configured"

		if enabled {
			statusSymbol = "✓"
			statusText = "configured"
		}

		fmt.Printf("  %s %s - %s (%s)\n",
			statusSymbol,
			integration.GetName(),
			integration.GetDescription(),
			statusText)
	}

	fmt.Println()
	fmt.Println("To setup an integration, run:")
	fmt.Println("  agentic-memorizer integrations setup <integration-name>")

	return nil
}
