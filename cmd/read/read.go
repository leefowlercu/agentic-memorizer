package read

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/index"
	"github.com/leefowlercu/agentic-memorizer/internal/output"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var ReadCmd = &cobra.Command{
	Use:   "read",
	Short: "Read the memory index",
	Long: "\nRead and display the memory index maintained by the daemon.\n\n" +
		"This command loads the memory index file and formats it for output. " +
		"It's designed to be used in Claude Code SessionStart hooks for fast index delivery.",
	RunE: runRead,
}

func init() {
	ReadCmd.Flags().String("format", config.DefaultConfig.Output.Format, "Output format (markdown/xml)")
	ReadCmd.Flags().Bool("wrap-json", config.DefaultConfig.Output.WrapJSON, "Wrap output in SessionStart hook JSON")
	ReadCmd.Flags().Bool("verbose", config.DefaultConfig.Output.Verbose, "Verbose output")

	viper.BindPFlag("output.format", ReadCmd.Flags().Lookup("format"))
	viper.BindPFlag("output.wrap_json", ReadCmd.Flags().Lookup("wrap-json"))
	viper.BindPFlag("output.verbose", ReadCmd.Flags().Lookup("verbose"))
}

func runRead(cmd *cobra.Command, args []string) error {
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	indexPath, err := config.GetIndexPath()
	if err != nil {
		return fmt.Errorf("failed to get index path: %w", err)
	}
	indexManager := index.NewManager(indexPath)

	// Try to load the computed index
	computed, err := indexManager.LoadComputed()
	if err != nil {
		// No index exists, create empty index with warning
		emptyIndex := &types.Index{
			Root:    cfg.MemoryRoot,
			Entries: []types.IndexEntry{},
			Stats:   types.IndexStats{},
		}

		formatter := output.NewFormatter(cfg.Output.Verbose, cfg.Output.ShowRecentDays)

		warningMessage := fmt.Sprintf(`Warning: No precomputed index found.

The background daemon has not created an index yet. To enable fast startup times:

1. Start the daemon:
   agentic-memorizer daemon start

2. Or enable daemon in config and restart:
   Edit ~/.agentic-memorizer/config.yaml and set:
   daemon:
     enabled: true

For now, showing empty index.`)

		var content string
		switch cfg.Output.Format {
		case "xml":
			content = formatter.FormatXML(emptyIndex)
		default:
			content = formatter.FormatMarkdown(emptyIndex)
		}

		// Prepend warning
		content = warningMessage + "\n\n" + content

		if cfg.Output.WrapJSON {
			jsonOutput, err := formatter.WrapJSON(content, emptyIndex)
			if err != nil {
				return fmt.Errorf("failed to wrap in JSON: %w", err)
			}
			fmt.Println(jsonOutput)
		} else {
			fmt.Print(content)
		}

		return nil
	}

	// Index exists, format and output
	formatter := output.NewFormatter(cfg.Output.Verbose, cfg.Output.ShowRecentDays)

	var content string
	switch cfg.Output.Format {
	case "xml":
		content = formatter.FormatXML(computed.Index)
	default:
		content = formatter.FormatMarkdown(computed.Index)
	}

	if cfg.Output.WrapJSON {
		jsonOutput, err := formatter.WrapJSON(content, computed.Index)
		if err != nil {
			return fmt.Errorf("failed to wrap in JSON: %w", err)
		}
		fmt.Println(jsonOutput)
	} else {
		fmt.Print(content)
	}

	return nil
}
