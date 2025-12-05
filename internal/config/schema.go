package config

// ConfigSchema describes all configuration settings including
// configurable settings (minimal and advanced) and hardcoded conventions.
type ConfigSchema struct {
	Sections  []SchemaSection
	Hardcoded []HardcodedSetting
}

// SchemaSection represents a config section (claude, daemon, etc.)
type SchemaSection struct {
	Name        string
	Description string
	Fields      []SchemaField
}

// SchemaField describes a single configuration field
type SchemaField struct {
	Name        string
	Type        string // "string", "int", "bool", "float64", "[]string"
	Default     any
	Tier        string // "minimal" or "advanced"
	HotReload   bool   // true if hot-reloadable without daemon restart
	Description string
}

// HardcodedSetting describes a non-configurable constant
type HardcodedSetting struct {
	Name   string
	Value  any
	Reason string
}

// GetConfigSchema returns the complete configuration schema
func GetConfigSchema() *ConfigSchema {
	return &ConfigSchema{
		Sections: []SchemaSection{
			{
				Name:        "memory_root",
				Description: "Root directory for memory files",
				Fields: []SchemaField{
					{
						Name:        "memory_root",
						Type:        "string",
						Default:     DefaultConfig.MemoryRoot,
						Tier:        "minimal",
						HotReload:   false,
						Description: "Directory containing files to index (requires daemon restart)",
					},
				},
			},
			{
				Name:        "claude",
				Description: "Claude API configuration",
				Fields: []SchemaField{
					{
						Name:        "api_key",
						Type:        "string",
						Default:     "",
						Tier:        "minimal",
						HotReload:   true,
						Description: "Claude API key (or use ANTHROPIC_API_KEY env var)",
					},
					{
						Name:        "model",
						Type:        "string",
						Default:     DefaultConfig.Claude.Model,
						Tier:        "minimal",
						HotReload:   true,
						Description: "Claude model to use for semantic analysis",
					},
					{
						Name:        "max_tokens",
						Type:        "int",
						Default:     DefaultConfig.Claude.MaxTokens,
						Tier:        "advanced",
						HotReload:   true,
						Description: "Maximum tokens per API request (1-8192)",
					},
					{
						Name:        "timeout",
						Type:        "int",
						Default:     DefaultConfig.Claude.Timeout,
						Tier:        "advanced",
						HotReload:   true,
						Description: "API request timeout in seconds (5-300)",
					},
					{
						Name:        "enable_vision",
						Type:        "bool",
						Default:     DefaultConfig.Claude.EnableVision,
						Tier:        "advanced",
						HotReload:   true,
						Description: "Enable vision API for image analysis",
					},
				},
			},
			{
				Name:        "analysis",
				Description: "File analysis settings",
				Fields: []SchemaField{
					{
						Name:        "max_file_size",
						Type:        "int",
						Default:     DefaultConfig.Analysis.MaxFileSize,
						Tier:        "advanced",
						HotReload:   true,
						Description: "Maximum file size in bytes for analysis (default: 10MB)",
					},
					{
						Name:        "skip_extensions",
						Type:        "[]string",
						Default:     DefaultConfig.Analysis.SkipExtensions,
						Tier:        "advanced",
						HotReload:   true,
						Description: "File extensions to skip during analysis",
					},
					{
						Name:        "skip_files",
						Type:        "[]string",
						Default:     DefaultConfig.Analysis.SkipFiles,
						Tier:        "advanced",
						HotReload:   true,
						Description: "Filenames to skip during analysis",
					},
					{
						Name:        "cache_dir",
						Type:        "string",
						Default:     DefaultConfig.Analysis.CacheDir,
						Tier:        "advanced",
						HotReload:   false,
						Description: "Directory for analysis cache (requires daemon restart)",
					},
				},
			},
			{
				Name:        "daemon",
				Description: "Background daemon configuration",
				Fields: []SchemaField{
					{
						Name:        "http_port",
						Type:        "int",
						Default:     DefaultConfig.Daemon.HTTPPort,
						Tier:        "minimal",
						HotReload:   true,
						Description: "HTTP API port (0 to disable)",
					},
					{
						Name:        "workers",
						Type:        "int",
						Default:     DefaultConfig.Daemon.Workers,
						Tier:        "advanced",
						HotReload:   true,
						Description: "Number of concurrent worker threads (1-20)",
					},
					{
						Name:        "rate_limit_per_min",
						Type:        "int",
						Default:     DefaultConfig.Daemon.RateLimitPerMin,
						Tier:        "advanced",
						HotReload:   true,
						Description: "Maximum Claude API calls per minute (1-200)",
					},
					{
						Name:        "debounce_ms",
						Type:        "int",
						Default:     DefaultConfig.Daemon.DebounceMs,
						Tier:        "advanced",
						HotReload:   true,
						Description: "File change debounce delay in milliseconds (0-10000)",
					},
					{
						Name:        "full_rebuild_interval_minutes",
						Type:        "int",
						Default:     DefaultConfig.Daemon.FullRebuildIntervalMinutes,
						Tier:        "advanced",
						HotReload:   true,
						Description: "Minutes between full index rebuilds (0 to disable)",
					},
					{
						Name:        "log_file",
						Type:        "string",
						Default:     DefaultConfig.Daemon.LogFile,
						Tier:        "advanced",
						HotReload:   false,
						Description: "Log file path (requires daemon restart)",
					},
					{
						Name:        "log_level",
						Type:        "string",
						Default:     DefaultConfig.Daemon.LogLevel,
						Tier:        "advanced",
						HotReload:   true,
						Description: "Log level (debug, info, warn, error)",
					},
				},
			},
			{
				Name:        "mcp",
				Description: "MCP server configuration",
				Fields: []SchemaField{
					{
						Name:        "log_file",
						Type:        "string",
						Default:     DefaultConfig.MCP.LogFile,
						Tier:        "advanced",
						HotReload:   false,
						Description: "MCP server log file path (requires MCP restart)",
					},
					{
						Name:        "log_level",
						Type:        "string",
						Default:     DefaultConfig.MCP.LogLevel,
						Tier:        "advanced",
						HotReload:   false,
						Description: "MCP server log level (requires MCP restart)",
					},
					{
						Name:        "daemon_host",
						Type:        "string",
						Default:     DefaultConfig.MCP.DaemonHost,
						Tier:        "advanced",
						HotReload:   false,
						Description: "Daemon HTTP host for MCP server (requires MCP restart)",
					},
					{
						Name:        "daemon_port",
						Type:        "int",
						Default:     DefaultConfig.MCP.DaemonPort,
						Tier:        "advanced",
						HotReload:   false,
						Description: "Daemon HTTP port for MCP server (requires MCP restart)",
					},
				},
			},
			{
				Name:        "graph",
				Description: "FalkorDB knowledge graph configuration",
				Fields: []SchemaField{
					{
						Name:        "host",
						Type:        "string",
						Default:     DefaultConfig.Graph.Host,
						Tier:        "minimal",
						HotReload:   false,
						Description: "FalkorDB host (requires daemon restart)",
					},
					{
						Name:        "port",
						Type:        "int",
						Default:     DefaultConfig.Graph.Port,
						Tier:        "minimal",
						HotReload:   false,
						Description: "FalkorDB port (default: 6379)",
					},
					{
						Name:        "database",
						Type:        "string",
						Default:     DefaultConfig.Graph.Database,
						Tier:        "advanced",
						HotReload:   false,
						Description: "Graph database name",
					},
					{
						Name:        "password",
						Type:        "string",
						Default:     "",
						Tier:        "advanced",
						HotReload:   false,
						Description: "FalkorDB password (or use FALKORDB_PASSWORD env var)",
					},
					{
						Name:        "similarity_threshold",
						Type:        "float64",
						Default:     DefaultConfig.Graph.SimilarityThreshold,
						Tier:        "advanced",
						HotReload:   true,
						Description: "Similarity threshold for related files (0.0-1.0)",
					},
					{
						Name:        "max_similar_files",
						Type:        "int",
						Default:     DefaultConfig.Graph.MaxSimilarFiles,
						Tier:        "advanced",
						HotReload:   true,
						Description: "Maximum related files to return (1-100)",
					},
				},
			},
			{
				Name:        "embeddings",
				Description: "Embedding provider configuration",
				Fields: []SchemaField{
					{
						Name:        "api_key",
						Type:        "string",
						Default:     "",
						Tier:        "minimal",
						HotReload:   true,
						Description: "OpenAI API key for embeddings (or use OPENAI_API_KEY env var)",
					},
					{
						Name:        "provider",
						Type:        "string",
						Default:     DefaultConfig.Embeddings.Provider,
						Tier:        "advanced",
						HotReload:   true,
						Description: "Embedding provider (only 'openai' currently supported)",
					},
					{
						Name:        "model",
						Type:        "string",
						Default:     DefaultConfig.Embeddings.Model,
						Tier:        "advanced",
						HotReload:   true,
						Description: "Embedding model (text-embedding-3-small, text-embedding-3-large, text-embedding-ada-002)",
					},
					{
						Name:        "dimensions",
						Type:        "int",
						Default:     DefaultConfig.Embeddings.Dimensions,
						Tier:        "advanced",
						HotReload:   true,
						Description: "Vector dimensions (must match model: 1536 for small/ada-002, 3072 for large)",
					},
				},
			},
			{
				Name:        "integrations",
				Description: "Integration framework configuration",
				Fields: []SchemaField{
					{
						Name:        "enabled",
						Type:        "[]string",
						Default:     DefaultConfig.Integrations.Enabled,
						Tier:        "advanced",
						HotReload:   false,
						Description: "List of enabled integrations (managed by setup/remove commands)",
					},
				},
			},
		},
		Hardcoded: []HardcodedSetting{
			{
				Name:   "ClaudeAPIKeyEnv",
				Value:  ClaudeAPIKeyEnv,
				Reason: "Standard Anthropic convention",
			},
			{
				Name:   "EmbeddingsAPIKeyEnv",
				Value:  EmbeddingsAPIKeyEnv,
				Reason: "Standard OpenAI convention",
			},
			{
				Name:   "GraphPasswordEnv",
				Value:  GraphPasswordEnv,
				Reason: "Standard FalkorDB convention",
			},
			{
				Name:   "AppDirName",
				Value:  AppDirName,
				Reason: "Application directory convention",
			},
			{
				Name:   "ConfigFile",
				Value:  ConfigFile,
				Reason: "Configuration file naming convention",
			},
			{
				Name:   "IndexFile",
				Value:  IndexFile,
				Reason: "Index file naming convention",
			},
			{
				Name:   "DaemonLogFile",
				Value:  DaemonLogFile,
				Reason: "Daemon log file naming convention",
			},
			{
				Name:   "DaemonPIDFile",
				Value:  DaemonPIDFile,
				Reason: "Daemon PID file naming convention",
			},
			{
				Name:   "MCPLogFile",
				Value:  MCPLogFile,
				Reason: "MCP log file naming convention",
			},
			{
				Name:   "EmbeddingsCacheEnabled",
				Value:  EmbeddingsCacheEnabled,
				Reason: "Always enabled for performance - no use case for disabling",
			},
			{
				Name:   "EmbeddingsBatchSize",
				Value:  EmbeddingsBatchSize,
				Reason: "Optimized for OpenAI API rate limits",
			},
			{
				Name:   "OutputShowRecentDays",
				Value:  OutputShowRecentDays,
				Reason: "Default recent files window (7 days)",
			},
		},
	}
}
