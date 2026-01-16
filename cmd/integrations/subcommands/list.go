package subcommands

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
)

// Flag variables
var (
	listVerbose bool
	listHarness string
	listType    string
)

// ListCmd lists available integrations.
var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available integrations",
	Long: `List available integrations.

Displays all available integrations for AI coding assistants. Use filters to show
integrations for a specific harness or type.`,
	Example: `  # List all integrations
  memorizer integrations list

  # List integrations for Claude Code
  memorizer integrations list --harness claude-code

  # List only MCP integrations
  memorizer integrations list --type mcp

  # Show verbose details
  memorizer integrations list --verbose`,
	PreRunE: validateList,
	RunE:    runList,
}

func init() {
	ListCmd.Flags().BoolVarP(&listVerbose, "verbose", "v", false, "Show detailed information")
	ListCmd.Flags().StringVarP(&listHarness, "harness", "H", "", "Filter by harness name")
	ListCmd.Flags().StringVarP(&listType, "type", "t", "", "Filter by integration type (hook, mcp, plugin)")
}

func validateList(cmd *cobra.Command, args []string) error {
	// Validate type filter
	if listType != "" {
		validTypes := map[string]bool{"hook": true, "mcp": true, "plugin": true}
		if !validTypes[listType] {
			return fmt.Errorf("invalid type %q; must be one of: hook, mcp, plugin", listType)
		}
	}

	cmd.SilenceUsage = true
	return nil
}

func runList(cmd *cobra.Command, args []string) error {
	reg := registry()
	var allIntegrations []integrations.Integration

	// Filter by harness if specified
	if listHarness != "" {
		allIntegrations = reg.ListByHarness(listHarness)
	} else if listType != "" {
		allIntegrations = reg.ListByType(integrations.IntegrationType(listType))
	} else {
		allIntegrations = reg.List()
	}

	// Further filter by type if both filters specified
	if listHarness != "" && listType != "" {
		filtered := make([]integrations.Integration, 0)
		for _, i := range allIntegrations {
			if i.Type() == integrations.IntegrationType(listType) {
				filtered = append(filtered, i)
			}
		}
		allIntegrations = filtered
	}

	if len(allIntegrations) == 0 {
		fmt.Println("No integrations found matching the criteria")
		return nil
	}

	// Print output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if listVerbose {
		fmt.Fprintln(w, "NAME\tHARNESS\tTYPE\tSTATUS\tDESCRIPTION")
		fmt.Fprintln(w, "----\t-------\t----\t------\t-----------")
	} else {
		fmt.Fprintln(w, "NAME\tHARNESS\tTYPE\tSTATUS")
		fmt.Fprintln(w, "----\t-------\t----\t------")
	}

	for _, i := range allIntegrations {
		status, _ := i.Status()
		statusStr := "Unknown"
		if status != nil {
			statusStr = formatStatus(status.Status)
		}

		if listVerbose {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				i.Name(),
				i.Harness(),
				formatIntegrationType(i.Type()),
				statusStr,
				i.Description(),
			)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				i.Name(),
				i.Harness(),
				formatIntegrationType(i.Type()),
				statusStr,
			)
		}
	}

	w.Flush()
	return nil
}
