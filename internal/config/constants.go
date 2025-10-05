package config

var DefaultSkipExtensions = []string{".zip", ".tar", ".gz", ".exe", ".bin", ".dmg", ".iso"}
var DefaultSkipFiles = []string{"memorizer", "memorizer-config.yaml"}

var DefaultConfig = Config{
	MemoryRoot: "~/.claude/memory",
	CacheDir:   "~/.claude/memory/.cache",
	Claude: ClaudeConfig{
		APIKeyEnv:      "ANTHROPIC_API_KEY",
		Model:          "claude-sonnet-4-5-20250929",
		MaxTokens:      1500,
		EnableVision:   true,
		TimeoutSeconds: 30,
	},
	Output: OutputConfig{
		Format:         "markdown",
		Verbose:        false,
		ShowRecentDays: 7,
	},
	Analysis: AnalysisConfig{
		Enable:         true,
		MaxFileSize:    10485760, // 10 MB
		Parallel:       3,
		SkipExtensions: DefaultSkipExtensions,
		SkipFiles:      DefaultSkipFiles,
	},
}
