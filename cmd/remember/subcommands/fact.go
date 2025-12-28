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

var factID string

var FactCmd = &cobra.Command{
	Use:   "fact <fact-text>",
	Short: "Store a fact in memory",
	Long: "\nStore a user-defined fact in the knowledge graph.\n\n" +
		"Facts are injected into agent contexts via hooks, providing persistent context " +
		"across sessions. Use facts to store preferences, project-specific information, " +
		"or any context you want the agent to remember.\n\n" +
		"Constraints:\n" +
		"- Minimum content length: 10 characters\n" +
		"- Maximum content length: 500 characters\n" +
		"- Maximum total facts: 50\n" +
		"- Duplicate content is rejected",
	Example: `  # Create a new fact
  memorizer remember fact "I prefer Go for backend services"

  # Update an existing fact by ID
  memorizer remember fact "I prefer Go and Rust for backend services" --id abc123

  # Store project-specific context
  memorizer remember fact "This project uses PostgreSQL 15 with pgvector extension"`,
	Args:    cobra.ExactArgs(1),
	PreRunE: validateFact,
	RunE:    runFact,
}

func init() {
	FactCmd.Flags().StringVar(&factID, "id", "", "Fact ID for updating an existing fact (upsert)")
}

func validateFact(cmd *cobra.Command, args []string) error {
	content := args[0]

	// Validate content length
	if len(content) < graph.MinFactContentLength {
		return fmt.Errorf("fact content must be at least %d characters (got %d)",
			graph.MinFactContentLength, len(content))
	}
	if len(content) > graph.MaxFactContentLength {
		return fmt.Errorf("fact content must not exceed %d characters (got %d)",
			graph.MaxFactContentLength, len(content))
	}

	// Validate UUID format if --id provided
	if factID != "" {
		if _, err := uuid.Parse(factID); err != nil {
			return fmt.Errorf("invalid fact id format; expected UUID (got %q)", factID)
		}
	}

	// All validation passed - errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runFact(cmd *cobra.Command, args []string) error {
	content := args[0]

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
		MemoryRoot: cfg.MemoryRoot,
	}

	graphManager := graph.NewManager(graphConfig, logger)
	if err := graphManager.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to store fact; FalkorDB is not running. Start it with 'memorizer graph start'")
	}
	defer graphManager.Close()

	facts := graphManager.Facts()

	// If updating existing fact
	if factID != "" {
		return updateFact(ctx, facts, factID, content)
	}

	// Creating new fact - check for duplicates and count limit
	return createFact(ctx, facts, content)
}

func createFact(ctx context.Context, facts *graph.Facts, content string) error {
	// Check for duplicate content
	existing, err := facts.GetByContent(ctx, content)
	if err == nil && existing != nil {
		return fmt.Errorf("a fact with this content already exists (id: %s)", existing.ID)
	}

	// Check max facts limit
	count, err := facts.Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to check facts count; %w", err)
	}
	if count >= int64(graph.MaxTotalFacts) {
		return fmt.Errorf("maximum number of facts (%d) reached; forget a fact before adding new ones",
			graph.MaxTotalFacts)
	}

	// Create the fact
	fact, err := facts.Create(ctx, content, "cli")
	if err != nil {
		return fmt.Errorf("failed to create fact; %w", err)
	}

	return outputSuccess(fmt.Sprintf("Fact created with ID: %s", fact.ID))
}

func updateFact(ctx context.Context, facts *graph.Facts, id, content string) error {
	// Verify fact exists
	existing, err := facts.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("fact with id %s not found", id)
	}

	// Check for duplicate content (excluding the fact being updated)
	duplicate, err := facts.GetByContent(ctx, content)
	if err == nil && duplicate != nil && duplicate.ID != existing.ID {
		return fmt.Errorf("a fact with this content already exists (id: %s)", duplicate.ID)
	}

	// Update the fact
	_, err = facts.Update(ctx, id, content)
	if err != nil {
		return fmt.Errorf("failed to update fact; %w", err)
	}

	return outputSuccess(fmt.Sprintf("Fact updated: %s", id))
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
