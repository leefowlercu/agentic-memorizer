package read

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/export"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
)

// Flag variables
var (
	readFormat   string
	readEnvelope string
	readOutput   string
	readMaxFiles int
	readQuiet    bool
)

// ReadCmd is the read command.
var ReadCmd = &cobra.Command{
	Use:   "read",
	Short: "Export the knowledge graph",
	Long: `Export the knowledge graph in various formats.

The read command exports the memorizer knowledge graph to stdout or a file.
It connects directly to FalkorDB and does not require the daemon to be running.

Available formats:
  xml   - Structured XML format (default)
  json  - Standard JSON format
  toon  - Token-Optimized Notation for LLMs (~40% smaller)

Available envelopes:
  none        - Raw output without wrapping (default)
  claude-code - Claude Code SessionStart hook format
  gemini-cli  - Gemini CLI SessionStart hook format`,
	Example: `  # Export as XML to stdout
  memorizer read

  # Export as JSON to a file
  memorizer read --format json --output graph.json

  # Export in TOON format for Claude Code
  memorizer read --format toon --envelope claude-code

  # Export with file limit
  memorizer read --max-files 100`,
	PreRunE: validateRead,
	RunE:    runRead,
}

func init() {
	ReadCmd.Flags().StringVarP(&readFormat, "format", "f", "xml", "Output format (xml, json, toon)")
	ReadCmd.Flags().StringVarP(&readEnvelope, "envelope", "e", "none", "Envelope wrapper (none, claude-code, gemini-cli)")
	ReadCmd.Flags().StringVarP(&readOutput, "output", "o", "", "Output file (default: stdout)")
	ReadCmd.Flags().IntVar(&readMaxFiles, "max-files", 0, "Maximum number of files to export (0 = unlimited)")
	ReadCmd.Flags().BoolVarP(&readQuiet, "quiet", "q", false, "Suppress statistics output")
}

func validateRead(cmd *cobra.Command, args []string) error {
	// Validate format
	validFormats := map[string]bool{"xml": true, "json": true, "toon": true}
	if !validFormats[readFormat] {
		return fmt.Errorf("invalid format %q; must be one of: xml, json, toon", readFormat)
	}

	// Validate envelope
	validEnvelopes := map[string]bool{"none": true, "claude-code": true, "gemini-cli": true}
	if !validEnvelopes[readEnvelope] {
		return fmt.Errorf("invalid envelope %q; must be one of: none, claude-code, gemini-cli", readEnvelope)
	}

	cmd.SilenceUsage = true
	return nil
}

func runRead(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	cfg := config.Get()

	// Create graph client with typed config
	graphCfg := graph.Config{
		Host:               cfg.Graph.Host,
		Port:               cfg.Graph.Port,
		GraphName:          cfg.Graph.Name,
		PasswordEnv:        cfg.Graph.PasswordEnv,
		MaxRetries:         cfg.Graph.MaxRetries,
		RetryDelay:         time.Duration(cfg.Graph.RetryDelayMs) * time.Millisecond,
		EmbeddingDimension: cfg.Embeddings.Dimensions,
		WriteQueueSize:     cfg.Graph.WriteQueueSize,
	}

	g := graph.NewFalkorDBGraph(graph.WithConfig(graphCfg))

	// Connect to graph
	if err := g.Start(ctx); err != nil {
		return fmt.Errorf("failed to connect to graph database; %w", err)
	}
	defer func() { _ = g.Stop(ctx) }()

	// Create exporter
	exporter := export.NewExporter(g)

	// Build export options
	opts := export.ExportOptions{
		Format:   readFormat,
		Envelope: readEnvelope,
		MaxFiles: readMaxFiles,
	}

	// Perform export
	output, stats, err := exporter.Export(ctx, opts)
	if err != nil {
		return fmt.Errorf("export failed; %w", err)
	}

	// Write output
	if readOutput != "" {
		if err := os.WriteFile(readOutput, output, 0644); err != nil {
			return fmt.Errorf("failed to write output file; %w", err)
		}
		if !readQuiet {
			fmt.Fprintf(os.Stderr, "Exported %d files, %d directories to %s (%d bytes)\n",
				stats.FileCount, stats.DirectoryCount, readOutput, stats.OutputSize)
		}
	} else {
		fmt.Print(string(output))
		if !readQuiet {
			fmt.Fprintf(os.Stderr, "\n# Exported %d files, %d directories (%d bytes) in %v\n",
				stats.FileCount, stats.DirectoryCount, stats.OutputSize, stats.Duration.Round(time.Millisecond))
		}
	}

	return nil
}
