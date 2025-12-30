package subcommands

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/google/uuid"
	"github.com/leefowlercu/agentic-memorizer/internal/config"
	"github.com/leefowlercu/agentic-memorizer/internal/format"
	"github.com/leefowlercu/agentic-memorizer/internal/graph"
	"github.com/spf13/cobra"
)

var FactCmd = &cobra.Command{
	Use:   "fact <fact-id>",
	Short: "Remove a fact from memory",
	Long: "\nRemove a stored fact from the knowledge graph by its UUID.\n\n" +
		"Facts are permanently deleted and cannot be recovered. " +
		"Use 'memorizer read facts' to list all facts and their IDs.",
	Example: `  # Remove a fact by ID
  memorizer forget fact abc12345-6789-0abc-def0-123456789abc

  # List facts first to find IDs
  memorizer read facts`,
	Args:    cobra.ExactArgs(1),
	PreRunE: validateForgetFact,
	RunE:    runForgetFact,
}

func validateForgetFact(cmd *cobra.Command, args []string) error {
	factID := args[0]

	// Validate UUID format
	if _, err := uuid.Parse(factID); err != nil {
		return fmt.Errorf("invalid fact id format; expected UUID (got %q)", factID)
	}

	// All validation passed - errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runForgetFact(cmd *cobra.Command, args []string) error {
	factID := args[0]

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
		return fmt.Errorf("failed to delete fact; FalkorDB is not running. Start it with 'memorizer graph start'")
	}
	defer graphManager.Close()

	facts := graphManager.Facts()

	// Verify fact exists
	existing, err := facts.GetByID(ctx, factID)
	if err != nil || existing == nil {
		return fmt.Errorf("fact with id %s not found", factID)
	}

	// Delete the fact
	if err := facts.Delete(ctx, factID); err != nil {
		return fmt.Errorf("failed to delete fact; %w", err)
	}

	return outputSuccess(fmt.Sprintf("Fact deleted: %s", factID))
}

func outputSuccess(message string) error {
	status := format.NewStatus(format.StatusSuccess, message)

	formatter, err := format.GetFormatter("text")
	if err != nil {
		return fmt.Errorf("failed to get formatter; %w", err)
	}

	output, err := formatter.Format(status)
	if err != nil {
		return fmt.Errorf("failed to format output; %w", err)
	}

	fmt.Println(output)
	return nil
}
