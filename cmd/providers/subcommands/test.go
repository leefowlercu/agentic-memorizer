package subcommands

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/leefowlercu/agentic-memorizer/internal/providers"
)

// TestCmd tests connectivity to an AI provider.
var TestCmd = &cobra.Command{
	Use:   "test <provider-name>",
	Short: "Test connectivity to an AI provider",
	Long: "Test connectivity to an AI provider.\n\n" +
		"Verifies that the specified provider is properly configured and can " +
		"communicate with its API. This sends a minimal test request to confirm " +
		"authentication and connectivity.",
	Example: `  # Test Anthropic semantic provider
  memorizer providers test anthropic

  # Test OpenAI embeddings provider
  memorizer providers test openai-embeddings`,
	Args:    cobra.ExactArgs(1),
	PreRunE: validateTest,
	RunE:    runTest,
}

func validateTest(cmd *cobra.Command, args []string) error {
	// All errors after this are runtime errors
	cmd.SilenceUsage = true
	return nil
}

func runTest(cmd *cobra.Command, args []string) error {
	providerName := args[0]

	// Create a registry and register all known providers
	registry := providers.NewRegistry()
	registerAllProviders(registry)

	// Try to find the provider
	if sp, err := registry.GetSemantic(providerName); err == nil {
		return testSemanticProvider(sp)
	}

	if ep, err := registry.GetEmbeddings(providerName); err == nil {
		return testEmbeddingsProvider(ep)
	}

	return fmt.Errorf("provider %q not found", providerName)
}

func testSemanticProvider(p providers.SemanticProvider) error {
	fmt.Printf("Testing semantic provider: %s\n", p.Name())

	if !p.Available() {
		return fmt.Errorf("provider %s is not available (missing API key or configuration)", p.Name())
	}

	fmt.Println("  Status: available")
	fmt.Println("  Sending test request...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send a minimal test request
	req := providers.SemanticInput{
		Path:     "/test/file.txt",
		MIMEType: "text/plain",
		Type:     providers.SemanticInputText,
		Text:     "This is a test file for connectivity verification.",
	}

	start := time.Now()
	result, err := p.Analyze(ctx, req)
	duration := time.Since(start)

	if err != nil {
		return fmt.Errorf("test failed; %w", err)
	}

	fmt.Printf("  Response received in %v\n", duration.Round(time.Millisecond))
	fmt.Printf("  Tokens used: %d\n", result.TokensUsed)
	fmt.Printf("  Summary: %s\n", truncate(result.Summary, 80))
	fmt.Println("  Test: PASSED")

	return nil
}

func testEmbeddingsProvider(p providers.EmbeddingsProvider) error {
	fmt.Printf("Testing embeddings provider: %s\n", p.Name())

	if !p.Available() {
		return fmt.Errorf("provider %s is not available (missing API key or configuration)", p.Name())
	}

	fmt.Println("  Status: available")
	fmt.Printf("  Model: %s (%d dimensions)\n", p.ModelName(), p.Dimensions())
	fmt.Println("  Sending test request...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send a minimal test request
	req := providers.EmbeddingsRequest{
		Content:     "This is a test sentence for connectivity verification.",
		ChunkID:     "test-chunk",
		ContentHash: "test-hash",
	}

	start := time.Now()
	result, err := p.Embed(ctx, req)
	duration := time.Since(start)

	if err != nil {
		return fmt.Errorf("test failed; %w", err)
	}

	fmt.Printf("  Response received in %v\n", duration.Round(time.Millisecond))
	fmt.Printf("  Embedding dimensions: %d\n", result.Dimensions)
	fmt.Printf("  Tokens used: %d\n", result.TokensUsed)
	fmt.Println("  Test: PASSED")

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
