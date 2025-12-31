package subcommands

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/spf13/cobra"
)

var DetectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Detect installed agent frameworks on system",
	Long: "\nDetect which agent frameworks are installed on this system.\n\n" +
		"Scans for configuration files and directories of supported frameworks like Claude Code, " +
		"Continue.dev, Cline, etc.",
	Example: `  # Detect installed agent frameworks
  memorizer integrations detect`,
	PreRunE: validateDetect,
	RunE:    runDetect,
}

func validateDetect(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runDetect(cmd *cobra.Command, args []string) error {
	registry := integrations.GlobalRegistry()
	detected := registry.DetectAvailable()

	if len(detected) == 0 {
		status := format.NewStatus(format.StatusInfo, "No agent frameworks detected on this system")

		// Add supported frameworks as details
		for _, integration := range registry.List() {
			status.AddDetail(fmt.Sprintf("- %s", integration.GetName()))
		}

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

	section := format.NewSection("Detected Integrations").AddDivider()
	list := format.NewList(format.ListTypeUnordered)

	for _, integration := range detected {
		enabled, _ := integration.IsEnabled()
		statusSymbol := format.SymbolInfo
		statusText := "not configured"

		if enabled {
			statusSymbol = format.SymbolSuccess
			statusText = "configured"
		}

		itemContent := fmt.Sprintf("%s %s - %s (%s)",
			statusSymbol,
			integration.GetName(),
			integration.GetDescription(),
			statusText)
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

	// Add hint message
	fmt.Println()
	fmt.Println("To setup an integration, run: memorizer integrations setup <integration-name>")

	return nil
}
