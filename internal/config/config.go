package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func InitConfig() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.agentic-memorizer")
	viper.AddConfigPath(".")

	viper.SetDefault("memory_root", DefaultConfig.MemoryRoot)
	viper.SetDefault("cache_dir", DefaultConfig.CacheDir)
	viper.SetDefault("claude.api_key", DefaultConfig.Claude.APIKey)
	viper.SetDefault("claude.api_key_env", DefaultConfig.Claude.APIKeyEnv)
	viper.SetDefault("claude.model", DefaultConfig.Claude.Model)
	viper.SetDefault("claude.max_tokens", DefaultConfig.Claude.MaxTokens)
	viper.SetDefault("claude.enable_vision", DefaultConfig.Claude.EnableVision)
	viper.SetDefault("claude.timeout_seconds", DefaultConfig.Claude.TimeoutSeconds)
	viper.SetDefault("output.format", DefaultConfig.Output.Format)
	viper.SetDefault("output.verbose", DefaultConfig.Output.Verbose)
	viper.SetDefault("output.show_recent_days", DefaultConfig.Output.ShowRecentDays)
	viper.SetDefault("analysis.enable", DefaultConfig.Analysis.Enable)
	viper.SetDefault("analysis.max_file_size", DefaultConfig.Analysis.MaxFileSize)
	viper.SetDefault("analysis.parallel", DefaultConfig.Analysis.Parallel)
	viper.SetDefault("analysis.skip_extensions", DefaultConfig.Analysis.SkipExtensions)
	viper.SetDefault("analysis.skip_files", DefaultConfig.Analysis.SkipFiles)

	viper.SetEnvPrefix("MEMORIZER")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("failed to read config; %w", err)
		}
	}

	return nil
}

func GetConfig() (*Config, error) {
	var cfg Config

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config; %w", err)
	}

	cfg.MemoryRoot = ExpandHome(cfg.MemoryRoot)
	cfg.CacheDir = ExpandHome(cfg.CacheDir)

	if cfg.Claude.APIKey == "" && cfg.Claude.APIKeyEnv != "" {
		cfg.Claude.APIKey = os.Getenv(cfg.Claude.APIKeyEnv)
	}

	return &cfg, nil
}

func WriteConfig(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config; %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file; %w", err)
	}

	return nil
}

func ExpandHome(path string) string {
	if len(path) == 0 || path[0] != '~' {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	if len(path) == 1 {
		return home
	}

	return filepath.Join(home, path[2:])
}

func (c *Config) GetAPIKey() string {
	return c.Claude.APIKey
}
