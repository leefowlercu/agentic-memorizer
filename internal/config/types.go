package config

type Config struct {
	MemoryRoot   string             `mapstructure:"memory_root" yaml:"memory_root"`
	Claude       ClaudeConfig       `mapstructure:"claude" yaml:"claude"`
	Output       OutputConfig       `mapstructure:"output" yaml:"output"`
	Analysis     AnalysisConfig     `mapstructure:"analysis" yaml:"analysis"`
	Daemon       DaemonConfig       `mapstructure:"daemon" yaml:"daemon"`
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
	Verbose        bool   `mapstructure:"verbose" yaml:"verbose"`
	ShowRecentDays int    `mapstructure:"show_recent_days" yaml:"show_recent_days"`
}

type AnalysisConfig struct {
	Enable         bool     `mapstructure:"enable" yaml:"enable"`
	MaxFileSize    int64    `mapstructure:"max_file_size" yaml:"max_file_size"`
	Parallel       int      `mapstructure:"parallel" yaml:"parallel"`
	SkipExtensions []string `mapstructure:"skip_extensions" yaml:"skip_extensions"`
	SkipFiles      []string `mapstructure:"skip_files" yaml:"skip_files"`
	CacheDir       string   `mapstructure:"cache_dir" yaml:"cache_dir"`
}

type DaemonConfig struct {
	Enabled                    bool   `mapstructure:"enabled" yaml:"enabled"`
	DebounceMs                 int    `mapstructure:"debounce_ms" yaml:"debounce_ms"`
	Workers                    int    `mapstructure:"workers" yaml:"workers"`
	RateLimitPerMin            int    `mapstructure:"rate_limit_per_min" yaml:"rate_limit_per_min"`
	FullRebuildIntervalMinutes int    `mapstructure:"full_rebuild_interval_minutes" yaml:"full_rebuild_interval_minutes"`
	HealthCheckPort            int    `mapstructure:"health_check_port" yaml:"health_check_port"`
	LogFile                    string `mapstructure:"log_file" yaml:"log_file"`
	LogLevel                   string `mapstructure:"log_level" yaml:"log_level"`
}

// IntegrationsConfig represents the complete integrations configuration section
type IntegrationsConfig struct {
	Enabled []string                     `mapstructure:"enabled" yaml:"enabled"`
	Configs map[string]IntegrationConfig `mapstructure:"configs" yaml:"configs"`
}

// IntegrationConfig represents the configuration for a specific integration
type IntegrationConfig struct {
	Type         string         `mapstructure:"type" yaml:"type"`
	Enabled      bool           `mapstructure:"enabled" yaml:"enabled"`
	OutputFormat string         `mapstructure:"output_format" yaml:"output_format"`
	Settings     map[string]any `mapstructure:"settings" yaml:"settings"`
}
