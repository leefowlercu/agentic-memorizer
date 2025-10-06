package config

type Config struct {
	MemoryRoot string         `mapstructure:"memory_root" yaml:"memory_root"`
	CacheDir   string         `mapstructure:"cache_dir" yaml:"cache_dir"`
	Claude     ClaudeConfig   `mapstructure:"claude" yaml:"claude"`
	Output     OutputConfig   `mapstructure:"output" yaml:"output"`
	Analysis   AnalysisConfig `mapstructure:"analysis" yaml:"analysis"`
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
	WrapJSON       bool   `mapstructure:"wrap_json" yaml:"wrap_json"`
	Verbose        bool   `mapstructure:"verbose" yaml:"verbose"`
	ShowRecentDays int    `mapstructure:"show_recent_days" yaml:"show_recent_days"`
}

type AnalysisConfig struct {
	Enable         bool     `mapstructure:"enable" yaml:"enable"`
	MaxFileSize    int64    `mapstructure:"max_file_size" yaml:"max_file_size"`
	Parallel       int      `mapstructure:"parallel" yaml:"parallel"`
	SkipExtensions []string `mapstructure:"skip_extensions" yaml:"skip_extensions"`
	SkipFiles      []string `mapstructure:"skip_files" yaml:"skip_files"`
}
