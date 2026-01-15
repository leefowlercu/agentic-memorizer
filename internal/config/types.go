package config

import "os"

// Config is the root configuration structure for the application.
type Config struct {
	LogLevel   string           `yaml:"log_level" mapstructure:"log_level"`
	LogFile    string           `yaml:"log_file" mapstructure:"log_file"`
	Daemon     DaemonConfig     `yaml:"daemon" mapstructure:"daemon"`
	Graph      GraphConfig      `yaml:"graph" mapstructure:"graph"`
	Semantic   SemanticConfig   `yaml:"semantic" mapstructure:"semantic"`
	Embeddings EmbeddingsConfig `yaml:"embeddings" mapstructure:"embeddings"`
	Defaults   DefaultsConfig   `yaml:"defaults" mapstructure:"defaults"`
}

// DaemonConfig holds daemon-related configuration.
type DaemonConfig struct {
	HTTPPort        int           `yaml:"http_port" mapstructure:"http_port"`
	HTTPBind        string        `yaml:"http_bind" mapstructure:"http_bind"`
	ShutdownTimeout int           `yaml:"shutdown_timeout" mapstructure:"shutdown_timeout"`
	PIDFile         string        `yaml:"pid_file" mapstructure:"pid_file"`
	RegistryPath    string        `yaml:"registry_path" mapstructure:"registry_path"`
	RebuildInterval int           `yaml:"rebuild_interval" mapstructure:"rebuild_interval"` // seconds, 0 = disabled
	Metrics         MetricsConfig `yaml:"metrics" mapstructure:"metrics"`
}

// MetricsConfig holds metrics collection configuration.
type MetricsConfig struct {
	CollectionInterval int `yaml:"collection_interval" mapstructure:"collection_interval"`
}

// GraphConfig holds FalkorDB/graph database configuration.
type GraphConfig struct {
	Host           string `yaml:"host" mapstructure:"host"`
	Port           int    `yaml:"port" mapstructure:"port"`
	Name           string `yaml:"name" mapstructure:"name"`
	PasswordEnv    string `yaml:"password_env" mapstructure:"password_env"`
	MaxRetries     int    `yaml:"max_retries" mapstructure:"max_retries"`
	RetryDelayMs   int    `yaml:"retry_delay_ms" mapstructure:"retry_delay_ms"`
	WriteQueueSize int    `yaml:"write_queue_size" mapstructure:"write_queue_size"`
}

// SemanticConfig holds semantic analysis provider configuration.
type SemanticConfig struct {
	Provider  string  `yaml:"provider" mapstructure:"provider"`
	Model     string  `yaml:"model" mapstructure:"model"`
	RateLimit int     `yaml:"rate_limit" mapstructure:"rate_limit"`
	APIKey    *string `yaml:"api_key,omitempty" mapstructure:"api_key"`
	APIKeyEnv string  `yaml:"api_key_env" mapstructure:"api_key_env"`
}

// ResolveAPIKey returns the API key from config or falls back to environment variable.
func (c *SemanticConfig) ResolveAPIKey() string {
	if c.APIKey != nil && *c.APIKey != "" {
		return *c.APIKey
	}
	return os.Getenv(c.APIKeyEnv)
}

// EmbeddingsConfig holds embeddings provider configuration.
type EmbeddingsConfig struct {
	Enabled    bool    `yaml:"enabled" mapstructure:"enabled"`
	Provider   string  `yaml:"provider" mapstructure:"provider"`
	Model      string  `yaml:"model" mapstructure:"model"`
	Dimensions int     `yaml:"dimensions" mapstructure:"dimensions"`
	APIKey     *string `yaml:"api_key,omitempty" mapstructure:"api_key"`
	APIKeyEnv  string  `yaml:"api_key_env" mapstructure:"api_key_env"`
}

// DefaultsConfig holds default skip/include patterns for new remembered paths.
type DefaultsConfig struct {
	Skip    SkipDefaults    `yaml:"skip" mapstructure:"skip"`
	Include IncludeDefaults `yaml:"include" mapstructure:"include"`
}

// SkipDefaults holds default patterns to skip.
type SkipDefaults struct {
	Extensions  []string `yaml:"extensions,flow" mapstructure:"extensions"`
	Directories []string `yaml:"directories,flow" mapstructure:"directories"`
	Files       []string `yaml:"files,flow" mapstructure:"files"`
	Hidden      bool     `yaml:"hidden" mapstructure:"hidden"`
}

// IncludeDefaults holds default patterns to include (override skip).
type IncludeDefaults struct {
	Extensions  []string `yaml:"extensions,flow" mapstructure:"extensions"`
	Directories []string `yaml:"directories,flow" mapstructure:"directories"`
	Files       []string `yaml:"files,flow" mapstructure:"files"`
}

// ResolveAPIKey returns the API key from config or falls back to environment variable.
func (c *EmbeddingsConfig) ResolveAPIKey() string {
	if c.APIKey != nil && *c.APIKey != "" {
		return *c.APIKey
	}
	return os.Getenv(c.APIKeyEnv)
}
