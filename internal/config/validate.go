package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field      string
	Value      any
	Rule       string
	Message    string
	Suggestion string
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	msg := fmt.Sprintf("validation error: %s", e.Message)
	if e.Field != "" {
		msg = fmt.Sprintf("validation error in field '%s': %s", e.Field, e.Message)
	}
	if e.Suggestion != "" {
		msg += fmt.Sprintf("\nSuggestion: %s", e.Suggestion)
	}
	return msg
}

// Validator accumulates validation errors
type Validator struct {
	Errors []ValidationError
}

// AddError adds a validation error
func (v *Validator) AddError(field, rule, message, suggestion string, value any) {
	v.Errors = append(v.Errors, ValidationError{
		Field:      field,
		Value:      value,
		Rule:       rule,
		Message:    message,
		Suggestion: suggestion,
	})
}

// HasErrors returns true if there are validation errors
func (v *Validator) HasErrors() bool {
	return len(v.Errors) > 0
}

// Error returns a formatted error message with all validation errors
func (v *Validator) Error() string {
	if !v.HasErrors() {
		return ""
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("configuration validation failed with %d error(s):\n\n", len(v.Errors)))

	for i, err := range v.Errors {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, err.Error()))
		if i < len(v.Errors)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// checkDeprecatedKeys logs warnings for deprecated configuration keys
func checkDeprecatedKeys() {
	// Check for deprecated analysis.parallel key
	if viper.IsSet("analysis.parallel") {
		parallelValue := viper.GetInt("analysis.parallel")
		slog.Warn(
			"deprecated configuration key detected",
			"key", "analysis.parallel",
			"value", parallelValue,
			"message", "analysis.parallel is no longer used and has been removed",
			"suggestion", "use daemon.workers (default: 3) to control parallel processing",
		)
	}

	// Check for removed output section
	if viper.IsSet("output.format") || viper.IsSet("output.show_recent_days") {
		slog.Warn(
			"deprecated configuration key detected",
			"key", "output",
			"message", "output section has been removed; format is now a CLI flag only",
			"suggestion", "use --format flag with the 'read' command instead",
		)
	}

	// Check for removed claude settings
	if viper.IsSet("claude.api_key_env") {
		slog.Warn(
			"deprecated configuration key detected",
			"key", "claude.api_key_env",
			"message", "claude.api_key_env has been removed; ANTHROPIC_API_KEY is now the hardcoded env var",
			"suggestion", "set ANTHROPIC_API_KEY environment variable or use claude.api_key directly",
		)
	}

	// Check for removed mcp.daemon_url
	if viper.IsSet("mcp.daemon_url") {
		slog.Warn(
			"deprecated configuration key detected",
			"key", "mcp.daemon_url",
			"message", "mcp.daemon_url has been replaced with mcp.daemon_host and mcp.daemon_port",
			"suggestion", "use mcp.daemon_host and mcp.daemon_port instead",
		)
	}
}

// ValidateConfig validates the complete configuration
func ValidateConfig(cfg *Config) error {
	v := &Validator{}

	// Check for deprecated keys and log warnings
	checkDeprecatedKeys()

	// Phase 1: Basic field validation
	validateMemoryRoot(v, cfg)
	validateClaude(v, cfg)
	validateAnalysis(v, cfg)
	validateDaemon(v, cfg)
	validateMCP(v, cfg)
	validateGraph(v, cfg)
	validateEmbeddings(v, cfg)

	if v.HasErrors() {
		return v
	}

	return nil
}

// validateMemoryRoot validates the memory root directory
func validateMemoryRoot(v *Validator, cfg *Config) {
	if cfg.MemoryRoot == "" {
		v.AddError("memory_root", "required", "memory_root is required", "Set memory_root to a valid directory path", nil)
		return
	}

	// Expand home directory
	expandedPath := ExpandHome(cfg.MemoryRoot)

	// Check if path is safe (no parent directory traversal)
	if strings.Contains(cfg.MemoryRoot, "..") {
		v.AddError("memory_root", "security", "memory_root contains parent directory references (..)", "Use an absolute path or home-relative path without '..'", cfg.MemoryRoot)
		return
	}

	// Check if directory exists (warn only, don't fail - init creates it)
	if stat, err := os.Stat(expandedPath); err != nil {
		if !os.IsNotExist(err) {
			v.AddError("memory_root", "access", fmt.Sprintf("cannot access memory_root: %v", err), "Check file permissions", cfg.MemoryRoot)
		}
		// Directory doesn't exist is OK - init will create it
	} else if !stat.IsDir() {
		v.AddError("memory_root", "type", "memory_root exists but is not a directory", "Specify a directory path, not a file", cfg.MemoryRoot)
	}
}

// validateClaude validates Claude API configuration
func validateClaude(v *Validator, cfg *Config) {
	// API key validation - note: key may come from hardcoded env var (ANTHROPIC_API_KEY)
	// We don't error here since analysis.enabled is derived from API key presence
	// and the daemon will simply skip semantic analysis if no key is set

	// Validate model name (basic check - just ensure it's not empty)
	if cfg.Claude.Model == "" {
		v.AddError("claude.model", "required", "claude.model is required", "Set to a valid Claude model (e.g., claude-sonnet-4-5-20250929)", nil)
	}

	// Validate max_tokens range
	if cfg.Claude.MaxTokens < 1 || cfg.Claude.MaxTokens > 8192 {
		v.AddError("claude.max_tokens", "range", fmt.Sprintf("max_tokens %d is out of valid range (1-8192)", cfg.Claude.MaxTokens), "Set max_tokens between 1 and 8192", cfg.Claude.MaxTokens)
	}

	// Validate timeout range (5-300 seconds)
	if cfg.Claude.Timeout < 5 || cfg.Claude.Timeout > 300 {
		v.AddError("claude.timeout", "range", fmt.Sprintf("timeout %d is out of valid range (5-300 seconds)", cfg.Claude.Timeout), "Set timeout between 5 and 300 seconds", cfg.Claude.Timeout)
	}
}

// validateAnalysis validates analysis configuration
func validateAnalysis(v *Validator, cfg *Config) {
	// Validate max_file_size
	if cfg.Analysis.MaxFileSize < 0 {
		v.AddError("analysis.max_file_size", "range", "max_file_size cannot be negative", "Set max_file_size to a positive number or 0 for unlimited", cfg.Analysis.MaxFileSize)
	}

	// Validate cache dir
	if cfg.Analysis.CacheDir == "" {
		v.AddError("analysis.cache_dir", "required", "cache_dir is required", "Set cache_dir to a valid directory path", nil)
	} else if strings.Contains(cfg.Analysis.CacheDir, "..") {
		v.AddError("analysis.cache_dir", "security", "cache_dir contains parent directory references (..)", "Use an absolute path or home-relative path without '..'", cfg.Analysis.CacheDir)
	}
}

// validateDaemon validates daemon configuration
func validateDaemon(v *Validator, cfg *Config) {
	// Validate debounce
	if cfg.Daemon.DebounceMs < 0 || cfg.Daemon.DebounceMs > 10000 {
		v.AddError("daemon.debounce_ms", "range", fmt.Sprintf("debounce_ms %d is out of valid range (0-10000)", cfg.Daemon.DebounceMs), "Set debounce_ms between 0 and 10000 milliseconds", cfg.Daemon.DebounceMs)
	}

	// Validate workers
	if cfg.Daemon.Workers < 1 || cfg.Daemon.Workers > 20 {
		v.AddError("daemon.workers", "range", fmt.Sprintf("workers %d is out of valid range (1-20)", cfg.Daemon.Workers), "Set workers between 1 and 20", cfg.Daemon.Workers)
	}

	// Validate rate limit
	if cfg.Daemon.RateLimitPerMin < 1 || cfg.Daemon.RateLimitPerMin > 200 {
		v.AddError("daemon.rate_limit_per_min", "range", fmt.Sprintf("rate_limit_per_min %d is out of valid range (1-200)", cfg.Daemon.RateLimitPerMin), "Set rate_limit_per_min between 1 and 200", cfg.Daemon.RateLimitPerMin)
	}

	// Validate full rebuild interval
	if cfg.Daemon.FullRebuildIntervalMinutes < 0 {
		v.AddError("daemon.full_rebuild_interval_minutes", "range", "full_rebuild_interval_minutes cannot be negative", "Set to 0 to disable or a positive number of minutes", cfg.Daemon.FullRebuildIntervalMinutes)
	}

	// Validate HTTP port
	if cfg.Daemon.HTTPPort < 0 || cfg.Daemon.HTTPPort > 65535 {
		v.AddError("daemon.http_port", "range", fmt.Sprintf("http_port %d is out of valid range (0-65535)", cfg.Daemon.HTTPPort), "Set to 0 to disable or a valid port number (1-65535)", cfg.Daemon.HTTPPort)
	}

	// Validate log level
	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLogLevels, cfg.Daemon.LogLevel) {
		v.AddError("daemon.log_level", "enum", fmt.Sprintf("invalid log_level '%s', must be one of: %v", cfg.Daemon.LogLevel, validLogLevels), "Set log_level to 'debug', 'info', 'warn', or 'error'", cfg.Daemon.LogLevel)
	}

	// Validate log file path
	if cfg.Daemon.LogFile == "" {
		v.AddError("daemon.log_file", "required", "log_file is required", "Set log_file to a valid file path", nil)
	} else if strings.Contains(cfg.Daemon.LogFile, "..") {
		v.AddError("daemon.log_file", "security", "log_file contains parent directory references (..)", "Use an absolute path or home-relative path without '..'", cfg.Daemon.LogFile)
	}
}

// validateMCP validates MCP server configuration
func validateMCP(v *Validator, cfg *Config) {
	// Validate log level
	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLogLevels, cfg.MCP.LogLevel) {
		v.AddError("mcp.log_level", "enum", fmt.Sprintf("invalid log_level '%s', must be one of: %v", cfg.MCP.LogLevel, validLogLevels), "Set log_level to 'debug', 'info', 'warn', or 'error'", cfg.MCP.LogLevel)
	}

	// Validate log file path
	if cfg.MCP.LogFile == "" {
		v.AddError("mcp.log_file", "required", "log_file is required", "Set log_file to a valid file path", nil)
	} else if strings.Contains(cfg.MCP.LogFile, "..") {
		v.AddError("mcp.log_file", "security", "log_file contains parent directory references (..)", "Use an absolute path or home-relative path without '..'", cfg.MCP.LogFile)
	}

	// Validate daemon port
	if cfg.MCP.DaemonPort < 0 || cfg.MCP.DaemonPort > 65535 {
		v.AddError("mcp.daemon_port", "range", fmt.Sprintf("daemon_port %d is out of valid range (0-65535)", cfg.MCP.DaemonPort), "Set to 0 to disable daemon integration or a valid port number (1-65535)", cfg.MCP.DaemonPort)
	}

	// Validate daemon host is set if port is configured
	if cfg.MCP.DaemonPort > 0 && cfg.MCP.DaemonHost == "" {
		v.AddError("mcp.daemon_host", "required", "daemon_host is required when daemon_port is set", "Set daemon_host to the daemon hostname (e.g., 'localhost')", nil)
	}
}

// validateGraph validates FalkorDB graph configuration.
// FalkorDB is a required dependency - there is no option to disable it.
func validateGraph(v *Validator, cfg *Config) {
	// Validate host
	if cfg.Graph.Host == "" {
		v.AddError("graph.host", "required", "graph.host is required", "Set graph.host to FalkorDB hostname (e.g., 'localhost')", nil)
	}

	// Validate port range
	if cfg.Graph.Port < 1 || cfg.Graph.Port > 65535 {
		v.AddError("graph.port", "range", fmt.Sprintf("graph.port %d is out of valid range (1-65535)", cfg.Graph.Port), "Set graph.port to a valid port number (default: 6379)", cfg.Graph.Port)
	}

	// Validate similarity threshold range
	if cfg.Graph.SimilarityThreshold < 0.0 || cfg.Graph.SimilarityThreshold > 1.0 {
		v.AddError("graph.similarity_threshold", "range", fmt.Sprintf("similarity_threshold %.2f is out of valid range (0.0-1.0)", cfg.Graph.SimilarityThreshold), "Set similarity_threshold between 0.0 and 1.0", cfg.Graph.SimilarityThreshold)
	}

	// Validate max similar files
	if cfg.Graph.MaxSimilarFiles < 1 || cfg.Graph.MaxSimilarFiles > 100 {
		v.AddError("graph.max_similar_files", "range", fmt.Sprintf("max_similar_files %d is out of valid range (1-100)", cfg.Graph.MaxSimilarFiles), "Set max_similar_files between 1 and 100", cfg.Graph.MaxSimilarFiles)
	}
}

// OpenAI embedding model dimensions
var openAIEmbeddingModels = map[string]int{
	"text-embedding-3-small": 1536,
	"text-embedding-3-large": 3072,
	"text-embedding-ada-002": 1536,
}

// validateEmbeddings validates embeddings provider configuration.
func validateEmbeddings(v *Validator, cfg *Config) {
	// Skip validation if embeddings is disabled
	// Note: embeddings.enabled is derived from API key presence in GetConfig()
	if !cfg.Embeddings.Enabled {
		return
	}

	// If enabled, API key must be present (already resolved from env in GetConfig)
	if cfg.Embeddings.APIKey == "" {
		v.AddError("embeddings.api_key", "required", "embeddings.api_key is required when embeddings is enabled", "Set OPENAI_API_KEY environment variable or configure api_key in config", nil)
	}

	// Validate provider (only OpenAI is currently supported)
	validProviders := []string{"openai"}
	if cfg.Embeddings.Provider != "" && !contains(validProviders, cfg.Embeddings.Provider) {
		v.AddError("embeddings.provider", "enum", fmt.Sprintf("provider '%s' is not supported", cfg.Embeddings.Provider), "Only 'openai' is currently supported", cfg.Embeddings.Provider)
	}

	// Validate model and dimensions for OpenAI provider
	if cfg.Embeddings.Provider == "" || cfg.Embeddings.Provider == "openai" {
		expectedDim, validModel := openAIEmbeddingModels[cfg.Embeddings.Model]
		if !validModel && cfg.Embeddings.Model != "" {
			v.AddError("embeddings.model", "enum", fmt.Sprintf("unknown OpenAI embedding model: %s", cfg.Embeddings.Model), "Use text-embedding-3-small, text-embedding-3-large, or text-embedding-ada-002", cfg.Embeddings.Model)
		}

		// Validate dimensions match model
		if validModel && cfg.Embeddings.Dimensions != expectedDim {
			v.AddError("embeddings.dimensions", "model_match", fmt.Sprintf("dimensions %d don't match model %s (expected %d)", cfg.Embeddings.Dimensions, cfg.Embeddings.Model, expectedDim), fmt.Sprintf("Use dimensions %d for model %s", expectedDim, cfg.Embeddings.Model), cfg.Embeddings.Dimensions)
		}
	}
}

// contains checks if a slice contains a string
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// SafePath validates that a path is safe (no directory traversal)
func SafePath(path string) error {
	if strings.Contains(path, "..") {
		return fmt.Errorf("path contains parent directory reference (..): %s", path)
	}

	// Check for absolute path or home-relative path
	if !filepath.IsAbs(path) && !strings.HasPrefix(path, "~") {
		return fmt.Errorf("path should be absolute or home-relative (~/...): %s", path)
	}

	return nil
}

// ValidateBinaryPath validates a binary path for execution
func ValidateBinaryPath(path string) error {
	// Check for safety
	if err := SafePath(path); err != nil {
		return err
	}

	// Expand home directory
	expandedPath := ExpandHome(path)

	// Check if file exists
	stat, err := os.Stat(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("binary does not exist: %s", path)
		}
		return fmt.Errorf("cannot access binary: %w", err)
	}

	// Check if it's a regular file
	if stat.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", path)
	}

	// Check if it's executable
	if stat.Mode()&0111 == 0 {
		return fmt.Errorf("binary is not executable: %s (permissions: %s)", path, stat.Mode())
	}

	return nil
}
