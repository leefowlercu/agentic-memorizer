package config

const (
	AppDirName    = ".agentic-memorizer"
	MemoryDirName = "memory"
	CacheDirName  = ".cache"
	ConfigFile    = "config.yaml"
	IndexFile     = "index.json"
	DaemonLogFile = "daemon.log"
	DaemonPIDFile = "daemon.pid"
	MCPLogFile    = "mcp.log"
)

var DefaultSkipExtensions = []string{".zip", ".tar", ".gz", ".exe", ".bin", ".dmg", ".iso"}
var DefaultSkipFiles = []string{"agentic-memorizer"}

var DefaultConfig = Config{
	MemoryRoot: "~/" + AppDirName + "/" + MemoryDirName,
	Claude: ClaudeConfig{
		APIKeyEnv:      "ANTHROPIC_API_KEY",
		Model:          "claude-sonnet-4-5-20250929",
		MaxTokens:      1500,
		EnableVision:   true,
		TimeoutSeconds: 30,
	},
	Output: OutputConfig{
		Format:         "xml",
		ShowRecentDays: 7,
	},
	Analysis: AnalysisConfig{
		Enable:         true,
		MaxFileSize:    10485760, // 10 MB
		SkipExtensions: DefaultSkipExtensions,
		SkipFiles:      DefaultSkipFiles,
		CacheDir:       "~/" + AppDirName + "/" + CacheDirName,
	},
	Daemon: DaemonConfig{
		DebounceMs:                 500,
		Workers:                    3,
		RateLimitPerMin:            20,
		FullRebuildIntervalMinutes: 60,
		HTTPPort:                   0, // Disabled by default
		LogFile:                    "~/" + AppDirName + "/" + DaemonLogFile,
		LogLevel:                   "info",
	},
	MCP: MCPConfig{
		LogFile:      "~/" + AppDirName + "/" + MCPLogFile,
		LogLevel:     "info",
		DaemonSSEURL: "", // No automatic connection - must be explicitly configured
	},
	Integrations: IntegrationsConfig{
		Enabled: []string{}, // Empty by default - populated during init/setup/remove commands
	},
}
