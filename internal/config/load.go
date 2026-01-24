package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Load reads and returns the typed configuration.
// It searches for configuration files in priority order:
//  1. Directory specified by MEMORIZER_CONFIG_DIR environment variable
//  2. ~/.config/memorizer/
//  3. Current working directory (.)
//
// If no config file is found, returns an error directing user to run initialize.
// If a config file exists but is invalid, returns a validation error.
func Load() (*Config, error) {
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")

	// Setup environment variable support
	v.SetEnvPrefix("MEMORIZER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Register default values
	setViperDefaults(v)

	// Add config paths in priority order
	if envPath := os.Getenv("MEMORIZER_CONFIG_DIR"); envPath != "" {
		v.AddConfigPath(envPath)
	}

	if home := os.Getenv("HOME"); home != "" {
		v.AddConfigPath(filepath.Join(home, ".config", "memorizer"))
	}

	v.AddConfigPath(".")

	// Read config file
	err := v.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, fmt.Errorf("no config file found; run 'memorizer initialize' to create one")
		}
		return nil, fmt.Errorf("failed to read config; %w", err)
	}

	return unmarshalConfig(v)
}

// LoadFromPath reads configuration from a specific file path.
func LoadFromPath(path string) (*Config, error) {
	v := viper.New()

	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// Setup environment variable support
	v.SetEnvPrefix("MEMORIZER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Register default values
	setViperDefaults(v)

	err := v.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read config from %s; %w", path, err)
	}

	return unmarshalConfig(v)
}

// LoadWithDefaults returns configuration using defaults only.
// Use this in contexts where config file is not required (e.g., initialize command).
func LoadWithDefaults() *Config {
	cfg := NewDefaultConfig()
	return &cfg
}

// unmarshalConfig converts viper config to typed Config struct.
func unmarshalConfig(v *viper.Viper) (*Config, error) {
	cfg := &Config{}

	err := v.Unmarshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config; %w", err)
	}

	if err := Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// setViperDefaults registers all default configuration values with a viper instance.
func setViperDefaults(v *viper.Viper) {
	v.SetDefault("log_level", DefaultLogLevel)
	v.SetDefault("log_file", DefaultLogFile)

	// Daemon defaults
	v.SetDefault("daemon.http_port", DefaultDaemonHTTPPort)
	v.SetDefault("daemon.http_bind", DefaultDaemonHTTPBind)
	v.SetDefault("daemon.shutdown_timeout", DefaultDaemonShutdownTimeout)
	v.SetDefault("daemon.pid_file", DefaultDaemonPIDFile)
	v.SetDefault("daemon.metrics.collection_interval", DefaultDaemonMetricsInterval)
	v.SetDefault("daemon.event_bus.buffer_size", DefaultDaemonEventBusBufferSize)
	v.SetDefault("daemon.event_bus.critical_queue_capacity", DefaultDaemonEventBusCriticalQueueCapacity)

	// Graph defaults
	v.SetDefault("graph.host", DefaultGraphHost)
	v.SetDefault("graph.port", DefaultGraphPort)
	v.SetDefault("graph.name", DefaultGraphName)
	v.SetDefault("graph.password_env", DefaultGraphPasswordEnv)
	v.SetDefault("graph.max_retries", DefaultGraphMaxRetries)
	v.SetDefault("graph.retry_delay_ms", DefaultGraphRetryDelayMs)
	v.SetDefault("graph.write_queue_size", DefaultGraphWriteQueueSize)

	// Semantic defaults
	v.SetDefault("semantic.provider", DefaultSemanticProvider)
	v.SetDefault("semantic.model", DefaultSemanticModel)
	v.SetDefault("semantic.rate_limit", DefaultSemanticRateLimit)
	v.SetDefault("semantic.api_key_env", DefaultSemanticAPIKeyEnv)

	// Embeddings defaults
	v.SetDefault("embeddings.enabled", DefaultEmbeddingsEnabled)
	v.SetDefault("embeddings.provider", DefaultEmbeddingsProvider)
	v.SetDefault("embeddings.model", DefaultEmbeddingsModel)
	v.SetDefault("embeddings.dimensions", DefaultEmbeddingsDimensions)
	v.SetDefault("embeddings.api_key_env", DefaultEmbeddingsAPIKeyEnv)
}
