package cache

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/cmd/cache/subcommands"
	"github.com/spf13/cobra"
)

var CacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the semantic analysis cache",
	Long: "\nManage the semantic analysis cache that stores AI-generated file summaries and metadata.\n\n" +
		"The cache uses content-addressable storage with SHA-256 hashes, enabling cache hits " +
		"across file renames and automatic invalidation on content changes. Cache entries are " +
		"versioned to detect stale entries after application upgrades.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("please specify a subcommand: status or clear")
	},
}

func init() {
	CacheCmd.AddCommand(subcommands.StatusCmd)
	CacheCmd.AddCommand(subcommands.ClearCmd)
}
