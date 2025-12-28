package config

// Application directory and file names
const (
	AppDirName    = ".memorizer"
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
	OpenAIAPIKeyEnv = "OPENAI_API_KEY"
	GoogleAPIKeyEnv = "GOOGLE_API_KEY"
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
var DefaultSkipFiles = []string{"memorizer"}

// Provider-specific defaults
const (
	DefaultSemanticProvider = "claude"
	DefaultClaudeModel      = "claude-sonnet-4-5-20250929"
	DefaultOpenAIModel      = "gpt-5.2-chat-latest"
	DefaultGeminiModel      = "gemini-2.5-flash"
)

// Provider-specific rate limits (requests per minute)
const (
	DefaultClaudeRateLimit = 20  // Conservative for API quotas
	DefaultOpenAIRateLimit = 60  // Tier 1: 500 RPM, suggest conservative
	DefaultGeminiRateLimit = 100 // Paid tier: 2000 RPM, suggest conservative
)

// DefaultConfig provides sensible defaults for all configuration settings.
// INTERNAL settings (not shown in initialized config but available for power users):
// - semantic.max_tokens, semantic.max_file_size, semantic.skip_extensions, semantic.skip_files
// - daemon.debounce_ms, daemon.workers, daemon.full_rebuild_interval_minutes
// - graph.similarity_threshold, graph.max_similar_files
var DefaultConfig = Config{
	MemoryRoot: "~/" + AppDirName + "/" + MemoryDirName,
	Semantic: SemanticConfig{
		Enabled:         true, // Derived from API key presence in GetConfig()
		Provider:        DefaultSemanticProvider,
		Model:           DefaultClaudeModel,
		MaxTokens:       1500,
		Timeout:         30,                      // API request timeout in seconds
		EnableVision:    true,                    // Enable vision API for image analysis
		MaxFileSize:     10485760,                // 10 MB
		SkipExtensions:  DefaultSkipExtensions,
		SkipFiles:       DefaultSkipFiles,
		CacheDir:        "~/" + AppDirName + "/" + CacheDirName,
		RateLimitPerMin: DefaultClaudeRateLimit, // Default to Claude's conservative limit
	},
	Daemon: DaemonConfig{
		DebounceMs:                 500,
		Workers:                    3,
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
