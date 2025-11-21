package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// ValidateConfig validates the complete configuration
func ValidateConfig(cfg *Config) error {
	v := &Validator{}

	// Phase 1: Basic field validation
	validateMemoryRoot(v, cfg)
	validateClaude(v, cfg)
	validateOutput(v, cfg)
	validateAnalysis(v, cfg)
	validateDaemon(v, cfg)
	validateMCP(v, cfg)

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
	// Check if either api_key or api_key_env is set
	if cfg.Claude.APIKey == "" && cfg.Claude.APIKeyEnv == "" {
		v.AddError("claude.api_key_env", "required", "either claude.api_key or claude.api_key_env must be set", "Set ANTHROPIC_API_KEY environment variable or configure api_key in config", nil)
	}

	// Validate model name (basic check - just ensure it's not empty)
	if cfg.Claude.Model == "" {
		v.AddError("claude.model", "required", "claude.model is required", "Set to a valid Claude model (e.g., claude-sonnet-4-5-20250929)", nil)
	}

	// Validate max_tokens range
	if cfg.Claude.MaxTokens < 1 || cfg.Claude.MaxTokens > 8192 {
		v.AddError("claude.max_tokens", "range", fmt.Sprintf("max_tokens %d is out of valid range (1-8192)", cfg.Claude.MaxTokens), "Set max_tokens between 1 and 8192", cfg.Claude.MaxTokens)
	}

	// Validate timeout
	if cfg.Claude.TimeoutSeconds < 1 || cfg.Claude.TimeoutSeconds > 300 {
		v.AddError("claude.timeout_seconds", "range", fmt.Sprintf("timeout_seconds %d is out of valid range (1-300)", cfg.Claude.TimeoutSeconds), "Set timeout_seconds between 1 and 300", cfg.Claude.TimeoutSeconds)
	}
}

// validateOutput validates output configuration
func validateOutput(v *Validator, cfg *Config) {
	// Validate format enum
	validFormats := []string{"xml", "markdown", "json"}
	if !contains(validFormats, cfg.Output.Format) {
		v.AddError("output.format", "enum", fmt.Sprintf("invalid format '%s', must be one of: %v", cfg.Output.Format, validFormats), "Set format to 'xml', 'markdown', or 'json'", cfg.Output.Format)
	}

	// Validate show_recent_days range
	if cfg.Output.ShowRecentDays < 0 || cfg.Output.ShowRecentDays > 365 {
		v.AddError("output.show_recent_days", "range", fmt.Sprintf("show_recent_days %d is out of valid range (0-365)", cfg.Output.ShowRecentDays), "Set show_recent_days between 0 and 365", cfg.Output.ShowRecentDays)
	}
}

// validateAnalysis validates analysis configuration
func validateAnalysis(v *Validator, cfg *Config) {
	// Validate max_file_size
	if cfg.Analysis.MaxFileSize < 0 {
		v.AddError("analysis.max_file_size", "range", "max_file_size cannot be negative", "Set max_file_size to a positive number or 0 for unlimited", cfg.Analysis.MaxFileSize)
	}

	// Validate parallel workers
	if cfg.Analysis.Parallel < 1 || cfg.Analysis.Parallel > 20 {
		v.AddError("analysis.parallel", "range", fmt.Sprintf("parallel %d is out of valid range (1-20)", cfg.Analysis.Parallel), "Set parallel between 1 and 20 workers", cfg.Analysis.Parallel)
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

	// Validate health check port
	if cfg.Daemon.HealthCheckPort < 0 || cfg.Daemon.HealthCheckPort > 65535 {
		v.AddError("daemon.health_check_port", "range", fmt.Sprintf("health_check_port %d is out of valid range (0-65535)", cfg.Daemon.HealthCheckPort), "Set to 0 to disable or a valid port number (1-65535)", cfg.Daemon.HealthCheckPort)
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
