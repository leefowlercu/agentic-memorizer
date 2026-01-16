package subcommands

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
)

// Flag variables
var (
	statusAll bool
)

// StatusCmd shows integration status.
var StatusCmd = &cobra.Command{
	Use:   "status [integration-name]",
	Short: "Show integration status",
	Long: `Show integration status.

Displays the current status of a specific integration or all integrations.
Shows whether the integration is installed, the configuration path, and any errors.`,
	Example: `  # Show status of a specific integration
  memorizer integrations status claude-code-hook

  # Show status of all integrations
  memorizer integrations status --all`,
	Args:    cobra.MaximumNArgs(1),
	PreRunE: validateStatus,
	RunE:    runStatus,
}

func init() {
	StatusCmd.Flags().BoolVarP(&statusAll, "all", "a", false, "Show status of all integrations")
}

func validateStatus(cmd *cobra.Command, args []string) error {
	// Must provide either integration name or --all flag
	if len(args) == 0 && !statusAll {
		return fmt.Errorf("must provide integration name or use --all flag")
	}

	// If integration name provided, verify it exists
	if len(args) > 0 {
		if _, err := lookupIntegration(args[0]); err != nil {
			return err
		}
	}

	cmd.SilenceUsage = true
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	reg := registry()

	if statusAll || len(args) == 0 {
		// Show status of all integrations
		return showAllStatus(reg)
	}

	// Show status of specific integration
	integrationName := args[0]
	integration, err := lookupIntegration(integrationName)
	if err != nil {
		return err
	}

	return showIntegrationStatus(integration)
}

func showAllStatus(reg *integrations.Registry) error {
	allIntegrations := reg.List()

	if len(allIntegrations) == 0 {
		fmt.Println("No integrations available")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tHARNESS\tTYPE\tSTATUS\tDETAILS")
	fmt.Fprintln(w, "----\t-------\t----\t------\t-------")

	for _, i := range allIntegrations {
		status, err := i.Status()
		statusStr := "Unknown"
		details := ""

		if err != nil {
			statusStr = "Error"
			details = err.Error()
		} else if status != nil {
			statusStr = formatStatus(status.Status)
			details = status.Message
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			i.Name(),
			i.Harness(),
			formatIntegrationType(i.Type()),
			statusStr,
			details,
		)
	}

	w.Flush()
	return nil
}

func showIntegrationStatus(i integrations.Integration) error {
	fmt.Printf("Integration: %s\n", i.Name())
	fmt.Printf("Harness:     %s\n", i.Harness())
	fmt.Printf("Type:        %s\n", formatIntegrationType(i.Type()))
	fmt.Printf("Description: %s\n", i.Description())
	fmt.Println()

	status, err := i.Status()
	if err != nil {
		fmt.Printf("Status:      Error\n")
		fmt.Printf("Error:       %v\n", err)
		return nil
	}

	fmt.Printf("Status:      %s\n", formatStatus(status.Status))
	if status.Message != "" {
		fmt.Printf("Message:     %s\n", status.Message)
	}
	if status.ConfigPath != "" {
		fmt.Printf("Config:      %s\n", status.ConfigPath)
	}
	if status.BackupPath != "" {
		fmt.Printf("Backup:      %s\n", status.BackupPath)
	}
	if !status.InstalledAt.IsZero() {
		fmt.Printf("Installed:   %s\n", status.InstalledAt.Format(time.RFC3339))
	}

	return nil
}
