package config

// Application directory and file names
const (
	AppDirName    = ".agentic-memorizer"
	MemoryDirName = "memory"
	CacheDirName  = ".cache"
	ConfigFile    = "config.yaml"
	DaemonLogFile = "daemon.log"
	DaemonPIDFile = "daemon.pid"
	MCPLogFile    = "mcp.log"
)

// Hardcoded environment variable names (convention over configuration)
// These define which environment variables are checked for credentials.
// The actual values (API keys, passwords) come from the environment or config file.
const (
	ClaudeAPIKeyEnv = "ANTHROPIC_API_KEY"
)

// Hardcoded Graph environment variable names
const (
	GraphPasswordEnv = "FALKORDB_PASSWORD"
)

// Hardcoded Embeddings environment variable names and internal settings
const (
	EmbeddingsAPIKeyEnv = "OPENAI_API_KEY"
	// Internal settings - not configurable
	EmbeddingsCacheEnabled = true // Always enabled for performance
	EmbeddingsBatchSize    = 100  // Optimized for OpenAI API rate limits
)

// Embeddings defaults - can be overridden in config.yaml
const (
	DefaultEmbeddingsProvider   = "openai"
	DefaultEmbeddingsModel      = "text-embedding-3-small"
	DefaultEmbeddingsDimensions = 1536
)

// Hardcoded Output settings (convention over configuration)
const (
	OutputShowRecentDays = 7
)

// Default skip patterns for analysis
var DefaultSkipExtensions = []string{".zip", ".tar", ".gz", ".exe", ".bin", ".dmg", ".iso"}
var DefaultSkipFiles = []string{"agentic-memorizer"}

// DefaultConfig provides sensible defaults for all configuration settings.
// INTERNAL settings (not shown in initialized config but available for power users):
// - claude.max_tokens, analysis.max_file_size, analysis.skip_extensions, analysis.skip_files
// - daemon.debounce_ms, daemon.workers, daemon.rate_limit_per_min, daemon.full_rebuild_interval_minutes
// - graph.similarity_threshold, graph.max_similar_files, integrations.enabled
var DefaultConfig = Config{
	MemoryRoot: "~/" + AppDirName + "/" + MemoryDirName,
	Claude: ClaudeConfig{
		Model:        "claude-sonnet-4-5-20250929",
		MaxTokens:    1500,
		Timeout:      30,   // API request timeout in seconds
		EnableVision: true, // Enable vision API for image analysis
	},
	Analysis: AnalysisConfig{
		Enabled:        true,     // Derived from API key presence in GetConfig()
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
		LogFile:    "~/" + AppDirName + "/" + MCPLogFile,
		LogLevel:   "info",
		DaemonHost: "localhost",
		DaemonPort: 0, // Disabled by default; set during initialize from daemon.http_port
	},
	Integrations: IntegrationsConfig{
		Enabled: []string{}, // Empty by default - populated during init/setup/remove commands
	},
	Graph: GraphConfig{
		Host:                "localhost",
		Port:                6379,
		Database:            "memorizer",
		Password:            "",
		SimilarityThreshold: 0.7,
		MaxSimilarFiles:     10,
	},
	Embeddings: EmbeddingsConfig{
		Enabled:    false, // Derived from API key presence in GetConfig()
		APIKey:     "",
		Provider:   DefaultEmbeddingsProvider,
		Model:      DefaultEmbeddingsModel,
		Dimensions: DefaultEmbeddingsDimensions,
	},
}
