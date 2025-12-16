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

	// Reset viper state to support hot-reload functionality.
	// Clears cached config values, allowing InitConfig() to re-read
	// updated config files without requiring process restart.
	viper.Reset()

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Add app directory to config search path (respects MEMORIZER_APP_DIR)
	if appDir, err := GetAppDir(); err == nil {
		viper.AddConfigPath(appDir)
	}
	viper.AddConfigPath(".") // Current directory fallback

	// User-facing settings (see MinimalConfig for initialized config surface area)
	viper.SetDefault("memory_root", DefaultConfig.MemoryRoot)
	viper.SetDefault("claude.api_key", DefaultConfig.Claude.APIKey)
	viper.SetDefault("claude.model", DefaultConfig.Claude.Model)
	viper.SetDefault("daemon.http_port", DefaultConfig.Daemon.HTTPPort)
	viper.SetDefault("daemon.log_level", DefaultConfig.Daemon.LogLevel)
	viper.SetDefault("mcp.log_level", DefaultConfig.MCP.LogLevel)
	viper.SetDefault("graph.host", DefaultConfig.Graph.Host)
	viper.SetDefault("graph.port", DefaultConfig.Graph.Port)
	viper.SetDefault("graph.database", DefaultConfig.Graph.Database)
	viper.SetDefault("graph.password", DefaultConfig.Graph.Password)
	viper.SetDefault("embeddings.api_key", DefaultConfig.Embeddings.APIKey)

	// Power-user settings (not included in minimal initialization config)
	viper.SetDefault("claude.max_tokens", DefaultConfig.Claude.MaxTokens)
	viper.SetDefault("claude.timeout", DefaultConfig.Claude.Timeout)
	viper.SetDefault("analysis.enabled", DefaultConfig.Analysis.Enabled)
	viper.SetDefault("analysis.max_file_size", DefaultConfig.Analysis.MaxFileSize)
	viper.SetDefault("analysis.skip_extensions", DefaultConfig.Analysis.SkipExtensions)
	viper.SetDefault("analysis.skip_files", DefaultConfig.Analysis.SkipFiles)
	viper.SetDefault("analysis.cache_dir", DefaultConfig.Analysis.CacheDir)
	viper.SetDefault("daemon.debounce_ms", DefaultConfig.Daemon.DebounceMs)
	viper.SetDefault("daemon.workers", DefaultConfig.Daemon.Workers)
	viper.SetDefault("daemon.rate_limit_per_min", DefaultConfig.Daemon.RateLimitPerMin)
	viper.SetDefault("daemon.full_rebuild_interval_minutes", DefaultConfig.Daemon.FullRebuildIntervalMinutes)
	viper.SetDefault("daemon.log_file", DefaultConfig.Daemon.LogFile)
	viper.SetDefault("mcp.log_file", DefaultConfig.MCP.LogFile)
	viper.SetDefault("mcp.daemon_host", DefaultConfig.MCP.DaemonHost)
	viper.SetDefault("mcp.daemon_port", DefaultConfig.MCP.DaemonPort)
	viper.SetDefault("graph.similarity_threshold", DefaultConfig.Graph.SimilarityThreshold)
	viper.SetDefault("graph.max_similar_files", DefaultConfig.Graph.MaxSimilarFiles)
	viper.SetDefault("integrations.enabled", DefaultConfig.Integrations.Enabled)
	viper.SetDefault("embeddings.enabled", DefaultConfig.Embeddings.Enabled)
	viper.SetDefault("embeddings.provider", DefaultConfig.Embeddings.Provider)
	viper.SetDefault("embeddings.model", DefaultConfig.Embeddings.Model)
	viper.SetDefault("embeddings.dimensions", DefaultConfig.Embeddings.Dimensions)

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

	// Validate paths for safety BEFORE expansion - prevent path traversal attacks.
	// Reject paths containing '..' to ensure they stay within expected boundaries.
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

	// Resolve API keys from hardcoded environment variable names
	if cfg.Claude.APIKey == "" {
		cfg.Claude.APIKey = os.Getenv(ClaudeAPIKeyEnv)
	}
	if cfg.Graph.Password == "" {
		cfg.Graph.Password = os.Getenv(GraphPasswordEnv)
	}
	if cfg.Embeddings.APIKey == "" {
		cfg.Embeddings.APIKey = os.Getenv(EmbeddingsAPIKeyEnv)
	}

	// Derive analysis.enabled from Claude API key presence.
	// Semantic analysis requires Claude API - automatically disable if no key.
	// This simplifies config and prevents runtime errors from missing credentials.
	if cfg.Claude.APIKey == "" {
		cfg.Analysis.Enabled = false
	}

	// Derive embeddings.enabled from embeddings API key presence.
	// Embeddings require OpenAI API - automatically disable if no key.
	// This simplifies config and prevents runtime errors from missing credentials.
	if cfg.Embeddings.APIKey == "" {
		cfg.Embeddings.Enabled = false
	}

	return &cfg, nil
}

func WriteConfig(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config; %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file; %w", err)
	}

	return nil
}

// MinimalConfig contains only user-facing settings for initial configuration.
// Internal settings use defaults and are not written to the initialized config file.
type MinimalConfig struct {
	MemoryRoot   string                    `yaml:"memory_root"`
	Claude       MinimalClaudeConfig       `yaml:"claude,omitempty"`
	Daemon       MinimalDaemonConfig       `yaml:"daemon,omitempty"`
	MCP          MinimalMCPConfig          `yaml:"mcp,omitempty"`
	Graph        MinimalGraphConfig        `yaml:"graph,omitempty"`
	Embeddings   MinimalEmbeddingsConfig   `yaml:"embeddings,omitempty"`
	Integrations MinimalIntegrationsConfig `yaml:"integrations,omitempty"`
}

type MinimalClaudeConfig struct {
	APIKey string `yaml:"api_key,omitempty"`
	Model  string `yaml:"model,omitempty"`
}

type MinimalDaemonConfig struct {
	HTTPPort int    `yaml:"http_port"`
	LogLevel string `yaml:"log_level,omitempty"`
}

type MinimalMCPConfig struct {
	LogLevel   string `yaml:"log_level,omitempty"`
	DaemonHost string `yaml:"daemon_host,omitempty"`
	DaemonPort int    `yaml:"daemon_port,omitempty"`
}

type MinimalGraphConfig struct {
	Host     string `yaml:"host,omitempty"`
	Port     int    `yaml:"port,omitempty"`
	Password string `yaml:"password,omitempty"`
}

type MinimalEmbeddingsConfig struct {
	APIKey string `yaml:"api_key,omitempty"`
}

type MinimalIntegrationsConfig struct {
	Enabled []string `yaml:"enabled,omitempty"`
}

// ToMinimalConfig converts a full Config to a MinimalConfig for writing.
// Only user-facing settings are included; internal settings use defaults.
func (c *Config) ToMinimalConfig() *MinimalConfig {
	minimal := &MinimalConfig{
		MemoryRoot: c.MemoryRoot,
		Claude: MinimalClaudeConfig{
			APIKey: c.Claude.APIKey,
			Model:  c.Claude.Model,
		},
		Daemon: MinimalDaemonConfig{
			HTTPPort: c.Daemon.HTTPPort,
			LogLevel: c.Daemon.LogLevel,
		},
		MCP: MinimalMCPConfig{
			LogLevel: c.MCP.LogLevel,
		},
		Graph: MinimalGraphConfig{
			Host:     c.Graph.Host,
			Port:     c.Graph.Port,
			Password: c.Graph.Password,
		},
	}

	// Only include MCP daemon connectivity if enabled
	if c.MCP.DaemonPort > 0 {
		minimal.MCP.DaemonHost = c.MCP.DaemonHost
		minimal.MCP.DaemonPort = c.MCP.DaemonPort
	}

	// Only include embeddings API key if set
	if c.Embeddings.APIKey != "" {
		minimal.Embeddings.APIKey = c.Embeddings.APIKey
	}

	// Only include integrations if any are enabled
	if len(c.Integrations.Enabled) > 0 {
		minimal.Integrations.Enabled = c.Integrations.Enabled
	}

	return minimal
}

// WriteMinimalConfig writes only user-facing configuration settings.
// Internal settings are omitted and will use defaults when loaded.
func WriteMinimalConfig(path string, cfg *Config) error {
	minimal := cfg.ToMinimalConfig()

	data, err := yaml.Marshal(minimal)
	if err != nil {
		return fmt.Errorf("failed to marshal minimal config; %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
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
