package subcommands

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/spf13/cobra"
)

var HealthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check health of all integrations",
	Long: "\nCheck the health status of all configured integrations.\n\n" +
		"Performs comprehensive health checks including:\n" +
		"- Configuration file accessibility\n" +
		"- Settings validity\n" +
		"- Binary path verification\n" +
		"- Integration-specific checks\n\n" +
		"This is more thorough than 'validate' and includes runtime checks.\n\n" +
		"Exit codes:\n" +
		"  0 - All checked integrations are healthy\n" +
		"  1 - One or more integrations have issues (useful for CI/CD scripts)",
	Example: `  # Check health of all integrations
  agentic-memorizer integrations health

  # Check health of specific integration
  agentic-memorizer integrations health claude-code-hook`,
	PreRunE: validateHealth,
	RunE:    runHealth,
}

func init() {
	HealthCmd.Flags().StringSlice("integrations", []string{}, "Specific integrations to check (default: all)")
}

func validateHealth(cmd *cobra.Command, args []string) error {
	specificIntegrations, _ := cmd.Flags().GetStringSlice("integrations")

	// Validate integration names if specified
	if len(specificIntegrations) > 0 {
		registry := integrations.GlobalRegistry()
		for _, name := range specificIntegrations {
			if _, err := registry.Get(name); err != nil {
				return fmt.Errorf("integration %q not found; %w\n\nRun 'agentic-memorizer integrations list' to see available integrations", name, err)
			}
		}
	}

	// All validation passed - errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runHealth(cmd *cobra.Command, args []string) error {
	registry := integrations.GlobalRegistry()
	specificIntegrations, _ := cmd.Flags().GetStringSlice("integrations")

	var toCheck []integrations.Integration

	if len(specificIntegrations) > 0 {
		// Check specific integrations
		for _, name := range specificIntegrations {
			integration, err := registry.Get(name)
			if err != nil {
				return fmt.Errorf("integration %q not found; %w", name, err)
			}
			toCheck = append(toCheck, integration)
		}
	} else {
		// Check all integrations
		toCheck = registry.List()
	}

	if len(toCheck) == 0 {
		status := format.NewStatus(format.StatusInfo, "No integrations to check")
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
	section := format.NewSection("Integration Health Check").AddDivider()

	hasIssues := false
	healthyCount := 0
	unconfiguredCount := 0
	issueCount := 0

	for _, integration := range toCheck {
		name := integration.GetName()
		integrationSection := format.NewSection(fmt.Sprintf("Checking %s", name)).SetLevel(1)

		// Check if enabled
		enabled, err := integration.IsEnabled()
		if err != nil {
			integrationSection.AddKeyValue("Status", fmt.Sprintf("Status check failed: %v", err))
			hasIssues = true
			issueCount++

			// Add to section as subsection
			section.AddSubsection(integrationSection)
			continue
		}

		if !enabled {
			integrationSection.AddKeyValue("Status", "Not configured (skipping health checks)")
			unconfiguredCount++

			// Add to section
			section.AddSubsection(integrationSection)
			continue
		}

		// Perform health checks
		issuesFound := false
		var checks []string

		// Check 1: Framework detection
		detected, err := integration.Detect()
		if err != nil {
			checks = append(checks, fmt.Sprintf("%s Detection failed: %v", format.SymbolError, err))
			issuesFound = true
		} else if !detected {
			checks = append(checks, fmt.Sprintf("%s Framework not detected (may not be installed)", format.SymbolWarning))
			issuesFound = true
		} else {
			checks = append(checks, fmt.Sprintf("%s Framework detected", format.SymbolSuccess))
		}

		// Check 2: Configuration validation
		if err := integration.Validate(); err != nil {
			checks = append(checks, fmt.Sprintf("%s Validation failed: %v", format.SymbolError, err))
			issuesFound = true
		} else {
			checks = append(checks, fmt.Sprintf("%s Configuration valid", format.SymbolSuccess))
		}

		// Add check results to integration section
		for _, check := range checks {
			integrationSection.AddKeyValue("", check)
		}

		// Summary for this integration
		if issuesFound {
			integrationSection.AddKeyValue("Status", "Issues found")
			hasIssues = true
			issueCount++
		} else {
			integrationSection.AddKeyValue("Status", "Healthy")
			healthyCount++
		}

		section.AddSubsection(integrationSection)
	}

	// Add summary subsection
	summarySection := format.NewSection("Summary").SetLevel(1).AddDivider()
	summarySection.AddKeyValuef("Total checked", "%d", len(toCheck))
	summarySection.AddKeyValuef("Healthy", "%d", healthyCount)
	summarySection.AddKeyValuef("Issues", "%d", issueCount)
	summarySection.AddKeyValuef("Unconfigured", "%d", unconfiguredCount)
	section.AddSubsection(summarySection)

	// Format and write output
	formatter, err := format.GetFormatter("text")
	if err != nil {
		return fmt.Errorf("failed to get formatter; %w", err)
	}
	output, err := formatter.Format(section)
	if err != nil {
		return fmt.Errorf("failed to format output; %w", err)
	}
	fmt.Println(output)

	fmt.Println()

	if hasIssues {
		status := format.NewStatus(format.StatusError, fmt.Sprintf("Health check found %d integration(s) with issues", issueCount))
		statusOutput, err := formatter.Format(status)
		if err != nil {
			return fmt.Errorf("failed to format status; %w", err)
		}
		fmt.Println(statusOutput)
		return fmt.Errorf("health check found %d integration(s) with issues", issueCount)
	}

	status := format.NewStatus(format.StatusSuccess, "All configured integrations are healthy")
	statusOutput, err := formatter.Format(status)
	if err != nil {
		return fmt.Errorf("failed to format status; %w", err)
	}
	fmt.Println(statusOutput)

	return nil
}
