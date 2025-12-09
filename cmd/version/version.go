package version

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/internal/version"
	"github.com/spf13/cobra"
)

var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information",
	Long: "\nDisplay version information for the memorizer binary.\n\n" +
		"Shows the semantic version number, git commit hash, and build timestamp. " +
		"Version information is injected at build time using ldflags when building " +
		"with 'make build-release' or 'make install-release'.",
	Example: `  # Display version information
  memorizer version`,
	PreRunE: validateVersion,
	RunE:    runVersion,
}

func validateVersion(cmd *cobra.Command, args []string) error {
	// No validation needed for version command
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runVersion(cmd *cobra.Command, args []string) error {
	PrintVersion()
	return nil
}

// PrintVersion outputs version information in a detailed multi-line format
func PrintVersion() {
	section := format.NewSection("Version Information").AddDivider()
	section.AddKeyValue("Version", version.GetShortVersion())
	section.AddKeyValue("Commit", version.GetGitCommit())
	section.AddKeyValue("Built", version.GetBuildDate())

	formatter, err := format.GetFormatter("text")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	output, err := formatter.Format(section)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println(output)
}
