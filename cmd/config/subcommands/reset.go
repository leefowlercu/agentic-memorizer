package subcommands

import (
	"fmt"
	"os"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/spf13/cobra"
)

var (
	resetConfirm bool
)

// ResetCmd resets the configuration to defaults.
var ResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to default values",
	Long: "Reset configuration to default values.\n\n" +
		"This command removes the configuration file, reverting all settings " +
		"to their default values. A backup of the current configuration is " +
		"created before deletion. Use --confirm to skip the confirmation prompt.",
	Example: `  # Reset configuration (prompts for confirmation)
  memorizer config reset

  # Reset configuration without confirmation
  memorizer config reset --confirm`,
	PreRunE: validateReset,
	RunE:    runReset,
}

func init() {
	ResetCmd.Flags().BoolVar(&resetConfirm, "confirm", false, "Skip confirmation prompt")
}

func validateReset(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runReset(cmd *cobra.Command, args []string) error {
	configPath := config.GetConfigPath()

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("No configuration file found. Using defaults.")
		return nil
	}

	// Confirm reset if not already confirmed
	if !resetConfirm {
		fmt.Printf("This will reset configuration to defaults and remove: %s\n", configPath)
		fmt.Print("Are you sure? [y/N]: ")

		var response string
		fmt.Scanln(&response)

		if response != "y" && response != "Y" {
			fmt.Println("Reset cancelled.")
			return nil
		}
	}

	// Create backup
	backupPath := fmt.Sprintf("%s.backup.%d", configPath, time.Now().Unix())
	if err := copyFile(configPath, backupPath); err != nil {
		return fmt.Errorf("failed to create backup; %w", err)
	}
	fmt.Printf("Backup created: %s\n", backupPath)

	// Remove config file
	if err := os.Remove(configPath); err != nil {
		return fmt.Errorf("failed to remove config file; %w", err)
	}

	fmt.Println("Configuration reset to defaults.")
	fmt.Println("Restart the daemon to apply changes.")
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
