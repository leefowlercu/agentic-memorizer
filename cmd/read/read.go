package read

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/index"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	_ "github.com/leefowlercu/agentic-memorizer/internal/integrations/adapters/claude" // Register Claude adapter
	"github.com/leefowlercu/agentic-memorizer/internal/integrations/output"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var ReadCmd = &cobra.Command{
	Use:   "read",
	Short: "Read the memory index",
	Long: `Read and display the memory index maintained by the daemon.

This command loads the precomputed index file and formats it for output. The index
contains metadata and semantic analysis for all files in your memory directory.

The read command is typically called by agent framework hooks (like Claude Code's
SessionStart hooks) to load the memory index into the agent's context.

Performance: ~10-50ms when daemon is running (loads precomputed index)`,
	Example: `  # Plain XML output (structured format)
  agentic-memorizer read

  # Plain markdown output (human-readable)
  agentic-memorizer read --format markdown

  # Plain JSON output (programmatic access)
  agentic-memorizer read --format json

  # Integration-wrapped output for Claude Code
  agentic-memorizer read --format xml --integration claude-code

  # Verbose output with additional details
  agentic-memorizer read --verbose`,
	RunE: runRead,
}

func init() {
	ReadCmd.Flags().String("format", config.DefaultConfig.Output.Format, "Output format (xml/markdown/json)")
	ReadCmd.Flags().String("integration", "", "Format output for specific integration (claude-code, etc)")
	ReadCmd.Flags().Bool("verbose", config.DefaultConfig.Output.Verbose, "Verbose output")

	viper.BindPFlag("output.format", ReadCmd.Flags().Lookup("format"))
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
		// No index exists, show warning with empty index
		return handleEmptyIndex(cmd, cfg)
	}

	// Get format and integration from flags
	formatStr, _ := cmd.Flags().GetString("format")
	if formatStr == "" {
		formatStr = cfg.Output.Format
	}

	integrationName, _ := cmd.Flags().GetString("integration")

	// If integration flag is provided, use integration-specific formatting
	if integrationName != "" {
		return outputForIntegration(integrationName, computed.Index, formatStr)
	}

	// Default: plain output using new output processors
	return outputPlain(computed.Index, formatStr)
}

func handleEmptyIndex(cmd *cobra.Command, cfg *config.Config) error {
	emptyIndex := &types.Index{
		Root:    cfg.MemoryRoot,
		Entries: []types.IndexEntry{},
		Stats:   types.IndexStats{},
	}

	formatStr, _ := cmd.Flags().GetString("format")
	if formatStr == "" {
		formatStr = cfg.Output.Format
	}

	integrationName, _ := cmd.Flags().GetString("integration")

	// Warning message for empty index
	warningMessage := `Warning: No precomputed index found.

The background daemon has not created an index yet. To enable fast startup times:

1. Start the daemon:
   agentic-memorizer daemon start

2. Or enable daemon in config and restart:
   Edit ~/.agentic-memorizer/config.yaml and set:
   daemon:
     enabled: true

For now, showing empty index.

`

	// If integration-specific output requested
	if integrationName != "" {
		// Note: Integration formatters don't include warnings - that's the shell's job
		return outputForIntegration(integrationName, emptyIndex, formatStr)
	}

	// Plain output with warning
	fmt.Print(warningMessage)
	return outputPlain(emptyIndex, formatStr)
}

func outputForIntegration(name string, idx *types.Index, formatStr string) error {
	registry := integrations.GlobalRegistry()
	integration, err := registry.Get(name)
	if err != nil {
		return fmt.Errorf("integration %q not found: %w", name, err)
	}

	// Parse output format
	format, err := integrations.ParseOutputFormat(formatStr)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	// Format output using integration
	output, err := integration.FormatOutput(idx, format)
	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	fmt.Print(output)
	return nil
}

func outputPlain(idx *types.Index, formatStr string) error {
	// Parse output format
	format, err := integrations.ParseOutputFormat(formatStr)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	// Create appropriate processor
	var processor output.OutputProcessor
	switch format {
	case integrations.FormatXML:
		processor = output.NewXMLProcessor()
	case integrations.FormatMarkdown:
		processor = output.NewMarkdownProcessor()
	case integrations.FormatJSON:
		processor = output.NewJSONProcessor()
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	// Format and output
	content, err := processor.Format(idx)
	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	fmt.Print(content)
	return nil
}
