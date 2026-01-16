// Package forget implements the forget command for unregistering directories.
package forget

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/registry"
)

// Flag variables for the forget command.
var (
	forgetKeepData bool
)

// ForgetCmd is the forget command for unregistering directories.
var ForgetCmd = &cobra.Command{
	Use:   "forget <path>",
	Short: "Stop tracking a remembered directory",
	Long: "Stop tracking a remembered directory and optionally clean up associated data.\n\n" +
		"By default, forgetting a directory removes it from the registry and deletes " +
		"all associated file state data. Use --keep-data to preserve the data in the " +
		"knowledge graph while only removing the directory from tracking.",
	Example: `  # Stop tracking a directory (removes associated data)
  memorizer forget ~/projects/old-app

  # Stop tracking but preserve data in the knowledge graph
  memorizer forget ~/projects/archived --keep-data`,
	Args:    cobra.ExactArgs(1),
	PreRunE: validateForget,
	RunE:    runForget,
}

func init() {
	ForgetCmd.Flags().BoolVar(&forgetKeepData, "keep-data", false,
		"Keep data in the knowledge graph (don't delete file states)")
}

func validateForget(cmd *cobra.Command, args []string) error {
	// All validation passed - errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runForget(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	path := args[0]

	// Expand ~ to home directory
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory; %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path; %w", err)
	}
	absPath = filepath.Clean(absPath)

	// Open registry
	registryPath := config.ExpandPath(config.Get().Daemon.RegistryPath)
	reg, err := registry.Open(ctx, registryPath)
	if err != nil {
		return fmt.Errorf("failed to open registry; %w", err)
	}
	defer reg.Close()

	// Check if path exists in registry
	_, err = reg.GetPath(ctx, absPath)
	if err != nil {
		if err == registry.ErrPathNotFound {
			return fmt.Errorf("path is not remembered: %s", absPath)
		}
		return fmt.Errorf("failed to check path; %w", err)
	}

	// Delete file states unless --keep-data is specified
	if !forgetKeepData {
		err = reg.DeleteFileStatesForPath(ctx, absPath)
		if err != nil {
			return fmt.Errorf("failed to delete file states; %w", err)
		}
	}

	// Remove path from registry
	err = reg.RemovePath(ctx, absPath)
	if err != nil {
		return fmt.Errorf("failed to forget path; %w", err)
	}

	if forgetKeepData {
		fmt.Printf("Forgot: %s (data preserved)\n", absPath)
	} else {
		fmt.Printf("Forgot: %s\n", absPath)
	}

	return nil
}
