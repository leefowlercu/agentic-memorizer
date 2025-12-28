package read

import (
	"github.com/leefowlercu/agentic-memorizer/cmd/read/subcommands"
	"github.com/spf13/cobra"
)

var ReadCmd = &cobra.Command{
	Use:   "read",
	Short: "Read from memory",
	Long: "\nRead and display data from the knowledge graph.\n\n" +
		"Use subcommands to specify what to read:\n" +
		"- files: Read the file memory index\n" +
		"- facts: Read stored user-defined facts",
}

func init() {
	ReadCmd.AddCommand(subcommands.FilesCmd)
	ReadCmd.AddCommand(subcommands.FactsCmd)
}
