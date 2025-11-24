package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var (
	// configMu protects InitConfig from concurrent access
	// This is necessary because viper uses global state
	configMu sync.Mutex
)

func InitConfig() error {
	configMu.Lock()
	defer configMu.Unlock()

	// Reset viper state to force fresh config read during reload
	// This clears cached config values and allows reading updated config files
	viper.Reset()

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Add app directory to config search path (respects MEMORIZER_APP_DIR)
	if appDir, err := GetAppDir(); err == nil {
		viper.AddConfigPath(appDir)
	}
	viper.AddConfigPath(".") // Current directory fallback

	viper.SetDefault("memory_root", DefaultConfig.MemoryRoot)
	viper.SetDefault("claude.api_key", DefaultConfig.Claude.APIKey)
	viper.SetDefault("claude.api_key_env", DefaultConfig.Claude.APIKeyEnv)
	viper.SetDefault("claude.model", DefaultConfig.Claude.Model)
	viper.SetDefault("claude.max_tokens", DefaultConfig.Claude.MaxTokens)
	viper.SetDefault("claude.enable_vision", DefaultConfig.Claude.EnableVision)
	viper.SetDefault("claude.timeout_seconds", DefaultConfig.Claude.TimeoutSeconds)
	viper.SetDefault("output.format", DefaultConfig.Output.Format)
	viper.SetDefault("output.show_recent_days", DefaultConfig.Output.ShowRecentDays)
	viper.SetDefault("analysis.enable", DefaultConfig.Analysis.Enable)
	viper.SetDefault("analysis.max_file_size", DefaultConfig.Analysis.MaxFileSize)
	viper.SetDefault("analysis.skip_extensions", DefaultConfig.Analysis.SkipExtensions)
	viper.SetDefault("analysis.skip_files", DefaultConfig.Analysis.SkipFiles)
	viper.SetDefault("analysis.cache_dir", DefaultConfig.Analysis.CacheDir)
	viper.SetDefault("daemon.debounce_ms", DefaultConfig.Daemon.DebounceMs)
	viper.SetDefault("daemon.workers", DefaultConfig.Daemon.Workers)
	viper.SetDefault("daemon.rate_limit_per_min", DefaultConfig.Daemon.RateLimitPerMin)
	viper.SetDefault("daemon.full_rebuild_interval_minutes", DefaultConfig.Daemon.FullRebuildIntervalMinutes)
	viper.SetDefault("daemon.http_port", DefaultConfig.Daemon.HTTPPort)
	viper.SetDefault("daemon.log_file", DefaultConfig.Daemon.LogFile)
	viper.SetDefault("daemon.log_level", DefaultConfig.Daemon.LogLevel)
	viper.SetDefault("mcp.log_file", DefaultConfig.MCP.LogFile)
	viper.SetDefault("mcp.log_level", DefaultConfig.MCP.LogLevel)

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

	// Validate paths for safety BEFORE expansion
	if strings.Contains(cfg.MemoryRoot, "..") {
		return nil, fmt.Errorf("memory_root contains parent directory reference (..): %s", cfg.MemoryRoot)
	}
	if strings.Contains(cfg.Analysis.CacheDir, "..") {
		return nil, fmt.Errorf("analysis.cache_dir contains parent directory reference (..): %s", cfg.Analysis.CacheDir)
	}
	if strings.Contains(cfg.Daemon.LogFile, "..") {
		return nil, fmt.Errorf("daemon.log_file contains parent directory reference (..): %s", cfg.Daemon.LogFile)
	}
	if strings.Contains(cfg.MCP.LogFile, "..") {
		return nil, fmt.Errorf("mcp.log_file contains parent directory reference (..): %s", cfg.MCP.LogFile)
	}

	cfg.MemoryRoot = ExpandHome(cfg.MemoryRoot)
	cfg.Analysis.CacheDir = ExpandHome(cfg.Analysis.CacheDir)
	cfg.Daemon.LogFile = ExpandHome(cfg.Daemon.LogFile)
	cfg.MCP.LogFile = ExpandHome(cfg.MCP.LogFile)

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

func GetConfigPath() string {
	return viper.ConfigFileUsed()
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
// Checks MEMORIZER_APP_DIR environment variable first, then falls back to ~/.agentic-memorizer
func GetAppDir() (string, error) {
	// Check environment variable first
	if appDir := os.Getenv("MEMORIZER_APP_DIR"); appDir != "" {
		// Expand home directory if path starts with ~
		expanded := ExpandHome(appDir)

		// Validate path safety
		if err := SafePath(expanded); err != nil {
			return "", fmt.Errorf("invalid MEMORIZER_APP_DIR; %w", err)
		}

		return expanded, nil
	}

	// Fall back to default
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory; %w", err)
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

// ResetForTesting resets viper state for testing purposes.
// This allows tests to use different config files without interference.
// Should only be called from test code.
func ResetForTesting() {
	configMu.Lock()
	defer configMu.Unlock()
	viper.Reset()
}
