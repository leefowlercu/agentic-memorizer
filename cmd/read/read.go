package read

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
	"github.com/spf13/cobra"
)

var ReadCmd = &cobra.Command{
	Use:   "read",
	Short: "Read the memory index",
	Long: "\nRead and display the memory index maintained by the daemon.\n\n" +
		"This command exports the graph-native memory index from FalkorDB and formats it for output. " +
		"The index contains metadata and semantic analysis for all files in your memory directory.\n\n" +
		"The read command is typically called by agent framework hooks (like Claude Code's " +
		"SessionStart hooks) to load the memory index into the agent's context.\n\n" +
		"Uses the graph-native format with flattened FileEntry structures.",
	Example: `  # Plain XML output (structured format)
  memorizer read

  # Plain markdown output (human-readable)
  memorizer read --format markdown

  # Plain JSON output (programmatic access)
  memorizer read --format json

  # Verbose output with insights and related files
  memorizer read -v

  # Output wrapped for Claude Code SessionStart hook
  memorizer read --integration claude-code-hook

  # Output wrapped for Gemini CLI SessionStart hook
  memorizer read --integration gemini-cli-hook`,
	PreRunE: validateRead,
	RunE:    runRead,
}

func init() {
	ReadCmd.Flags().String("format", "xml", "Output format (xml/markdown/json)")
	ReadCmd.Flags().BoolP("verbose", "v", false, "Include related files per entry and graph insights")
	ReadCmd.Flags().String("integration", "", "Wrap output for specific integration (e.g., claude-code-hook)")
}

func validateRead(cmd *cobra.Command, args []string) error {
	// Validate format flag
	formatStr, _ := cmd.Flags().GetString("format")
	if formatStr != "" {
		validFormats := []string{"xml", "markdown", "json"}
		valid := false
		for _, f := range validFormats {
			if formatStr == f {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid format %q (must be one of: xml, markdown, json)", formatStr)
		}
	}

	// Validate integration flag if provided
	integrationName, _ := cmd.Flags().GetString("integration")
	if integrationName != "" {
		registry := integrations.GlobalRegistry()
		available := registry.List()
		if _, err := registry.Get(integrationName); err != nil {
			var names []string
			for _, i := range available {
				names = append(names, i.GetName())
			}
			return fmt.Errorf("integration %q not found (available: %s)", integrationName, strings.Join(names, ", "))
		}
	}

	// All validation passed - errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runRead(cmd *cobra.Command, args []string) error {
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config; %w", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config; %w", err)
	}

	// Connect to FalkorDB
	ctx := context.Background()

	// Use a discarded logger to suppress graph manager initialization logs
	// The read command outputs clean data for consumption by integrations
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	graphConfig := graph.ManagerConfig{
		Client: graph.ClientConfig{
			Host:     cfg.Graph.Host,
			Port:     cfg.Graph.Port,
			Database: cfg.Graph.Database,
			Password: cfg.Graph.Password,
		},
		Schema:     graph.DefaultSchemaConfig(),
		MemoryRoot: cfg.MemoryRoot,
	}

	graphManager := graph.NewManager(graphConfig, logger)
	if err := graphManager.Initialize(ctx); err != nil {
		// Graph not available, show warning with empty index
		return handleEmptyIndex(cmd, cfg)
	}
	defer graphManager.Close()

	// Get flags
	formatStr, _ := cmd.Flags().GetString("format")
	if formatStr == "" {
		formatStr = "xml" // Hardcoded default
	}
	verbose, _ := cmd.Flags().GetBool("verbose")
	integrationName, _ := cmd.Flags().GetString("integration")

	exporter := graph.NewExporter(graphManager, logger)

	// Export graph-native format
	// Verbose mode includes related files per entry and graph insights
	graphIdx, err := exporter.ToGraphIndex(ctx, cfg.MemoryRoot, verbose)
	if err != nil {
		return fmt.Errorf("failed to export graph; %w", err)
	}

	// If integration specified, wrap output in integration-specific envelope
	if integrationName != "" {
		return outputForIntegration(graphIdx, integrationName, formatStr)
	}

	return outputGraph(graphIdx, formatStr)
}

func handleEmptyIndex(cmd *cobra.Command, cfg *config.Config) error {
	emptyIndex := &types.GraphIndex{
		MemoryRoot: cfg.MemoryRoot,
		Files:      []types.FileEntry{},
		Stats:      types.IndexStats{},
	}

	formatStr, _ := cmd.Flags().GetString("format")
	if formatStr == "" {
		formatStr = "xml" // Hardcoded default
	}

	// Warning message for empty index
	warningMessage := `Warning: No data found in FalkorDB graph.

The graph database is empty or not connected. Ensure:

1. FalkorDB is running (e.g., via Docker):
   docker run -p 6379:6379 falkordb/falkordb

2. The daemon is started to populate the graph:
   memorizer daemon start

3. Graph is enabled in config (~/.memorizer/config.yaml):
   graph:
     enabled: true

For now, showing empty index.

`

	fmt.Print(warningMessage)
	return outputGraph(emptyIndex, formatStr)
}

// outputForIntegration wraps output in integration-specific envelope
func outputForIntegration(idx *types.GraphIndex, integrationName, formatStr string) error {
	registry := integrations.GlobalRegistry()
	integration, err := registry.Get(integrationName)
	if err != nil {
		return fmt.Errorf("integration %q not found; %w", integrationName, err)
	}

	format, err := integrations.ParseOutputFormat(formatStr)
	if err != nil {
		return fmt.Errorf("invalid format; %w", err)
	}

	content, err := integration.FormatOutput(idx, format)
	if err != nil {
		return fmt.Errorf("failed to format output for integration; %w", err)
	}

	fmt.Print(content)
	return nil
}

// outputGraph outputs the GraphIndex format using the format package
func outputGraph(idx *types.GraphIndex, formatStr string) error {
	// Get the appropriate formatter
	formatter, err := format.GetFormatter(formatStr)
	if err != nil {
		return fmt.Errorf("failed to get formatter; %w", err)
	}

	// Wrap the GraphIndex in GraphContent
	graphContent := format.NewGraphContent(idx)

	// Format and output
	content, err := formatter.Format(graphContent)
	if err != nil {
		return fmt.Errorf("failed to format output; %w", err)
	}

	fmt.Print(content)
	return nil
}
