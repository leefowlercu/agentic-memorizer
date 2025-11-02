package subcommands

import (
	"fmt"

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
  agentic-memorizer integrations health claude-code`,
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
		fmt.Println("No integrations to check")
		return nil
	}

	fmt.Println("Integration Health Check")
	fmt.Println("========================")
	fmt.Println()

	hasIssues := false
	healthyCount := 0
	unconfiguredCount := 0
	issueCount := 0

	for _, integration := range toCheck {
		name := integration.GetName()
		fmt.Printf("Checking %s...\n", name)

		// Check if enabled
		enabled, err := integration.IsEnabled()
		if err != nil {
			fmt.Printf("  ✗ Status check failed: %v\n", err)
			hasIssues = true
			issueCount++
			fmt.Println()
			continue
		}

		if !enabled {
			fmt.Printf("  ○ Not configured (skipping health checks)\n")
			unconfiguredCount++
			fmt.Println()
			continue
		}

		// Perform health checks
		issuesFound := false

		// Check 1: Framework detection
		detected, err := integration.Detect()
		if err != nil {
			fmt.Printf("  ✗ Detection failed: %v\n", err)
			issuesFound = true
		} else if !detected {
			fmt.Printf("  ⚠ Framework not detected (may not be installed)\n")
			issuesFound = true
		} else {
			fmt.Printf("  ✓ Framework detected\n")
		}

		// Check 2: Configuration validation
		if err := integration.Validate(); err != nil {
			fmt.Printf("  ✗ Validation failed: %v\n", err)
			issuesFound = true
		} else {
			fmt.Printf("  ✓ Configuration valid\n")
		}

		// Summary for this integration
		if issuesFound {
			fmt.Printf("  Status: Issues found\n")
			hasIssues = true
			issueCount++
		} else {
			fmt.Printf("  Status: Healthy\n")
			healthyCount++
		}

		fmt.Println()
	}

	// Overall summary
	fmt.Println("Summary")
	fmt.Println("-------")
	fmt.Printf("Total checked: %d\n", len(toCheck))
	fmt.Printf("Healthy: %d\n", healthyCount)
	fmt.Printf("Issues: %d\n", issueCount)
	fmt.Printf("Unconfigured: %d\n", unconfiguredCount)
	fmt.Println()

	if hasIssues {
		return fmt.Errorf("health check found %d integration(s) with issues", issueCount)
	}

	fmt.Println("✓ All configured integrations are healthy")
	return nil
}
