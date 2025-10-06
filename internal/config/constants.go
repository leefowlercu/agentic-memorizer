package config

var DefaultSkipExtensions = []string{".zip", ".tar", ".gz", ".exe", ".bin", ".dmg", ".iso"}
var DefaultSkipFiles = []string{"agentic-memorizer"}

var DefaultConfig = Config{
	MemoryRoot: "~/.agentic-memorizer/memory",
	CacheDir:   "~/.agentic-memorizer/memory/.cache",
	Claude: ClaudeConfig{
		APIKeyEnv:      "ANTHROPIC_API_KEY",
		Model:          "claude-sonnet-4-5-20250929",
		MaxTokens:      1500,
		EnableVision:   true,
		TimeoutSeconds: 30,
	},
	Output: OutputConfig{
		Format:         "xml",
		WrapJSON:       false,
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
