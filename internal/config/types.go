package config

type Config struct {
	MemoryRoot string         `mapstructure:"memory_root"`
	CacheDir   string         `mapstructure:"cache_dir"`
	Claude     ClaudeConfig   `mapstructure:"claude"`
	Output     OutputConfig   `mapstructure:"output"`
	Analysis   AnalysisConfig `mapstructure:"analysis"`
}

type ClaudeConfig struct {
	APIKey         string `mapstructure:"api_key"`
	APIKeyEnv      string `mapstructure:"api_key_env"`
	Model          string `mapstructure:"model"`
	MaxTokens      int    `mapstructure:"max_tokens"`
	EnableVision   bool   `mapstructure:"enable_vision"`
	TimeoutSeconds int    `mapstructure:"timeout_seconds"`
}

type OutputConfig struct {
	Format         string `mapstructure:"format"`
	WrapJSON       bool   `mapstructure:"wrap_json"`
	Verbose        bool   `mapstructure:"verbose"`
	ShowRecentDays int    `mapstructure:"show_recent_days"`
}

type AnalysisConfig struct {
	Enable         bool     `mapstructure:"enable"`
	MaxFileSize    int64    `mapstructure:"max_file_size"`
	Parallel       int      `mapstructure:"parallel"`
	SkipExtensions []string `mapstructure:"skip_extensions"`
	SkipFiles      []string `mapstructure:"skip_files"`
}
