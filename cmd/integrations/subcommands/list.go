package subcommands

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/spf13/cobra"
)

var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available integrations",
	Long: "\nList all integrations registered in the system.\n\n" +
		"Shows the name, description, and current status (configured/not configured) of each integration.",
	PreRunE: validateList,
	RunE:    runList,
}

func validateList(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runList(cmd *cobra.Command, args []string) error {
	registry := integrations.GlobalRegistry()
	all := registry.List()

	if len(all) == 0 {
		status := format.NewStatus(format.StatusInfo, "No integrations registered")
		formatter, err := format.GetFormatter("text")
		if err != nil {
			return fmt.Errorf("failed to get formatter; %w", err)
		}
		output, err := formatter.Format(status)
		if err != nil {
			return fmt.Errorf("failed to format output; %w", err)
		}
		fmt.Println(output)
		return nil
	}

	// Build main section
	section := format.NewSection("Available Integrations").AddDivider()

	// Create a list for integrations
	list := format.NewList(format.ListTypeUnordered)

	for _, integration := range all {
		status := "not configured"
		var severity format.StatusSeverity

		enabled, err := integration.IsEnabled()
		if err != nil {
			status = fmt.Sprintf("error: %v", err)
			severity = format.StatusError
		} else if enabled {
			status = "configured"
			severity = format.StatusSuccess
		} else {
			severity = format.StatusInfo
		}

		// Get appropriate status symbol
		statusSymbol := format.GetStatusSymbol(severity)

		// Build multi-line item content
		itemContent := fmt.Sprintf("%s %s\nDescription: %s\nVersion:     %s\nStatus:      %s",
			statusSymbol,
			integration.GetName(),
			integration.GetDescription(),
			integration.GetVersion(),
			status)

		list.AddItem(itemContent)
	}

	// Format and write output
	formatter, err := format.GetFormatter("text")
	if err != nil {
		return fmt.Errorf("failed to get formatter; %w", err)
	}

	// Format section header
	sectionOutput, err := formatter.Format(section)
	if err != nil {
		return fmt.Errorf("failed to format section; %w", err)
	}
	fmt.Println(sectionOutput)

	// Format list
	listOutput, err := formatter.Format(list)
	if err != nil {
		return fmt.Errorf("failed to format list; %w", err)
	}
	fmt.Println(listOutput)

	return nil
}
