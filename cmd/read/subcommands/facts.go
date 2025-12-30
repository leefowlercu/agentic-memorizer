package subcommands

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/leefowlercu/agentic-memorizer/internal/integrations"
	"github.com/leefowlercu/agentic-memorizer/pkg/types"
	"github.com/spf13/cobra"
)

var FactsCmd = &cobra.Command{
	Use:   "facts",
	Short: "Read stored facts",
	Long: "\nRead and display all stored facts from the knowledge graph.\n\n" +
		"Facts are user-defined context items that are injected into agent sessions " +
		"via hooks. This command is typically called by UserPromptSubmit or BeforeAgent hooks " +
		"to provide persistent context to the agent.",
	Example: `  # Plain XML output (structured format, default)
  memorizer read facts

  # Plain JSON output (programmatic access)
  memorizer read facts --format json

  # Output wrapped for Claude Code UserPromptSubmit hook
  memorizer read facts --integration claude-code-hook

  # Output wrapped for Gemini CLI BeforeAgent hook
  memorizer read facts --integration gemini-cli-hook`,
	PreRunE: validateReadFacts,
	RunE:    runReadFacts,
}

func init() {
	FactsCmd.Flags().String("format", "xml", "Output format (xml/json)")
	FactsCmd.Flags().String("integration", "", "Wrap output for specific integration (e.g., claude-code-hook)")
}

func validateReadFacts(cmd *cobra.Command, args []string) error {
	// Validate format flag
	formatStr, _ := cmd.Flags().GetString("format")
	if formatStr != "" {
		validFormats := []string{"xml", "json"}
		valid := false
		for _, f := range validFormats {
			if formatStr == f {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid format %q (must be one of: xml, json)", formatStr)
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

func runReadFacts(cmd *cobra.Command, args []string) error {
	if err := config.InitConfig(); err != nil {
		return fmt.Errorf("failed to initialize config; %w", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config; %w", err)
	}

	// Connect to FalkorDB
	ctx := context.Background()
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
		return fmt.Errorf("failed to read facts; FalkorDB is not running. Start it with 'memorizer graph start'")
	}
	defer graphManager.Close()

	// Get flags
	formatStr, _ := cmd.Flags().GetString("format")
	if formatStr == "" {
		formatStr = "xml" // Hardcoded default
	}
	integrationName, _ := cmd.Flags().GetString("integration")

	// Fetch all facts
	facts := graphManager.Facts()
	factNodes, err := facts.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list facts; %w", err)
	}

	// Build FactsIndex
	factsIndex := buildFactsIndex(factNodes)

	// If integration specified, wrap output in integration-specific envelope
	if integrationName != "" {
		return outputFactsForIntegration(factsIndex, integrationName, formatStr)
	}

	return outputFacts(factsIndex, formatStr)
}

func buildFactsIndex(factNodes []graph.FactNode) *types.FactsIndex {
	facts := make([]types.Fact, len(factNodes))
	for i, fn := range factNodes {
		facts[i] = types.Fact{
			ID:        fn.ID,
			Content:   fn.Content,
			CreatedAt: fn.CreatedAt,
			UpdatedAt: fn.UpdatedAt,
			Source:    fn.Source,
		}
	}

	return &types.FactsIndex{
		Generated: time.Now(),
		Facts:     facts,
		Stats: types.FactStats{
			TotalFacts: len(facts),
			MaxFacts:   graph.MaxTotalFacts,
		},
	}
}

// outputFactsForIntegration wraps output in integration-specific envelope
func outputFactsForIntegration(idx *types.FactsIndex, integrationName, formatStr string) error {
	registry := integrations.GlobalRegistry()
	integration, err := registry.Get(integrationName)
	if err != nil {
		return fmt.Errorf("integration %q not found; %w", integrationName, err)
	}

	outputFormat, err := integrations.ParseOutputFormat(formatStr)
	if err != nil {
		return fmt.Errorf("invalid format; %w", err)
	}

	content, err := integration.FormatFactsOutput(idx, outputFormat)
	if err != nil {
		return fmt.Errorf("failed to format output for integration; %w", err)
	}

	fmt.Print(content)
	return nil
}

// outputFacts outputs the FactsIndex format using the format package
func outputFacts(idx *types.FactsIndex, formatStr string) error {
	// Get the appropriate formatter
	formatter, err := format.GetFormatter(formatStr)
	if err != nil {
		return fmt.Errorf("failed to get formatter; %w", err)
	}

	// Wrap the FactsIndex in FactsContent
	factsContent := format.NewFactsContent(idx)

	// Format and output
	content, err := formatter.Format(factsContent)
	if err != nil {
		return fmt.Errorf("failed to format output; %w", err)
	}

	fmt.Print(content)
	return nil
}
