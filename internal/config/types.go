package config

type Config struct {
	MemoryRoot   string             `mapstructure:"memory_root" yaml:"memory_root"`
	Claude       ClaudeConfig       `mapstructure:"claude" yaml:"claude"`
	Output       OutputConfig       `mapstructure:"output" yaml:"output"`
	Analysis     AnalysisConfig     `mapstructure:"analysis" yaml:"analysis"`
	Daemon       DaemonConfig       `mapstructure:"daemon" yaml:"daemon"`
	MCP          MCPConfig          `mapstructure:"mcp" yaml:"mcp"`
	Integrations IntegrationsConfig `mapstructure:"integrations" yaml:"integrations"`
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
	HealthCheckPort            int    `mapstructure:"health_check_port" yaml:"health_check_port"`
	SSENotifyPort              int    `mapstructure:"sse_notify_port" yaml:"sse_notify_port"`
	LogFile                    string `mapstructure:"log_file" yaml:"log_file"`
	LogLevel                   string `mapstructure:"log_level" yaml:"log_level"`
}

type MCPConfig struct {
	LogFile      string `mapstructure:"log_file" yaml:"log_file"`
	LogLevel     string `mapstructure:"log_level" yaml:"log_level"`
	DaemonSSEURL string `mapstructure:"daemon_sse_url" yaml:"daemon_sse_url"`
}

// IntegrationsConfig represents the complete integrations configuration section.
// The Enabled list tracks which integrations have been configured via setup commands.
// Integration-specific configuration (hooks, tools, etc.) is stored in framework-specific
// files (e.g., ~/.claude.json, ~/.claude/settings.json) rather than in this config file.
type IntegrationsConfig struct {
	Enabled []string `mapstructure:"enabled" yaml:"enabled"`
}
