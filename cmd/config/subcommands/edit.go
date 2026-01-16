package subcommands

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
)

// EditCmd opens the configuration file in an editor.
var EditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit the configuration file in your default editor",
	Long: "Edit the configuration file in your default editor.\n\n" +
		"Opens the memorizer configuration file in the editor specified by " +
		"the EDITOR environment variable. If EDITOR is not set, attempts to " +
		"use common editors (vim, vi, nano) in order.",
	Example: `  # Edit configuration with default editor
  memorizer config edit

  # Edit with a specific editor
  EDITOR=code memorizer config edit`,
	PreRunE: validateEdit,
	RunE:    runEdit,
}

func validateEdit(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runEdit(cmd *cobra.Command, args []string) error {
	configPath := config.GetConfigPath()

	// Ensure config directory exists
	if err := config.EnsureConfigDir(); err != nil {
		return fmt.Errorf("failed to create config directory; %w", err)
	}

	// Create config file if it doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create empty config file with comment
		content := "# Memorizer configuration\n# See documentation for available options\n"
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create config file; %w", err)
		}
	}

	// Find editor
	editor := findEditor()
	if editor == "" {
		return fmt.Errorf("no editor found; set EDITOR environment variable")
	}

	// Open editor
	editorCmd := exec.Command(editor, configPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("editor exited with error; %w", err)
	}

	fmt.Println("Configuration saved. Restart the daemon to apply changes.")
	return nil
}

func findEditor() string {
	// Check EDITOR environment variable first
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}

	// Check VISUAL environment variable
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}

	// Try common editors
	editors := []string{"vim", "vi", "nano", "emacs"}
	for _, editor := range editors {
		if _, err := exec.LookPath(editor); err == nil {
			return editor
		}
	}

	return ""
}
