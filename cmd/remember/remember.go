package remember

import (
	"github.com/leefowlercu/agentic-memorizer/cmd/remember/subcommands"
	"github.com/spf13/cobra"
)

var RememberCmd = &cobra.Command{
	Use:   "remember",
	Short: "Add items to memory",
	Long: "\nAdd items to memory for the agent to remember.\n\n" +
		"Use subcommands to specify what type of item to remember:\n" +
		"- file: Copy files or directories into the memory directory\n" +
		"- fact: Store a user-defined fact for agent context",
}

func init() {
	RememberCmd.AddCommand(subcommands.FileCmd)
	RememberCmd.AddCommand(subcommands.FactCmd)
}
