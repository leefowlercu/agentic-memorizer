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
	viper.AddConfigPath("$HOME/" + AppDirName)
	viper.AddConfigPath(".")

	viper.SetDefault("memory_root", DefaultConfig.MemoryRoot)
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
	viper.SetDefault("analysis.cache_dir", DefaultConfig.Analysis.CacheDir)
	viper.SetDefault("daemon.enabled", DefaultConfig.Daemon.Enabled)
	viper.SetDefault("daemon.debounce_ms", DefaultConfig.Daemon.DebounceMs)
	viper.SetDefault("daemon.workers", DefaultConfig.Daemon.Workers)
	viper.SetDefault("daemon.rate_limit_per_min", DefaultConfig.Daemon.RateLimitPerMin)
	viper.SetDefault("daemon.full_rebuild_interval_minutes", DefaultConfig.Daemon.FullRebuildIntervalMinutes)
	viper.SetDefault("daemon.health_check_port", DefaultConfig.Daemon.HealthCheckPort)
	viper.SetDefault("daemon.log_file", DefaultConfig.Daemon.LogFile)
	viper.SetDefault("daemon.log_level", DefaultConfig.Daemon.LogLevel)

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
	cfg.Analysis.CacheDir = ExpandHome(cfg.Analysis.CacheDir)
	cfg.Daemon.LogFile = ExpandHome(cfg.Daemon.LogFile)

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

// GetAppDir returns the application directory path.
// Returns ~/.agentic-memorizer (expanded from ~)
func GetAppDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, AppDirName), nil
}

// GetIndexPath returns the path to the precomputed index file.
// The index is stored at ~/.agentic-memorizer/index.json
func GetIndexPath() (string, error) {
	appDir, err := GetAppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(appDir, IndexFile), nil
}

// GetPIDPath returns the daemon PID file path.
// The PID file is stored at ~/.agentic-memorizer/daemon.pid
func GetPIDPath() (string, error) {
	appDir, err := GetAppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(appDir, DaemonPIDFile), nil
}
