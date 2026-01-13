package config

import (
	"errors"
	"fmt"
	"strings"
)

// ValidationError represents a config validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation failures.
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	if len(e) == 1 {
		return e[0].Error()
	}

	var b strings.Builder
	b.WriteString("config validation failed:\n")
	for _, err := range e {
		b.WriteString("  - ")
		b.WriteString(err.Error())
		b.WriteString("\n")
	}
	return b.String()
}

// validSemanticProviders lists recognized semantic analysis providers.
var validSemanticProviders = map[string]bool{
	"anthropic": true,
	"openai":    true,
	"google":    true,
}

// validEmbeddingsProviders lists recognized embeddings providers.
var validEmbeddingsProviders = map[string]bool{
	"openai": true,
	"google": true,
}

// Validate checks the configuration for errors.
// Returns ValidationErrors if validation fails.
func Validate(cfg *Config) error {
	var errs ValidationErrors

	// Validate daemon config
	if cfg.Daemon.HTTPPort < 1 || cfg.Daemon.HTTPPort > 65535 {
		errs = append(errs, ValidationError{
			Field:   "daemon.http_port",
			Message: fmt.Sprintf("must be between 1 and 65535, got %d", cfg.Daemon.HTTPPort),
		})
	}

	if cfg.Daemon.HTTPBind == "" {
		errs = append(errs, ValidationError{
			Field:   "daemon.http_bind",
			Message: "must not be empty",
		})
	}

	if cfg.Daemon.ShutdownTimeout < 1 {
		errs = append(errs, ValidationError{
			Field:   "daemon.shutdown_timeout",
			Message: fmt.Sprintf("must be at least 1 second, got %d", cfg.Daemon.ShutdownTimeout),
		})
	}

	if cfg.Daemon.PIDFile == "" {
		errs = append(errs, ValidationError{
			Field:   "daemon.pid_file",
			Message: "must not be empty",
		})
	}

	if cfg.Daemon.RegistryPath == "" {
		errs = append(errs, ValidationError{
			Field:   "daemon.registry_path",
			Message: "must not be empty",
		})
	}

	if cfg.Daemon.Metrics.CollectionInterval < 1 {
		errs = append(errs, ValidationError{
			Field:   "daemon.metrics.collection_interval",
			Message: fmt.Sprintf("must be at least 1 second, got %d", cfg.Daemon.Metrics.CollectionInterval),
		})
	}

	// Validate graph config
	if cfg.Graph.Host == "" {
		errs = append(errs, ValidationError{
			Field:   "graph.host",
			Message: "must not be empty",
		})
	}

	if cfg.Graph.Port < 1 || cfg.Graph.Port > 65535 {
		errs = append(errs, ValidationError{
			Field:   "graph.port",
			Message: fmt.Sprintf("must be between 1 and 65535, got %d", cfg.Graph.Port),
		})
	}

	if cfg.Graph.Name == "" {
		errs = append(errs, ValidationError{
			Field:   "graph.name",
			Message: "must not be empty",
		})
	}

	if cfg.Graph.MaxRetries < 0 {
		errs = append(errs, ValidationError{
			Field:   "graph.max_retries",
			Message: fmt.Sprintf("must be non-negative, got %d", cfg.Graph.MaxRetries),
		})
	}

	if cfg.Graph.RetryDelayMs < 0 {
		errs = append(errs, ValidationError{
			Field:   "graph.retry_delay_ms",
			Message: fmt.Sprintf("must be non-negative, got %d", cfg.Graph.RetryDelayMs),
		})
	}

	if cfg.Graph.WriteQueueSize < 1 {
		errs = append(errs, ValidationError{
			Field:   "graph.write_queue_size",
			Message: fmt.Sprintf("must be at least 1, got %d", cfg.Graph.WriteQueueSize),
		})
	}

	// Validate semantic config
	if cfg.Semantic.Provider == "" {
		errs = append(errs, ValidationError{
			Field:   "semantic.provider",
			Message: "must not be empty",
		})
	} else if !validSemanticProviders[cfg.Semantic.Provider] {
		errs = append(errs, ValidationError{
			Field:   "semantic.provider",
			Message: fmt.Sprintf("must be one of: anthropic, openai, google; got %q", cfg.Semantic.Provider),
		})
	}

	if cfg.Semantic.Model == "" {
		errs = append(errs, ValidationError{
			Field:   "semantic.model",
			Message: "must not be empty",
		})
	}

	if cfg.Semantic.RateLimit < 1 {
		errs = append(errs, ValidationError{
			Field:   "semantic.rate_limit",
			Message: fmt.Sprintf("must be at least 1, got %d", cfg.Semantic.RateLimit),
		})
	}

	// Validate embeddings config (only if enabled)
	if cfg.Embeddings.Enabled {
		if cfg.Embeddings.Provider == "" {
			errs = append(errs, ValidationError{
				Field:   "embeddings.provider",
				Message: "must not be empty when embeddings are enabled",
			})
		} else if !validEmbeddingsProviders[cfg.Embeddings.Provider] {
			errs = append(errs, ValidationError{
				Field:   "embeddings.provider",
				Message: fmt.Sprintf("must be one of: openai, google; got %q", cfg.Embeddings.Provider),
			})
		}

		if cfg.Embeddings.Model == "" {
			errs = append(errs, ValidationError{
				Field:   "embeddings.model",
				Message: "must not be empty when embeddings are enabled",
			})
		}

		if cfg.Embeddings.Dimensions < 1 {
			errs = append(errs, ValidationError{
				Field:   "embeddings.dimensions",
				Message: fmt.Sprintf("must be at least 1, got %d", cfg.Embeddings.Dimensions),
			})
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

// IsValidationError checks if an error is a validation error.
func IsValidationError(err error) bool {
	var ve ValidationError
	var ves ValidationErrors
	return errors.As(err, &ve) || errors.As(err, &ves)
}
