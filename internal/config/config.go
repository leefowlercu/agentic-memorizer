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
	viper.SetDefault("semantic.provider", DefaultConfig.Semantic.Provider)
	viper.SetDefault("semantic.api_key", DefaultConfig.Semantic.APIKey)
	viper.SetDefault("semantic.model", DefaultConfig.Semantic.Model)
	viper.SetDefault("daemon.http_port", DefaultConfig.Daemon.HTTPPort)
	viper.SetDefault("daemon.log_level", DefaultConfig.Daemon.LogLevel)
	viper.SetDefault("mcp.log_level", DefaultConfig.MCP.LogLevel)
	viper.SetDefault("graph.host", DefaultConfig.Graph.Host)
	viper.SetDefault("graph.port", DefaultConfig.Graph.Port)
	viper.SetDefault("graph.database", DefaultConfig.Graph.Database)
	viper.SetDefault("graph.password", DefaultConfig.Graph.Password)
	viper.SetDefault("embeddings.api_key", DefaultConfig.Embeddings.APIKey)

	// Power-user settings (not included in minimal initialization config)
	viper.SetDefault("semantic.enabled", DefaultConfig.Semantic.Enabled)
	viper.SetDefault("semantic.max_tokens", DefaultConfig.Semantic.MaxTokens)
	viper.SetDefault("semantic.timeout", DefaultConfig.Semantic.Timeout)
	viper.SetDefault("semantic.enable_vision", DefaultConfig.Semantic.EnableVision)
	viper.SetDefault("semantic.max_file_size", DefaultConfig.Semantic.MaxFileSize)
	viper.SetDefault("semantic.skip_extensions", DefaultConfig.Semantic.SkipExtensions)
	viper.SetDefault("semantic.skip_files", DefaultConfig.Semantic.SkipFiles)
	viper.SetDefault("semantic.cache_dir", DefaultConfig.Semantic.CacheDir)
	viper.SetDefault("semantic.rate_limit_per_min", DefaultConfig.Semantic.RateLimitPerMin)
	viper.SetDefault("daemon.debounce_ms", DefaultConfig.Daemon.DebounceMs)
	viper.SetDefault("daemon.workers", DefaultConfig.Daemon.Workers)
	viper.SetDefault("daemon.full_rebuild_interval_minutes", DefaultConfig.Daemon.FullRebuildIntervalMinutes)
	viper.SetDefault("daemon.log_file", DefaultConfig.Daemon.LogFile)
	viper.SetDefault("mcp.log_file", DefaultConfig.MCP.LogFile)
	viper.SetDefault("mcp.daemon_host", DefaultConfig.MCP.DaemonHost)
	viper.SetDefault("mcp.daemon_port", DefaultConfig.MCP.DaemonPort)
	viper.SetDefault("graph.similarity_threshold", DefaultConfig.Graph.SimilarityThreshold)
	viper.SetDefault("graph.max_similar_files", DefaultConfig.Graph.MaxSimilarFiles)
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
	if strings.Contains(cfg.Semantic.CacheDir, "..") {
		return nil, fmt.Errorf("semantic.cache_dir contains parent directory reference (..): %s", cfg.Semantic.CacheDir)
	}
	if strings.Contains(cfg.Daemon.LogFile, "..") {
		return nil, fmt.Errorf("daemon.log_file contains parent directory reference (..): %s", cfg.Daemon.LogFile)
	}
	if strings.Contains(cfg.MCP.LogFile, "..") {
		return nil, fmt.Errorf("mcp.log_file contains parent directory reference (..): %s", cfg.MCP.LogFile)
	}

	cfg.MemoryRoot = ExpandHome(cfg.MemoryRoot)
	cfg.Semantic.CacheDir = ExpandHome(cfg.Semantic.CacheDir)
	cfg.Daemon.LogFile = ExpandHome(cfg.Daemon.LogFile)
	cfg.MCP.LogFile = ExpandHome(cfg.MCP.LogFile)

	// Resolve API keys from hardcoded environment variable names
	// Support provider-specific env vars based on provider choice
	if cfg.Semantic.APIKey == "" {
		switch cfg.Semantic.Provider {
		case "claude":
			cfg.Semantic.APIKey = os.Getenv(ClaudeAPIKeyEnv)
		case "openai":
			cfg.Semantic.APIKey = os.Getenv("OPENAI_API_KEY")
		case "gemini":
			cfg.Semantic.APIKey = os.Getenv("GOOGLE_API_KEY")
		default:
			// Default to Claude if no provider specified
			cfg.Semantic.APIKey = os.Getenv(ClaudeAPIKeyEnv)
		}
	}

	if cfg.Graph.Password == "" {
		cfg.Graph.Password = os.Getenv(GraphPasswordEnv)
	}
	if cfg.Embeddings.APIKey == "" {
		cfg.Embeddings.APIKey = os.Getenv(EmbeddingsAPIKeyEnv)
	}

	// Derive semantic.enabled from API key presence
	// Semantic analysis requires provider API key - automatically disable if no key.
	// This simplifies config and prevents runtime errors from missing credentials.
	if cfg.Semantic.APIKey == "" {
		cfg.Semantic.Enabled = false
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
	Semantic     MinimalSemanticConfig     `yaml:"semantic,omitempty"`
	Daemon       MinimalDaemonConfig       `yaml:"daemon,omitempty"`
	MCP          MinimalMCPConfig          `yaml:"mcp,omitempty"`
	Graph        MinimalGraphConfig        `yaml:"graph,omitempty"`
	Embeddings   MinimalEmbeddingsConfig   `yaml:"embeddings,omitempty"`
	Integrations MinimalIntegrationsConfig `yaml:"integrations,omitempty"`
}

type MinimalSemanticConfig struct {
	Provider string `yaml:"provider,omitempty"`
	APIKey   string `yaml:"api_key,omitempty"`
	Model    string `yaml:"model,omitempty"`
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
		Semantic: MinimalSemanticConfig{
			Provider: c.Semantic.Provider,
			APIKey:   c.Semantic.APIKey,
			Model:    c.Semantic.Model,
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
