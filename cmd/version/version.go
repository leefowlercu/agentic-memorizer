package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/internal/version"
)

// VersionCmd displays version and build information.
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version and build information",
	Long: "Display version and build information.\n\n" +
		"Shows the semantic version, git commit hash, and build date " +
		"of the current memorizer binary. This information is useful " +
		"for troubleshooting and verifying the installed version.",
	Example: `  # Display version information
  memorizer version`,
	PreRunE: validateVersion,
	RunE:    runVersion,
}

func validateVersion(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	return nil
}

func runVersion(cmd *cobra.Command, args []string) error {
	info := version.Get()
	fmt.Fprintln(cmd.OutOrStdout(), info.String())
	return nil
}
