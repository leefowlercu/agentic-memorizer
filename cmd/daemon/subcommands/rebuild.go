package subcommands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/daemonclient"
)

var (
	rebuildFull    bool
	rebuildVerbose bool
)

// RebuildCmd triggers a rebuild of the knowledge graph.
var RebuildCmd = &cobra.Command{
	Use:   "rebuild",
	Short: "Rebuild the knowledge graph from remembered directories",
	Long: "Rebuild the knowledge graph from remembered directories.\n\n" +
		"This command triggers the daemon to re-walk all remembered directories and " +
		"update the knowledge graph. By default, it performs an incremental rebuild " +
		"that only processes files that have changed since the last analysis. Use " +
		"--full to force a complete rebuild of all files.",
	Example: `  # Incremental rebuild (only changed files)
  memorizer daemon rebuild

  # Full rebuild of all files
  memorizer daemon rebuild --full

  # Full rebuild with progress output
  memorizer daemon rebuild --full --verbose`,
	PreRunE: validateRebuild,
	RunE:    runRebuild,
}

func init() {
	RebuildCmd.Flags().BoolVar(&rebuildFull, "full", false, "Force full rebuild of all files")
	RebuildCmd.Flags().BoolVar(&rebuildVerbose, "verbose", false, "Show progress output")
}

func validateRebuild(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runRebuild(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()
	quiet := isQuiet(cmd)

	client, err := daemonclient.NewFromConfig(config.Get(),
		daemonclient.WithTimeout(daemonclient.RebuildTimeout),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize daemon client; %w", err)
	}

	if rebuildVerbose && !quiet {
		fmt.Fprintf(out, "Triggering %s rebuild...\n", rebuildType())
	}

	result, err := client.Rebuild(context.Background(), rebuildFull)
	if err != nil {
		return fmt.Errorf("rebuild failed; %w", err)
	}

	// Output result
	if !quiet {
		if rebuildVerbose {
			fmt.Fprintf(out, "Rebuild completed:\n")
			fmt.Fprintf(out, "  Status: %s\n", result.Status)
			fmt.Fprintf(out, "  Directories processed: %d\n", result.DirsProcessed)
			fmt.Fprintf(out, "  Files queued for analysis: %d\n", result.FilesQueued)
			fmt.Fprintf(out, "  Duration: %s\n", result.Duration)
		} else {
			fmt.Fprintf(out, "Rebuild %s: %d files queued\n", result.Status, result.FilesQueued)
		}
	}

	return nil
}

func rebuildType() string {
	if rebuildFull {
		return "full"
	}
	return "incremental"
}
