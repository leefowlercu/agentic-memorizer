package subcommands

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

var (
	readFilesFormat      string
	readFilesVerbose     bool
	readFilesIntegration string
)

var FilesCmd = &cobra.Command{
	Use:   "files",
	Short: "Read the file memory index",
	Long: "\nRead and display the file memory index maintained by the daemon.\n\n" +
		"This command exports the graph-native memory index from FalkorDB and formats it for output. " +
		"The index contains metadata and semantic analysis for all files in your memory directory.\n\n" +
		"The read files command is typically called by agent framework hooks (like Claude Code's " +
		"SessionStart hooks) to load the memory index into the agent's context.\n\n" +
		"Uses the graph-native format with flattened FileEntry structures.",
	Example: `  # Plain XML output (structured format, default)
  memorizer read files

  # Plain JSON output (programmatic access)
  memorizer read files --format json

  # Verbose output with insights and related files
  memorizer read files -v

  # Output wrapped for Claude Code SessionStart hook
  memorizer read files --integration claude-code-hook

  # Output wrapped for Gemini CLI SessionStart hook
  memorizer read files --integration gemini-cli-hook`,
	PreRunE: validateReadFiles,
	RunE:    runReadFiles,
}

func init() {
	FilesCmd.Flags().StringVar(&readFilesFormat, "format", "xml", "Output format (xml/json)")
	FilesCmd.Flags().BoolVarP(&readFilesVerbose, "verbose", "v", false, "Include related files per entry and graph insights")
	FilesCmd.Flags().StringVar(&readFilesIntegration, "integration", "", "Wrap output for specific integration (e.g., claude-code-hook)")
}

func validateReadFiles(cmd *cobra.Command, args []string) error {
	// Validate format flag
	if readFilesFormat != "" {
		validFormats := []string{"xml", "json"}
		valid := false
		for _, f := range validFormats {
			if readFilesFormat == f {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid format %q (must be one of: xml, json)", readFilesFormat)
		}
	}

	// Validate integration flag if provided
	if readFilesIntegration != "" {
		registry := integrations.GlobalRegistry()
		available := registry.List()
		if _, err := registry.Get(readFilesIntegration); err != nil {
			var names []string
			for _, i := range available {
				names = append(names, i.GetName())
			}
			return fmt.Errorf("integration %q not found (available: %s)", readFilesIntegration, strings.Join(names, ", "))
		}
	}

	// All validation passed - errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runReadFiles(cmd *cobra.Command, args []string) error {
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
		MemoryRoot: cfg.Memory.Root,
	}

	graphManager := graph.NewManager(graphConfig, logger)
	if err := graphManager.Initialize(ctx); err != nil {
		// Graph not available, show warning with empty index
		return handleEmptyFilesIndex(cmd, cfg)
	}
	defer graphManager.Close()

	// Get flags
	formatStr := readFilesFormat
	if formatStr == "" {
		formatStr = "xml" // Hardcoded default
	}
	verbose := readFilesVerbose
	integrationName := readFilesIntegration

	exporter := graph.NewExporter(graphManager, logger)

	// Export file index format
	// Verbose mode includes related files per entry and graph insights
	fileIdx, err := exporter.ToFileIndex(ctx, cfg.Memory.Root, verbose)
	if err != nil {
		return fmt.Errorf("failed to export file index; %w", err)
	}

	// If integration specified, wrap output in integration-specific envelope
	if integrationName != "" {
		return outputFilesForIntegration(fileIdx, integrationName, formatStr)
	}

	return outputFiles(fileIdx, formatStr)
}

func handleEmptyFilesIndex(cmd *cobra.Command, cfg *config.Config) error {
	emptyIndex := &types.FileIndex{
		MemoryRoot: cfg.Memory.Root,
		Files:      []types.FileEntry{},
		Stats:      types.IndexStats{},
	}

	formatStr := readFilesFormat
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
	return outputFiles(emptyIndex, formatStr)
}

// outputFilesForIntegration wraps output in integration-specific envelope
func outputFilesForIntegration(idx *types.FileIndex, integrationName, formatStr string) error {
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

// outputFiles outputs the FileIndex format using the format package
func outputFiles(idx *types.FileIndex, formatStr string) error {
	// Get the appropriate formatter
	formatter, err := format.GetFormatter(formatStr)
	if err != nil {
		return fmt.Errorf("failed to get formatter; %w", err)
	}

	// Wrap the FileIndex in FilesContent
	filesContent := format.NewFilesContent(idx)

	// Format and output
	content, err := formatter.Format(filesContent)
	if err != nil {
		return fmt.Errorf("failed to format output; %w", err)
	}

	fmt.Print(content)
	return nil
}
