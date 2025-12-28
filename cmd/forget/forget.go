package forget

import (
	"github.com/leefowlercu/agentic-memorizer/cmd/forget/subcommands"
	"github.com/spf13/cobra"
)

var ForgetCmd = &cobra.Command{
	Use:   "forget",
	Short: "Remove items from memory",
	Long: "\nRemove items from memory.\n\n" +
		"Use subcommands to specify what type of item to forget. Currently supported:\n" +
		"- fact: Remove a stored fact by ID",
}

func init() {
	ForgetCmd.AddCommand(subcommands.FactCmd)
}
