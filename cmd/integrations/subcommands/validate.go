package subcommands

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/spf13/cobra"
)

var ValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate integration configurations",
	Long: "\nValidate that all configured integrations are properly set up.\n\n" +
		"Checks each integration's configuration files and settings to ensure they are valid " +
		"and properly configured for use with agentic-memorizer.",
	RunE: runValidate,
}

func runValidate(cmd *cobra.Command, args []string) error {
	registry := integrations.GlobalRegistry()
	all := registry.List()

	if len(all) == 0 {
		fmt.Println("No integrations registered")
		return nil
	}

	fmt.Println("Validating integrations...")
	fmt.Println()

	hasErrors := false
	validCount := 0
	unconfiguredCount := 0

	for _, integration := range all {
		name := integration.GetName()

		// Check if enabled
		enabled, err := integration.IsEnabled()
		if err != nil {
			fmt.Printf("✗ %s: failed to check status: %v\n", name, err)
			hasErrors = true
			continue
		}

		if !enabled {
			fmt.Printf("○ %s: not configured (skipping validation)\n", name)
			unconfiguredCount++
			continue
		}

		// Validate
		if err := integration.Validate(); err != nil {
			fmt.Printf("✗ %s: %v\n", name, err)
			hasErrors = true
		} else {
			fmt.Printf("✓ %s: valid\n", name)
			validCount++
		}
	}

	fmt.Println()
	if hasErrors {
		fmt.Printf("Validation failed: %d valid, %d errors, %d unconfigured\n", validCount, len(all)-validCount-unconfiguredCount, unconfiguredCount)
		return fmt.Errorf("validation errors found")
	}

	fmt.Printf("All configured integrations are valid (%d valid, %d unconfigured)\n", validCount, unconfiguredCount)
	return nil
}
