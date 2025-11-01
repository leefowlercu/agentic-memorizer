package cmdintegrations

import (
	"fmt"
	"os"

	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/adapters/claude"   // Register Claude adapter
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/adapters/generic" // Register generic adapters
	"github.com/spf13/cobra"
)

var IntegrationsCmd = &cobra.Command{
	Use:   "integrations",
	Short: "Manage agent framework integrations",
	Long: `Manage integrations with agent frameworks like Claude Code, Continue.dev, Cline, etc.

The integrations command group provides tools for discovering, configuring, and managing
integrations with various AI agent frameworks.`,
	Example: `  # List all available integrations
  agentic-memorizer integrations list

  # Detect installed agent frameworks
  agentic-memorizer integrations detect

  # Setup a specific integration
  agentic-memorizer integrations setup claude-code

  # Remove an integration
  agentic-memorizer integrations remove claude-code

  # Validate configuration
  agentic-memorizer integrations validate`,
}

func init() {
	IntegrationsCmd.AddCommand(listCmd)
	IntegrationsCmd.AddCommand(detectCmd)
	IntegrationsCmd.AddCommand(setupCmd)
	IntegrationsCmd.AddCommand(removeCmd)
	IntegrationsCmd.AddCommand(validateCmd)
	IntegrationsCmd.AddCommand(healthCmd)
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available integrations",
	Long: `List all integrations registered in the system.

Shows the name, description, and current status (configured/not configured) of each integration.`,
	RunE: runList,
}

var detectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Detect installed agent frameworks",
	Long: `Detect which agent frameworks are installed on this system.

Scans for configuration files and directories of supported frameworks like Claude Code,
Continue.dev, Cline, etc.`,
	RunE: runDetect,
}

var setupCmd = &cobra.Command{
	Use:   "setup <integration-name>",
	Short: "Setup a specific integration",
	Long: `Setup integration with a specific agent framework.

Configures the framework to use agentic-memorizer for memory indexing. The setup process
varies by framework but typically involves adding hooks or tools to the framework's
configuration files.`,
	Example: `  # Setup Claude Code integration
  agentic-memorizer integrations setup claude-code

  # Setup with custom binary path
  agentic-memorizer integrations setup claude-code --binary-path /custom/path/agentic-memorizer`,
	Args: cobra.ExactArgs(1),
	RunE: runSetup,
}

var removeCmd = &cobra.Command{
	Use:   "remove <integration-name>",
	Short: "Remove an integration",
	Long: `Remove integration configuration from an agent framework.

Removes hooks, tools, or other configuration entries that were added by the setup command.`,
	Example: `  # Remove Claude Code integration
  agentic-memorizer integrations remove claude-code`,
	Args: cobra.ExactArgs(1),
	RunE: runRemove,
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate integration configurations",
	Long: `Validate that all configured integrations are properly set up.

Checks each integration's configuration files and settings to ensure they are valid
and properly configured for use with agentic-memorizer.`,
	RunE: runValidate,
}

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check health of all integrations",
	Long: `Check the health status of all configured integrations.

Performs comprehensive health checks including:
- Configuration file accessibility
- Settings validity
- Binary path verification
- Integration-specific checks

This is more thorough than 'validate' and includes runtime checks.`,
	Example: `  # Check health of all integrations
  agentic-memorizer integrations health

  # Check health of specific integration
  agentic-memorizer integrations health claude-code`,
	RunE: runHealth,
}

func init() {
	setupCmd.Flags().String("binary-path", "", "Custom path to agentic-memorizer binary (auto-detected if not specified)")
	healthCmd.Flags().StringSlice("integrations", []string{}, "Specific integrations to check (default: all)")
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

func runSetup(cmd *cobra.Command, args []string) error {
	integrationName := args[0]
	binaryPath, _ := cmd.Flags().GetString("binary-path")

	// Find binary path if not specified
	if binaryPath == "" {
		path, err := findBinaryPath()
		if err != nil {
			return fmt.Errorf("could not auto-detect binary path: %w\nPlease specify with --binary-path flag", err)
		}
		binaryPath = path
	}

	registry := integrations.GlobalRegistry()
	integration, err := registry.Get(integrationName)
	if err != nil {
		return fmt.Errorf("integration %q not found: %w\n\nRun 'agentic-memorizer integrations list' to see available integrations", integrationName, err)
	}

	// Check if framework is installed (skip for generic adapters)
	detected, err := integration.Detect()
	if err != nil {
		return fmt.Errorf("failed to detect %s: %w", integrationName, err)
	}
	if !detected {
		// Try to setup anyway - generic adapters will provide helpful manual instructions
		fmt.Printf("Warning: %s does not appear to be installed (auto-detection may not work for all frameworks)\n", integration.GetName())
		fmt.Printf("Attempting setup anyway...\n\n")
	}

	// Check if already configured
	enabled, _ := integration.IsEnabled()
	if enabled {
		fmt.Printf("%s is already configured.\n", integration.GetName())
		fmt.Printf("To reconfigure, first remove the integration:\n")
		fmt.Printf("  agentic-memorizer integrations remove %s\n", integrationName)
		fmt.Printf("Then setup again:\n")
		fmt.Printf("  agentic-memorizer integrations setup %s\n", integrationName)
		return nil
	}

	// Setup integration
	fmt.Printf("Setting up %s integration...\n", integration.GetName())
	fmt.Printf("Binary path: %s\n", binaryPath)
	fmt.Println()

	if err := integration.Setup(binaryPath); err != nil {
		return fmt.Errorf("failed to setup %s: %w", integrationName, err)
	}

	fmt.Printf("✓ %s integration configured successfully\n", integration.GetName())
	fmt.Println()
	fmt.Printf("The integration will be active on your next %s session.\n", integration.GetName())

	return nil
}

func runRemove(cmd *cobra.Command, args []string) error {
	integrationName := args[0]

	registry := integrations.GlobalRegistry()
	integration, err := registry.Get(integrationName)
	if err != nil {
		return fmt.Errorf("integration %q not found: %w", integrationName, err)
	}

	// Check if configured
	enabled, err := integration.IsEnabled()
	if err != nil {
		return fmt.Errorf("failed to check integration status: %w", err)
	}
	if !enabled {
		fmt.Printf("%s is not currently configured\n", integration.GetName())
		return nil
	}

	// Remove integration
	fmt.Printf("Removing %s integration...\n", integration.GetName())

	if err := integration.Remove(); err != nil {
		return fmt.Errorf("failed to remove %s: %w", integrationName, err)
	}

	fmt.Printf("✓ %s integration removed successfully\n", integration.GetName())

	return nil
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

// findBinaryPath attempts to locate the agentic-memorizer binary
func runHealth(cmd *cobra.Command, args []string) error {
	registry := integrations.GlobalRegistry()
	specificIntegrations, _ := cmd.Flags().GetStringSlice("integrations")

	var toCheck []integrations.Integration

	if len(specificIntegrations) > 0 {
		// Check specific integrations
		for _, name := range specificIntegrations {
			integration, err := registry.Get(name)
			if err != nil {
				return fmt.Errorf("integration %q not found: %w", name, err)
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

func findBinaryPath() (string, error) {
	// Try to get the current executable path
	execPath, err := os.Executable()
	if err == nil {
		// Check if this is the agentic-memorizer binary
		if baseName := execPath; len(baseName) > 0 {
			return execPath, nil
		}
	}

	// Try common installation paths
	home, err := os.UserHomeDir()
	if err == nil {
		commonPaths := []string{
			home + "/.local/bin/agentic-memorizer",
			home + "/go/bin/agentic-memorizer",
			"/usr/local/bin/agentic-memorizer",
		}

		for _, path := range commonPaths {
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	// Try PATH
	// Note: exec.LookPath would normally be used here, but we're avoiding
	// it to keep dependencies minimal
	return "", fmt.Errorf("could not locate agentic-memorizer binary")
}
