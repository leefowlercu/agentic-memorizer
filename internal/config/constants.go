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
	VoyageAPIKeyEnv = "VOYAGE_API_KEY"
	// Internal settings - not configurable
	EmbeddingsCacheEnabled = true // Always enabled for performance
	EmbeddingsBatchSize    = 100  // Optimized for batch processing
)

// Embeddings defaults - can be overridden in config.yaml
const (
	DefaultEmbeddingsProvider         = "openai"
	DefaultEmbeddingsModel            = "text-embedding-3-small"
	DefaultEmbeddingsDimensions       = 1536
	DefaultOpenAIEmbeddingsModel      = "text-embedding-3-small"
	DefaultOpenAIEmbeddingsDimensions = 1536
)

// Voyage embedding defaults
const (
	DefaultVoyageEmbeddingsModel      = "voyage-3"
	DefaultVoyageEmbeddingsDimensions = 1024
)

// Gemini embedding defaults
const (
	DefaultGeminiEmbeddingsModel      = "text-embedding-004"
	DefaultGeminiEmbeddingsDimensions = 768
)

// embeddingModelDimensions maps provider:model to dimension count
var embeddingModelDimensions = map[string]map[string]int{
	"openai": {
		"text-embedding-3-small": 1536,
		"text-embedding-3-large": 3072,
		"text-embedding-ada-002": 1536,
	},
	"voyage": {
		"voyage-3":         1024,
		"voyage-3-lite":    512,
		"voyage-code-3":    1024,
		"voyage-finance-2": 1024,
		"voyage-law-2":     1024,
	},
	"gemini": {
		"text-embedding-004": 768,
		"embedding-001":      768,
	},
}

// GetEmbeddingModelDimensions returns the dimension count for a provider/model pair.
// Returns 0 if the model is not recognized.
func GetEmbeddingModelDimensions(provider, model string) int {
	if models, ok := embeddingModelDimensions[provider]; ok {
		if dims, ok := models[model]; ok {
			return dims
		}
	}
	return 0
}

// Hardcoded Output settings (convention over configuration)
const (
	OutputShowRecentDays = 7
)

// Default skip patterns for file watching
var DefaultSkipExtensions = []string{".zip", ".tar", ".gz", ".exe", ".bin", ".dmg", ".iso"}
var DefaultSkipFiles = []string{"memorizer"}
var DefaultSkipDirs = []string{"node_modules", "vendor", "__pycache__", ".venv"}

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
// - semantic.max_tokens, semantic.max_file_size
// - daemon.debounce_ms, daemon.workers, daemon.full_rebuild_interval_minutes, daemon.skip_*
// - graph.similarity_threshold, graph.max_similar_files
var DefaultConfig = Config{
	Memory: MemoryConfig{
		Root: "~/" + AppDirName + "/" + MemoryDirName,
	},
	Semantic: SemanticConfig{
		Enabled:         true, // Derived from API key presence in GetConfig()
		Provider:        DefaultSemanticProvider,
		Model:           DefaultClaudeModel,
		MaxTokens:       1500,
		Timeout:         30,       // API request timeout in seconds
		EnableVision:    true,     // Enable vision API for image analysis
		MaxFileSize:     10485760, // 10 MB
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
		SkipHidden:                 true,
		SkipDirs:                   DefaultSkipDirs,
		SkipFiles:                  DefaultSkipFiles,
		SkipExtensions:             DefaultSkipExtensions,
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
