package initialize

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Valid semantic providers (from SPEC.md).
var validSemanticProviders = map[string]bool{
	"anthropic": true,
	"openai":    true,
	"google":    true,
}

// Valid embeddings providers (from SPEC.md).
var validEmbeddingsProviders = map[string]bool{
	"openai": true,
	"voyage": true,
	"google": true,
}

// validateUnattendedFlags validates flag combinations for unattended mode.
// This should be called in PreRunE to show usage on validation errors.
func validateUnattendedFlags(cmd *cobra.Command) error {
	// Check for --no-semantic conflicts
	if err := validateNoSemanticConflict(cmd); err != nil {
		return err
	}

	// FR-007: Check for --no-embeddings conflicts
	if err := validateNoEmbeddingsConflict(cmd); err != nil {
		return err
	}

	// Validate provider values if specified
	if err := validateProviderFlags(cmd); err != nil {
		return err
	}

	// Validate port values if specified
	if err := validatePortFlags(cmd); err != nil {
		return err
	}

	// Validate output format
	if err := validateOutputFlag(cmd); err != nil {
		return err
	}

	return nil
}

// validateNoSemanticConflict checks for --no-semantic conflicts.
func validateNoSemanticConflict(cmd *cobra.Command) error {
	if !cmd.Flags().Changed("no-semantic") || !initializeNoSemantic {
		return nil
	}

	conflictingFlags := []string{
		"semantic-provider",
		"semantic-model",
		"semantic-api-key",
	}

	for _, flag := range conflictingFlags {
		if cmd.Flags().Changed(flag) {
			return fmt.Errorf("cannot combine --no-semantic with --%s", flag)
		}
	}

	return nil
}

// validateNoEmbeddingsConflict checks FR-007: --no-embeddings conflicts.
func validateNoEmbeddingsConflict(cmd *cobra.Command) error {
	if !cmd.Flags().Changed("no-embeddings") || !initializeNoEmbeddings {
		return nil
	}

	// Check for conflicting embeddings flags
	conflictingFlags := []string{
		"embeddings-provider",
		"embeddings-model",
		"embeddings-api-key",
	}

	for _, flag := range conflictingFlags {
		if cmd.Flags().Changed(flag) {
			return fmt.Errorf("cannot combine --no-embeddings with --%s", flag)
		}
	}

	return nil
}

// validateProviderFlags validates provider flag values.
func validateProviderFlags(cmd *cobra.Command) error {
	// Validate semantic provider
	if cmd.Flags().Changed("semantic-provider") {
		if err := validateProvider(initializeSemanticProvider, validSemanticProviders, "semantic-provider"); err != nil {
			return err
		}
	}

	// Validate embeddings provider
	if cmd.Flags().Changed("embeddings-provider") {
		if err := validateProvider(initializeEmbeddingsProvider, validEmbeddingsProviders, "embeddings-provider"); err != nil {
			return err
		}
	}

	return nil
}

// validateProvider checks if a provider value is valid.
func validateProvider(provider string, validProviders map[string]bool, flagName string) error {
	if !validProviders[provider] {
		var validList []string
		for p := range validProviders {
			validList = append(validList, p)
		}
		return fmt.Errorf("--%s must be one of: %v; got %q", flagName, validList, provider)
	}
	return nil
}

// validatePortFlags validates port flag values.
func validatePortFlags(cmd *cobra.Command) error {
	// Validate graph port
	if cmd.Flags().Changed("graph-port") {
		if err := validatePort(initializeGraphPort, "graph-port"); err != nil {
			return err
		}
	}

	// Validate HTTP port
	if cmd.Flags().Changed("http-port") {
		if err := validatePort(initializeHTTPPort, "http-port"); err != nil {
			return err
		}
	}

	return nil
}

// validatePort checks if a port value is in valid range.
func validatePort(port int, flagName string) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("--%s must be between 1 and 65535; got %d", flagName, port)
	}
	return nil
}

// validateOutputFlag validates the output format flag.
func validateOutputFlag(cmd *cobra.Command) error {
	if cmd.Flags().Changed("output") {
		if initializeOutput != "text" && initializeOutput != "json" {
			return fmt.Errorf("--output must be 'text' or 'json'; got %q", initializeOutput)
		}
	}
	return nil
}

// validateRequiredAPIKeys validates that required API keys are present.
// This is called after resolution, not in PreRunE.
func validateRequiredAPIKeys(resolved *UnattendedConfig) error {
	// FR-006: semantic.api_key is required
	if resolved.SemanticEnabled && resolved.SemanticAPIKey == "" {
		return fmt.Errorf("semantic API key is required; provide via --semantic-api-key flag or %s environment variable",
			semanticProviderEnvVars[resolved.SemanticProvider])
	}

	// FR-006: embeddings.api_key is required when embeddings are enabled
	if resolved.EmbeddingsEnabled && resolved.EmbeddingsAPIKey == "" {
		return fmt.Errorf("embeddings API key is required when embeddings are enabled; provide via --embeddings-api-key flag or %s environment variable",
			embeddingsProviderEnvVars[resolved.EmbeddingsProvider])
	}

	return nil
}
