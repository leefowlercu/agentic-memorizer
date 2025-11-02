package subcommands

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/spf13/cobra"
)

var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available integrations",
	Long: "\nList all integrations registered in the system.\n\n" +
		"Shows the name, description, and current status (configured/not configured) of each integration.",
	RunE: runList,
}

func runList(cmd *cobra.Command, args []string) error {
	registry := integrations.GlobalRegistry()
	all := registry.List()

	if len(all) == 0 {
		fmt.Println("No integrations registered")
		return nil
	}

	fmt.Println("Available Integrations:")
	fmt.Println()

	for _, integration := range all {
		status := "not configured"
		statusSymbol := "○"

		enabled, err := integration.IsEnabled()
		if err != nil {
			status = fmt.Sprintf("error: %v", err)
			statusSymbol = "✗"
		} else if enabled {
			status = "configured"
			statusSymbol = "✓"
		}

		fmt.Printf("  %s %s\n", statusSymbol, integration.GetName())
		fmt.Printf("    Description: %s\n", integration.GetDescription())
		fmt.Printf("    Version:     %s\n", integration.GetVersion())
		fmt.Printf("    Status:      %s\n", status)
		fmt.Println()
	}

	return nil
}
