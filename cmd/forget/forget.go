// Package forget implements the forget command for unregistering directories.
package forget

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/internal/cmdutil"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/daemon"
	"github.com/leefowlercu/agentic-memorizer/internal/daemonclient"
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
	out := cmd.OutOrStdout()
	absPath, err := cmdutil.ResolvePath(args[0])
	if err != nil {
		return fmt.Errorf("failed to resolve path; %w", err)
	}

	client, err := daemonclient.NewFromConfig(config.Get())
	if err != nil {
		return fmt.Errorf("failed to initialize daemon client; %w", err)
	}

	_, err = client.Forget(ctx, daemon.ForgetRequest{
		Path:     absPath,
		KeepData: forgetKeepData,
	})
	if err != nil {
		return fmt.Errorf("forget request failed; %w", err)
	}

	if forgetKeepData {
		fmt.Fprintf(out, "Forgot: %s (data preserved)\n", absPath)
	} else {
		fmt.Fprintf(out, "Forgot: %s\n", absPath)
	}

	return nil
}
