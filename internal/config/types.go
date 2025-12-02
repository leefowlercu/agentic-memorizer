package config

type Config struct {
	MemoryRoot   string             `mapstructure:"memory_root" yaml:"memory_root"`
	Claude       ClaudeConfig       `mapstructure:"claude" yaml:"claude"`
	Output       OutputConfig       `mapstructure:"output" yaml:"output"`
	Analysis     AnalysisConfig     `mapstructure:"analysis" yaml:"analysis"`
	Daemon       DaemonConfig       `mapstructure:"daemon" yaml:"daemon"`
	MCP          MCPConfig          `mapstructure:"mcp" yaml:"mcp"`
	Integrations IntegrationsConfig `mapstructure:"integrations" yaml:"integrations"`
	Graph        GraphConfig        `mapstructure:"graph" yaml:"graph"`
	Embeddings   EmbeddingsConfig   `mapstructure:"embeddings" yaml:"embeddings"`
}

type ClaudeConfig struct {
	APIKey         string `mapstructure:"api_key" yaml:"api_key"`
	APIKeyEnv      string `mapstructure:"api_key_env" yaml:"api_key_env"`
	Model          string `mapstructure:"model" yaml:"model"`
	MaxTokens      int    `mapstructure:"max_tokens" yaml:"max_tokens"`
	EnableVision   bool   `mapstructure:"enable_vision" yaml:"enable_vision"`
	TimeoutSeconds int    `mapstructure:"timeout_seconds" yaml:"timeout_seconds"`
}

type OutputConfig struct {
	Format         string `mapstructure:"format" yaml:"format"`
	ShowRecentDays int    `mapstructure:"show_recent_days" yaml:"show_recent_days"`
}

type AnalysisConfig struct {
	Enable         bool     `mapstructure:"enable" yaml:"enable"`
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
	LogFile   string `mapstructure:"log_file" yaml:"log_file"`
	LogLevel  string `mapstructure:"log_level" yaml:"log_level"`
	DaemonURL string `mapstructure:"daemon_url" yaml:"daemon_url"` // Base URL for Daemon HTTP API
}

// IntegrationsConfig represents the complete integrations configuration section.
// The Enabled list tracks which integrations have been configured via setup commands.
// Integration-specific configuration (hooks, tools, etc.) is stored in framework-specific
// files (e.g., ~/.claude.json, ~/.claude/settings.json) rather than in this config file.
type IntegrationsConfig struct {
	Enabled []string `mapstructure:"enabled" yaml:"enabled"`
}

// GraphConfig contains FalkorDB knowledge graph configuration.
// FalkorDB is the required storage backend - there is no option to disable it.
type GraphConfig struct {
	Host                string  `mapstructure:"host" yaml:"host"`
	Port                int     `mapstructure:"port" yaml:"port"`
	Database            string  `mapstructure:"database" yaml:"database"`
	Password            string  `mapstructure:"password" yaml:"password"`
	PasswordEnv         string  `mapstructure:"password_env" yaml:"password_env"`
	SimilarityThreshold float64 `mapstructure:"similarity_threshold" yaml:"similarity_threshold"`
	MaxSimilarFiles     int     `mapstructure:"max_similar_files" yaml:"max_similar_files"`
}

// EmbeddingsConfig contains embedding provider configuration
type EmbeddingsConfig struct {
	Enabled       bool   `mapstructure:"enabled" yaml:"enabled"`
	Provider      string `mapstructure:"provider" yaml:"provider"`
	APIKey        string `mapstructure:"api_key" yaml:"api_key"`
	APIKeyEnv     string `mapstructure:"api_key_env" yaml:"api_key_env"`
	Model         string `mapstructure:"model" yaml:"model"`
	Dimensions    int    `mapstructure:"dimensions" yaml:"dimensions"`
	CacheEnabled  bool   `mapstructure:"cache_enabled" yaml:"cache_enabled"`
	BatchSize     int    `mapstructure:"batch_size" yaml:"batch_size"`
}
