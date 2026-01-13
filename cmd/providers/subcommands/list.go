package subcommands

import (
	"fmt"

	"github.com/leefowlercu/agentic-memorizer/internal/providers"
	"github.com/leefowlercu/agentic-memorizer/internal/providers/embeddings"
	"github.com/leefowlercu/agentic-memorizer/internal/providers/semantic"
	"github.com/spf13/cobra"
)

var (
	listVerbose bool
)

// ListCmd lists available AI providers.
var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available AI providers",
	Long: "List available AI providers.\n\n" +
		"Displays all registered semantic analysis and embeddings providers. " +
		"Use --verbose to see detailed configuration for each provider.",
	Example: `  # List all providers
  memorizer providers list

  # List with detailed information
  memorizer providers list --verbose`,
	PreRunE: validateList,
	RunE:    runList,
}

func init() {
	ListCmd.Flags().BoolVarP(&listVerbose, "verbose", "v", false, "Show detailed provider information")
}

func validateList(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runList(cmd *cobra.Command, args []string) error {
	// Create a registry and register all known providers
	registry := providers.NewRegistry()
	registerAllProviders(registry)

	// Get providers
	semanticProviders := registry.ListSemantic()
	embeddingsProviders := registry.ListEmbeddings()

	// Print semantic providers
	fmt.Println("Semantic Providers:")
	if len(semanticProviders) == 0 {
		fmt.Println("  (none registered)")
	} else {
		for _, p := range semanticProviders {
			printSemanticProvider(p, listVerbose)
		}
	}

	fmt.Println()

	// Print embeddings providers
	fmt.Println("Embeddings Providers:")
	if len(embeddingsProviders) == 0 {
		fmt.Println("  (none registered)")
	} else {
		for _, p := range embeddingsProviders {
			printEmbeddingsProvider(p, listVerbose)
		}
	}

	return nil
}

func printSemanticProvider(p providers.SemanticProvider, verbose bool) {
	status := "unavailable"
	if p.Available() {
		status = "available"
	}

	if verbose {
		fmt.Printf("  %s:\n", p.Name())
		fmt.Printf("    Status: %s\n", status)
		fmt.Printf("    Supports Vision: %v\n", p.SupportsVision())
		fmt.Printf("    Max Content Size: %d bytes\n", p.MaxContentSize())
		rateLimit := p.RateLimit()
		fmt.Printf("    Rate Limit: %d req/min, %d tokens/min\n",
			rateLimit.RequestsPerMinute, rateLimit.TokensPerMinute)
	} else {
		fmt.Printf("  %s (%s)\n", p.Name(), status)
	}
}

func printEmbeddingsProvider(p providers.EmbeddingsProvider, verbose bool) {
	status := "unavailable"
	if p.Available() {
		status = "available"
	}

	if verbose {
		fmt.Printf("  %s:\n", p.Name())
		fmt.Printf("    Status: %s\n", status)
		fmt.Printf("    Model: %s\n", p.ModelName())
		fmt.Printf("    Dimensions: %d\n", p.Dimensions())
		fmt.Printf("    Max Tokens: %d\n", p.MaxTokens())
		rateLimit := p.RateLimit()
		fmt.Printf("    Rate Limit: %d req/min, %d tokens/min\n",
			rateLimit.RequestsPerMinute, rateLimit.TokensPerMinute)
	} else {
		fmt.Printf("  %s (%s) - %d dimensions\n", p.Name(), status, p.Dimensions())
	}
}

// registerAllProviders registers all known providers with the registry.
func registerAllProviders(registry *providers.Registry) {
	// Register semantic providers
	registry.RegisterSemantic(semantic.NewAnthropicProvider())
	registry.RegisterSemantic(semantic.NewOpenAISemanticProvider())
	registry.RegisterSemantic(semantic.NewGoogleSemanticProvider())

	// Register embeddings providers
	registry.RegisterEmbeddings(embeddings.NewOpenAIEmbeddingsProvider())
	registry.RegisterEmbeddings(embeddings.NewVoyageEmbeddingsProvider())
	registry.RegisterEmbeddings(embeddings.NewGoogleEmbeddingsProvider())
}
