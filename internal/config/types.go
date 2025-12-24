package config

import "fmt"

type Config struct {
	MemoryRoot string           `mapstructure:"memory_root" yaml:"memory_root"`
	Claude     ClaudeConfig     `mapstructure:"claude" yaml:"claude"`
	Analysis   AnalysisConfig   `mapstructure:"analysis" yaml:"analysis"`
	Daemon     DaemonConfig     `mapstructure:"daemon" yaml:"daemon"`
	MCP        MCPConfig        `mapstructure:"mcp" yaml:"mcp"`
	Graph      GraphConfig      `mapstructure:"graph" yaml:"graph"`
	Embeddings EmbeddingsConfig `mapstructure:"embeddings" yaml:"embeddings"`
}

type ClaudeConfig struct {
	APIKey       string `mapstructure:"api_key" yaml:"api_key"`
	Model        string `mapstructure:"model" yaml:"model"`
	MaxTokens    int    `mapstructure:"max_tokens" yaml:"max_tokens"`
	Timeout      int    `mapstructure:"timeout" yaml:"timeout"`             // API request timeout in seconds (5-300)
	EnableVision bool   `mapstructure:"enable_vision" yaml:"enable_vision"` // Enable vision API for image analysis
}

type AnalysisConfig struct {
	Enabled        bool     `mapstructure:"enabled" yaml:"enabled"`
	MaxFileSize    int64    `mapstructure:"max_file_size" yaml:"max_file_size"`
	SkipExtensions []string `mapstructure:"skip_extensions" yaml:"skip_extensions"`
	SkipFiles      []string `mapstructure:"skip_files" yaml:"skip_files"`
	CacheDir       string   `mapstructure:"cache_dir" yaml:"cache_dir"`
}

type DaemonConfig struct {
	DebounceMs                 int    `mapstructure:"debounce_ms" yaml:"debounce_ms"`
	Workers                    int    `mapstructure:"workers" yaml:"workers"`
	RateLimitPerMin            int    `mapstructure:"rate_limit_per_min" yaml:"rate_limit_per_min"`
	FullRebuildIntervalMinutes int    `mapstructure:"full_rebuild_interval_minutes" yaml:"full_rebuild_interval_minutes"`
	HTTPPort                   int    `mapstructure:"http_port" yaml:"http_port"`
	LogFile                    string `mapstructure:"log_file" yaml:"log_file"`
	LogLevel                   string `mapstructure:"log_level" yaml:"log_level"`
}

type MCPConfig struct {
	LogFile    string `mapstructure:"log_file" yaml:"log_file"`
	LogLevel   string `mapstructure:"log_level" yaml:"log_level"`
	DaemonHost string `mapstructure:"daemon_host" yaml:"daemon_host"`
	DaemonPort int    `mapstructure:"daemon_port" yaml:"daemon_port"`
}

// GetDaemonURL returns the daemon HTTP API URL constructed from host and port.
// Returns empty string if daemon port is not configured (0).
func (m *MCPConfig) GetDaemonURL() string {
	if m.DaemonPort > 0 {
		return fmt.Sprintf("http://%s:%d", m.DaemonHost, m.DaemonPort)
	}
	return ""
}

// GraphConfig contains FalkorDB knowledge graph configuration.
// FalkorDB is the required storage backend - there is no option to disable it.
type GraphConfig struct {
	Host                string  `mapstructure:"host" yaml:"host"`
	Port                int     `mapstructure:"port" yaml:"port"`
	Database            string  `mapstructure:"database" yaml:"database"` // Graph database name
	Password            string  `mapstructure:"password" yaml:"password"`
	SimilarityThreshold float64 `mapstructure:"similarity_threshold" yaml:"similarity_threshold"`
	MaxSimilarFiles     int     `mapstructure:"max_similar_files" yaml:"max_similar_files"`
}

// EmbeddingsConfig contains embedding provider configuration.
// Provider, model, and dimensions have sensible defaults but can be overridden.
type EmbeddingsConfig struct {
	Enabled    bool   `mapstructure:"enabled" yaml:"enabled"`
	APIKey     string `mapstructure:"api_key" yaml:"api_key"`
	Provider   string `mapstructure:"provider" yaml:"provider"`     // Embedding provider (only "openai" currently supported)
	Model      string `mapstructure:"model" yaml:"model"`           // Embedding model (e.g., text-embedding-3-small)
	Dimensions int    `mapstructure:"dimensions" yaml:"dimensions"` // Vector dimensions (must match model)
}
