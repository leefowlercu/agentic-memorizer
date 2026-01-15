package config

import "github.com/spf13/viper"

// Default configuration values.
const (
	// Logging defaults.
	DefaultLogLevel = "info"
	DefaultLogFile  = "~/.config/memorizer/memorizer.log"

	// Daemon configuration defaults.
	DefaultDaemonHTTPPort        = 7600
	DefaultDaemonHTTPBind        = "127.0.0.1"
	DefaultDaemonShutdownTimeout = 30 // seconds
	DefaultDaemonPIDFile         = "~/.config/memorizer/daemon.pid"
	DefaultDaemonRegistryPath    = "~/.config/memorizer/registry.db"
	DefaultDaemonRebuildInterval = 3600 // 1 hour in seconds, 0 = disabled
	DefaultDaemonMetricsInterval = 15   // seconds

	// Graph configuration defaults.
	DefaultGraphHost           = "localhost"
	DefaultGraphPort           = 6379
	DefaultGraphName           = "memorizer"
	DefaultGraphPasswordEnv    = "MEMORIZER_GRAPH_PASSWORD"
	DefaultGraphMaxRetries     = 3
	DefaultGraphRetryDelayMs   = 1000 // 1 second
	DefaultGraphWriteQueueSize = 1000

	// Semantic provider defaults.
	DefaultSemanticProvider  = "anthropic"
	DefaultSemanticModel     = "claude-sonnet-4-5-20250929"
	DefaultSemanticRateLimit = 10
	DefaultSemanticAPIKeyEnv = "ANTHROPIC_API_KEY"

	// Embeddings provider defaults.
	DefaultEmbeddingsEnabled    = true
	DefaultEmbeddingsProvider   = "openai"
	DefaultEmbeddingsModel      = "text-embedding-3-large"
	DefaultEmbeddingsDimensions = 3072
	DefaultEmbeddingsAPIKeyEnv  = "OPENAI_API_KEY"

	// Skip/include defaults.
	DefaultSkipHidden = true
)

// DefaultSkipExtensions is the default list of file extensions to skip.
var DefaultSkipExtensions = []string{
	// Compiled binaries
	".exe", ".dll", ".so", ".dylib", ".bin",
	// Object files
	".o", ".a", ".lib", ".obj",
	// Bytecode
	".pyc", ".pyo", ".class",
	// Archives
	".zip", ".tar", ".gz", ".tgz", ".rar", ".7z", ".jar", ".war",
	// Source maps
	".map",
	// Temporary
	".tmp", ".temp", ".bak", ".swp", ".swo",
	// Logs
	".log",
}

// DefaultSkipDirectories is the default list of directories to skip.
var DefaultSkipDirectories = []string{
	// Version control
	".git", ".svn", ".hg",
	// Package managers
	"node_modules", "bower_components",
	// Python
	"__pycache__", ".pytest_cache", ".mypy_cache", ".tox", "venv", ".venv",
	// Build outputs
	"dist", "build", "target",
	// IDE
	".idea", ".vscode",
	// Coverage
	"coverage", ".nyc_output", "htmlcov",
}

// DefaultSkipFiles is the default list of files to skip.
var DefaultSkipFiles = []string{
	// OS-generated
	".DS_Store", "Thumbs.db", "desktop.ini",
	// Lock files (package managers)
	"package-lock.json", "yarn.lock", "pnpm-lock.yaml",
	"Cargo.lock", "go.sum", "Gemfile.lock",
	"poetry.lock", "composer.lock",
	// Minified bundles (glob patterns)
	"*.min.js", "*.min.css", "*.bundle.js",
	// Editor artifacts (glob patterns)
	"4913",
	"#*",
	"*~",
}

// NewDefaultConfig returns a Config populated with all default values.
func NewDefaultConfig() Config {
	return Config{
		LogLevel: DefaultLogLevel,
		LogFile:  DefaultLogFile,
		Daemon: DaemonConfig{
			HTTPPort:        DefaultDaemonHTTPPort,
			HTTPBind:        DefaultDaemonHTTPBind,
			ShutdownTimeout: DefaultDaemonShutdownTimeout,
			PIDFile:         DefaultDaemonPIDFile,
			RegistryPath:    DefaultDaemonRegistryPath,
			RebuildInterval: DefaultDaemonRebuildInterval,
			Metrics: MetricsConfig{
				CollectionInterval: DefaultDaemonMetricsInterval,
			},
		},
		Graph: GraphConfig{
			Host:           DefaultGraphHost,
			Port:           DefaultGraphPort,
			Name:           DefaultGraphName,
			PasswordEnv:    DefaultGraphPasswordEnv,
			MaxRetries:     DefaultGraphMaxRetries,
			RetryDelayMs:   DefaultGraphRetryDelayMs,
			WriteQueueSize: DefaultGraphWriteQueueSize,
		},
		Semantic: SemanticConfig{
			Provider:  DefaultSemanticProvider,
			Model:     DefaultSemanticModel,
			RateLimit: DefaultSemanticRateLimit,
			APIKey:    nil,
			APIKeyEnv: DefaultSemanticAPIKeyEnv,
		},
		Embeddings: EmbeddingsConfig{
			Enabled:    DefaultEmbeddingsEnabled,
			Provider:   DefaultEmbeddingsProvider,
			Model:      DefaultEmbeddingsModel,
			Dimensions: DefaultEmbeddingsDimensions,
			APIKey:     nil,
			APIKeyEnv:  DefaultEmbeddingsAPIKeyEnv,
		},
		Defaults: DefaultsConfig{
			Skip: SkipDefaults{
				Extensions:  DefaultSkipExtensions,
				Directories: DefaultSkipDirectories,
				Files:       DefaultSkipFiles,
				Hidden:      DefaultSkipHidden,
			},
			Include: IncludeDefaults{
				Extensions:  []string{},
				Directories: []string{},
				Files:       []string{},
			},
		},
	}
}

// setDefaults registers all default configuration values with viper.
// Called during Init() before reading config files.
func setDefaults() {
	viper.SetDefault("log_level", DefaultLogLevel)
	viper.SetDefault("log_file", DefaultLogFile)

	// Daemon defaults
	viper.SetDefault("daemon.http_port", DefaultDaemonHTTPPort)
	viper.SetDefault("daemon.http_bind", DefaultDaemonHTTPBind)
	viper.SetDefault("daemon.shutdown_timeout", DefaultDaemonShutdownTimeout)
	viper.SetDefault("daemon.pid_file", DefaultDaemonPIDFile)
	viper.SetDefault("daemon.registry_path", DefaultDaemonRegistryPath)
	viper.SetDefault("daemon.rebuild_interval", DefaultDaemonRebuildInterval)
	viper.SetDefault("daemon.metrics.collection_interval", DefaultDaemonMetricsInterval)

	// Graph defaults
	viper.SetDefault("graph.host", DefaultGraphHost)
	viper.SetDefault("graph.port", DefaultGraphPort)
	viper.SetDefault("graph.name", DefaultGraphName)
	viper.SetDefault("graph.password_env", DefaultGraphPasswordEnv)
	viper.SetDefault("graph.max_retries", DefaultGraphMaxRetries)
	viper.SetDefault("graph.retry_delay_ms", DefaultGraphRetryDelayMs)
	viper.SetDefault("graph.write_queue_size", DefaultGraphWriteQueueSize)

	// Semantic defaults
	viper.SetDefault("semantic.provider", DefaultSemanticProvider)
	viper.SetDefault("semantic.model", DefaultSemanticModel)
	viper.SetDefault("semantic.rate_limit", DefaultSemanticRateLimit)
	viper.SetDefault("semantic.api_key_env", DefaultSemanticAPIKeyEnv)

	// Embeddings defaults
	viper.SetDefault("embeddings.enabled", DefaultEmbeddingsEnabled)
	viper.SetDefault("embeddings.provider", DefaultEmbeddingsProvider)
	viper.SetDefault("embeddings.model", DefaultEmbeddingsModel)
	viper.SetDefault("embeddings.dimensions", DefaultEmbeddingsDimensions)
	viper.SetDefault("embeddings.api_key_env", DefaultEmbeddingsAPIKeyEnv)

	// Skip/include defaults
	viper.SetDefault("defaults.skip.extensions", DefaultSkipExtensions)
	viper.SetDefault("defaults.skip.directories", DefaultSkipDirectories)
	viper.SetDefault("defaults.skip.files", DefaultSkipFiles)
	viper.SetDefault("defaults.skip.hidden", DefaultSkipHidden)
	viper.SetDefault("defaults.include.extensions", []string{})
	viper.SetDefault("defaults.include.directories", []string{})
	viper.SetDefault("defaults.include.files", []string{})
}
