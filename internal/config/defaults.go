package config

import "github.com/spf13/viper"

// Default configuration values.
const (
	LogLevel = "info"
	LogFile  = "~/.config/memorizer/memorizer.log"

	// Daemon configuration defaults.
	DaemonHTTPPort        = 7600
	DaemonHTTPBind        = "127.0.0.1"
	DaemonShutdownTimeout = 30 // seconds
	DaemonPIDFile         = "~/.config/memorizer/daemon.pid"

	// Database configuration defaults.
	DatabaseRegistryPath = "~/.config/memorizer/registry.db"

	// Handler configuration defaults (sizes in bytes).
	HandlerTextMaxSize           = 10 * 1024 * 1024  // 10MB
	HandlerImageMaxSize          = 20 * 1024 * 1024  // 20MB
	HandlerImageRequireVision    = true
	HandlerPDFMaxSize            = 50 * 1024 * 1024  // 50MB
	HandlerPDFExtractText        = true
	HandlerPDFUseVision          = true
	HandlerRichDocumentMaxSize   = 50 * 1024 * 1024  // 50MB
	HandlerStructuredDataMaxSize = 50 * 1024 * 1024  // 50MB
	HandlerStructuredSampleSize  = 10

	// Watcher configuration defaults (durations in milliseconds).
	WatcherDebounceWindow    = 500  // 500ms
	WatcherDeleteGracePeriod = 5000 // 5 seconds

	// Provider configuration defaults.
	ProviderDefaultSemantic   = "anthropic"
	ProviderDefaultEmbeddings = "openai-embeddings"
	ProviderSemanticVersion   = 1
	ProviderEmbeddingsVersion = 1

	// Cache configuration defaults.
	CacheBaseDir  = "~/.config/memorizer/cache"
	CacheMaxSize  = 0 // 0 = unlimited
	CacheVersion  = 1
	CacheEnabled  = true
	CacheTTLHours = 0 // 0 = no expiry

	// Graph configuration defaults.
	GraphHost           = "localhost"
	GraphPort           = 6379
	GraphName           = "memorizer"
	GraphPasswordEnv    = "MEMORIZER_GRAPH_PASSWORD"
	GraphMaxRetries     = 3
	GraphRetryDelayMs   = 1000 // 1 second
	GraphWriteQueueSize = 1000
)

// setDefaults registers all default configuration values with viper.
// Called during Init() before reading config files.
func setDefaults() {
	viper.SetDefault("log_level", LogLevel)
	viper.SetDefault("log_file", LogFile)

	// Daemon defaults
	viper.SetDefault("daemon.http_port", DaemonHTTPPort)
	viper.SetDefault("daemon.http_bind", DaemonHTTPBind)
	viper.SetDefault("daemon.shutdown_timeout", DaemonShutdownTimeout)
	viper.SetDefault("daemon.pid_file", DaemonPIDFile)

	// Database defaults
	viper.SetDefault("database.registry_path", DatabaseRegistryPath)

	// Handler defaults
	viper.SetDefault("handlers.text.max_size", HandlerTextMaxSize)
	viper.SetDefault("handlers.image.max_size", HandlerImageMaxSize)
	viper.SetDefault("handlers.image.require_vision", HandlerImageRequireVision)
	viper.SetDefault("handlers.pdf.max_size", HandlerPDFMaxSize)
	viper.SetDefault("handlers.pdf.extract_text", HandlerPDFExtractText)
	viper.SetDefault("handlers.pdf.use_vision", HandlerPDFUseVision)
	viper.SetDefault("handlers.rich_document.max_size", HandlerRichDocumentMaxSize)
	viper.SetDefault("handlers.structured_data.max_size", HandlerStructuredDataMaxSize)
	viper.SetDefault("handlers.structured_data.sample_size", HandlerStructuredSampleSize)

	// Watcher defaults
	viper.SetDefault("watcher.debounce_window", WatcherDebounceWindow)
	viper.SetDefault("watcher.delete_grace_period", WatcherDeleteGracePeriod)

	// Provider defaults
	viper.SetDefault("providers.default_semantic", ProviderDefaultSemantic)
	viper.SetDefault("providers.default_embeddings", ProviderDefaultEmbeddings)
	viper.SetDefault("providers.semantic_version", ProviderSemanticVersion)
	viper.SetDefault("providers.embeddings_version", ProviderEmbeddingsVersion)

	// Cache defaults
	viper.SetDefault("cache.base_dir", CacheBaseDir)
	viper.SetDefault("cache.max_size", CacheMaxSize)
	viper.SetDefault("cache.version", CacheVersion)
	viper.SetDefault("cache.enabled", CacheEnabled)
	viper.SetDefault("cache.ttl_hours", CacheTTLHours)

	// Graph defaults
	viper.SetDefault("graph.host", GraphHost)
	viper.SetDefault("graph.port", GraphPort)
	viper.SetDefault("graph.name", GraphName)
	viper.SetDefault("graph.password_env", GraphPasswordEnv)
	viper.SetDefault("graph.max_retries", GraphMaxRetries)
	viper.SetDefault("graph.retry_delay_ms", GraphRetryDelayMs)
	viper.SetDefault("graph.write_queue_size", GraphWriteQueueSize)
}
